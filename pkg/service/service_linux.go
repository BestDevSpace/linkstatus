//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BestDevSpace/linkstatus/pkg/config"
)

const unitFile = "linkstatus-monitor.service"

func unitPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "systemd", "user", unitFile), nil
}

// Installed reports whether the systemd user unit file exists.
func Installed() (bool, error) {
	p, err := unitPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Running reports whether the user service is active.
func Running() (bool, error) {
	out, err := exec.Command("systemctl", "--user", "is-active", unitFile).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "inactive" || s == "failed" || s == "unknown" {
			return false, nil
		}
		return false, fmt.Errorf("systemctl is-active: %w (%s)", err, s)
	}
	return s == "active" || s == "activating", nil
}

// Install writes a systemd user unit, reloads systemd, enables and starts it.
func Install(exe string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl not found: %w", err)
	}
	exe, err := filepath.Abs(exe)
	if err != nil {
		return err
	}
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("executable: %w", err)
	}

	p, err := unitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	dataDir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	unit := fmt.Sprintf(`[Unit]
Description=linkstatus network monitor (user)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s monitor
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, quoteSystemdPath(exe))

	if err := os.WriteFile(p, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}

	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl --user daemon-reload: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", unitFile).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl --user enable --now: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func quoteSystemdPath(s string) string {
	if !strings.ContainsAny(s, " \t\n\"\\") {
		return s
	}
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

// Remove stops, disables, and deletes the user unit.
func Remove() error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl not found: %w", err)
	}
	p, err := unitPath()
	if err != nil {
		return err
	}
	_ = exec.Command("systemctl", "--user", "disable", "--now", unitFile).Run()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

// Describe returns human-readable install/run state for the TUI.
func Describe() (installed, running bool, hint string, err error) {
	installed, err = Installed()
	if err != nil {
		return false, false, "", err
	}
	running, err = Running()
	if err != nil {
		return installed, false, "", err
	}
	if !installed {
		return false, running, "No systemd user unit (use /service-install).", nil
	}
	if !running {
		return true, false, "Unit file present but not active; try: systemctl --user start " + unitFile, nil
	}
	return true, true, "systemd user service enabled; starts at login (user session).", nil
}
