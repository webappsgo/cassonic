# TODO

- admin: RESOLVED — the admin panel's route hierarchy
  (`src/server/handler/admin/admin.go` `Routes()`) now follows AI.md
  PART 17's nested structure: `/` (dashboard), `/config/*` (settings,
  library, scheduler, logs, backup, system info), and
  `/{admin_username}/*` (profile, preferences) with
  `validateAdminRoute`/`enforceAdminRouteHierarchy` rejecting anything
  outside that shape. Self-account handlers (`self.go`: `SelfRoot`,
  `Profile`, `SaveProfile`, `Preferences`, `SavePreferences`) and
  templates (`profile.html`, `preferences.html`) were added; nav updated
  in `base.html`/`dashboard.html`; tests added in `admin_test.go`. Org/
  cluster/multi-provider-auth sub-routes (`/config/security/auth/*`,
  `/config/users/`, `/config/orgs/`, `/config/cluster/`) remain
  intentionally absent per PART 34-36 (optional PARTs not activated in
  SPEC.md) — not a gap.

- admin: the admin panel currently has no JSON API mirror
  (`/api/{version}/server/{admin_path}/...`) for any of its routes —
  AI.md PART 14 requires every web route to have a matching JSON API
  route and vice versa. Discovered while implementing PART 17 (P3); out
  of scope for the route-hierarchy/self-account work since it's a
  separate, large new-feature undertaking (a full parallel JSON handler
  set plus RFC 7807 error envelopes per PART 13/14) rather than a fix to
  the existing web routes. Needs a dedicated implementation pass adding
  `/api/{api_version}/server/{admin_path}/...` counterparts for
  dashboard, config, library, scheduler, logs, backup, and the
  profile/preferences self-account routes.

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

- admin: the admin panel's i18n wiring (`Handler.i18n`, `resolveLocale`,
  `adminPageData.Lang`/`.T`) is currently scoped only to the first-run
  setup wizard pages (`setup_token.html`, `setup_wizard.html`, added in
  P4). None of the ~10 pre-existing admin templates (`dashboard.html`,
  `base.html`, `profile.html`, `preferences.html`, library/scheduler/
  logs/backup/system-info pages, etc.) call `{{call .T ...}}` — all of
  their user-facing strings are still hardcoded English. AI.md PART 31
  requires every user-facing string to be translated; discovered while
  implementing PART 17 (P4) but out of scope for the setup-wizard feature
  since retrofitting i18n across the entire existing admin panel is a
  separate, large pass. Needs a dedicated implementation pass adding
  `{{call .T ...}}` calls (and matching keys in all 7 locale files) to
  every existing admin template.

- testing: `tests/e2e.sh` (Read: AI.md PART 3, 29) is missing. AI.md's
  directory structure (PART 3, "Directory Structure") lists it as REQUIRED
  alongside `run_tests.sh`/`docker.sh`/`incus.sh`, and PART 29 specifies it
  Docker-wraps `go test -tags e2e ./tests/e2e/...` against a chromedp
  headless-Chromium harness (no `tests/e2e/` Go test package exists yet
  either). Discovered during the PART 0-6 bootstrap re-check; not created
  because designing the actual browser E2E scenarios (which pages/flows to
  cover, JS-off vs JS-on tiers) is feature-sized work requiring product
  decisions, not a scaffolding fill-in.

- cicd: root `Jenkinsfile` (Read: AI.md PART 3, 28) is missing.
  AI.md's directory structure (PART 3) and `cicd_conventions.md` both list
  it as required on every project alongside the GitHub/GitLab/Gitea/Forgejo
  workflow files (which already exist). Discovered during the PART 0-6
  bootstrap re-check; not created because authoring a correct declarative
  Jenkins pipeline that mirrors the existing `ci.yml`/`.gitlab-ci.yml`
  lint/test/security/release job matrix (PART 28) is CI/CD implementation
  work outside this pass's PART 0-6 scaffolding scope.
