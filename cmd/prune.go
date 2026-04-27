package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/retention"
)

var pruneCmd = &cobra.Command{
	Use:   "prune <directory>",
	Short: "Manually prune old snapshots from a directory",
	Args:  cobra.ExactArgs(1),
	RunE:  runPrune,
}

func init() {
	rootCmd.AddCommand(pruneCmd)
	pruneCmd.Flags().IntP("keep", "k", 5, "Number of most recent snapshots to keep per container")
}

func runPrune(cmd *cobra.Command, args []string) error {
	dir := args[0]
	keep, _ := cmd.Flags().GetInt("keep")

	color.New(color.Bold, color.FgCyan).Printf(" 🧹 Pruning snapshots in %s (keeping last %d)\n", dir, keep)
	fmt.Println()

	if err := retention.PruneDir(dir, keep); err != nil {
		return fmt.Errorf("prune failed: %v", err)
	}

	fmt.Println()
	color.New(color.FgGreen).Println(" Cleanup complete.")
	return nil
}
