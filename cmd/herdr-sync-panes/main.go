package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"herdr-sync-panes/internal/syncer"
	"herdr-sync-panes/pkg/herdr"

	"golang.org/x/term"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "open" && os.Args[1] != "sync") {
		fmt.Fprintln(os.Stderr, "usage: herdr-sync-panes open|sync")
		os.Exit(2)
	}
	socket := os.Getenv("HERDR_SOCKET_PATH")
	if socket == "" {
		fmt.Fprintln(os.Stderr, "HERDR_SOCKET_PATH is required")
		os.Exit(1)
	}
	client := herdr.Client{Socket: socket}
	var err error
	if os.Args[1] == "open" {
		target := actionPaneID()
		if target == "" {
			err = errors.New("no Herdr pane in plugin action context")
		} else {
			err = client.Open(target)
		}
	} else {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
		defer cancel()
		var anchor herdr.Pane
		anchor, err = popupAnchor()
		if err == nil {
			err = syncer.Run(ctx, client, os.Stdin, os.Stdout, anchor)
		}
	}
	if err != nil {
		if os.Args[1] == "sync" && term.IsTerminal(int(os.Stdin.Fd())) {
			logSyncError(err)
			fmt.Fprint(os.Stderr, "\x1b[2J\x1b[HERR\r\nENT")
			var line string
			fmt.Fscanln(os.Stdin, &line)
		} else {
			fmt.Fprintf(os.Stderr, "sync-panes: %v\n", err)
		}
		os.Exit(1)
	}
}

func logSyncError(err error) {
	dir := os.Getenv("HERDR_PLUGIN_STATE_DIR")
	if dir == "" {
		return
	}
	file, openErr := os.OpenFile(filepath.Join(dir, "errors.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if openErr != nil {
		return
	}
	defer file.Close()
	fmt.Fprintf(file, "%s %v\n", time.Now().Format(time.RFC3339), err)
}

func popupAnchor() (herdr.Pane, error) {
	var context struct {
		FocusedPaneID string `json:"focused_pane_id"`
		TabID         string `json:"tab_id"`
		WorkspaceID   string `json:"workspace_id"`
	}
	if err := json.Unmarshal([]byte(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON")), &context); err != nil {
		return herdr.Pane{}, errors.New("invalid popup context; refusing to broadcast")
	}
	expected := os.Getenv("HERDR_SYNC_EXPECTED_PANE_ID")
	if expected == "" || context.FocusedPaneID != expected || context.TabID == "" || context.WorkspaceID == "" {
		return herdr.Pane{}, errors.New("popup context changed or is incomplete; refusing to broadcast")
	}
	return herdr.Pane{ID: expected, TabID: context.TabID, WorkspaceID: context.WorkspaceID}, nil
}

func actionPaneID() string {
	var context struct {
		FocusedPaneID string `json:"focused_pane_id"`
	}
	_ = json.Unmarshal([]byte(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON")), &context)
	if context.FocusedPaneID != "" {
		return context.FocusedPaneID
	}
	return os.Getenv("HERDR_PANE_ID")
}
