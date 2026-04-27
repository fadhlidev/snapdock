# ⚒ SnapDock

**SnapDock** is a powerful CLI tool that provides "Git-like" behavior for Docker containers. It allows you to snapshot the full state of a running container—including its image configuration, environment variables, networks, and volumes—and restore it anywhere.

## 🚀 Features

- **Full State Snapshots**: Capture container config, networks, and mount points.
- **Volume Support**: Optional inclusion of volume data in the snapshot.
- **Security Audit**: Built-in scanner to identify secrets in your snapshots.
- **Automated Scheduling**: Cron-based automated backups with the SnapDock daemon.
- **Retention Policies**: "Keep Last N" policies to automatically manage disk space.
- **MCP Server**: Programmatically manage your Docker state via AI agents (Claude, etc.).
- **AES-256 Encryption**: Secure your sensitive environment variables with a passphrase.
- **Portability**: Snapshots are packed into a single `.sfx` (tar.gz) file.
- **State Comparison**: `diff` command to compare two snapshots.
- **Safety First**: `--dry-run` flag to preview restore actions.

## 📦 Installation

### From Binary
Download the latest release for your platform from the [Releases](https://github.com/fadhlidev/snapdock/releases) page.

### From Source
```bash
go install github.com/fadhlidev/snapdock@latest
```
*Note: Requires Go 1.25.8 or higher.*

## 🛠 Usage

### 1. Snapshot and State Management
Manage your container states on-demand.
```bash
# Basic snapshot
snapdock snapshot myapp

# With volumes and encryption
snapdock snapshot myapp --with-volumes --encrypt --output ./backups/

# Multi-snapshot diff
snapdock diff snap-v1.sfx snap-v2.sfx

# Security audit
snapdock audit snap-v1.sfx
```

### 2. Automated Scheduling
Run SnapDock as a background daemon to handle recurring backups.

**Step 1: Configure jobs**
```bash
# Add a daily backup for 'webapp' keeping only the last 7 snapshots
snapdock schedule add webapp --cron "@daily" --keep 7 -f backups.yaml
```

**Step 2: Start the daemon**
```bash
snapdock daemon start -f backups.yaml
```

### 3. Cleanup & Pruning
Manually or automatically clean up old snapshots.
```bash
# Keep only the last 5 snapshots in the directory
snapdock prune ./backups --keep 5
```

### 4. AI Integration (MCP)
SnapDock includes a **Model Context Protocol (MCP)** server, making it controllable by AI agents.

**Claude Desktop Configuration:**
```json
{
  "mcpServers": {
    "snapdock": {
      "command": "snapdock",
      "args": ["mcp"]
    }
  }
}
```
*AI Commands: "List my snapshots", "Snapshot the web container", "Audit the latest backup for leaks".*

## ⚙️ Configuration (`snapdock.yaml`)
Scheduled jobs are stored in a simple YAML format:
```yaml
jobs:
  - name: "daily-db-backup"
    container: "postgres-prod"
    schedule: "0 0 * * *"
    output: "./backups"
    retention:
      keep_last: 7
```

## 🤝 Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 📄 License

MIT License. See [LICENSE](LICENSE) for details.
