# TODO

- client: `--format {table|json|plain}` and `CASSONIC_FORMAT` were documented in
  docs/cli.md but never implemented — the client only has a boolean `--json` flag
  (`src/client/main.go`, `src/client/commands.go`) with no tri-state format system
  or `plain` renderer. Feature-sized (touches ~32 call sites in commands.go);
  docs/cli.md was corrected to describe only what's implemented (`--json`) pending
  this work. Implement a real `--format` flag + `plain` output mode, or drop the
  idea permanently, per PART 33.
- server: `DOMAIN` env var and the full `{fqdn}` resolution chain (PART 5 ->
  "Environment Variables" -> "URL Variable Resolution") are not implemented at
  all — no code resolves Reverse Proxy Headers -> `DOMAIN` -> `os.Hostname()` ->
  `$HOSTNAME` -> Global IP -> `localhost`. This affects email from-address
  generation, baseurl construction, and any templated `{fqdn}` value, not just
  TLS cert naming (which already has its own `--tls-domain` flag). Feature-sized:
  needs a shared fqdn-resolution helper used by src/server/service/email and
  wherever baseurl is computed. `ENABLE_TOR` was deliberately NOT added — PART 27
  explicitly states no such flag is needed; Tor auto-enables when the `tor`
  binary is present (src/main.go already does this).
- server: `icecast.NewManager` (src/server/service/icecast/icecast.go) is
  never called anywhere in src/server/server.go or main.go — only in tests.
  The whole Icecast source-relay Manager (mount streaming, source password
  encryption/decryption) is implemented but not wired into the live server
  at startup, so the feature does not actually run. Needs a
  `icecast.NewManager(db, ffmpegMgr, logger, subsonicKey)` call plus whatever
  route/lifecycle wiring PART 20 (or wherever Icecast source relay is
  specified) describes.
