# Binary Rules (PART 7, 8, 33)

Read: AI.md PART 7, 8, 33

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Use CGO (CGO_ENABLED=0 always)
- Skip platforms — build all 8
- Use `-musl` suffix in binary names
- Build on host — always via Docker (make dev/local/build)
- Put binaries outside `binaries/` directory

## CRITICAL - ALWAYS DO
- Single static binary with embedded assets (go:embed)
- CGO_ENABLED=0 always
- 8 platforms: linux/darwin/windows/freebsd x amd64/arm64
- Binary naming: `cassonic-{os}-{arch}` (windows adds .exe)
- CLI binary: `cassonic-cli` (REQUIRED on all projects)
- Build source always `./src`

## CLI FLAGS (NON-NEGOTIABLE)
```
--help / -h
--version / -v
--mode {production|development}
--config {dir}
--data {dir}
--log {dir}
--pid {file}
--address {listen}
--port {port}
--baseurl {path}
--debug
--status
--service {start,restart,stop,reload,--install,--uninstall,--disable,--help}
--daemon
--maintenance {backup,restore,update,mode,setup,--help}
--update [check|yes|branch {stable|beta|daily}]
```

Only `-h` and `-v` may have short flags.

## BUILD INFO VARS (required in main.go)
```go
var (
    Version      = "dev"
    CommitID     = "unknown"
    BuildDate    = "unknown"
    OfficialSite = ""
)
```

---
For complete details, see AI.md PART 7, 8, 33
