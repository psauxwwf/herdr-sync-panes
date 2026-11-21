package herdr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

type Pane struct {
	ID          string `json:"pane_id"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
}

type Client struct {
	Socket string
}

func (c Client) call(method string, params any, result any) error {
	conn, err := net.DialTimeout("unix", c.Socket, 3*time.Second)
	if err != nil {
		return fmt.Errorf("connect to Herdr: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	req := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params"`
	}{"sync-panes", method, params}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("%s request: %w", method, err)
	}
	var response struct {
		ID     string          `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return fmt.Errorf("%s response: %w", method, err)
	}
	if response.ID != req.ID {
		return fmt.Errorf("%s: unexpected response id", method)
	}
	if response.Error != nil {
		return fmt.Errorf("%s: %s: %s", method, response.Error.Code, response.Error.Message)
	}
	if len(response.Result) == 0 {
		return errors.New("Herdr response has no result")
	}
	if result != nil {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("%s result: %w", method, err)
		}
	}
	return nil
}

func (c Client) Pane(id string) (Pane, error) {
	var result struct {
		Pane Pane `json:"pane"`
	}
	err := c.call("pane.get", map[string]any{"pane_id": id}, &result)
	if err == nil && (result.Pane.ID == "" || result.Pane.TabID == "" || result.Pane.WorkspaceID == "") {
		err = errors.New("Herdr returned incomplete pane information")
	}
	return result.Pane, err
}

func (c Client) Panes(workspaceID string) ([]Pane, error) {
	var result struct {
		Panes []Pane `json:"panes"`
	}
	err := c.call("pane.list", map[string]any{"workspace_id": workspaceID}, &result)
	if err == nil && result.Panes == nil {
		err = errors.New("Herdr returned no pane list")
	}
	return result.Panes, err
}

func (c Client) Send(id, text string) error {
	return c.call("pane.send_text", map[string]any{"pane_id": id, "text": text}, nil)
}

func (c Client) Open(target string) error {
	return c.call("plugin.pane.open", map[string]any{
		"plugin_id": "herdr-sync-panes", "entrypoint": "control",
		"target_pane_id": target, "placement": "split", "direction": "down", "focus": true,
	}, nil)
}
