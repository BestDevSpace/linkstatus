//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BestDevSpace/linkstatus/pkg/config"
)

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// Installed reports whether the LaunchAgent plist is present.
func Installed() (bool, error) {
	p, err := plistPath()
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

// Running uses launchctl list to see if the job is loaded (exact label match).
func Running() (bool, error) {
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return false, fmt.Errorf("launchctl list: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == Label {
			return true, nil
		}
	}
	return false, nil
}

// Install writes the LaunchAgent, loads it, and enables it at login.
func Install(exe string) error {
	exe, err := filepath.Abs(exe)
	if err != nil {
		return err
	}
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	dataDir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	outLog := filepath.Join(dataDir, "monitor.stdout.log")
	errLog := filepath.Join(dataDir, "monitor.stderr.log")

	p, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}

	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>monitor</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, xmlEscape(Label), xmlEscape(exe), xmlEscape(outLog), xmlEscape(errLog))

	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	uid := strconv.Itoa(os.Getuid())
	domain := "gui/" + uid
	// Best-effort unload before (re)load.
	_ = exec.Command("launchctl", "bootout", domain, p).Run()
	_ = exec.Command("launchctl", "bootout", domain, Label).Run()

	if out, err := exec.Command("launchctl", "bootstrap", domain, p).CombinedOutput(); err != nil {
		// Older macOS: fall back to load -w
		if out2, err2 := exec.Command("launchctl", "load", "-w", p).CombinedOutput(); err2 != nil {
			return fmt.Errorf("launchctl bootstrap: %w (%s); load: %v (%s)", err, strings.TrimSpace(string(out)), err2, strings.TrimSpace(string(out2)))
		}
	}
	_ = exec.Command("launchctl", "enable", domain+"/"+Label).Run()
	return nil
}

// Remove unloads the LaunchAgent and deletes the plist.
func Remove() error {
	p, err := plistPath()
	if err != nil {
		return err
	}
	uid := strconv.Itoa(os.Getuid())
	domain := "gui/" + uid
	_ = exec.Command("launchctl", "bootout", domain, Label).Run()
	_ = exec.Command("launchctl", "bootout", domain, p).Run()
	_ = exec.Command("launchctl", "unload", p).Run()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
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
		return false, running, "No LaunchAgent plist (use /service-install).", nil
	}
	if !running {
		return true, false, "Plist present but job not loaded; try log out/in or: launchctl bootstrap gui/$UID ~/Library/LaunchAgents/"+Label+".plist", nil
	}
	return true, true, "LaunchAgent loaded; monitor runs at login.", nil
}
