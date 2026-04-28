package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/fadhlidev/snapdock/internal/audit"
	"github.com/fadhlidev/snapdock/internal/compose"
	"github.com/fadhlidev/snapdock/internal/config"
	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/internal/retention"
	"github.com/fadhlidev/snapdock/internal/snapshot"
	"github.com/fadhlidev/snapdock/pkg/types"
)

type MCPServer struct {
	server     *server.MCPServer
	socketPath string
}

func NewServer(version string, socketPath string) *MCPServer {
	s := server.NewMCPServer(
		"SnapDock MCP Server",
		version,
		server.WithToolCapabilities(true),
	)

	mcpSrv := &MCPServer{
		server:     s,
		socketPath: socketPath,
	}

	mcpSrv.registerTools()

	return mcpSrv
}

func (s *MCPServer) Start() error {
	return server.ServeStdio(s.server)
}

func (s *MCPServer) registerTools() {
	// list_snapshots
	s.server.AddTool(mcp.NewTool("list_snapshots",
		mcp.WithDescription("List available snapshots in a directory"),
		mcp.WithString("directory", mcp.Description("Directory to scan for snapshots (default: .)")),
	), s.handleListSnapshots)

	// snapshot_container
	s.server.AddTool(mcp.NewTool("snapshot_container",
		mcp.WithDescription("Create a snapshot of a running Docker container"),
		mcp.WithString("container", mcp.Required(), mcp.Description("Name or ID of the container to snapshot")),
		mcp.WithBoolean("with_volumes", mcp.Description("Include volume data in the snapshot")),
		mcp.WithString("output_dir", mcp.Description("Directory to save the snapshot (default: .)")),
		mcp.WithString("passphrase", mcp.Description("Passphrase for encrypting environment variables")),
	), s.handleSnapshotContainer)

	// inspect_snapshot
	s.server.AddTool(mcp.NewTool("inspect_snapshot",
		mcp.WithDescription("Inspect a snapshot file to view container details"),
		mcp.WithString("file", mcp.Required(), mcp.Description("Path to the .sfx snapshot file")),
	), s.handleInspectSnapshot)

	// restore_container
	s.server.AddTool(mcp.NewTool("restore_container",
		mcp.WithDescription("Restore a container from a snapshot file"),
		mcp.WithString("file", mcp.Required(), mcp.Description("Path to the .sfx snapshot file")),
		mcp.WithString("name", mcp.Description("Name for the restored container (default: original name with -restored suffix)")),
		mcp.WithBoolean("with_volumes", mcp.Description("Restore volume data if available")),
		mcp.WithString("passphrase", mcp.Description("Passphrase for decrypting environment variables if encrypted")),
	), s.handleRestoreContainer)

	// diff_snapshots
	s.server.AddTool(mcp.NewTool("diff_snapshots",
		mcp.WithDescription("Compare two snapshots and show differences"),
		mcp.WithString("file1", mcp.Required(), mcp.Description("Path to the first .sfx file")),
		mcp.WithString("file2", mcp.Required(), mcp.Description("Path to the second .sfx file")),
	), s.handleDiffSnapshots)

	// audit_snapshot
	s.server.AddTool(mcp.NewTool("audit_snapshot",
		mcp.WithDescription("Audit a snapshot for sensitive information"),
		mcp.WithString("file", mcp.Required(), mcp.Description("Path to the .sfx snapshot file")),
	), s.handleAuditSnapshot)

	// prune_snapshots
	s.server.AddTool(mcp.NewTool("prune_snapshots",
		mcp.WithDescription("Manually prune old snapshots from a directory"),
		mcp.WithString("directory", mcp.Required(), mcp.Description("Directory containing .sfx snapshots")),
		mcp.WithNumber("keep", mcp.Description("Number of most recent snapshots to keep (default: 5)")),
	), s.handlePruneSnapshots)

	// list_scheduler_jobs
	s.server.AddTool(mcp.NewTool("list_scheduler_jobs",
		mcp.WithDescription("List all scheduled backup jobs"),
		mcp.WithString("config_file", mcp.Description("Path to snapdock.yaml (default: snapdock.yaml)")),
	), s.handleListSchedulerJobs)

	// add_scheduler_job
	s.server.AddTool(mcp.NewTool("add_scheduler_job",
		mcp.WithDescription("Add a new scheduled snapshot job"),
		mcp.WithString("container", mcp.Required(), mcp.Description("Name or ID of the container")),
		mcp.WithString("schedule", mcp.Required(), mcp.Description("Cron schedule (e.g. '@daily', '0 0 * * *')")),
		mcp.WithString("name", mcp.Description("Custom name for the job")),
		mcp.WithNumber("keep", mcp.Description("Number of snapshots to keep (default: 7)")),
		mcp.WithString("output_dir", mcp.Description("Directory to save snapshots (default: .)")),
		mcp.WithBoolean("with_volumes", mcp.Description("Include volumes in snapshots")),
		mcp.WithBoolean("encrypt", mcp.Description("Encrypt environment variables")),
		mcp.WithString("config_file", mcp.Description("Path to snapdock.yaml (default: snapdock.yaml)")),
	), s.handleAddSchedulerJob)
	
	// snapshot_stack
	s.server.AddTool(mcp.NewTool("snapshot_stack",
		mcp.WithDescription("Create a snapshot of a Docker Compose stack"),
		mcp.WithString("project_name", mcp.Required(), mcp.Description("Name of the Docker Compose project")),
		mcp.WithString("compose_file", mcp.Description("Path to the docker-compose.yml file (auto-detected if not provided)")),
		mcp.WithBoolean("encrypt", mcp.Description("Encrypt environment variables in the snapshot")),
		mcp.WithString("output_dir", mcp.Description("Directory to save the snapshot (default: .)")),
		mcp.WithString("passphrase", mcp.Description("Passphrase for encrypting sensitive data")),
	), s.handleSnapshotStack)

	// restore_stack
	s.server.AddTool(mcp.NewTool("restore_stack",
		mcp.WithDescription("Restore a Docker Compose stack from a snapshot file"),
		mcp.WithString("file", mcp.Required(), mcp.Description("Path to the .sfx stack snapshot file")),
		mcp.WithString("name", mcp.Description("Optional name prefix for the restored stack services")),
		mcp.WithString("passphrase", mcp.Description("Passphrase for decrypting sensitive data if encrypted")),
	), s.handleRestoreStack)
}

func (s *MCPServer) handleListSnapshots(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dirPath := request.GetString("directory", ".")

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read directory: %v", err)), nil
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Snapshots in %s:\n\n", dirPath))

	found := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sfx") {
			found = true
			info, _ := entry.Info()
			builder.WriteString(fmt.Sprintf("- %s (%s)\n", entry.Name(), formatSize(info.Size())))
		}
	}

	if !found {
		return mcp.NewToolResultText("No snapshots found in " + dirPath), nil
	}

	return mcp.NewToolResultText(builder.String()), nil
}

func (s *MCPServer) handleSnapshotContainer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	containerName, err := request.RequireString("container")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	withVolumes := request.GetBool("with_volumes", false)
	outputDir := request.GetString("output_dir", ".")
	passphrase := request.GetString("passphrase", "")

	client, err := docker.NewClient(s.socketPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to Docker: %v", err)), nil
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to ping Docker: %v", err)), nil
	}

	snap, err := client.InspectContainer(ctx, containerName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to inspect container: %v", err)), nil
	}

	opts := types.SnapOptions{
		WithVolumes: withVolumes,
		Encrypted:   passphrase != "",
	}

	result, err := snapshot.Pack(ctx, client, snap, opts, outputDir, passphrase)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create snapshot: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Snapshot created successfully:\n- Path: %s\n- Size: %s\n- Checksum: %s",
		result.SfxPath, formatSize(result.SizeBytes), result.Checksum)), nil
}

func (s *MCPServer) handleInspectSnapshot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	snapType, err := snapshot.DetectSnapshotType(filePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to detect snapshot type: %v", err)), nil
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Snapshot: %s\n", filePath))
	builder.WriteString(fmt.Sprintf("Type: %s\n", snapType))

	if snapType == types.SnapshotTypeStack {
		extracted, err := snapshot.ExtractStack(filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to extract stack snapshot: %v", err)), nil
		}
		defer extracted.Cleanup()

		m := extracted.Manifest
		builder.WriteString(fmt.Sprintf("Project: %s\n", m.Project.Name))
		builder.WriteString(fmt.Sprintf("Created: %s\n", m.CreatedAt.Format("2006-01-02 15:04:05")))
		builder.WriteString(fmt.Sprintf("SnapDock Version: %s\n", m.SnapDockVersion))
		builder.WriteString(fmt.Sprintf("\nServices (%d):\n", len(m.Services)))
		for _, svc := range m.Services {
			builder.WriteString(fmt.Sprintf("- %s (%s)\n", svc.Name, svc.Image))
		}
		builder.WriteString(fmt.Sprintf("\nInfrastructure:\n"))
		builder.WriteString(fmt.Sprintf("- Networks: %d\n", len(m.Networks)))
		builder.WriteString(fmt.Sprintf("- Volumes: %d\n", len(m.Volumes)))
		if m.Options.Encrypted {
			builder.WriteString("- Encryption: AES-256 (Encrypted)\n")
		}
	} else {
		extracted, err := snapshot.Extract(filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to extract snapshot: %v", err)), nil
		}
		defer extracted.Cleanup()

		m := extracted.Manifest
		c := extracted.Container
		builder.WriteString(fmt.Sprintf("Container: %s (%s)\n", c.Name, c.ID[:12]))
		builder.WriteString(fmt.Sprintf("Image: %s\n", c.Image))
		builder.WriteString(fmt.Sprintf("Created: %s\n", m.CreatedAt.Format("2006-01-02 15:04:05")))
		builder.WriteString(fmt.Sprintf("SnapDock Version: %s\n", m.SnapDockVersion))
		builder.WriteString("\nConfiguration:\n")
		builder.WriteString(fmt.Sprintf("- Env vars: %d\n", len(c.Env)))
		builder.WriteString(fmt.Sprintf("- Networks: %d\n", len(extracted.Networks)))
		builder.WriteString(fmt.Sprintf("- Ports: %d\n", len(c.Ports)))
		builder.WriteString(fmt.Sprintf("- Mounts: %d\n", len(c.Mounts)))
		if m.Options.Encrypted {
			builder.WriteString("- Encryption: AES-256 (Encrypted)\n")
		}
	}

	return mcp.NewToolResultText(builder.String()), nil
}


func (s *MCPServer) handleRestoreContainer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sfxPath, err := request.RequireString("file")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	restoreName := request.GetString("name", "")
	withVolumes := request.GetBool("with_volumes", false)
	passphrase := request.GetString("passphrase", "")

	// Step 1: Verify checksum (optional, but good)
	if err := snapshot.VerifyChecksum(sfxPath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("checksum verification failed: %v", err)), nil
	}

	// Step 2: Extract snapshot
	extracted, err := snapshot.Extract(sfxPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to extract snapshot: %v", err)), nil
	}
	defer extracted.Cleanup()

	// Step 3: Decrypt env if needed
	encPath := filepath.Join(extracted.TempDir, "env.json.enc")
	if _, err := os.Stat(encPath); err == nil {
		if passphrase == "" {
			return mcp.NewToolResultError("snapshot is encrypted, but no passphrase provided"), nil
		}
		_, err := snapshot.DecryptEnv(extracted.TempDir, passphrase)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to decrypt environment: %v", err)), nil
		}
	}

	// Step 4: Connect to Docker
	client, err := docker.NewClient(s.socketPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to Docker: %v", err)), nil
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to ping Docker: %v", err)), nil
	}

	// Step 5: Recreate networks
	for _, net := range extracted.Networks {
		exists, _ := client.NetworkExists(ctx, net.Name)
		if !exists {
			netCfg := docker.NetworkConfig{
				Driver:  net.Driver,
				Subnet:  net.Subnet,
				Gateway: net.Gateway,
				Scope:   net.Scope,
			}
			_, err := client.CreateNetwork(ctx, net.Name, netCfg)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to create network %s: %v", net.Name, err)), nil
			}
		}
	}

	// Step 6: Pull image
	imageName := extracted.Container.Image
	_, err = client.PullImageIfMissing(ctx, imageName, os.Stderr) // Pull output to stderr
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to pull image: %v", err)), nil
	}

	// Step 7: Restore volumes
	if withVolumes {
		if err := snapshot.RestoreVolumes(ctx, client, extracted, extracted.TempDir); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to restore volumes: %v", err)), nil
		}
	}

	// Step 8: Create and start container
	if restoreName == "" {
		restoreName = extracted.Container.Name + "-restored"
	}

	containerCfg := s.buildContainerConfig(extracted, restoreName)
	result, err := client.CreateContainer(ctx, *containerCfg)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create container: %v", err)), nil
	}

	if err := client.StartContainer(ctx, result.ID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to start container: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Container restored successfully:\n- Name: %s\n- ID: %s", restoreName, result.ID[:12])), nil
}

func (s *MCPServer) handleDiffSnapshots(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file1, err := request.RequireString("file1")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	file2, err := request.RequireString("file2")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	type1, _ := snapshot.DetectSnapshotType(file1)
	type2, _ := snapshot.DetectSnapshotType(file2)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Comparison: %s vs %s\n", filepath.Base(file1), filepath.Base(file2)))
	builder.WriteString(fmt.Sprintf("Types: %s vs %s\n\n", type1, type2))

	if type1 != type2 {
		return mcp.NewToolResultText(builder.String() + "Cannot compare snapshots of different types."), nil
	}

	if type1 == types.SnapshotTypeStack {
		extracted1, err := snapshot.ExtractStack(file1)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to extract stack %s: %v", file1, err)), nil
		}
		defer extracted1.Cleanup()

		extracted2, err := snapshot.ExtractStack(file2)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to extract stack %s: %v", file2, err)), nil
		}
		defer extracted2.Cleanup()

		builder.WriteString("Services Diff:\n")
		services1 := make(map[string]string)
		for _, s := range extracted1.Manifest.Services {
			services1[s.Name] = s.Image
		}
		services2 := make(map[string]string)
		for _, s := range extracted2.Manifest.Services {
			services2[s.Name] = s.Image
		}

		allSvc := make(map[string]bool)
		for n := range services1 {
			allSvc[n] = true
		}
		for n := range services2 {
			allSvc[n] = true
		}

		var names []string
		for n := range allSvc {
			names = append(names, n)
		}
		sort.Strings(names)

		for _, n := range names {
			img1, ok1 := services1[n]
			img2, ok2 := services2[n]
			if ok1 && !ok2 {
				builder.WriteString(fmt.Sprintf("- %s (image: %s)\n", n, img1))
			} else if !ok1 && ok2 {
				builder.WriteString(fmt.Sprintf("+ %s (image: %s)\n", n, img2))
			} else if img1 != img2 {
				builder.WriteString(fmt.Sprintf("~ %s:\n  - %s\n  + %s\n", n, img1, img2))
			} else {
				builder.WriteString(fmt.Sprintf("  %s (unchanged)\n", n))
			}
		}
	} else {
		extracted1, err := snapshot.Extract(file1)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to extract %s: %v", file1, err)), nil
		}
		defer extracted1.Cleanup()

		extracted2, err := snapshot.Extract(file2)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to extract %s: %v", file2, err)), nil
		}
		defer extracted2.Cleanup()

		// Image
		if extracted1.Container.Image != extracted2.Container.Image {
			builder.WriteString(fmt.Sprintf("Image changed:\n- %s\n+ %s\n\n", extracted1.Container.Image, extracted2.Container.Image))
		} else {
			builder.WriteString(fmt.Sprintf("Image: %s (unchanged)\n\n", extracted1.Container.Image))
		}

		// Simple env diff
		enc1 := fileExists(filepath.Join(extracted1.TempDir, "env.json.enc"))
		enc2 := fileExists(filepath.Join(extracted2.TempDir, "env.json.enc"))
		if enc1 || enc2 {
			builder.WriteString("Environment Variables: (one or both snapshots are encrypted, cannot diff without passphrase)\n")
		} else {
			env1 := parseEnvMap(extracted1.TempDir)
			env2 := parseEnvMap(extracted2.TempDir)
			builder.WriteString("Environment Variables: ")
			s.diffEnvVars(&builder, env1, env2)
		}
	}

	return mcp.NewToolResultText(builder.String()), nil
}


func (s *MCPServer) handleAuditSnapshot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, err := request.RequireString("file")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	snapType, err := snapshot.DetectSnapshotType(filePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to detect snapshot type: %v", err)), nil
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Security Audit Results for %s (%s)\n\n", filepath.Base(filePath), snapType))

	scanner := audit.NewScanner()

	if snapType == types.SnapshotTypeStack {
		extracted, err := snapshot.ExtractStack(filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to extract stack: %v", err)), nil
		}
		defer extracted.Cleanup()

		totalFindings := 0
		for svcName, env := range extracted.Envs {
			findings := scanner.Scan(env)
			if len(findings) > 0 {
				builder.WriteString(fmt.Sprintf("Service: %s\n", svcName))
				for _, f := range findings {
					icon := "ℹ️"
					if f.Risk == audit.RiskCritical {
						icon = "❌"
						totalFindings++
					} else if f.Risk == audit.RiskWarning {
						icon = "⚠️"
						totalFindings++
					}
					builder.WriteString(fmt.Sprintf("  %s %s (%s): %s\n", icon, f.Key, f.Risk, f.Description))
				}
				builder.WriteString("\n")
			}
		}

		if totalFindings == 0 {
			builder.WriteString("✅ No sensitive information detected in any service environment variables.\n")
		} else {
			builder.WriteString(fmt.Sprintf("Summary: Total critical/warning findings across all services: %d\n", totalFindings))
		}
	} else {
		extracted, err := snapshot.Extract(filePath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to extract snapshot: %v", err)), nil
		}
		defer extracted.Cleanup()

		findings := scanner.Scan(extracted.Env)
		if len(findings) == 0 {
			builder.WriteString("✅ No sensitive information detected in environment variables.\n")
		} else {
			for _, f := range findings {
				icon := "ℹ️"
				if f.Risk == audit.RiskCritical {
					icon = "❌"
				} else if f.Risk == audit.RiskWarning {
					icon = "⚠️"
				}
				builder.WriteString(fmt.Sprintf("%s %s (%s): %s\n", icon, f.Key, f.Risk, f.Description))
			}
			builder.WriteString(fmt.Sprintf("\nSummary: Total findings: %d\n", len(findings)))
		}
	}

	builder.WriteString("\nRecommendation: We recommend using encryption (--encrypt) or a secret manager for sensitive data.")
	return mcp.NewToolResultText(builder.String()), nil
}


func (s *MCPServer) handlePruneSnapshots(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir, err := request.RequireString("directory")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	keep := request.GetInt("keep", 5)

	if err := retention.PruneDir(dir, keep); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("prune failed: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Pruning complete in %s. Kept last %d snapshots per container.", dir, keep)), nil
}

func (s *MCPServer) handleListSchedulerJobs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	configFile := request.GetString("config_file", "snapdock.yaml")

	cfg, err := config.Load(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return mcp.NewToolResultText("No configuration file found at " + configFile), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to load config: %v", err)), nil
	}

	if len(cfg.Jobs) == 0 {
		return mcp.NewToolResultText("No scheduled jobs found in " + configFile), nil
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Scheduled Jobs in %s:\n\n", configFile))
	for _, job := range cfg.Jobs {
		builder.WriteString(fmt.Sprintf("- %s: %s (container: %s, keep: %d)\n", job.Name, job.Schedule, job.Container, job.Retention.KeepLast))
	}

	return mcp.NewToolResultText(builder.String()), nil
}

func (s *MCPServer) handleAddSchedulerJob(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	container, err := request.RequireString("container")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	schedule, err := request.RequireString("schedule")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	name := request.GetString("name", "backup-"+container)
	keep := request.GetInt("keep", 7)
	outputDir := request.GetString("output_dir", ".")
	withVolumes := request.GetBool("with_volumes", false)
	encrypt := request.GetBool("encrypt", false)
	configFile := request.GetString("config_file", "snapdock.yaml")

	cfg, err := config.Load(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = &types.Config{}
		} else {
			return mcp.NewToolResultError(fmt.Sprintf("failed to load config: %v", err)), nil
		}
	}

	for _, job := range cfg.Jobs {
		if job.Name == name {
			return mcp.NewToolResultError(fmt.Sprintf("job with name '%s' already exists", name)), nil
		}
	}

	newJob := types.JobConfig{
		Name:      name,
		Container: container,
		Schedule:  schedule,
		Output:    outputDir,
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to save config: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Job '%s' added successfully to %s.\nNext run schedule: %s", name, configFile, schedule)), nil
}

func (s *MCPServer) buildContainerConfig(extracted *snapshot.ExtractedSnapshot, containerName string) *docker.ContainerConfig {
	container := extracted.Container

	cfg := &docker.ContainerConfig{
		Name:       containerName,
		Image:      container.Image,
		Cmd:        container.Cmd,
		Entrypoint: container.Entrypoint,
		WorkingDir: container.WorkingDir,
		User:       container.User,
		Hostname:   container.Hostname,
		Labels:     container.Labels,
		StopSignal: container.StopSignal,
	}

	// Try to read env.json
	envPath := filepath.Join(extracted.TempDir, "env.json")
	if envData, err := os.ReadFile(envPath); err == nil {
		var envVars []types.EnvVar
		if err := json.Unmarshal(envData, &envVars); err == nil {
			for _, e := range envVars {
				cfg.Env = append(cfg.Env, e.Key+"="+e.Value)
			}
		}
	}

	if len(cfg.Env) == 0 {
		cfg.Env = container.Env
	}

	for _, net := range extracted.Networks {
		cfg.Networks = append(cfg.Networks, docker.ContainerNetwork{
			Name:        net.Name,
			Aliases:     net.Aliases,
			IPv4Address: net.IPAddress,
		})
	}

	if len(container.Ports) > 0 {
		cfg.PortBindings = make(map[string][]docker.PortBinding)
		for _, p := range container.Ports {
			if p.HostPort != "" {
				cfg.PortBindings[p.ContainerPort] = append(cfg.PortBindings[p.ContainerPort], docker.PortBinding{
					HostIP:   p.HostIP,
					HostPort: p.HostPort,
				})
			}
		}
	}

	binds, tmpfs := snapshot.MountArgs(extracted.Mounts)
	cfg.Binds = binds
	cfg.Tmpfs = tmpfs

	cfg.CPUShares = container.Resources.CPUShares
	cfg.CPUQuota = container.Resources.CPUQuota
	cfg.MemoryMB = container.Resources.MemoryMB
	cfg.MemSwapMB = container.Resources.MemSwapMB

	return cfg
}

func (s *MCPServer) diffEnvVars(builder *strings.Builder, env1, env2 map[string]string) {
	allKeys := make(map[string]bool)
	for k := range env1 {
		allKeys[k] = true
	}
	for k := range env2 {
		allKeys[k] = true
	}

	var keys []string
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	hasDiff := false
	for _, k := range keys {
		v1, ok1 := env1[k]
		v2, ok2 := env2[k]

		if !ok1 && ok2 {
			builder.WriteString(fmt.Sprintf("\n+ %s=%s", k, v2))
			hasDiff = true
		} else if ok1 && !ok2 {
			builder.WriteString(fmt.Sprintf("\n- %s=%s", k, v1))
			hasDiff = true
		} else if v1 != v2 {
			builder.WriteString(fmt.Sprintf("\n- %s=%s", k, v1))
			builder.WriteString(fmt.Sprintf("\n+ %s=%s", k, v2))
			hasDiff = true
		}
	}

	if !hasDiff {
		builder.WriteString("(no differences)")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseEnvMap(tempDir string) map[string]string {
	envPath := filepath.Join(tempDir, "env.json")
	result := make(map[string]string)

	data, err := os.ReadFile(envPath)
	if err != nil {
		return result
	}

	var envVars []types.EnvVar
	if err := json.Unmarshal(data, &envVars); err != nil {
		return result
	}

	for _, e := range envVars {
		result[e.Key] = e.Value
	}
	return result
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

func (s *MCPServer) handleSnapshotStack(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectName, err := request.RequireString("project_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	composeFile := request.GetString("compose_file", "")
	encrypt := request.GetBool("encrypt", false)
	outputDir := request.GetString("output_dir", ".")
	passphrase := request.GetString("passphrase", "")

	if composeFile == "" {
		dir, err := os.Getwd()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get current directory: %v", err)), nil
		}
		composeFile, err = compose.FindComposeFile(dir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to find compose file: %v", err)), nil
		}
	}

	project, err := compose.ParseComposeFile(composeFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse compose file: %v", err)), nil
	}

	if projectName != "" && project.Name != projectName {
		return mcp.NewToolResultError(fmt.Sprintf("project name mismatch: expected %q, got %q", projectName, project.Name)), nil
	}

	client, err := docker.NewClient(s.socketPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to Docker: %v", err)), nil
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to ping Docker: %v", err)), nil
	}

	opts := types.SnapOptions{
		Encrypted: encrypt,
	}

	result, err := snapshot.PackStack(ctx, client, project, opts, outputDir, passphrase)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create stack snapshot: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Stack snapshot created successfully:\n- Path: %s\n- Size: %s\n- Checksum: %s\n- Services: %d",
		result.SfxPath, formatSize(result.SizeBytes), result.Checksum, result.ServiceCount)), nil
}

func (s *MCPServer) handleRestoreStack(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sfxPath, err := request.RequireString("file")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	restoreName := request.GetString("name", "")
	passphrase := request.GetString("passphrase", "")

	// Detect type
	snapType, err := snapshot.DetectSnapshotType(sfxPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to detect snapshot type: %v", err)), nil
	}

	if snapType != types.SnapshotTypeStack {
		return mcp.NewToolResultError(fmt.Sprintf("not a stack snapshot: type=%q", snapType)), nil
	}

	// Verify checksum
	if err := snapshot.VerifyChecksum(sfxPath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("checksum verification failed: %v", err)), nil
	}

	// Extract
	extracted, err := snapshot.ExtractStack(sfxPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to extract stack snapshot: %v", err)), nil
	}
	defer extracted.Cleanup()

	// Connect
	client, err := docker.NewClient(s.socketPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to connect to Docker: %v", err)), nil
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to ping Docker: %v", err)), nil
	}

	// Restore
	opts := snapshot.RestoreOptions{
		NewName: restoreName,
	}

	err = snapshot.RestoreStack(ctx, client, extracted, opts, passphrase)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to restore stack: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Stack restored successfully:\n- Project: %s\n- Services: %d",
		extracted.Manifest.Project.Name, len(extracted.Compose.Services))), nil
}


