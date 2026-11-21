package syncer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"herdr-sync-panes/pkg/herdr"

	"golang.org/x/term"
)

const stopKey = byte(0x1d) // Ctrl+]

type paneAPI interface {
	Pane(string) (herdr.Pane, error)
	Panes(string) ([]herdr.Pane, error)
	Send(string, string) error
}

func targetIDs(panes []herdr.Pane, self herdr.Pane) ([]string, error) {
	var targets []string
	found := false
	for _, p := range panes {
		if p.ID == self.ID && p.TabID == self.TabID && p.WorkspaceID == self.WorkspaceID {
			found = true
		} else if p.ID != "" && p.TabID == self.TabID && p.WorkspaceID == self.WorkspaceID {
			targets = append(targets, p.ID)
		}
	}
	if !found {
		return nil, errors.New("control pane is no longer in its original tab")
	}
	sort.Strings(targets)
	return targets, nil
}

func Run(ctx context.Context, api paneAPI, input *os.File, output io.Writer, selfID string) error {
	if selfID == "" {
		return errors.New("HERDR_PANE_ID is missing; refusing to broadcast")
	}
	if !term.IsTerminal(int(input.Fd())) {
		return errors.New("sync requires an interactive terminal")
	}
	self, err := api.Pane(selfID)
	if err != nil {
		return err
	}
	unlock, err := lockTab(self.TabID)
	if err != nil {
		return err
	}
	defer unlock()

	panes, err := api.Panes(self.WorkspaceID)
	if err != nil {
		return err
	}
	targets, err := targetIDs(panes, self)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return errors.New("no other panes in this tab")
	}
	fmt.Fprintf(output, "SYNC ON: input goes to %d pane(s) in this tab.\r\n", len(targets))
	fmt.Fprint(output, "Ctrl+] stops sync. Input is not shown here.\r\n")

	state, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return fmt.Errorf("enable raw input: %w", err)
	}
	defer term.Restore(int(input.Fd()), state)

	return broadcast(ctx, api, input, self, targets)
}

func broadcast(ctx context.Context, api paneAPI, input io.Reader, self herdr.Pane, targets []string) error {
	type readResult struct {
		data []byte
		err  error
	}
	requests := make(chan struct{})
	reads := make(chan readResult)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-requests:
			}
			buf := make([]byte, 1024)
			n, err := input.Read(buf)
			select {
			case reads <- readResult{buf[:n], err}:
			case <-done:
				return
			}
		}
	}()

	var pending []byte
	for {
		select {
		case <-ctx.Done():
			return nil
		case requests <- struct{}{}:
		}
		var read readResult
		select {
		case <-ctx.Done():
			return nil
		case read = <-reads:
		}
		text, remaining, stop, err := decodeInput(pending, read.data)
		if err != nil {
			return err
		}
		pending = remaining
		if text != "" {
			current, err := api.Panes(self.WorkspaceID)
			if err != nil {
				return fmt.Errorf("cannot verify target panes; sync stopped: %w", err)
			}
			currentTargets, err := targetIDs(current, self)
			if err != nil || strings.Join(currentTargets, "\x00") != strings.Join(targets, "\x00") {
				return errors.New("panes changed; sync stopped before sending input")
			}
			for _, target := range targets {
				if err := api.Send(target, text); err != nil {
					return fmt.Errorf("send to %s failed; some panes may have received input: %w", target, err)
				}
			}
		}
		if stop {
			return nil
		}
		if read.err != nil {
			if read.err == io.EOF && len(pending) == 0 {
				return nil
			}
			return fmt.Errorf("read input: %w", read.err)
		}
	}
}

// decodeInput retains an incomplete UTF-8 rune across reads. Herdr's JSON API
// accepts strings, so invalid bytes must not silently turn into replacement text.
func decodeInput(pending, input []byte) (text string, rest []byte, stop bool, err error) {
	data := append(append([]byte(nil), pending...), input...)
	for i := 0; i < len(data); {
		if data[i] == stopKey {
			return string(data[:i]), nil, true, nil
		}
		if !utf8.FullRune(data[i:]) {
			return string(data[:i]), append([]byte(nil), data[i:]...), false, nil
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return "", nil, false, errors.New("invalid UTF-8 input; sync stopped")
		}
		i += size
	}
	return string(data), nil, false, nil
}
