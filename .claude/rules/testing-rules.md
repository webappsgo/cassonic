# Testing Rules (PART 29, 30, 31)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Run `go build`, `go test`, or any Go binary on the host machine
- Use project directory for runtime/test data — source code only
- Use `docker-compose.yml` or `docker-compose.dev.yml` as AI — human only
- Use bare `mktemp -d` without org/project prefix
- Hardcode `/tmp` directly — always `${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX`
- Run systemctl, iptables, mount, or reboot on host — always inside container/VM
- Put non-docs files in `docs/` directory
- Hardcode any human-readable string — every string must use a translation key
- Error or crash on unsupported `--lang` value — silently fall back to English

## CRITICAL - ALWAYS DO

- Test coverage: 60% minimum before commit or CI pass
- All testing inside `casjaysdev/go:latest` container
- Temp dirs: `/tmp/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX/`
- AI docker testing: `docker-compose.test.yml` copied to temp dir, never in-place
- Clean up temp dirs after use
- Host-affecting ops (reboot, network config, service install): use Incus/VM/container

## Testing (PART 29)

### Temp Directory Structure

```
/tmp/{project_org}/{internal_name}-XXXXXX/
├── volumes/
│   ├── config/
│   └── data/
└── coverage.out  (via $COVDIR)
```

### AI Docker Compose Rules

| File | AI Usage |
|------|----------|
| `docker-compose.yml` | ❌ FORBIDDEN (human/production only) |
| `docker-compose.dev.yml` | ❌ FORBIDDEN (human/dev only) |
| `docker-compose.test.yml` | ✅ ONLY (copy to temp dir first) |

### Host System Safety

| Test Need | Where to Run |
|-----------|-------------|
| systemd service install | `incus exec test-cassonic -- systemctl ...` |
| Firewall integration | `docker run --cap-add=NET_ADMIN ...` |
| Network interface | `ip netns exec {ns} ...` or Incus |
| Package install | Inside build/test container |
| Reboot behavior | `incus restart test-cassonic` |
| Filesystem/mount ops | Incus or VM |

## Documentation (PART 30)

- Every project must have ReadTheDocs documentation
- `docs/` is ONLY for MkDocs/ReadTheDocs files — no source code
- Engine: MkDocs with Material theme
- Theme: dark/light/auto switching (see frontend-rules.md)
- RTD URL: `https://{project_org}-{project_name}.readthedocs.io` (or custom)

### Required docs/ files

| File | Required |
|------|:--------:|
| `docs/index.md` | ✓ |
| `docs/installation.md` | ✓ |
| `docs/configuration.md` | ✓ |
| `docs/api.md` | ✓ |
| `docs/admin.md` | ✓ |
| `docs/security.md` | ✓ |
| `docs/integrations.md` | ✓ |
| `docs/development.md` | ✓ |
| `docs/requirements.txt` | ✓ |
| `mkdocs.yml` | ✓ (project root) |
| `.readthedocs.yaml` | ✓ (project root) |

## I18N & A11Y (PART 31)

### Core Rules

- Every human-readable string MUST use a translation key — no hardcoded text
- Default language: English (`en`)
- Fallback chain: `?lang=` query param → `lang` cookie → `Accept-Language` → `en`
- `?lang=` sets cookie (1 year), persists across requests
- Missing key: fall back to English silently
- Unsupported language: fall back to English silently — NEVER error or crash
- Build-time validation: all languages must have same keys as `en.json`
- UTF-8 everywhere: files, database, HTTP responses

### Supported Languages (all binaries: server, CLI, agent)

| Code | Language | Direction |
|------|----------|-----------|
| `en` | English | ltr |
| `es` | Spanish | ltr |
| `zh` | Chinese (Mandarin) | ltr |
| `fr` | French | ltr |
| `ar` | Arabic | rtl |
| `de` | German | ltr |
| `ja` | Japanese | ltr |

### What Gets Translated (everything humans read)

- Web frontend, admin panel, API error messages
- Swagger/OpenAPI descriptions, email templates
- Server CLI output, client CLI output, agent output
- Health page, cookie consent, privacy/terms pages

For complete details, see AI.md PART 29, 30, 31
