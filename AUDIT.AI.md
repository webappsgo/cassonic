# Project Audit

Started: 2026-06-04
Mode: Analysis-only (no fixes applied)
Auditor: Claude (audit skill)

Scope: full spec compliance audit of cassonic against AI.md, IDEA.md, CLAUDE.md, and `.claude/rules/*.md`.

## Summary

The project is largely compliant. No critical Top-19 violations were found (no bcrypt, no CGO, no Dockerfile at root, no strconv.ParseBool, no SELECT *, no client-side rendering, GeoIP uses maxminddb-golang correctly, AES-256-GCM with Argon2id present in backup/crypto, all required scheduler tasks registered, all 7 locales present with identical key counts, all required Makefile targets present, all key endpoints exist (`/health`, `/version`, `/swagger/`, `/graphql/`, `/metrics`, `/server/about`, `/server/help`), Tor auto-enable logic present, ENTRYPOINT/STOPSIGNAL/EXPOSE 80 correct in `docker/Dockerfile`).

The findings below are all MEDIUM/LOW severity except one HIGH involving the documented `-h` / `-v` short flags not being registered, and one HIGH involving an empty `src/admin/` directory committed to the tree.

---

## Pass 1: Security

No CRITICAL findings. Argon2id is used for passwords (see `src/server/service/crypto/crypto.go`, `src/server/handler/api/auth.go`, `src/server/handler/ampache/users.go`, `src/server/handler/subsonic/crypto.go`). SHA-256 is used for token hashing. AES-256-GCM with Argon2id key derivation is used for backup encryption (`src/server/service/backup/backup.go:3`, `src/server/service/crypto/crypto.go:23`). No hardcoded secrets, no SQL injection patterns, no `strconv.ParseBool` usage. `crypto/rand` used everywhere security-relevant; `math/rand` used only for benign jitter (`src/server/service/icecast/mount.go:6,272`).

### 1.1 [MEDIUM] `math/rand` in icecast reconnect backoff — non-security, but flag for review
- File: `src/server/service/icecast/mount.go:272`
- What: `j := rand.Intn(i + 1)` using `math/rand`
- Spec: PART 11 requires CSPRNG for security-sensitive values
- Actual: jitter in reconnect loop — not security-sensitive, acceptable per spec exception, but Go 1.20+ no longer requires seeding so confirm intent

---

## Pass 2: Code Quality

### 2.1 [MEDIUM] Inline comments on code lines
Project rule: "Comments always ABOVE, never inline". Five instances found, all in one file:
- `src/server/service/tags/writer_ogg.go:260` — `out.WriteByte(0)            // stream structure version`
- `src/server/service/tags/writer_ogg.go:261` — `out.WriteByte(headerType)   // header type flag`
- `src/server/service/tags/writer_ogg.go:262` — `out.Write(original[6:14])  // granule position (8 bytes)`
- `src/server/service/tags/writer_ogg.go:263` — `out.Write(original[14:18]) // bitstream serial number (4 bytes)`
- `src/server/service/tags/writer_ogg.go:264` — `out.Write(original[18:22]) // page sequence number (4 bytes)`

Fix: move each comment to its own line above the code.

### 2.2 [LOW] Empty source directory committed
- Directory: `src/admin/` exists but contains no files
- Admin code actually lives at `src/server/handler/admin/`
- Action: delete the empty `src/admin/` directory

### 2.3 [LOW] `--update branch=` handling silently no-ops
- File: `src/main.go:511-516`
- Code: when `branch=X` is supplied the function prints the message then `return`s; later `_ = branch` is dead. The branch is never actually persisted or used by `checker.CheckLatest()`
- Spec (PART 23): `--update branch=stable|beta|daily` should select a release channel
- Severity: LOW — feature stubbed, does not crash

---

## Pass 3: Logic and Correctness

No data races, deadlocks, off-by-one, or nil deref issues found in spot checks. No `panic("not implemented")` in production code.

### 3.1 [LOW] `--update branch=` early-return swallows subsequent actions
- File: `src/main.go:511-516`
- When user runs `--update branch=stable`, the program returns immediately, never performs `check` or `yes`. If the operator expects "set channel then check", this is wrong; if they expect "set channel only", it is incomplete because the channel is never persisted to config
- Recommend: persist branch to `cfg.Update.Branch` (or equivalent) and `config.Save`

---

## Pass 4: Documentation Completeness

### 4.1 [LOW] CLAUDE.md exceeds loader length guideline
- File: `CLAUDE.md` is 109 lines
- Guideline (audit pass 4): CLAUDE.md should be a short loader (≤20 lines), not a duplicate spec
- Project-specific: the project's own `.claude/rules/*.md` system imposes this structure, so this is intentional. **Flagging only, no action recommended.**

### 4.2 [MEDIUM] TODO.AI.md has no checkbox tracking
- File: `TODO.AI.md` (1392 lines)
- Issue: contains ~0 `[ ]` and ~0 `[x]` markers — entries are plain bullet lists describing features, not actionable tasks
- Spec: `TODO.AI.md` is meant to track ≥3 pending tasks with checkbox state, items removed/marked done as work completes
- Action: convert feature lists to checkbox tasks OR rename to `FEATURES.AI.md` and create a real TODO.AI.md tracking remaining work (e.g. items in this audit)

### 4.3 [LOW] Triple sync (man pages, completions) not present
- Missing: `man/` directory and `completions/` directory
- The cassonic binary has an interactive `--help`; spec audit pass 4 requires `man/cassonic.1` and `completions/_cassonic_completions.bash`
- Action: generate man page and bash completion from the flag set in `src/main.go`

---

## Pass 5: Spec and Rules Compliance

### 5.1 [HIGH] `-h` and `-v` short flags advertised but not registered
- File: `src/main.go:56-86`
- Spec: PART 8 / binary-rules.md — "Only `-h` and `-v` may have short flags"
- Help text: `printHelp()` advertises `--version / -v`
- Actual code: only `--help` and `--version` long flags are registered via `flag.Bool`. `-h` and `-v` are NOT defined; running `cassonic -v` will fail with "flag provided but not defined: -v"
- Action: add `flag.BoolVar(flagHelp, "h", false, ...)` and `flag.BoolVar(flagVersion, "v", false, ...)`

### 5.2 [HIGH] `src/admin/` empty directory committed
- Directory: `src/admin/`
- Project rule: every committed directory should have purpose; empty source dirs imply unfinished work
- Likely cause: original scaffold intended `src/admin/` for the admin panel, but implementation went to `src/server/handler/admin/`
- Action: `rmdir src/admin`

### 5.3 [MEDIUM] `cassonic-agent` referenced in CLAUDE.md but not implemented
- File: `CLAUDE.md` — "agent = cassonic-agent (optional, runs on remote machines)"
- Code search: zero references to `cassonic-agent` in `src/`, `docker/`, or `Makefile`
- Spec: marked "optional", so absence is allowed
- Action (choose one):
  - Remove the line from CLAUDE.md to avoid implying a non-existent binary
  - OR scaffold `src/agent/` and add build target

### 5.4 [LOW] Plural-form Go package directories
- Go convention from project-rules.md: "singular directory names (handler/, model/, not handlers/, models/)"
- Findings: `src/paths/`, `src/server/metrics/`, `src/common/errors/`, `src/server/service/tags/`
- Note: `paths`, `metrics`, `errors`, and `tags` are conventional Go stdlib-style package names (similar to `os`, `net`, `bytes`). Project rule is strict-singular; recommend either:
  - Renaming to `path/`, `metric/`, `error/`, `tag/` (will break many imports), OR
  - Documenting these as approved exceptions in `IDEA.md` → `### Security decisions & exceptions`

### 5.5 [LOW] CLAUDE.md "Current Project State" is stale
- File: `CLAUDE.md` last section claims: "Last bootstrap: 2026-05-28 · Current task: Initial project scaffolding · Relevant PARTs: 0-7, 26, 27, 28, 30"
- Reality: ~130 Go source files, full music server scaffolded across all PARTs (Subsonic, Ampache, native API, scheduler with 17 jobs, GeoIP, backup with AES-256-GCM, Tor integration, i18n with 7 locales, etc.)
- Action: update to reflect the actual current state

### 5.6 [LOW] IDEA.md missing some now-implemented surfaces
- File: `IDEA.md` — `## Business logic` enumerates Subsonic, Ampache, native API, etc.
- Code present but IDEA.md does not list explicitly: GraphQL endpoint (`src/graphql/`, `/graphql/` route), Swagger UI surface (`/swagger/`), Prometheus metrics surface (`/metrics`)
- These are required-by-spec (PART 13, 14, 15, 21) so not new features — but `IDEA.md` should mention them as part of the API surface

---

## Pass 6: Code Flow Trace

### 6.1 [MEDIUM] `--update branch=` dead variable
- File: `src/main.go:516` — `_ = branch`
- The parsed `branch` value is discarded; it does not influence `checker.CheckLatest()`. See 2.3 / 3.1 above.

### 6.2 [LOW] `flagService` "start" prints guidance instead of acting
- File: `src/main.go:392-394` — `case "start": fmt.Println("cassonic: use your init system...")`
- PART 24 wording can be read either way; if `--service start` is meant to actually start the installed service via `systemctl start cassonic`, this is incomplete
- If intentional (the daemon should be controlled by init), document in `--service --help`

### 6.3 No env-var documentation issues found
- `os.Getenv` is used for `NO_COLOR`, `MODE`, `DEBUG` — all documented in CLAUDE.md / spec
- No undocumented env vars detected

---

## What is NOT broken (verified compliant)

- Password hashing: Argon2id throughout
- Token hashing: SHA-256
- No bcrypt, no `strconv.ParseBool`, no `SELECT *`, no CGO
- Dockerfile location: `docker/Dockerfile` ✔
- ENTRYPOINT, STOPSIGNAL, EXPOSE 80, ENV MODE=development ✔
- GeoIP library: `oschwald/maxminddb-golang` ✔ (not `geoip2-golang`)
- Backup: AES-256-GCM + Argon2id KDF ✔
- Scheduler: all 12 required jobs registered (`ssl_renewal`, `geoip_update`, `blocklist_update`, `cve_update`, `session_cleanup`, `token_cleanup`, `log_rotation`, `backup_daily`, `backup_hourly`, `healthcheck_self`, `tor_health`, `cluster_heartbeat`) plus 5 cassonic-specific jobs
- Metrics: prefixed `cassonic_` ✔
- Endpoints: `/health`, `/api/v1/health`, `/version`, `/api/version`, `/api/v1/version`, `/swagger/`, `/graphql/`, `/metrics`, `/server/about`, `/server/help` all present
- Tor: auto-enable when `tor` binary found ✔
- i18n: all 7 locales (en, es, fr, de, zh, ja, ar), identical key count (65 each) ✔
- Makefile: exactly the 7 spec targets (dev, local, build, test, release, docker, clean), PROJECTNAME/PROJECTORG from git ✔
- No forbidden root files (no CHANGELOG.md, SUMMARY.md, COMPLIANCE.md, .env, root Dockerfile, root docker-compose.yml)
- No forbidden root dirs (no config/, data/, logs/, tmp/, build/, dist/)
- docs/ has all required MkDocs files
- LICENSE.md present; MIT
- No TODO/FIXME/HACK markers in production code (the only "TODO" hits are ID3 frame names in tag parser, not actual TODO comments)

---

## Action Summary (by severity)

HIGH (2):
1. `-h`/`-v` short flags not registered — `src/main.go`
2. Empty `src/admin/` directory committed

MEDIUM (4):
3. Inline comments in `src/server/service/tags/writer_ogg.go` (5 lines)
4. `TODO.AI.md` lacks checkbox tracking
5. `cassonic-agent` referenced in CLAUDE.md but not implemented (decide: scaffold or remove reference)
6. `--update branch=` parses branch but discards it (`src/main.go:516`)

LOW (6):
7. Plural Go package dir names — confirm exception or rename
8. CLAUDE.md "Current Project State" stale
9. IDEA.md does not enumerate GraphQL/Swagger/metrics endpoints
10. CLAUDE.md exceeds 20-line loader guideline (intentional, no action)
11. `math/rand` jitter — confirm non-security intent
12. `--service start` prints guidance only — confirm intent and document
13. Missing `man/cassonic.1` and `completions/_cassonic_completions.bash` (triple sync)
