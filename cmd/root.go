package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "snapdock",
	Short: "SnapDock — Git-like snapshots for Docker containers",
	Long: fmt.Sprintf(`
%s
  Snapshot, restore, and diff Docker container state.
  Supports environment variables, networks, mounts, and volumes.

  %s
    snapdock snapshot myapp
    snapdock restore  myapp-2025-04-24.sfx
    snapdock inspect  myapp-2025-04-24.sfx
    snapdock diff     snap-v1.sfx snap-v2.sfx
    snapdock stack snapshot myproject
    snapdock stack restore  myproject-stack.sfx
`,
		color.New(color.FgRed, color.Bold).Sprint("⚒  SnapDock"),
		color.New(color.FgYellow).Sprint("Examples:"),
	),
	SilenceUsage: false,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("socket", "/var/run/docker.sock", "Docker daemon socket path")
	rootCmd.PersistentFlags().StringP("file", "f", "snapdock.yaml", "Configuration file path")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
}
