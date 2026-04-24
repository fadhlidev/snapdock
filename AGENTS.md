# AGENTS.md

## Development Environment

- **Nix flake**: Run `nix develop` to enter the dev shell with Go toolchain
- **Go version**: 1.25.8 (from go.mod)

## Commands

- Build: `go build ./...`
- Test: `go test ./...`
- Run: `go run ./...`

## Notes

- This is a new Go project; no additional tooling or CI configured yet

## Commit Messages
Format: `[action]: [message]` (e.g., `add: new endpoint`, `fix: memory leak in cache`)
