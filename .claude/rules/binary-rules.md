# Binary Rules (PART 7, 8, 33)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use CGO — always `CGO_ENABLED=0` (pure Go, static binary)
- Embed security databases (GeoIP, blocklists, CVE) — download at runtime
- Run `go build` or `go test` on the host machine
- Ship multiple binaries when one suffices
- Build without `-s -w` strip flags for releases
- Use CGO or external C dependencies

## CRITICAL - ALWAYS DO

- Build with `CGO_ENABLED=0` always — pure Go static binary
- Embed assets via Go `embed` package (templates, static files)
- Build inside `casjaysdev/go:latest` Docker container
- Use `-buildvcs=false` in all CI/Docker builds
- Use `-s -w -trimpath` ldflags for release builds
- Handle SIGTERM, SIGINT, SIGHUP signals properly
- Create PID file by default
- Auto-create config on first run with safe defaults

## Single Static Binary Requirements

| Requirement | Value |
|-------------|-------|
| Type | Single static binary |
| CGO | NEVER — `CGO_ENABLED=0` |
| Assets | Embedded via `embed` package |
| Templates | `src/server/template/` |
| Static files | `src/server/static/` |
| Build command | `go build -buildvcs=false -trimpath -ldflags "-s -w ..."` |

## External Security Data (NEVER Embedded)

Downloaded at runtime into `{data_dir}/security/`:
- GeoIP databases (ASN, Country, City, WHOIS) — daily updates
- IP/domain blocklists — daily updates
- CVE databases — daily updates

## CLI Binary (`cassonic-cli`)

Located at `./src/client/` — separate main package for admin/management CLI.

For complete details, see AI.md PART 7, 8, 33
