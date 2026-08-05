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

- auth: AI.md's unified "Auth Routes" / "Scoped Login Redirect" section
  (~line 16060) specifies all auth routes live under `/server/auth/*`
  (e.g. `/server/auth/login`, `/server/auth/logout`), but the codebase
  mounts them at root (`r.Get("/login", h.Login)` etc. in
  `src/server/handler/web/web.go`'s `Routes()`). Discovered while
  implementing PART 17 admin auth (P2): the admin login flow was wired
  into the existing shared `/login` route rather than doing an
  unplanned site-wide route-path rename, since this deviation predates
  and is broader than the PART 17 admin-panel scope that triggered this
  work. Needs either a full `/login`→`/server/auth/login` (and
  `/logout`→`/server/auth/logout`) migration across routes, templates,
  and redirects, or a written decision that the root-level paths are an
  intentional, documented deviation.

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
