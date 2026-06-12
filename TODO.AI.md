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
- [x] man page + shell completions (triple sync) — man/cassonic.1, completions/_cassonic_completions.bash, completions/_cassonic_completions.zsh
- [x] --cache flag added to src/main.go (spec PART 8)
- [x] --shell flag added with completions/init subcommands (spec PART 8)
- [x] --backup-dir renamed to --backup in src/main.go; deprecated alias kept
- [x] --service start note already correct (no change needed)
- [x] Makefile GO_DOCKER image fixed: golang:alpine → $(REGISTRY):build

---

## ACTIVE TASKS

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
