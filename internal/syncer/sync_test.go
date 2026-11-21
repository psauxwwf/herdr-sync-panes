package syncer

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"herdr-sync-panes/pkg/herdr"
)

func TestDecodeInput(t *testing.T) {
	tests := []struct {
		name    string
		pending []byte
		input   []byte
		text    string
		rest    []byte
		stop    bool
		wantErr bool
	}{
		{"plain and control", nil, []byte("a\x03\x1b[A\r"), "a\x03\x1b[A\r", nil, false, false},
		{"partial unicode", nil, []byte{'a', 0xd0}, "a", []byte{0xd0}, false, false},
		{"unicode continuation", []byte{0xd0}, []byte{0xb6}, "ж", nil, false, false},
		{"stop after text", nil, []byte("ab\x1dignore"), "ab", nil, true, false},
		{"stop only", nil, []byte{0x1d}, "", nil, true, false},
		{"invalid utf8", nil, []byte{0xff}, "", nil, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, rest, stop, err := decodeInput(tt.pending, tt.input)
			if text != tt.text || !reflect.DeepEqual(rest, tt.rest) || stop != tt.stop || (err != nil) != tt.wantErr {
				t.Fatalf("got %q, %v, %v, %v", text, rest, stop, err)
			}
		})
	}
}

type fakeAPI struct {
	list     []herdr.Pane
	sent     map[string]string
	failSend string
}

func (f *fakeAPI) Pane(id string) (herdr.Pane, error) {
	for _, p := range f.list {
		if p.ID == id {
			return p, nil
		}
	}
	return herdr.Pane{}, errors.New("pane not found")
}
func (f *fakeAPI) Panes(string) ([]herdr.Pane, error) { return f.list, nil }
func (f *fakeAPI) Send(id, text string) error {
	if id == f.failSend {
		return errors.New("disconnected")
	}
	f.sent[id] += text
	return nil
}

func TestTargetIDs(t *testing.T) {
	self := herdr.Pane{ID: "control", TabID: "t1", WorkspaceID: "w1"}
	panes := []herdr.Pane{
		self, {ID: "b", TabID: "t1", WorkspaceID: "w1"},
		{ID: "a", TabID: "t1", WorkspaceID: "w1"},
		{ID: "other-tab", TabID: "t2", WorkspaceID: "w1"},
		{ID: "other-workspace", TabID: "t1", WorkspaceID: "w2"},
	}
	got, err := targetIDs(panes, self)
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := targetIDs(panes[1:], self); err == nil {
		t.Fatal("missing control pane must be rejected")
	}
}

type chunkReader struct{ chunks [][]byte }

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	chunk := r.chunks[0]
	r.chunks = r.chunks[1:]
	return copy(p, chunk), nil
}

func TestBroadcastUTF8AndControlKeys(t *testing.T) {
	self := herdr.Pane{ID: "control", TabID: "t", WorkspaceID: "w"}
	api := &fakeAPI{
		list: []herdr.Pane{self, {ID: "a", TabID: "t", WorkspaceID: "w"}, {ID: "b", TabID: "t", WorkspaceID: "w"}},
		sent: map[string]string{},
	}
	input := &chunkReader{chunks: [][]byte{{'a', 0xd0}, {0xb6, 0x03, 0x1b, '[', 'A', '\r'}, {stopKey}}}
	err := broadcast(context.Background(), api, input, self, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if api.sent[id] != "aж\x03\x1b[A\r" {
			t.Fatalf("%s received %q", id, api.sent[id])
		}
	}
}

func TestBroadcastStopsOnTargetChange(t *testing.T) {
	self := herdr.Pane{ID: "control", TabID: "t", WorkspaceID: "w"}
	api := &fakeAPI{list: []herdr.Pane{self, {ID: "a", TabID: "other", WorkspaceID: "w"}}, sent: map[string]string{}}
	err := broadcast(context.Background(), api, strings.NewReader("dangerous command\r"), self, []string{"a"})
	if err == nil || len(api.sent) != 0 {
		t.Fatalf("expected failure without delivery; err=%v sent=%v", err, api.sent)
	}
}

func TestBroadcastReportsPartialFailure(t *testing.T) {
	self := herdr.Pane{ID: "control", TabID: "t", WorkspaceID: "w"}
	api := &fakeAPI{
		list: []herdr.Pane{self, {ID: "a", TabID: "t", WorkspaceID: "w"}, {ID: "b", TabID: "t", WorkspaceID: "w"}},
		sent: map[string]string{}, failSend: "b",
	}
	err := broadcast(context.Background(), api, strings.NewReader("x"), self, []string{"a", "b"})
	if err == nil || !strings.Contains(err.Error(), "some panes may have received") || api.sent["a"] != "x" {
		t.Fatalf("expected partial delivery warning; err=%v sent=%v", err, api.sent)
	}
}
