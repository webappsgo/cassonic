# Makefile Rules (PART 26)

Read: AI.md PART 26

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## SIX TARGETS ONLY. DO NOT ADD MORE.
| Target | Purpose |
|--------|---------|
| `dev` | Quick build to temp dir |
| `local` | Production test build to binaries/ |
| `build` | All 8 platforms to binaries/ |
| `test` | Unit tests with coverage |
| `release` | Release with source archive |
| `docker` | Build and push container |
| `clean` | Remove binaries/ and releases/ |

## RULES
- PROJECTNAME/PROJECTORG inferred from git — NEVER hardcoded
- VERSION from release.txt > env fallback
- Go always via Docker (GO_DOCKER macro)
- Cache dirs: GODIR, GOCACHE, GOMODCACHE
- Always `@mkdir -p` before Docker mount targets
- LDFLAGS embeds Version, CommitID, BuildDate, OfficialSite

## NEVER USE MAKEFILE IN CI/CD
CI/CD uses explicit go build commands, not make targets.

---
For complete details, see AI.md PART 26
