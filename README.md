# linkstatus

**linkstatus** is a small **internet connectivity monitor** for **macOS** and **Linux**. It periodically checks reachability with **ICMP echo** (ping-like) to public resolvers, falls back to **DNS** when ICMP suggests a problem, stores results in a **local SQLite** database, and can show a **terminal dashboard** plus **desktop notifications** when the link flips between up and down.

There is no Windows build in the release matrix; the codebase targets darwin/linux.

## What it does

- **Probes** configurable hosts on an interval, with bounded timeouts and CPU-friendly defaults.
- **Scores** each sample with a 1–5 latency rating and labels (Excellent → Critical).
- **Persists** history under `~/.linkstatus/` (`linkstatus.db`).
- **Dashboard (TUI)** shows uptime, averages, and a time strip of recent ratings for the last 1h / 6h / 24h.
- **Background monitor** (`linkstatus monitor` or an installed user service) runs the probe loop; the GUI is **read-only** and only displays what the monitor has written.
- **Notifications** on macOS (`osascript`) and Linux (`notify-send` / libnotify) when connectivity changes.

## Install

### Homebrew (macOS / Linux, custom tap)

```bash
brew tap BestDevSpace/tap
brew install --cask linkstatus
```

(Releases publish a **Cask** to the tap; use `--cask` as appropriate for your setup.)

### Prebuilt binaries

See [GitHub Releases](https://github.com/BestDevSpace/linkstatus/releases) for `linux_amd64`, `linux_arm64`, `darwin_amd64`, and `darwin_arm64` archives and checksums.

### From source

Requires **Go 1.23+**.

```bash
git clone https://github.com/BestDevSpace/linkstatus.git
cd linkstatus
CGO_ENABLED=0 go build -ldflags="-s -w" -o linkstatus .
```

The [Makefile](Makefile) can build all four platform binaries inside Docker (`make bin`) if you prefer not to install Go locally.

## Usage

### Dashboard (default)

```bash
linkstatus
```

Opens the full-screen terminal UI. Only **one** dashboard instance runs at a time (file lock under `~/.linkstatus/gui.lock`).

Hidden alias: `linkstatus gui` (same behavior).

### Foreground monitor

```bash
linkstatus monitor
```

Runs the probe loop in the foreground: logs to stdout if attached to a TTY, writes to SQLite, and sends **up/down** notifications when status changes. Only **one** monitor (or background service) at a time (`~/.linkstatus/monitor.lock`).

Use **Ctrl+C** to stop.

### Background service (from the TUI)

Inside the dashboard, type slash commands (Tab completes):

| Command | Alias | Meaning |
|--------|--------|--------|
| `/service-install` | `/svc-install` | Install **macOS LaunchAgent** or **systemd user unit** so `linkstatus monitor` runs at login |
| `/service-remove` | `/svc-remove` | Remove the service |
| `/service-status` | `/svc-status` | Show installed/running hints |
| `/help` | `/h` | Short help |
| `/status` | | Latest sample from DB |
| `/stats [duration]` | | Stats over window (default 24h); optional Go duration e.g. `6h` |
| `/refresh` | | Reload panels + samples from DB |
| `/clear` | | Clear command output pane |
| `/quit` | `/q` | Hint: use `q` or Ctrl+C |

**macOS:** plist at `~/Library/LaunchAgents/io.github.bestdevspace.linkstatus.monitor.plist`, logs under `~/.linkstatus/monitor.*.log`.

**Linux:** unit at `~/.config/systemd/user/linkstatus-monitor.service` (`systemctl --user`).

### Version

```bash
linkstatus --version
```

Release builds embed the tag via `-ldflags` (`cmd.Version`).

## Configuration

Path: **`~/.linkstatus/config.yaml`** (created when you save from tooling that calls `config.Save`; otherwise defaults apply if the file is missing).

| Key | Meaning | Default (indicative) |
|-----|---------|----------------------|
| `probe_interval` | Between probe cycles | `5s` (clamped 3s–30m) |
| `probe_timeout` | Per-target timeout | `1200ms` (clamped) |
| `ping_targets` | ICMP targets | `8.8.8.8`, `1.1.1.1`, `9.9.9.9` |
| `dns_targets` | Resolver:port for DNS check | `:53` on same IPs |
| `dns_domain` | Name to resolve | `google.com` |
| `rating_thresholds` | Stored in config | Matches latency bands; **current scoring uses fixed thresholds in code** (`pkg/rating`) |

Interval is auto-bumped if it would overlap probe work (must exceed timeout + slack).

## How probing works (high level)

1. **ICMP** runs against all `ping_targets` (parallel per target).
2. If status is **down**, a **DNS** probe runs against `dns_targets` for `dns_domain` to distinguish “no ICMP” vs broader failure.
3. One row per cycle is inserted into SQLite with status, rating, latency, optional error text.
4. The TUI reads aggregates and recent rows only; it does **not** run probes.

## Linux notes

- **Desktop notifications** need `notify-send` (e.g. `libnotify-bin` on Debian/Ubuntu).
- **ICMP** may require privileges or capabilities on some systems (unprivileged ping policies vary by distro/kernel).

## Development

```bash
go test ./...
CGO_ENABLED=0 go build .
```

CI runs tests and `goreleaser check` on push/PR. Releases are triggered by **`v*`** tags (see `.github/workflows/release.yml`).

## Repository

- **Module:** `github.com/BestDevSpace/linkstatus`
- **Homebrew tap (Cask push):** `BestDevSpace/homebrew-tap` (separate repo; release token needs **Contents** write on that repo).

For AI-assisted work in this repo, see **[CLAUDE.md](CLAUDE.md)**.
