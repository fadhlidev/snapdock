package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/config"
	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/pkg/types"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage scheduled snapshot jobs",
}

var scheduleAddCmd = &cobra.Command{
	Use:   "add <container>",
	Short: "Add a new scheduled snapshot job",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduleAdd,
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleAddCmd)

	scheduleAddCmd.Flags().StringP("name", "n", "", "Custom name for the job")
	scheduleAddCmd.Flags().StringP("cron", "c", "@daily", "Cron schedule (e.g. '0 0 * * *' or '@hourly')")
	scheduleAddCmd.Flags().IntP("keep", "k", 7, "Number of snapshots to keep")
	scheduleAddCmd.Flags().StringP("output", "o", ".", "Output directory for snapshots")
	scheduleAddCmd.Flags().Bool("volumes", false, "Include volumes in snapshots")
	scheduleAddCmd.Flags().Bool("encrypt", false, "Encrypt environment variables")
	scheduleAddCmd.Flags().String("type", "container", "Type of snapshot: 'container' or 'stack'")
	scheduleAddCmd.Flags().String("compose-file", "", "Path to docker-compose.yml (for stack snapshots)")
}

func runScheduleAdd(cmd *cobra.Command, args []string) error {
	container := args[0]
	name, _ := cmd.Flags().GetString("name")
	cron, _ := cmd.Flags().GetString("cron")
	keep, _ := cmd.Flags().GetInt("keep")
	outputDir, _ := cmd.Flags().GetString("output")
	withVolumes, _ := cmd.Flags().GetBool("volumes")
	encrypt, _ := cmd.Flags().GetBool("encrypt")
	jobType, _ := cmd.Flags().GetString("type")
	composeFile, _ := cmd.Flags().GetString("compose-file")
	configFile, _ := cmd.Flags().GetString("file")

	if name == "" {
		name = fmt.Sprintf("backup-%s", container)
	}

	// Load existing config or create new
	cfg, err := config.Load(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = &types.Config{}
		} else {
			return err
		}
	}

	// Check if job name already exists
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return fmt.Errorf("job with name '%s' already exists", name)
		}
	}

	newJob := types.JobConfig{
		Name:        name,
		Type:        types.SnapshotType(jobType),
		Container:   container,
		ComposeFile: composeFile,
		Schedule:    cron,
		Output:      outputDir,
		Options: types.JobOptions{
			WithVolumes: withVolumes,
			Encrypt:     encrypt,
		},
		Retention: types.RetentionConfig{
			KeepLast: keep,
		},
	}

	cfg.Jobs = append(cfg.Jobs, newJob)

	if err := config.Save(configFile, cfg); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	output.Successf("Added job '%s' to %s", color.CyanString(name), color.YellowString(configFile))
	fmt.Printf(" Next run: %s\n", cron)
	
	return nil
}
