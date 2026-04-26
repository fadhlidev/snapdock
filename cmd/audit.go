package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/audit"
	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/internal/snapshot"
)

var auditCmd = &cobra.Command{
	Use:   "audit <snapshot.sfx>",
	Short: "Audit a snapshot for sensitive information",
	Args:  cobra.ExactArgs(1),
	RunE:  runAudit,
}

func init() {
	rootCmd.AddCommand(auditCmd)
}

func runAudit(cmd *cobra.Command, args []string) error {
	sfxPath := args[0]

	output.Infof("Auditing snapshot %s...", color.YellowString(filepath.Base(sfxPath)))

	// Step 1: Verify checksum
	s := output.NewSpinner("Verifying checksum...")
	s.Start()
	if err := snapshot.VerifyChecksum(sfxPath); err != nil {
		s.Stop()
		output.Warningf("Checksum verification failed: %v", err)
	} else {
		s.Stop()
		output.Success("Checksum verified")
	}

	// Step 2: Extract snapshot
	s = output.NewSpinner("Extracting snapshot...")
	s.Start()
	extracted, err := snapshot.Extract(sfxPath)
	if err != nil {
		s.Stop()
		output.Errorf("Failed to extract snapshot: %v", err)
		return err
	}
	defer extracted.Cleanup()
	s.Stop()

	// Step 3: Run scan
	scanner := audit.NewScanner()
	findings := scanner.Scan(extracted.Env)

	// Step 4: Report findings
	fmt.Println()
	color.New(color.Bold).Println("  Security Audit Results")
	fmt.Println()

	if len(findings) == 0 {
		output.Success("No sensitive information detected in environment variables.")
		fmt.Println()
		return nil
	}

	criticalCount := 0
	warningCount := 0

	for _, f := range findings {
		switch f.Risk {
		case audit.RiskCritical:
			criticalCount++
			fmt.Printf("  %s %-20s %s\n", color.RedString("✖"), color.New(color.Bold, color.FgRed).Sprint(f.Key), color.HiBlackString(f.Value))
		case audit.RiskWarning:
			warningCount++
			fmt.Printf("  %s %-20s %s\n", color.YellowString("⚠"), color.New(color.Bold, color.FgYellow).Sprint(f.Key), color.HiBlackString(f.Value))
		default:
			fmt.Printf("  %s %-20s %s\n", color.CyanString("ℹ"), color.New(color.Bold, color.FgCyan).Sprint(f.Key), color.HiBlackString(f.Value))
		}
		fmt.Printf("    %s %s\n", color.HiBlackString("└─"), f.Description)
		fmt.Println()
	}

	fmt.Println("  Summary:")
	if criticalCount > 0 {
		fmt.Printf("  - %s %d critical issues found\n", color.RedString("✖"), criticalCount)
	}
	if warningCount > 0 {
		fmt.Printf("  - %s %d warnings found\n", color.YellowString("⚠"), warningCount)
	}
	fmt.Println()

	if criticalCount > 0 {
		output.Warning("Found critical security risks! We recommend using '--encrypt' during snapshot or removing secrets from the container and using a secret manager.")
	} else {
		output.Info("Audit complete with some warnings. Review the findings above.")
	}
	fmt.Println()

	return nil
}
