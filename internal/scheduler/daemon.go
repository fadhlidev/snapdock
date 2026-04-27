package scheduler

import (
	"context"
	"fmt"
	"os"

	"github.com/robfig/cron/v3"

	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/internal/retention"
	"github.com/fadhlidev/snapdock/internal/snapshot"
	"github.com/fadhlidev/snapdock/pkg/types"
)

type Daemon struct {
	cron       *cron.Cron
	config     *types.Config
	socketPath string
}

func NewDaemon(cfg *types.Config, socketPath string) *Daemon {
	return &Daemon{
		cron:       cron.New(cron.WithSeconds()), // Support seconds if needed
		config:     cfg,
		socketPath: socketPath,
	}
}

func (d *Daemon) Start() {
	for _, job := range d.config.Jobs {
		j := job // copy for closure
		d.cron.AddFunc(j.Schedule, func() {
			d.runJob(j)
		})
		output.Infof("Scheduled job '%s' for container '%s' with schedule '%s'", j.Name, j.Container, j.Schedule)
	}

	d.cron.Start()
}

func (d *Daemon) Stop() {
	d.cron.Stop()
}

func (d *Daemon) runJob(job types.JobConfig) {
	ctx := context.Background()
	output.Infof("[%s] Starting scheduled snapshot...", job.Name)

	client, err := docker.NewClient(d.socketPath)
	if err != nil {
		output.Errorf("[%s] Failed to connect to Docker: %v", job.Name, err)
		return
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		output.Errorf("[%s] Docker is not responding: %v", job.Name, err)
		return
	}

	snap, err := client.InspectContainer(ctx, job.Container)
	if err != nil {
		output.Errorf("[%s] Failed to inspect container: %v", job.Name, err)
		return
	}

	opts := types.SnapOptions{
		WithVolumes: job.Options.WithVolumes,
		Encrypted:   job.Options.Encrypt,
	}

	// For scheduled jobs, we might need a way to provide passphrase if encrypted.
	// For now, we'll assume unencrypted or we'd need a way to get it from env.
	passphrase := os.Getenv("SNAPDOCK_PASSPHRASE")

	outputDir := job.Output
	if outputDir == "" {
		outputDir = "."
	}

	result, err := snapshot.Pack(ctx, client, snap, opts, outputDir, passphrase)
	if err != nil {
		output.Errorf("[%s] Snapshot failed: %v", job.Name, err)
		return
	}

	output.Successf("[%s] Snapshot complete: %s (%s)", job.Name, result.SfxPath, formatSize(result.SizeBytes))
	
	// Apply retention
	if job.Retention.KeepLast > 0 {
		if err := retention.PruneDir(outputDir, job.Retention.KeepLast); err != nil {
			output.Errorf("[%s] Retention failed: %v", job.Name, err)
		}
	}
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
