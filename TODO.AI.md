# TODO

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
- docs: `docs/configuration.md`'s "Full server.yml Reference" block has
  drifted from `src/config/config.go` — it documents `baseurl` (actual key:
  `base_url`), `timezone` (no such field exists), `library.paths`/
  `library.extensions` (actual: `paths.music`, no extensions field),
  `database.dir`/`database.valkey_url` (actual: `database.path`, no Valkey
  support), and a top-level `smtp:` section (actual: `email:`). Predates the
  DOMAIN/{fqdn} work. Needs a full pass reconciling the doc against the real
  `Config` struct, per testing-rules.md "keep docs/ in sync with the app".
