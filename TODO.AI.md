# TODO.AI.md — cassonic remaining tasks

Bootstrap completed: 2026-05-28
Last updated: 2026-06-08
Last audit: 2026-06-04 (AUDIT.AI.md — findings resolved below)

---

## RESOLVED — Audit findings (2026-06-04)

All HIGH and MEDIUM findings confirmed fixed in code:

- [x] Register `-h` and `-v` short flags in `src/main.go` — `flag.BoolVar` aliases present
- [x] Remove empty `src/admin/` directory — directory deleted
- [x] Move 5 inline comments above code lines in `src/server/service/tags/writer_ogg.go:260-264` — done
- [x] `cassonic-agent` reference in `CLAUDE.md` — marked optional and not yet scaffolded (per spec, agent is optional)
- [x] `--update branch=` dead variable — `checker.WithBranch(branch)` now used
- [x] Plural Go package dir exceptions documented in `IDEA.md` § Go package directory naming exceptions
- [x] `math/rand` in icecast confirmed non-security jitter — explanatory comment added
- [x] Ampache shares (`share_create`, `share_edit`, `share_delete`) — implemented using share store
- [x] Ampache preference create/edit/delete stubs — replaced with proper error 4710
- [x] Ampache social stubs (`toggleFollow`, comments) — comments cleaned up, proper empty responses

LOW items remaining:
- [ ] Update `CLAUDE.md` "Current Project State" section
- [ ] `--service start` guidance — confirm intentional and add note to `--service --help` output
- [ ] Generate `man/cassonic.1` and `completions/_cassonic_completions.bash` for triple sync

---

## ACTIVE TASKS

### [ ] man page and shell completions (triple sync)
Read: binary-rules.md

Scope: `man/cassonic.1`, `completions/_cassonic_completions.bash`, `completions/_cassonic_completions.zsh`
- Generate man page from the flag set in `src/main.go`
- Generate bash completion script
- Generate zsh completion script

### [ ] --service start documentation
Read: AI.md PART 24

Scope: `src/main.go` service flag handler
- Add note to `--service --help` output that `start` defers to the host init system
- Document why: cassonic binary does not manage the init system directly

### [ ] CLAUDE.md current state update
Scope: `CLAUDE.md` § Current Project State
- Update last updated date
- Update current task description
- List implemented subsystems accurately

---

## CI/CD (PART 28)

### [ ] GitHub Actions workflows
Read: AI.md PART 28, cicd-rules.md

Scope: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/workflows/build-toolchain.yml`, `.github/workflows/docker.yml`
- All workflows missing; need to be created
- `ensure-build-image` pre-flight job gates all downstream jobs
- Pin all third-party actions to full commit SHA
- CGO_ENABLED=0 in all build steps
- truffleHog secret scan
- govulncheck when go.sum present
- 8-platform release build
- Renovate config present (renovate.json exists)

---

## TESTING (PART 29)

### [ ] Integration test scripts
Read: AI.md PART 29, testing-rules.md

Scope: `tests/run_tests.sh`, `tests/docker.sh`, `tests/incus.sh`
- Minimum required scripts not present — only directory exists
- Tests must run via `docker-compose.test.yml` in temp dir (`/tmp/local/cassonic-XXXXXX/`)
- Both content types required per route: `Accept: text/html` and `Accept: text/plain` for frontend; `Accept: application/json` and `Accept: text/plain` for API
- 60% minimum coverage threshold

### [ ] Expand unit test coverage
Read: AI.md PART 29

Scope: `src/**/*_test.go`
- Current tests: `src/server/handler/subsonic/subsonic_test.go` (1001 lines), `src/server/handler/api/health_test.go` (612 lines), `src/server/store/store_test.go`
- Goal: 60% coverage minimum; `make test` must pass
- Priority: config package, paths package, crypto service, backup service, scheduler jobs

---

## DOCUMENTATION (PART 30)

### [ ] MkDocs content completeness
Read: AI.md PART 30, testing-rules.md

Scope: `docs/*.md`
- Files exist but may be skeletal — verify all required sections are present
- Required: index.md, installation.md, configuration.md, api.md, cli.md, admin.md, security.md, integrations.md, development.md
- Every feature affecting operators/admins/integrators must be documented
