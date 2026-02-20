# Windows Browser Guard — Copilot Instructions

## Project
Go daemon for Windows that monitors `HKLM\SOFTWARE\Policies` and blocks forced browser extension installations via Group Policy. Windows-only (`golang.org/x/sys/windows`).

Module: `github.com/kad/WindowsBrowserGuard`  
Entry: `./cmd/WindowsBrowserGuard/main.go`  
Binary: `WindowsBrowserGuard.exe`

## Package layout
| Package | Role |
|---|---|
| `cmd/WindowsBrowserGuard` | CLI (cobra), config file loading, `runApp` |
| `pkg/monitor` | Registry watch loop, diff, policy enforcement |
| `pkg/registry` | Low-level registry read/write/delete via `advapi32` syscalls |
| `pkg/telemetry` | OTel traces + logs + metrics; `Printf`/`Println` wrappers |
| `pkg/admin` | Admin check, privilege elevation |
| `pkg/detection` | Extension ID detection logic |
| `pkg/buffers` | Reusable byte/name buffer pools |
| `pkg/pathutils` | Registry path helpers |

## Key conventions
- **Logging**: always use `telemetry.Printf`/`Println` (not `fmt`). They fan out to stdout, log file, and OTel logs.
- **Registry close errors**: always wrap defers — `defer func() { _ = windows.RegCloseKey(h) }()`
- **Config file**: `config.json` next to exe (auto-detected via `os.Executable`). Fields: `OTLPEndpoint`, `OTLPHeaders`, `LogPath`, `DryRun`, `Quiet`. CLI flags override.
- **OTLP URL schemes**: `grpc://` `grpcs://` `http://` `https://` — parsed in `pkg/telemetry/endpoint.go`.
- **Lint**: `golangci-lint run ./...` must pass. Config in `.golangci.yml`. gosec G103/G115/G204/G302/G304 are excluded (intentional Windows syscall usage).
- **Build**: `.\build.ps1` (clean → vet → lint → build).

## Error handling
- Return errors up; log with `telemetry.Printf` at call sites.
- Ignore unchecked returns with `_ =` or `_, _ =` (not blank `//nolint`).
- `gosec` noisy rules excluded in `.golangci.yml` — do not add per-line `//nolint` for G103/G115.

## Installation / task scheduler
- `Install.ps1` self-elevates, writes `config.json`, registers Task Scheduler action:  
  `WindowsBrowserGuard.exe --config="<install-path>\config.json"` as SYSTEM.
- No PowerShell wrapper — the exe is executed directly.
- Log path default: `C:\ProgramData\WindowsBrowserGuard\monitor.log`.

## CI/CD
- `ci.yml`: build + vet (windows-latest), golangci-lint (windows-latest), goreleaser check (ubuntu).
- `release.yml`: goreleaser on `v*` tags — produces ZIP with exe + `Install.ps1` + `config.example.json` + `docs/*.ps1`.
- Go version sourced from `go.mod` via `go-version-file`.
