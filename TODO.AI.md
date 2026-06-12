# TODO.AI.md — cassonic remaining tasks

Bootstrap completed: 2026-05-28
Last updated: 2026-06-10

---

## COMPLETED

- [x] All audit findings resolved (2026-06-04)
- [x] GitHub Actions workflows — ci.yml, release.yml, build-toolchain.yml, docker.yml, beta.yml, daily.yml
- [x] Integration test scripts — tests/run_tests.sh, tests/docker.sh, tests/incus.sh (+ additional scripts)
- [x] Ampache shares, preferences, social — fully implemented
- [x] IDEA.md compliance — fixed structure, removed HOW details, removed extra top-level sections
- [x] CLAUDE.md current state — updated

---

## ACTIVE TASKS

### [ ] man page and shell completions (triple sync)
Read: binary-rules.md, AI.md PART 7, 8

Scope: `man/cassonic.1`, `completions/_cassonic_completions.bash`, `completions/_cassonic_completions.zsh`
- Generate man page from the flag set in `src/main.go`
- Generate bash completion script
- Generate zsh completion script

### [ ] --service start help note
Read: AI.md PART 24

Scope: `src/main.go` service flag handler or wherever `--service --help` output is built
- Add note to `--service --help` output that `start` defers to the host init system
- Document why: cassonic binary does not manage the init system directly

### [ ] MkDocs content completeness
Read: AI.md PART 30, testing-rules.md

Scope: `docs/*.md`
- Files exist but may be skeletal — verify all required sections are present
- Required: index.md, installation.md, configuration.md, api.md, cli.md, admin.md, security.md, integrations.md, development.md
- Every feature affecting operators/admins/integrators must be documented

### [ ] Expand unit test coverage
Read: AI.md PART 29

Scope: `src/**/*_test.go`
- Current tests: src/server/handler/subsonic/subsonic_test.go, src/server/handler/api/health_test.go, src/server/store/store_test.go
- Goal: 60% coverage minimum; `make test` must pass
- Priority: config package, paths package, crypto service, backup service, scheduler jobs
