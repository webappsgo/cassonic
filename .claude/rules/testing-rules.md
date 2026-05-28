# Testing Rules (PART 29, 30, 31)

Read: AI.md PART 29, 30, 31

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## TESTING (PART 29)

### CRITICAL - NEVER DO
- Run builds/tests/binaries directly on host machine — containers only
- Use project directory for test/runtime data — temp dirs only
- Run `docker compose up` with docker-compose.yml or docker-compose.dev.yml (human-only files)
- Mount project directory paths as volumes
- Use bare `/tmp/` — must use `/tmp/{project_org}/{internal_name}-XXXXXX/` structure

### CRITICAL - ALWAYS DO
- All builds in Docker (golang:alpine); all binary execution in containers
- Temp dir pattern: `/tmp/{project_org}/{internal_name}-XXXXXX/` (always includes org prefix + random suffix)
- AI testing: use docker-compose.test.yml ONLY, copied to temp dir, run from temp dir
- Both test types REQUIRED: Go unit tests (*_test.go) AND integration tests (./tests/*.sh)
- 60% minimum coverage threshold (CI enforces; aim for 100%)

### TEST TYPES
| Type | Location | Purpose |
|------|----------|---------|
| Go unit tests | *_test.go | Package logic, function coverage, no server needed |
| Integration tests | ./tests/*.sh | Full running server, real HTTP, endpoint coverage |

### REQUIRED SCRIPTS
- tests/run_tests.sh, tests/docker.sh, tests/incus.sh (minimum required)

### CONTENT NEGOTIATION TESTING (ALL ROUTES REQUIRED)
- Frontend routes: test with `Accept: text/html` AND `Accept: text/plain`
- API routes: test with `Accept: application/json` AND `Accept: text/plain`
- .txt endpoints: test robots.txt, /.well-known/security.txt, API endpoint.txt extension

### HOST SYSTEM SAFETY IN TESTS
- systemd tests → incus exec
- firewall tests → docker --cap-add=NET_ADMIN
- network tests → ip netns exec or Incus
- Never run systemctl/iptables/mount/package installs on host

## READTHEDOCS (PART 30)
- docs/ is ONLY for MkDocs/ReadTheDocs files — NEVER source code or scripts there
- Required files: index.md, installation.md, configuration.md, api.md, cli.md, admin.md, security.md, integrations.md, development.md, requirements.txt
- docs/ must cover browser, admin, API, configuration, and integration surfaces
- Every feature affecting operators/admins/integrators/users MUST be in docs/

## I18N & A11Y (PART 31)
- Every human-readable string MUST be translatable — 100% coverage, no hardcoded UI text
- Supported: en, es, zh, fr, ar (rtl), de, ja — ALL binaries (server, CLI, agent)
- Language fallback chain: ?lang= param (sets cookie) → lang cookie → Accept-Language header → en default
- Missing translation key → fall back to English; unsupported lang → silent fallback to English
- Build-time check: all languages must have same keys as en.json
- Locale files: src/common/i18n/locales/{lang}.json
- WCAG AA minimum (4.5:1 normal text, 3:1 large text, 3:1 UI components)
- Never convey information by color alone
- ARIA roles: landmark elements, aria-live for dynamic content, aria-label for controls
- Skip link at top of each page (keyboard navigation)
- Focus management: modal open → first focusable; modal close → trigger element

---
For complete details, see AI.md PART 29, 30, 31
