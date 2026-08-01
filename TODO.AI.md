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
