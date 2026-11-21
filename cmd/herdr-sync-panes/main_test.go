package main

import "testing"

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
