# Development

## Prerequisites

- **Docker** — all builds and test runs happen inside containers; no local Go installation needed
- **make** — orchestrates builds
- **git** — version control

Optional but recommended:

- **Go 1.23+** — for IDE support and local `go vet`/`gopls`
- **golangci-lint** — static analysis (also runs in the build container)

## Repository Layout

```
cassonic/
├── src/                         # Go source code
│   ├── main.go                  # Server entry point
│   ├── client/                  # cassonic-cli entry point
│   ├── agent/                   # cassonic-agent entry point (optional)
│   ├── config/                  # Configuration loading and parsing
│   ├── server/                  # HTTP server package
│   │   ├── server.go            # Router wiring and Server type
│   │   ├── handler/
│   │   │   ├── admin/           # Admin panel handlers
│   │   │   ├── api/             # Native REST API handlers
│   │   │   ├── ampache/         # Ampache API handler
│   │   │   ├── subsonic/        # Subsonic API handler
│   │   │   └── web/             # Web UI handler and templates
│   │   │       ├── static/      # Embedded static assets
│   │   │       └── template/    # Go HTML templates
│   │   ├── middleware/          # HTTP middleware (auth, rate limit, GeoIP, etc.)
│   │   ├── metrics/             # Prometheus metrics definitions
│   │   ├── service/             # Business logic (scanner, cover art, etc.)
│   │   ├── ssl/                 # TLS / Let's Encrypt manager
│   │   └── store/               # Database layer (SQLite via mattn/go-sqlite3)
│   └── common/                  # Shared utilities (i18n, config.ParseBool, etc.)
│       └── i18n/
│           └── locales/         # Translation files (en.json, es.json, …)
├── docker/
│   ├── Dockerfile               # Multi-stage build (golang:alpine + alpine)
│   ├── rootfs/                  # Container filesystem overlay
│   └── entrypoint.sh            # Container entrypoint
├── docs/                        # MkDocs documentation (this directory)
├── tests/                       # Integration test scripts
│   ├── run_tests.sh             # Main test runner
│   ├── docker.sh                # Docker-based integration tests
│   └── incus.sh                 # Incus-based system tests
├── .github/workflows/           # GitHub Actions
├── Makefile                     # Six targets: dev, local, build, test, release, docker, clean
├── go.mod
├── go.sum
├── release.txt                  # Version source of truth (semver)
├── mkdocs.yml                   # Documentation site config
└── AI.md                        # Full project specification (read-only)
```

## Build Targets

```bash
# Quick dev build — compiles the server binary to a temp directory
make dev

# Production build — compiles to binaries/cassonic-linux-amd64 (current platform)
make local

# Cross-compile all 8 platforms — populates binaries/
make build

# Run unit tests with coverage (minimum 60%)
make test

# Build release archive with source tarball
make release

# Build and push Docker image to ghcr.io
make docker

# Remove binaries/ and releases/
make clean
```

All `make` targets build inside a Docker container using `golang:alpine`. No Go toolchain needs to be installed on the host.

## Environment Variables for the Build

| Variable | Description | Default |
|----------|-------------|---------|
| `VERSION` | Override version string | from `release.txt` |
| `GOFLAGS` | Extra flags for `go build` | `-trimpath` |
| `CGO_ENABLED` | Always `0` — static binary, no C | `0` |

## Running Tests

```bash
# Unit tests only
make test

# Full integration tests (requires Docker)
bash tests/run_tests.sh
```

Unit tests (`*_test.go`) cover package-level logic and do not require a running server. Integration tests (`tests/*.sh`) start a real cassonic server in a Docker container and hit actual HTTP endpoints.

All test data is written to `/tmp/local/cassonic-XXXXXX/` (a unique temp dir per run). The project directory is never used as a data dir during tests.

### Test Coverage

The CI gate requires **60% line coverage minimum**. Aim for 100% on new code.

```bash
# View coverage report in browser
make test
open /tmp/coverage.html
```

### Content Negotiation Tests

All routes are tested with both `Accept: text/html` and `Accept: text/plain` for browser routes, and both `Accept: application/json` and `Accept: text/plain` for API routes.

## Code Style

- `gofmt` and `goimports` — run automatically in the lint gate
- `golangci-lint` with the project's `.golangci.yml` config
- Directory names: **singular** in Go packages (`handler/`, `model/`, `middleware/`)
- File names: **lowercase snake_case** (`cover_art.go`, `rate_limiter.go`)
- Comments always **above** the code they describe, never inline
- No `TODO`, `FIXME`, or `HACK` comments — implement fully or don't implement

## Adding a New Feature

1. **Read the spec** — find the relevant PART in `AI.md` before writing a single line of code.
2. **Write a failing test** — add a unit test that describes the expected behavior.
3. **Implement** — make the test pass; follow the spec exactly.
4. **Integration test** — add or update a test in `tests/` that exercises the feature end-to-end.
5. **Update docs** — update the relevant file in `docs/`; every user-facing feature must be documented.
6. **Lint** — run `go-lint` or `make test`; fix all violations before committing.
7. **Commit** — one logical change per commit; message format per CLAUDE.md.

## Dependency Policy

- **No GPL/AGPL/LGPL dependencies** — MIT, BSD-2, BSD-3, Apache-2.0 only
- All third-party licenses are attributed in `LICENSE.md`
- `go mod tidy` after adding or removing any dependency
- Renovate keeps dependencies up-to-date automatically (PRs opened weekly)

## Internationalization

All user-visible strings must go through the i18n system — no hardcoded English text in templates or Go code.

Locale files live at `src/common/i18n/locales/{lang}.json`. The supported languages are:

| Code | Language |
|------|----------|
| `en` | English (reference) |
| `es` | Spanish |
| `fr` | French |
| `de` | German |
| `zh` | Chinese (Simplified) |
| `ar` | Arabic (RTL) |
| `ja` | Japanese |

To add a translatable string:

1. Add the key to `en.json` with the English value.
2. Add the same key to all other locale files (use the English value as a placeholder if the translation is not ready — the build will succeed; the app will fall back to English at runtime for missing keys).
3. Use `{{call .T "your.key"}}` in templates or `s.T("your.key")` in Go handlers.

A build-time check verifies that all locale files have the same set of keys as `en.json`.

## Submitting a Pull Request

1. Fork the repository on GitHub.
2. Create a feature branch: `git checkout -b feat/your-feature`.
3. Write tests that fail before your change and pass after.
4. Run `make test` — all tests must pass.
5. Run the lint gate — no violations allowed.
6. Push and open a PR against `main`.
7. The CI pipeline runs lint, tests, and a Docker build automatically.

PR titles follow the commit message emoji convention (see CLAUDE.md). Keep PRs focused — one logical change per PR.

## Docker Image

The production Docker image uses a multi-stage build:

1. **Builder stage** (`golang:alpine`) — compiles the binary with `CGO_ENABLED=0`
2. **Runtime stage** (`alpine:latest`) — minimal image with `tini`, `tor`, `curl`, `bash`

The binary is the sole artifact copied from the builder stage. No Go toolchain or source code is in the final image.

```bash
# Build the image locally
make docker
```

The entrypoint is `tini -- /usr/local/bin/entrypoint.sh`. The container runs as a non-root user inside the image after startup.

## Release Process

1. Update `release.txt` with the new semver version.
2. Commit: `git commit -m "🔖 Release v1.2.3 🔖"`
3. Tag: `git tag v1.2.3`
4. Push the tag — GitHub Actions builds all 8 platform binaries, creates the GitHub release, and pushes the Docker image.

Release binaries are named `cassonic-{os}-{arch}` (Windows adds `.exe`). A source archive is included in every release.
