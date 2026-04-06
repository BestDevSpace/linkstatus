package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/BestDevSpace/linkstatus/pkg/config"
	"github.com/BestDevSpace/linkstatus/pkg/instance"
	"github.com/BestDevSpace/linkstatus/pkg/notify"
	"github.com/BestDevSpace/linkstatus/pkg/probe"
	storePkg "github.com/BestDevSpace/linkstatus/pkg/store"
	workerPkg "github.com/BestDevSpace/linkstatus/pkg/worker"

	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Run the probe loop (for LaunchAgent / systemd; desktop notifications on up/down)",
	Long: "Acquires the monitor lock, probes on the configured interval, writes to ~/.linkstatus/, and\n" +
		"sends desktop notifications when connectivity flips between up and down. Intended to be run\n" +
		"by a background service (see /service-install in the GUI).",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMonitorLoop()
	},
}

func runMonitorLoop() error {
	fl, err := instance.TryMonitorLock()
	if err != nil {
		return err
	}
	defer fl.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dataDir, err := config.ConfigDir()
	if err != nil {
		return fmt.Errorf("getting config dir: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	store, err := storePkg.New(dataDir)
	if err != nil {
		return fmt.Errorf("initializing store: %w", err)
	}
	defer store.Close()

	// ProbeInterval / ProbeTimeout already normalized in config.Load (CPU-friendly bounds).
	tmo := cfg.ProbeTimeout
	icmpProbe := probe.NewICMPProbe(cfg.PingTargets, 1, tmo)
	dnsProbe := probe.NewDNSProbe(cfg.DNSTargets, cfg.DNSDomain, tmo)

	logToStdout := isatty.IsTerminal(uintptr(os.Stdout.Fd()))
	if logToStdout {
		fmt.Printf("linkstatus monitor (every %s, probe timeout %s); Ctrl+C to stop\n", cfg.ProbeInterval, tmo)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	ticker := time.NewTicker(cfg.ProbeInterval)
	defer ticker.Stop()

	var logFn func(string)
	if logToStdout {
		logFn = func(line string) { fmt.Println(line) }
	}
	lastStatus := ""
	runCycle := func() {
		st, err := workerPkg.RunProbe(store, icmpProbe, dnsProbe, logFn)
		if err != nil {
			return
		}
		notify.MaybeConnectivity(lastStatus, st)
		lastStatus = st
	}
	runCycle()

	for {
		select {
		case <-sig:
			if logToStdout {
				fmt.Println("Stopping monitor.")
			}
			return nil
		case <-ticker.C:
			runCycle()
		}
	}
}
