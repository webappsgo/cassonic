# Project Audit

Started: 2026-07-31

Fixed items are recorded under "Completed this run". Unchecked items below are
OUTSTANDING — they need a design decision or feature-sized work and were
deliberately not auto-fixed during the audit.

## Pass 1: Security (outstanding)

- [ ] icecast: source-password encryption-at-rest is incomplete. `decryptOrPlaintext`
      (src/server/service/icecast/source.go) returns `""` for any `enc:`-prefixed value
      and is always called with a nil key; there is no encrypt-on-save path and the
      `enc:` format does not match `crypto.Encrypt` output. IDEA.md + PART 11 require
      credentials encrypted at rest. Fix = thread `crypto.DeriveKey` through
      NewManager -> Connect and define the on-disk format; feature-sized, touches public
      constructors, so flagged rather than half-implemented.
- [ ] csrf: PART 11 mandates CSRF protection — token layer (double-submit HMAC via
      `csrf_token_secret`), `SameSite=Strict` cookies, and `Sec-Fetch-*` validation on
      state-changing requests. None is implemented: no CSRF code exists and session
      cookies use `SameSiteLaxMode` (src/server/handler/web/web.go). Requires CSRF
      middleware + secret storage; feature-sized.

## Pass 5: Spec Compliance (outstanding)

- [ ] config: env-var config overrides are not implemented in the binary. PART 5
      specifies the binary reads DEBUG, ENABLE_TOR, DOMAIN, CONFIG_DIR, DATA_DIR, PORT,
      ADDRESS, MODE directly (e.g. `os.Getenv("DEBUG")`, `os.Getenv("ENABLE_TOR")`).
      Currently these work only via docker/rootfs entrypoint.sh flag-translation, not for
      a bare binary. docs/configuration.md correctly describes the intended behavior; the
      Go code is the gap. Config-precedence layer needed (env > flag > file per PART 5/12).
- [ ] makefile: GO_DOCKER still diverges from PART 26 beyond the fixed items — uses
      `$(REGISTRY):build` image + `/build` workdir + no `--memory`/`--cpus` limits, vs
      spec's `casjaysdev/go:latest` + `/app` + resource limits. Project deliberately uses
      its own build image and CI is green; left as-is pending user decision.

## Pass 4: Documentation (outstanding)

- [ ] docs/cli.md documents CASSONIC_SERVER / CASSONIC_COLOR / CASSONIC_FORMAT env vars
      that src/client never reads (only CASSONIC_TOKEN is honored). Fix docs or wire vars.
- [ ] api: the inline OpenAPI spec (`openAPISpec()` in src/server/server.go) covers ~12 of
      the ~39 native endpoints documented in docs/api.md. Expand the machine-readable spec.
- [ ] config: exported struct types in src/config/config.go (Config, ServerConfig, etc.)
      lack doc comments while their fields and funcs are documented (minor).

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
</content>
</invoke>
