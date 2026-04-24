# Contributing to SnapDock

Thank you for your interest in contributing! We welcome all types of contributions, from bug reports to new features.

## 🐛 Bug Reports

If you find a bug, please open an issue and include:
- Your operating system and Docker version.
- Exact commands you ran.
- Error messages and logs (use `--verbose`).
- What you expected to happen vs what actually happened.

## 💡 Feature Suggestions

Have an idea to make SnapDock better? Open an issue with the "feature request" label. Please describe the use case and how it would benefit other users.

## 🛠 Development Setup

SnapDock uses a **Nix** development shell for a consistent build environment.

1. Ensure you have Nix installed with flakes enabled.
2. Enter the development environment:
   ```bash
   nix develop
   ```
3. Run tests:
   ```bash
   go test ./...
   ```
4. Build locally:
   ```bash
   go build .
   ```

## 📝 Pull Request Guidelines

1. Fork the repository and create your branch from `main`.
2. Follow Go idiomatic patterns and ensure code is formatted with `gofmt`.
3. Add tests for any new features or bug fixes.
4. Ensure the build passes: `go build ./...`.
5. Update documentation if necessary.
6. Commit messages should follow the format: `[action]: [message]` (e.g., `add: new endpoint`).

---

Happy coding! ⚒
