package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
		err = syncer.Run(ctx, client, os.Stdin, os.Stdout, os.Getenv("HERDR_PANE_ID"))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r\nsync-panes: %v\r\n", err)
		if os.Args[1] == "sync" && term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprint(os.Stderr, "Press Enter to close.\r\n")
			var line string
			fmt.Fscanln(os.Stdin, &line)
		}
		os.Exit(1)
	}
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
