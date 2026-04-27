package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/config"
	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/internal/scheduler"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the SnapDock scheduler daemon",
	Long:  `Start a background process that executes snapshots according to the schedules defined in the configuration file.`,
	RunE:  runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the scheduler daemon",
	RunE:  runDaemon,
}

func runDaemon(cmd *cobra.Command, args []string) error {
	configFile, _ := cmd.Flags().GetString("file")
	socketPath, _ := cmd.Flags().GetString("socket")

	output.Infof("Loading configuration from %s...", color.YellowString(configFile))
	cfg, err := config.Load(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			output.Errorf("Configuration file %s not found. Use 'snapdock schedule add' to create one.", configFile)
		} else {
			output.Errorf("Failed to load configuration: %v", err)
		}
		return err
	}

	d := scheduler.NewDaemon(cfg, socketPath)
	
	fmt.Println()
	color.New(color.Bold, color.FgCyan).Println(" 🕒 SnapDock Scheduler Daemon")
	fmt.Printf(" Running with %d jobs configured.\n", len(cfg.Jobs))
	fmt.Println(" Press Ctrl+C to stop.")
	fmt.Println()

	d.Start()

	// Wait for interrupt signal to gracefully shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	output.Info("Shutting down gracefully...")
	d.Stop()
	
	return nil
}
