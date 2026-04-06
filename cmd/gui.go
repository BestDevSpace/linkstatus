package cmd

import "github.com/spf13/cobra"

// guiCmd is a backward-compatible alias for the default (root) dashboard.
var guiCmd = &cobra.Command{
	Use:    "gui",
	Hidden: true,
	Short:  "Same as running linkstatus with no arguments",
	Args:   cobra.NoArgs,
	RunE:   runGUI,
}
