# ⚒ SnapDock

**SnapDock** is a powerful CLI tool that provides "Git-like" behavior for Docker containers. It allows you to snapshot the full state of a running container—including its image configuration, environment variables, networks, and volumes—and restore it anywhere.

## 🚀 Features

- **Full State Snapshots**: Capture container config, networks, and mount points.
- **Volume Support**: Optional inclusion of volume data in the snapshot.
- **Security**: AES-256 encryption for sensitive environment variables.
- **Portability**: Snapshots are packed into a single `.sfx` (tar.gz) file.
- **Safety First**: `--dry-run` flag to preview restore actions.
- **State Comparison**: `diff` command to compare two snapshots.
- **Rich UI**: Polished CLI with colors, spinners, and icons.

## 📦 Installation

### From Binary
Download the latest release for your platform from the [Releases](https://github.com/fadhlidev/snapdock/releases) page.

### From Source
```bash
go install github.com/fadhlidev/snapdock@latest
```

## 🛠 Usage

### 1. Snapshot a Container
Create a portable snapshot of a running container.
```bash
# Basic snapshot
snapdock snapshot myapp

# With volumes and encryption
snapdock snapshot myapp --with-volumes --encrypt --output ./backups/
```

### 2. List Snapshots
List all `.sfx` files in a directory.
```bash
snapdock list ./backups/
```

### 3. Inspect Snapshot
View the contents of a snapshot without restoring it.
```bash
snapdock inspect myapp-2025-04-24.sfx
```

### 4. Restore Container
Recreate a container from a snapshot.
```bash
# Preview the restore
snapdock restore myapp-2025-04-24.sfx --dry-run

# Full restore
snapdock restore myapp-2025-04-24.sfx --name myapp-restored --with-volumes
```

### 5. Diff Snapshots
Compare two snapshots to see what changed (image, env vars, ports, mounts).
```bash
snapdock diff snap-v1.sfx snap-v2.sfx
```

## 🤝 Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 📄 License

MIT License. See [LICENSE](LICENSE) for details.
