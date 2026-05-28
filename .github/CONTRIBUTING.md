# Contributing to cassonic

Thank you for your interest in contributing to cassonic.

## Local Setup

```bash
# Clone the repo
git clone https://github.com/local/cassonic
cd cassonic

# Build (requires Docker — Go runs inside container only)
make dev

# Run unit tests
make test

# Build for all platforms
make build
```

All builds run inside Docker. Do not run `go build` or `go test` directly on your host machine.

## Branch and PR Workflow

- Base branch: `main`
- Create a feature branch: `git checkout -b feat/your-feature`
- Keep commits focused; one logical change per commit
- Open a pull request against `main`
- All CI checks must pass before merge

## Code Standards

- Go code must be `gofmt`-formatted before committing
- `CGO_ENABLED=0` always — pure static binary
- All boolean parsing via `config.ParseBool()`, never `strconv.ParseBool()`
- No inline YAML comments — comments above the setting only
- No hardcoded machine-specific values (hostname, IP, memory, CPU count)
- Parameterized DB queries always — no string-interpolated SQL
- Passwords: Argon2id only. Tokens: SHA-256 hash only. Never plaintext.

## Tests

- Add or update `*_test.go` unit tests for any changed package logic
- Add or update `./tests/*.sh` integration tests for any changed endpoint or behavior
- `make test` must pass with no regressions

## Documentation

- Update `docs/` pages for any user-facing, admin-facing, or operator-facing changes
- API changes require updating `docs/api.md`
- Configuration changes require updating `docs/configuration.md`

## Security Issues

Do NOT open public issues for security vulnerabilities. See [SECURITY.md](SECURITY.md) for the private reporting process.

## Commit Messages

Use the format: `{emoji} Title (≤64 chars) {emoji}`

Emoji guide: ✨ feat · 🐛 fix · 📝 docs · 🎨 style · ♻️ refactor · ⚡ perf · ✅ test · 🔧 chore · 🔒 security
