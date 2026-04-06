package instance

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
	"github.com/BestDevSpace/linkstatus/pkg/config"
)

// TryMonitorLock acquires an exclusive lock for the probe loop (`linkstatus monitor` or the background service).
func TryMonitorLock() (*flock.Flock, error) {
	return tryLock("monitor.lock", "another linkstatus monitor (or background service) is already running")
}

// TryGUILock acquires an exclusive lock for the terminal GUI (only one TUI at a time).
func TryGUILock() (*flock.Flock, error) {
	return tryLock("gui.lock", "another linkstatus dashboard is already running")
}

func tryLock(name, busyMsg string) (*flock.Flock, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name)
	fl := flock.New(path)
	ok, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	if !ok {
		return nil, fmt.Errorf("%s (%s)", busyMsg, path)
	}
	return fl, nil
}
