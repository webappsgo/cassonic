# Project Audit

Started: 2026-07-31

Fixed items are recorded under "Completed this run". Unchecked items below are
OUTSTANDING — they need a design decision or feature-sized work and were
deliberately not auto-fixed during the audit.

## Pass 5: Spec Compliance (outstanding)

- [ ] makefile: GO_DOCKER still diverges from PART 26 beyond the fixed items — uses
      `$(REGISTRY):build` image + `/build` workdir + no `--memory`/`--cpus` limits, vs
      spec's `casjaysdev/go:latest` + `/app` + resource limits. Project deliberately uses
      its own build image and CI is green; left as-is pending user decision.
- [ ] server: `DOMAIN` env var and the full `{fqdn}` resolution chain (PART 5) are not
      implemented — logged as its own item in TODO.AI.md (feature-sized: affects email
      from-address and baseurl generation, not just TLS cert naming).

## Completed this run

- git hygiene: untracked 7 committed build/test/coverage artifacts
  (.buildlog_tmp.txt .buildout.txt .coverage.out .coverage3.out .coverage_subsonic.out
  .testout.txt .vetout.txt) and deleted 16 loose log/coverage files from the worktree.
- gitignore: added *.out, .coverage*, and the build/test txt artifact patterns.
- makefile: removed `-it` from GO_DOCKER (broke non-interactive `make test`/`make build`)
  and added `-e GOFLAGS=-buildvcs=false`; both realign with PART 26.
- SECURITY (HIGH) SQL injection: added ORDER BY column allowlists in ListArtists /
  ListAlbums (src/server/store/music_store.go); the untrusted ?sort= param was
  interpolated straight into the query.
- SECURITY (HIGH) empty auth secret: added config.EnsureSecrets + main.go first-run wiring
  to generate a 32-byte crypto/rand JWTSecret when empty, preventing a publicly
  reproducible AES key over stored Subsonic passwords.
- BUG detached scans: 3 scan goroutines used the cancelled request context; switched to
  context.Background() (ampache/browse.go x2, subsonic/system.go).
- BUG ICY metadata overflow: clamped metadata content to 255 16-byte blocks so long titles
  no longer overflow the single length byte (icecast/source.go).
- SECURITY (HIGH) icecast credential encryption-at-rest: threaded crypto.DeriveKey through
  NewManager -> Connect; added EncryptSourcePass (matches the existing Subsonic-password
  AES-256-GCM pattern) and wired encrypt-on-save into the icecast API handlers
  (src/server/service/icecast/source.go, icecast.go; src/server/handler/api/icecast.go).
- SECURITY (HIGH) CSRF protection: added double-submit cookie CSRF middleware
  (src/server/middleware/csrf.go) — SameSite=Strict token cookie, X-CSRF-Token
  header/form validation, Sec-Fetch-Site cross-site check, Bearer/public/websocket/
  exempt-path bypasses per PART 16; switched session cookies to SameSite=Strict and
  added CSRF-cookie regeneration on login/logout to prevent fixation
  (src/server/handler/web/web.go); wired into the middleware chain (src/server/server.go)
  and config (src/config/config.go: WebConfig/CSRFConfig).
- SPEC COMPLIANCE: env-var config precedence — ADDRESS, PORT, MODE, DEBUG, CONFIG_DIR,
  DATA_DIR now read natively in src/main.go with CLI flag > env var > file > embedded
  default precedence, matching PART 29's "Configuration precedence" table. Also fixed a
  `strconv.ParseBool` direct call to use `config.ParseBool` per PART 5's explicit rule.
  `ENABLE_TOR` intentionally not added — PART 27 states no such flag is needed since Tor
  auto-enables when the `tor` binary is present (already implemented).
- DOC: docs/cli.md corrected to describe only the `--json` flag the client actually reads;
  CASSONIC_FORMAT tracked as a feature request in TODO.AI.md rather than silently dropped.
- DOC: expanded the inline OpenAPI spec (`openAPISpec()` in src/server/server.go) toward
  full coverage of docs/api.md's native endpoints.
- DOC: added missing doc comments to exported types in src/config/config.go.
