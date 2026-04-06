package cmd

import (
	"fmt"
	"os"

	"github.com/BestDevSpace/linkstatus/pkg/instance"
	"github.com/BestDevSpace/linkstatus/pkg/tui"

	"github.com/spf13/cobra"
)

// Version is set at link time (e.g. GoReleaser ldflags); default for dev builds.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "linkstatus",
	Short: "Internet connection monitor — terminal dashboard + optional background service",
	Long: "Default: open the terminal dashboard (read-only charts and samples from the local database).\n\n" +
		"  linkstatus              Open the GUI (same as the legacy `linkstatus gui`).\n" +
		"  linkstatus monitor      Run probes in the foreground; use this command in a LaunchAgent (macOS)\n" +
		"                          or systemd user unit (Linux) for notifications on up/down changes.\n" +
		"                          Data lives under ~/.linkstatus/.\n\n" +
		"From the GUI, use /service-install and /service-remove to register or remove the background\n" +
		"monitor so it starts again when you log in (macOS: LaunchAgent in ~/Library/LaunchAgents, same\n" +
		"mechanism Homebrew uses for brew services; Linux: systemd user unit under ~/.config/systemd/user).",
	Args: cobra.NoArgs,
	RunE: runGUI,
}

func runGUI(cmd *cobra.Command, args []string) error {
	fl, err := instance.TryGUILock()
	if err != nil {
		return err
	}
	defer fl.Unlock()
	return tui.Run()
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = Version
	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(guiCmd)
}
