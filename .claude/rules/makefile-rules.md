# Makefile Rules (PART 26)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Add Makefile targets beyond the 6 core targets
- Hardcode PROJECTNAME or PROJECTORG (always infer from git remote)
- Run `go build` or `go test` on host — always inside `casjaysdev/go:latest`
- Use `/tmp` root directly — always `/tmp/{project_org}/{internal_name}-XXXXXX/`
- Create `release.txt` with anything except a semantic version
- Add `v` prefix to text versions (dev, beta, daily) — ONLY numeric semver gets `v`

## CRITICAL - ALWAYS DO

- Infer PROJECTNAME/PROJECTORG from `git remote get-url origin`
- Use `$PWD` (not `$(pwd)`) in docker `-v` flags (static analysis)
- Use `$(PWD)` (Makefile variable) inside Makefile rules
- Set `-e GOFLAGS=-buildvcs=false` in all docker `go build`/`go test` invocations
- Output coverage/test data to `/tmp/${PROJECTORG}/${PROJECTNAME}-XXXXXX/`
- Use `casjaysdev/go:latest` as build container image

## The 6 Core Targets (NO MORE)

| Target | Purpose | Output |
|--------|---------|--------|
| `dev` | Quick dev build | `${TMPDIR}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX/` |
| `local` | Production test build | `binaries/` (with version) |
| `build` | Full release (8 platforms) | `binaries/` |
| `test` | Run unit tests | Coverage report in temp dir |
| `release` | Release with source archive | `releases/` |
| `docker` | Build and push container | `$REGISTRY` |

## Version Rules

- Source: `release.txt` file (single-line `MAJOR.MINOR.PATCH`) > env `VERSION` > `devel`
- Version tag `v` prefix: ONLY for numeric semver (`0.2.0` → `v0.2.0`)
- Text versions NEVER get `v`: `dev` stays `dev`, `beta` stays `beta`, `daily` stays `daily`
- Timestamps never get `v`: `20251218` stays `20251218`

## Binary Naming

Pattern: `{project_name}[-type]-{os}-{arch}[.exe]`

| Binary | Local | Dist (linux/amd64) |
|--------|-------|---------------------|
| Server | `cassonic` | `cassonic-linux-amd64` |
| CLI | `cassonic-cli` | `cassonic-cli-linux-amd64` |

## Build Matrix (8 platforms)

| OS | Architectures |
|----|---------------|
| Linux | amd64, arm64 |
| macOS (Darwin) | amd64, arm64 |
| Windows | amd64, arm64 |
| FreeBSD | amd64, arm64 |

## Makefile Pattern

```makefile
PROJECTNAME := $(shell git remote get-url origin 2>/dev/null | sed -E 's|.*/([^/]+)(\.git)?$$|\1|' || basename "$$(pwd)")
PROJECTORG  := $(shell git remote get-url origin 2>/dev/null | sed -E 's|.*/([^/]+)/[^/]+(\.git)?$$|\1|' || basename "$$(dirname "$$(pwd)")")
VERSION     ?= $(shell cat release.txt 2>/dev/null || echo "devel")
BUILD_DATE  := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT_ID   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "N/A")
GO_DOCKER   := docker run --rm -v $PWD:/app -w /app -e CGO_ENABLED=0 -e GOFLAGS=-buildvcs=false casjaysdev/go:latest
```

For complete details, see AI.md PART 26
