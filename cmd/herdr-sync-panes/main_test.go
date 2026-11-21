package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"herdr-sync-panes/pkg/herdr"
)

func TestActionPaneID(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "environment-pane")
	t.Setenv("HERDR_PLUGIN_CONTEXT_JSON", `{"focused_pane_id":"context-pane"}`)
	if got := actionPaneID(); got != "context-pane" {
		t.Fatalf("got %q", got)
	}

	t.Setenv("HERDR_PLUGIN_CONTEXT_JSON", `{}`)
	if got := actionPaneID(); got != "environment-pane" {
		t.Fatalf("fallback got %q", got)
	}
}

func TestPopupAnchor(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_CONTEXT_JSON", `{"focused_pane_id":"w1:p1","tab_id":"w1:t1","workspace_id":"w1"}`)
	t.Setenv("HERDR_SYNC_EXPECTED_PANE_ID", "w1:p1")
	got, err := popupAnchor()
	want := herdr.Pane{ID: "w1:p1", TabID: "w1:t1", WorkspaceID: "w1"}
	if err != nil || got != want {
		t.Fatalf("got %+v, %v", got, err)
	}
	t.Setenv("HERDR_SYNC_EXPECTED_PANE_ID", "w1:p2")
	if _, err := popupAnchor(); err == nil {
		t.Fatal("mismatched focus must fail closed")
	}
	t.Setenv("HERDR_SYNC_EXPECTED_PANE_ID", "")
	if _, err := popupAnchor(); err == nil {
		t.Fatal("missing expected pane must fail closed")
	}
	t.Setenv("HERDR_PLUGIN_CONTEXT_JSON", `{"focused_pane_id":"w1:p1","workspace_id":"w1"}`)
	t.Setenv("HERDR_SYNC_EXPECTED_PANE_ID", "w1:p1")
	if _, err := popupAnchor(); err == nil {
		t.Fatal("missing tab must fail closed")
	}
	t.Setenv("HERDR_PLUGIN_CONTEXT_JSON", "{")
	if _, err := popupAnchor(); err == nil {
		t.Fatal("invalid context must fail closed")
	}
}

func TestLogSyncError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", dir)
	logSyncError(errors.New("test failure"))
	data, err := os.ReadFile(filepath.Join(dir, "errors.log"))
	if err != nil || !strings.Contains(string(data), "test failure") {
		t.Fatalf("got %q, %v", data, err)
	}
}
