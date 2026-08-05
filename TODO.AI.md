# TODO

- admin: the admin panel's route hierarchy
  (`src/server/handler/admin/admin.go` `Routes()`) is flat —
  `/`, `/system`, `/library`, `/scheduler`, `/config`, `/logs`, `/backup` —
  but AI.md PART 17 (ADMIN PANEL) specifies a much deeper nested hierarchy
  under `/server/{admin_path}/config/...`, including
  `/config/settings`, `/config/security/auth/{oidc,ldap,saml}`,
  `/config/users/`, `/config/orgs/`, and `/config/cluster/`. Discovered while
  implementing the `SaveConfig` form-persist feature; out of scope for that
  change since it only covers `server.yml` persistence, not route
  restructuring. Needs a full PART 17 re-read and a route-hierarchy rework
  (or a written decision that the flat structure is an intentional,
  documented deviation) before the admin panel can be called PART 17
  compliant.

- store: `sqliteUserStore.GetAPITokenByHash` (`expires_at > datetime('now')`,
  `user_store.go:321`) and `PurgeExpiredSessions`
  (`expires_at < datetime('now')`, `user_store.go:447`) compare a
  RFC3339-formatted (`...T...Z`) `expires_at` column value against
  `datetime('now')`, which SQLite renders as `YYYY-MM-DD HH:MM:SS` (space,
  no `T`/`Z`) — the two formats never compare equal/ordered correctly as
  strings, so token-expiry and session-purge checks can silently
  pass/fail incorrectly. Discovered while implementing the PART 17 admin
  store (`admin_store.go`), which hit and fixed the identical bug in
  `PurgeExpiredAdminSessions` by wrapping the column in `datetime(...)` to
  normalize both sides before comparing. Apply the same fix to the two
  `user_store.go` call sites above; out of scope for the PART 17 admin
  work since it's pre-existing and unrelated to admin accounts.
