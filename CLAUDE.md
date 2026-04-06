# CLAUDE.md — LLM context for linkstatus

Use this file when editing or reasoning about **github.com/BestDevSpace/linkstatus**. It summarizes architecture, boundaries, and conventions so changes stay consistent.

## Purpose

CLI tool: **monitor internet connectivity** on **macOS and Linux** using **ICMP** (primary) and **DNS** (when ICMP looks down), **persist** samples to **SQLite** under `~/.linkstatus/`, optional **desktop notifications** on up/down transitions, and a **termui** dashboard that is **read-only** (no probing in the GUI).

## Entry points

- `main.go` → `cmd.Execute()`
- **`cmd/root.go`**: default command runs **`runGUI`** → `tui.Run()`; sets `Version` from ldflags (`dev` locally).
- **`cmd/gui.go`**: hidden `gui` subcommand, same as root.
- **`cmd/monitor.go`**: `linkstatus monitor` — probe loop, notifications, SQLite writes; uses monitor file lock.

## Critical invariants (do not break casually)

1. **Single writer for probes:** Only `linkstatus monitor` (or the background service running the same code path) should run the probe loop. **`pkg/instance.TryMonitorLock()`** enforces one monitor via `~/.linkstatus/monitor.lock`.
2. **GUI does not probe:** `pkg/tui.Run()` opens the store and refreshes UI from DB; it must stay non-probing so users can open the dashboard without doubling probe traffic.
3. **Single GUI instance:** `TryGUILock()` → `gui.lock`.
4. **Service label:** `pkg/service.Label` = `io.github.bestdevspace.linkstatus.monitor` — used for LaunchAgent (darwin) and conceptually aligned with systemd unit naming on Linux (`linkstatus-monitor.service` file under user systemd).

## Layout

| Path | Role |
|------|------|
| `cmd/` | Cobra commands: root (GUI), `monitor`, hidden `gui` |
| `pkg/config/` | Viper load/save `~/.linkstatus/config.yaml`, duration clamping (`normalizeProbeSettings`) |
| `pkg/probe/` | `ICMPProbe`, `DNSProbe`, `Result` |
| `pkg/worker/` | `RunProbe` — one cycle ICMP then optional DNS, rating, `store.InsertEntry` |
| `pkg/store/` | SQLite (`modernc.org/sqlite`, CGO-free), `probe_logs` schema |
| `pkg/rating/` | `Rate(latencyMs)` → 1–5; labels; **thresholds are fixed in code**, not read from `config.RatingThresholds` today |
| `pkg/tui/` | termui dashboard, slash commands, dot charts, service install/remove/status |
| `pkg/notify/` | macOS `osascript`, Linux `notify-send` |
| `pkg/service/` | `service.go` (constants) + `service_darwin.go` / `service_linux.go` / `service_unsupported.go` |
| `pkg/instance/` | flock locks |

## Configuration

- **Dir:** `~/.linkstatus/` from `config.ConfigDir()`
- **File:** `config.yaml` via `config.ConfigPath()`
- **Load:** `config.Load()` — missing file → defaults; bad unmarshal → defaults (lenient)
- **Save:** `config.Save`, `config.Reset`
- **`rating_thresholds`:** present in struct and YAML round-trip; **scoring still uses `pkg/rating.Rate` hardcoded breakpoints** unless someone wires config into rating (would be an intentional change)

## Probe pipeline

1. `monitor` loads config, opens store, builds ICMP + DNS probes from config targets/timeouts.
2. Each tick: `worker.RunProbe` → ICMP; if `down`, try DNS; compute `rating.Rate(result.LatencyMs)`; insert row; `notify.MaybeConnectivity(prev, cur)` on change.

## TUI commands

Defined in `pkg/tui/app.go` (`slashCommands`, `execInput`). Tab completion in `pkg/tui/complete.go`. Service operations call `pkg/service.Install/Remove/Describe` with `os.Executable()` path.

## Build / release

- **CGO:** `CGO_ENABLED=0` for portable binaries (Makefile, goreleaser, CI build).
- **Go version:** `go.mod` (1.23).
- **Releases:** GoReleaser (`.goreleaser.yaml`), GitHub Actions on `v*` tags; **`release.replace_existing_artifacts`** helps re-runs against the same tag.
- **Homebrew:** `homebrew_casks` → `BestDevSpace/homebrew-tap`; PAT secret **`HOMEBREW_TAP_GITHUB_TOKEN`** must authorize **homebrew-tap** (fine-grained PAT repo list).

## Testing

```bash
go test ./...
```

`pkg/tui/complete_test.go` covers completion behavior.

## Style for patches

- Match existing naming and small-package layout.
- Avoid expanding scope: GUI stays read-only; probe logic belongs in `worker` + `probe`.
- Respect locks and single-monitor assumption.
- Do not add heavy dependencies without strong reason; project is intentionally minimal.
