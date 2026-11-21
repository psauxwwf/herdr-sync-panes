package herdr

import (
	"encoding/json"
	"net"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSocketAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "herdr.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 4; i++ {
			conn, err := listener.Accept()
			if err != nil {
				done <- err
				return
			}
			var req struct {
				ID     string         `json:"id"`
				Method string         `json:"method"`
				Params map[string]any `json:"params"`
			}
			err = json.NewDecoder(conn).Decode(&req)
			if err != nil {
				conn.Close()
				done <- err
				return
			}
			result := map[string]any{"type": "ok"}
			switch req.Method {
			case "pane.get":
				result["pane"] = Pane{ID: "self", TabID: "tab", WorkspaceID: "ws"}
			case "pane.list":
				result["panes"] = []Pane{{ID: "a", TabID: "tab", WorkspaceID: "ws"}}
			case "pane.send_text":
				if req.Params["pane_id"] != "a" || req.Params["text"] != "\x1b[A\x03" {
					t.Errorf("unexpected send params: %v", req.Params)
				}
			case "plugin.pane.open":
				env, ok := req.Params["env"].(map[string]any)
				if req.Params["placement"] != "popup" || req.Params["width"] != float64(6) ||
					req.Params["height"] != float64(4) || !ok || env["HERDR_SYNC_EXPECTED_PANE_ID"] != "self" {
					t.Errorf("unexpected open params: %v", req.Params)
				}
			default:
				t.Errorf("unexpected method %s", req.Method)
			}
			err = json.NewEncoder(conn).Encode(map[string]any{"id": req.ID, "result": result})
			conn.Close()
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	client := Client{Socket: path}
	p, err := client.Pane("self")
	if err != nil || p.ID != "self" {
		t.Fatalf("pane: %v %v", p, err)
	}
	panes, err := client.Panes("ws")
	if err != nil || !reflect.DeepEqual(panes, []Pane{{ID: "a", TabID: "tab", WorkspaceID: "ws"}}) {
		t.Fatalf("panes: %v %v", panes, err)
	}
	if err := client.Send("a", "\x1b[A\x03"); err != nil {
		t.Fatal(err)
	}
	if err := client.Open("self"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
