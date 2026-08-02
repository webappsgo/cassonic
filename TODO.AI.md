# TODO

- client: `--format {table|json|plain}` and `CASSONIC_FORMAT` were documented in
  docs/cli.md but never implemented — the client only has a boolean `--json` flag
  (`src/client/main.go`, `src/client/commands.go`) with no tri-state format system
  or `plain` renderer. Feature-sized (touches ~32 call sites in commands.go);
  docs/cli.md was corrected to describe only what's implemented (`--json`) pending
  this work. Implement a real `--format` flag + `plain` output mode, or drop the
  idea permanently, per PART 33.
- admin: `SaveConfig` (`src/server/handler/admin/admin.go`) is a stub — it
  redirects with `flash=saved` but never parses the POSTed form or persists
  any value back to `server.yml`. This predates the DOMAIN/{fqdn} work; the
  new `domain`/`trusted_proxies` fields were added to `config.html` for
  visibility only (matching the existing Port/Mode/Debug display-only
  pattern) and are not yet actually saveable. Per config-rules.md every
  server.yml setting must be live-reloadable and admin-UI editable — almost
  none of Auth/Scanner/Icecast/Scrobble/FFmpeg/Email/Features/Web/Paths is
  exposed in the admin panel at all. Needs a real form-parse-and-persist
  implementation (write via `config.Save`, live-reload in-memory `cfg`)
  covering every section, per PART 12/PART 17.
- config: `(*Config).Validate()` (`src/config/config.go`) is dead code — no
  caller invokes it anywhere outside tests (`src/main.go` loads/saves config
  but never validates it). A malformed `server.yml` (e.g. bad CIDR in
  `trusted_proxies`, invalid `mode`) is silently accepted at startup. Wire
  `cfg.Validate()` into `src/main.go` after config load/defaults/env/flag
  overrides, exit(1) with a clear message on failure.
- docs: `docs/configuration.md`'s "Full server.yml Reference" block has
  drifted from `src/config/config.go` — it documents `baseurl` (actual key:
  `base_url`), `timezone` (no such field exists), `library.paths`/
  `library.extensions` (actual: `paths.music`, no extensions field),
  `database.dir`/`database.valkey_url` (actual: `database.path`, no Valkey
  support), and a top-level `smtp:` section (actual: `email:`). Predates the
  DOMAIN/{fqdn} work. Needs a full pass reconciling the doc against the real
  `Config` struct, per testing-rules.md "keep docs/ in sync with the app".
- build: pre-existing go-lint violations, unrelated to the DOMAIN/{fqdn} work
  (neither file was touched by it): `docker/Dockerfile` uses `golang:alpine`
  instead of `casjaysdev/go:latest`, and its two `go build` calls (lines 20,
  24) are missing inline `-buildvcs=false` and `-trimpath` (also missing
  from LDFLAGS on lines 21/25); `.gitlab-ci.yml` has four `go build` calls
  (lines 22, 23, 82, 85) missing inline `-buildvcs=false` and two LDFLAGS
  blocks (lines 21, 75) missing `-trimpath`. Fix Dockerfile/CI build
  commands to match the Makefile's `GO_DOCKER`/`LDFLAGS` pattern, per
  docker-rules.md/go_conventions.md.
