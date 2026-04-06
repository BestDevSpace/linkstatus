package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// MaybeConnectivity shows a desktop notification when connectivity changes (up ↔ down).
// prev is the last persisted status ("up" or "down"); empty skips (no notification on first sample).
func MaybeConnectivity(prev, cur string) {
	if prev == "" || prev == cur {
		return
	}
	switch cur {
	case "down":
		_ = show("Linkstatus", "Internet connection lost")
	case "up":
		_ = show("Linkstatus", "Internet connection restored")
	}
}

// Info shows a desktop notification (macOS / Linux with notify-send), ignoring errors.
func Info(title, body string) {
	_ = show(title, body)
}

func show(title, body string) error {
	switch runtime.GOOS {
	case "darwin":
		return showDarwin(title, body)
	case "linux":
		return showLinux(title, body)
	default:
		return nil
	}
}

func showDarwin(title, body string) error {
	esc := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return s
	}
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, esc(body), esc(title))
	return exec.Command("osascript", "-e", script).Run()
}

func showLinux(title, body string) error {
	// Requires `notify-send` (libnotify), common on GNOME, KDE, XFCE, etc.
	return exec.Command("notify-send", "-a", "Linkstatus", "--app-name", "Linkstatus", title, body).Run()
}
