package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"

	"github.com/BestDevSpace/linkstatus/pkg/config"
	"github.com/BestDevSpace/linkstatus/pkg/notify"
	"github.com/BestDevSpace/linkstatus/pkg/rating"
	"github.com/BestDevSpace/linkstatus/pkg/service"
	"github.com/BestDevSpace/linkstatus/pkg/store"
)

const (
	maxSuggest        = 8
	maxCmdLogLines    = 16
	inputPrompt       = " › "
	refreshPeriod     = 2 * time.Second
	recentSampleLines = 14
)

// slashCommands: tab-complete and suggestions (include aliases).
var slashCommands = []string{
	"/clear",
	"/help",
	"/h",
	"/quit",
	"/q",
	"/refresh",
	"/service-install",
	"/service-remove",
	"/service-status",
	"/stats",
	"/status",
	"/svc-install",
	"/svc-remove",
	"/svc-status",
}

// App is a read-only TUI: stats and recent samples come from the database (written by `linkstatus monitor` or the background service).
type App struct {
	store *store.Store

	p1h, p6h, p24h *widgets.Paragraph
	logBox         *widgets.Paragraph
	suggest        *widgets.Paragraph
	input          *widgets.Paragraph
	helpBar        *widgets.Paragraph

	inputBuf string

	// cmdLogLines: output from slash commands (not overwritten by DB refresh).
	cmdLogLines []string
	// sampleLines: last probe rows from DB (refreshed periodically).
	sampleLines []string
}

// Run starts the full-screen TUI (blocking). Does not run probes.
func Run() error {
	dataDir, err := config.ConfigDir()
	if err != nil {
		return fmt.Errorf("config dir: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("data dir: %w", err)
	}

	st, err := store.New(dataDir)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer st.Close()

	if err := termui.Init(); err != nil {
		return fmt.Errorf("termui: %w", err)
	}
	defer termui.Close()

	app := &App{store: st}
	app.buildWidgets()
	app.layout(termui.TerminalDimensions())
	app.refreshPanels()
	app.syncLogFromStore()
	app.updateHelpBar()

	go app.refreshLoop()

	for e := range termui.PollEvents() {
		if app.handleEvent(e) {
			return nil
		}
	}
	return nil
}

func (a *App) buildWidgets() {
	title := func(s string) string { return fmt.Sprintf(" %s ", s) }

	a.p1h = widgets.NewParagraph()
	a.p1h.Title = title("Connection · last 1 hour")
	a.p1h.Border = true

	a.p6h = widgets.NewParagraph()
	a.p6h.Title = title("Connection · last 6 hours")
	a.p6h.Border = true

	a.p24h = widgets.NewParagraph()
	a.p24h.Title = title("Connection · last 24 hours")
	a.p24h.Border = true

	a.logBox = widgets.NewParagraph()
	a.logBox.Title = " Command output · monitor samples "
	a.logBox.Border = true

	a.suggest = widgets.NewParagraph()
	a.suggest.Title = " Commands "
	a.suggest.Border = true

	a.input = widgets.NewParagraph()
	a.input.Border = true

	a.helpBar = widgets.NewParagraph()
	a.helpBar.Border = false
}

func (a *App) layout(w, h int) {
	if w < 40 || h < 16 {
		return
	}

	topH := h*55/100
	if topH < 10 {
		topH = 10
	}
	third := w / 3
	a.p1h.SetRect(0, 0, third, topH)
	a.p6h.SetRect(third, 0, 2*third, topH)
	a.p24h.SetRect(2*third, 0, w, topH)

	logH := h - topH - 10
	if logH < 4 {
		logH = 4
	}
	a.logBox.SetRect(0, topH, w, topH+logH)

	y := topH + logH
	a.suggest.SetRect(0, y, w, y+3)
	y += 3
	a.input.SetRect(0, y, w, y+3)
	y += 3
	a.helpBar.SetRect(0, y, w, h)
}

func (a *App) render() {
	termui.Render(a.p1h, a.p6h, a.p24h, a.logBox, a.suggest, a.input, a.helpBar)
}

func (a *App) refreshPanels() {
	now := time.Now()
	a.fillPanel(a.p1h, now.Add(-time.Hour), now)
	a.fillPanel(a.p6h, now.Add(-6*time.Hour), now)
	a.fillPanel(a.p24h, now.Add(-24*time.Hour), now)
}

func (a *App) fillPanel(p *widgets.Paragraph, since, until time.Time) {
	st, err := a.store.GetStats(since)
	if err != nil {
		p.Text = fmt.Sprintf("Error: %v", err)
		return
	}
	ratings, err := a.store.GetBucketAverageRatings(since, until, dotsPerPanel)
	if err != nil {
		p.Text = fmt.Sprintf("Error: %v", err)
		return
	}
	if st.TotalProbes == 0 {
		p.Text = "No data in this window.\nRun linkstatus monitor or use /service-install."
		return
	}
	window := until.Sub(since)
	dots := formatDotRows(ratings)
	p.Text = DotLegend(window) + dots + "\n" + fmt.Sprintf(
		"Uptime:    %.1f%%\nUp/Down:   %d / %d\nAvg lat:   %.0f ms\nAvg score: %.1f/5 (%s)",
		st.UptimePercent,
		st.UpProbes,
		st.DownProbes,
		st.AvgLatency,
		st.AvgRating,
		rating.RatingLabel(int(st.AvgRating+0.5)),
	)
}

func formatSampleLine(e store.ProbeLogEntry) string {
	statusIcon := "UP"
	if e.Status == "down" {
		statusIcon = "DOWN"
	}
	label := rating.RatingLabel(e.Rating)
	return fmt.Sprintf("[%s] %s | %d/5 (%s) | %.1fms",
		e.Timestamp.Format("15:04:05"), statusIcon, e.Rating, label, e.LatencyMs)
}

// syncLogFromStore reloads sample lines from SQLite; command output is preserved.
func (a *App) syncLogFromStore() {
	entries, err := a.store.GetRecentEntries(recentSampleLines)
	if err != nil {
		a.sampleLines = []string{fmt.Sprintf("Error loading samples: %v", err)}
		a.rebuildLogBox()
		return
	}
	if len(entries) == 0 {
		a.sampleLines = nil
		a.rebuildLogBox()
		return
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	lines := make([]string, len(entries))
	for i := range entries {
		lines[i] = formatSampleLine(entries[i])
	}
	a.sampleLines = lines
	a.rebuildLogBox()
}

func (a *App) rebuildLogBox() {
	var parts []string
	if len(a.cmdLogLines) > 0 {
		parts = append(parts, "── command output ──")
		parts = append(parts, a.cmdLogLines...)
	}
	if len(a.sampleLines) > 0 {
		if len(parts) > 0 {
			parts = append(parts, "")
		}
		parts = append(parts, "── monitor samples ──")
		parts = append(parts, a.sampleLines...)
	}
	if len(parts) == 0 {
		a.logBox.Text = "Type a /command — results stay here (refreshed separately from probe samples).\nNo samples yet — run linkstatus monitor or /service-install."
		return
	}
	a.logBox.Text = strings.Join(parts, "\n")
}

func (a *App) pushLog(line string) {
	a.cmdLogLines = append(a.cmdLogLines, line)
	if len(a.cmdLogLines) > maxCmdLogLines {
		a.cmdLogLines = a.cmdLogLines[len(a.cmdLogLines)-maxCmdLogLines:]
	}
	a.rebuildLogBox()
}

func (a *App) updateHelpBar() {
	a.helpBar.Text = formatServiceBar() + " · [/] Tab completes · q quit · Ctrl+C quit"
}

func (a *App) refreshLoop() {
	t := time.NewTicker(refreshPeriod)
	defer t.Stop()
	for range t.C {
		a.refreshPanels()
		a.syncLogFromStore()
		a.updateHelpBar()
		a.updateInputDisplay()
		a.render()
	}
}

func (a *App) updateInputDisplay() {
	prefix := inputPrompt
	if a.inputBuf == "" {
		a.input.Text = prefix + "_"
		return
	}
	a.input.Text = prefix + a.inputBuf + "_"
}

func (a *App) updateSuggestions() {
	if !strings.HasPrefix(a.inputBuf, "/") {
		a.suggest.Text = ""
		return
	}
	q := strings.ToLower(strings.TrimSpace(a.inputBuf))
	var hits []string
	for _, c := range slashCommands {
		if strings.HasPrefix(strings.ToLower(c), q) {
			hits = append(hits, c)
		}
	}
	sort.Strings(hits)
	if len(hits) > maxSuggest {
		hits = hits[:maxSuggest]
	}
	if len(hits) == 0 {
		a.suggest.Text = "(no matching commands)"
		return
	}
	a.suggest.Text = strings.Join(hits, "  ·  ")
}

func (a *App) handleEvent(e termui.Event) bool {
	switch e.Type {
	case termui.ResizeEvent:
		payload := e.Payload.(termui.Resize)
		a.layout(payload.Width, payload.Height)
		a.refreshPanels()
		a.syncLogFromStore()
		a.updateHelpBar()
		a.updateInputDisplay()
		a.updateSuggestions()
		a.render()
		return false

	case termui.KeyboardEvent:
		id := e.ID
		switch id {
		case "<C-c>":
			return true
		case "q", "Q":
			if a.inputBuf == "" {
				return true
			}
			fallthrough
		case "<Escape>":
			a.inputBuf = ""
			a.updateInputDisplay()
			a.updateSuggestions()
			a.render()
			return false
		case "<Enter>":
			a.execInput()
			return false
		case "<Tab>", "\t":
			if a.inputBuf != "" {
				next := tabComplete(a.inputBuf)
				if next != a.inputBuf {
					a.inputBuf = next
					a.updateInputDisplay()
					a.updateSuggestions()
					a.render()
				}
			}
			return false
		case "<Backspace>":
			if len(a.inputBuf) > 0 {
				a.inputBuf = a.inputBuf[:len(a.inputBuf)-1]
			}
			a.updateInputDisplay()
			a.updateSuggestions()
			a.render()
			return false
		case "<Space>":
			a.inputBuf += " "
			a.updateInputDisplay()
			a.updateSuggestions()
			a.render()
			return false
		default:
			if strings.HasPrefix(id, "<") {
				return false
			}
			a.inputBuf += id
			a.updateInputDisplay()
			a.updateSuggestions()
			a.render()
			return false
		}
	}
	return false
}

func (a *App) execInput() {
	line := strings.TrimSpace(a.inputBuf)
	a.inputBuf = ""
	a.updateInputDisplay()
	a.suggest.Text = ""
	a.render()

	if line == "" {
		return
	}
	if !strings.HasPrefix(line, "/") {
		a.pushLog("Commands must start with / (see /help)")
		a.render()
		return
	}

	parts := strings.Fields(line)
	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "/quit", "/q":
		a.pushLog("Use q or Ctrl+C to quit.")
	case "/help", "/h":
		a.pushLog("/help /status /refresh /stats /service-install /service-remove /service-status /clear — Tab completes commands; bottom bar shows service state.")
	case "/clear":
		a.cmdLogLines = nil
		a.rebuildLogBox()
		a.pushLog("Cleared command output above.")
	case "/refresh":
		a.refreshPanels()
		a.syncLogFromStore()
		a.pushLog("Panels and sample log refreshed from database.")
	case "/status":
		latest, err := a.store.GetLatestEntry()
		if err != nil {
			a.pushLog("No data yet.")
			break
		}
		icon := "DOWN"
		if latest.Status == "up" {
			icon = "UP"
		}
		a.pushLog(fmt.Sprintf("Latest: %s | %d/5 | %.0fms | %s",
			icon, latest.Rating, latest.LatencyMs, latest.Timestamp.Format("2006-01-02 15:04:05")))
	case "/stats":
		since := 24 * time.Hour
		if len(parts) > 1 {
			if d, err := time.ParseDuration(parts[1]); err == nil {
				since = d
			}
		}
		st, err := a.store.GetStats(time.Now().Add(-since))
		if err != nil || st.TotalProbes == 0 {
			a.pushLog("No stats for that range.")
			break
		}
		a.pushLog(fmt.Sprintf("Stats %.0fh: uptime %.1f%% avg %.0fms rating %.1f/5",
			since.Hours(), st.UptimePercent, st.AvgLatency, st.AvgRating))

	case "/service-install", "/svc-install":
		exe, err := os.Executable()
		if err != nil {
			a.pushLog(fmt.Sprintf("Could not resolve executable: %v", err))
			break
		}
		if err := service.Install(exe); err != nil {
			a.pushLog(fmt.Sprintf("ERROR — service install failed: %v", err))
			notify.Info("Linkstatus", "Service install failed.")
			break
		}
		a.pushLog("OK — background monitor service installed (starts at login). macOS: LaunchAgent · Linux: systemd --user.")
		notify.Info("Linkstatus", "Monitor service installed.")

	case "/service-remove", "/svc-remove":
		if err := service.Remove(); err != nil {
			a.pushLog(fmt.Sprintf("ERROR — service remove failed: %v", err))
			notify.Info("Linkstatus", "Service remove failed.")
			break
		}
		a.pushLog("OK — background monitor service removed (launchd/systemd unloaded).")
		notify.Info("Linkstatus", "Monitor service removed.")

	case "/service-status", "/svc-status":
		installed, running, hint, err := service.Describe()
		if err != nil {
			a.pushLog(fmt.Sprintf("ERROR — status: %v", err))
			break
		}
		a.pushLog(fmt.Sprintf("/service-status → installed=%v running=%v", installed, running))
		a.pushLog(hint)

	default:
		a.pushLog("Unknown command. Try /help")
	}
	a.updateHelpBar()
	a.render()
}
