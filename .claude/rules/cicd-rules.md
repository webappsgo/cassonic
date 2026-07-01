# CI/CD Rules (PART 28)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Create `build-toolchain.yml` for Go/Rust projects (forbidden — they use maintained images)
- Use `golang:alpine` or any other image in CI — must use `casjaysdev/go:latest`
- Pin Actions to tags — always full commit SHA
- Run tests without coverage
- Allow coverage below 60%
- Let security jobs run on push/PR (schedule only — weekly cron)
- Add non-security jobs to the weekly schedule (only security jobs run on schedule)
- Use `gitea.*` context on GitHub — use `github.*`

## CRITICAL - ALWAYS DO

- All CI jobs: `container: image: casjaysdev/go:latest`
- All Go jobs: `CGO_ENABLED: "0"` and `-buildvcs=false`
- Coverage threshold: 60% minimum (fail CI if below)
- Coverage output: `/tmp/${{ github.repository_owner }}/<project>-XXXXXX/coverage.out`
- Security jobs (vuln-check, secret-scan, image-scan): `schedule: cron: '0 6 * * 1'`
- Non-security jobs: `if: github.event_name != 'schedule'`
- All Actions pinned to full commit SHA with version comment
- `concurrency: cancel-in-progress: true`

## Required Workflows (5 files)

| File | Purpose |
|------|---------|
| `.github/workflows/ci.yml` | Lint, test (60% coverage), build, vuln-check |
| `.github/workflows/release.yml` | Build binaries for all 8 platforms, GitHub Release |
| `.github/workflows/beta.yml` | Same as release but for beta branch |
| `.github/workflows/daily.yml` | Nightly build, optional pre-release |
| `.github/workflows/docker.yml` | Build and push multi-arch Docker image to GHCR |

## SHA-Pinned Actions

| Action | SHA | Version |
|--------|-----|---------|
| `actions/checkout` | `9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0` | v7.0.0 |
| `actions/upload-artifact` | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` | v7.0.1 |
| `actions/download-artifact` | `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c` | v8.0.1 |
| `softprops/action-gh-release` | `718ea10b132b3b2eba29c1007bb80653f286566b` | v3.0.1 |
| `docker/setup-qemu-action` | `06116385d9baf250c9f4dcb4858b16962ea869c3` | v4.1.0 |
| `docker/setup-buildx-action` | `d7f5e7f509e45cec5c76c4d5afdd7de93d0b3df5` | v4.1.0 |
| `docker/login-action` | `650006c6eb7dba73a995cc03b0b2d7f5ca915bee` | v4.2.0 |
| `docker/build-push-action` | `f9f3042f7e2789586610d6e8b85c8f03e5195baf` | v7.2.0 |
| `docker/metadata-action` | `80c7e94dd9b9319bd5eb7a0e0fe9291e23a2a2e9` | v6.1.0 |

## CI Jobs Structure

```yaml
jobs:
  lint:        # go vet + staticcheck (not on schedule)
  test:        # coverage >= 60% (not on schedule)
  build:       # go build needs lint+test (not on schedule)
  vuln-check:  # govulncheck (schedule + push/PR)
```

## Coverage Enforcement Pattern

```yaml
- name: Run tests with coverage
  run: |
    mkdir -p "/tmp/${{ github.repository_owner }}"
    COVDIR=$(mktemp -d "/tmp/${{ github.repository_owner }}/$(basename "${{ github.repository }}")-XXXXXX")
    echo "COVDIR=$COVDIR" >> "$GITHUB_ENV"
    go test -cover -coverprofile="$COVDIR/coverage.out" ./...
- name: Enforce coverage threshold
  run: |
    THRESHOLD=60
    PCT=$(go tool cover -func="$COVDIR/coverage.out" | awk '/^total:/ {gsub("%","",$3); print int($3)}')
    if [ "$PCT" -lt "$THRESHOLD" ]; then
      echo "::error::coverage $PCT% < threshold $THRESHOLD%"
      exit 1
    fi
```

## docker.yml Context (GitHub, not Gitea)

Use `github.*` context — NOT `gitea.*`:
- `github.actor` (not `gitea.actor`)
- `github.ref` (not `gitea.ref`)
- `github.server_url` (not `gitea.server_url`)
- `github.repository` (not `gitea.repository`)
- `secrets.GITHUB_TOKEN` (not `secrets.GITEA_TOKEN`)

For complete details, see AI.md PART 28
