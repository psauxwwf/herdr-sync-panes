package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Keep only one broadcaster for a tab, including across plugin invocations.
func lockTab(tabID string) (func(), error) {
	dir := os.Getenv("HERDR_PLUGIN_STATE_DIR")
	if dir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(cache, "herdr-sync-panes")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	sum := sha256.Sum256([]byte(os.Getenv("HERDR_SOCKET_PATH") + "\x00" + tabID))
	path := filepath.Join(dir, "sync-"+hex.EncodeToString(sum[:16])+".lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open sync lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, errors.New("sync is already active in this tab")
		}
		return nil, fmt.Errorf("lock sync: %w", err)
	}
	return func() { _ = file.Close() }, nil
}
