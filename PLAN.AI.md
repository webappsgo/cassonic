# PLAN.AI.md — PART 17 Admin Panel Implementation

Status: proposed, awaiting review before any code is written.
Scope source: AI.md PART 17 (29532–32015), cross-refs PART 11, 24, 34.
Optional-PART gate: **no SPEC.md exists** → PARTs 33 (agents), 34
(multi-user), 35 (orgs), 36 (custom domains), and PART 10 clustering are
**NOT activated**. Everything they gate stays ABSENT (optional-rules.md).

---

## 0. Current State vs Spec (the gap)

| Concern | Current impl | PART 17 requires |
|---------|--------------|------------------|
| Admin identity | `users.is_admin` flag (Subsonic role) | Separate `admins` table |
| Admin session | `cassonic_session` cookie → `sessions` (users.db) | `admin_session` cookie → `admin_sessions` (server.db), 30d |
| Admin auth | `requireAdmin` checks `users.IsAdmin` | Validate against `admins` table |
| Routes | Flat `/server/admin/{system,library,scheduler,config,logs,backup}` | Nested `/{admin_username}/*` + `/config/*` tree |
| Admin path | Literal `/server/admin` mount | Configurable `server.admin_path` |
| API version | Literal `/api/v1` | Configurable `server.api_version` |
| Setup | none (`--maintenance setup` prints paths only) | First-run setup token + wizard |
| MFA | `users.totp_*` cols unused for admin; no WebAuthn | TOTP + WebAuthn/Passkey + recovery keys (REQUIRED support) |
| Invites | none | Single-use, expiring admin invite links |
| OIDC/LDAP/SAML | none | Group-based admin sync w/ local fallback |
| Notif prefs | none | Per-admin categories + account/notification email split |
| Audit log | none | Admin actions audited (`config/logs/audit`) |

**Key architectural decisions made (flagged for review):**

1. **`users.is_admin` is NOT the server admin.** It is the Subsonic/Ampache
   privileged-user role (createUser, scan, etc.) and MUST stay. Server admins
   are a distinct account type in a new `admins` table (PART 17 §Server Admin
   vs Regular Users). Do not conflate the two.
2. **Migration for deployed installs (DECISION — review):** on startup, if
   `admins` is empty AND a `users` row with `is_admin=1` exists, run a one-time
   idempotent seed copying `(username, email, password_hash, source='local',
   is_primary=1)` into `admins`, and mark setup complete. This preserves panel
   login continuity for existing installs without needing the plaintext
   password. If no `is_admin` user exists (truly fresh DB), fall through to the
   setup-token flow. Alternative (rejected as default): force every existing
   install through setup-token re-onboarding — cleaner but locks out live
   admins on upgrade. **Open Q for review:** existing hashes may not be
   Argon2id; store the copied hash as-is with a scheme tag and re-hash on next
   successful login (`needsRehash`).
3. **`config/users` stays ABSENT.** PART 17 ties it to multi-user (PART 34,
   inactive). Core Subsonic user CRUD remains in the Subsonic/REST API only.
4. **Reserved-path & api_version live-reload deferred.** admin_path change =
   graceful re-mount is complex; Phase 1 ships the config field + startup mount
   only; hot re-mount is Phase 3.

---

## Phase sequence (dependency-ordered)

```
P0 config plumbing ─┬─ P1 data model+store ─┬─ P2 admin auth/session ─┬─ P3 route tree
                    │                        │                         ├─ P4 setup wizard
                    │                        │                         ├─ P5 MFA (TOTP+WebAuthn)
                    │                        │                         ├─ P6 invites
                    │                        │                         ├─ P7 OIDC/LDAP/SAML sync
                    │                        │                         └─ P8 notif prefs + email split
                    └──────────────────────────────────────────────── P9 absent-feature verification
```
P3–P8 each depend on P2; P5/P6/P7 also depend on P4 (setup must create the
primary admin first). P9 runs last as a compliance gate.

---

## P0 — Config plumbing (`server.admin_path`, `server.api_version`)

| File | Change |
|------|--------|
| `src/config/config.go` | Add `AdminPath string yaml:"admin_path"` (default `admin`) + `APIVersion string yaml:"api_version"` (default `v1`) to `ServerConfig`; validation ( `[a-z0-9-]`, 2–32, no leading/trailing `-`, reserved-path blocklist per PART 17); accessor helpers `AdminPath()`, `APIVersion()`, `APIBasePath()` |
| `src/server/server.go` (~208–247) | Mount admin at `"/server/"+cfg.Server.AdminPath` instead of literal; expose path/apiversion to templates |
| `src/server/handler/admin/admin.go` | Replace hardcoded `/server/admin/...` redirects with configured path |
| `server.yml` default template | Add `admin_path: admin`, `api_version: v1` |

Breaks/compat: default keeps `/server/admin` and `/api/v1` — non-breaking.
Tests: config validation table test (valid/invalid/reserved paths); route
mounts at custom path; reserved path rejected → reverts to `admin`.

---

## P1 — Data model + AdminStore

New DDL in `server.db` (`src/server/store/sqlite.go` `serverSchema`, all
`CREATE TABLE IF NOT EXISTS` = idempotent migration):

| Table | Key columns |
|-------|-------------|
| `admins` | id, username UNIQUE, email (account email), notification_email, password_hash, hash_scheme, is_primary, is_enabled, source (`local`/`oidc:*`/`ldap:*`/`saml:*`), external_id, groups (JSON), last_sync, totp_secret, totp_enabled, login_attempts, locked_until, created_at, updated_at |
| `admin_sessions` | token_hash PK, admin_id FK, remember_me, created_at, expires_at, ip (not shown cross-admin) |
| `admin_api_tokens` | id, admin_id FK, token_hash UNIQUE, name, last_used_at, expires_at |
| `admin_invites` | id, username, token_hash UNIQUE, max_uses (default 1), used_count, expires_at, created_by, created_at |
| `admin_recovery_codes` | id, admin_id FK, code_hash, used_at |
| `admin_webauthn_credentials` | id, admin_id FK, credential_id, public_key, sign_count, name, created_at |
| `admin_notification_prefs` | admin_id PK, per-category bools (security locked ON), delivery channel |
| `admin_auth_providers` | id, kind (oidc/ldap/saml), name, enabled, config (JSON), admin_groups (JSON) |
| `admin_setup` | singleton: setup_complete bool, setup_token_hash, token_created_at |
| `audit_log` | id, admin_username, action, target, detail (JSON), ip, created_at |

| File | Change |
|------|--------|
| `src/server/model/admin.go` | NEW — `Admin`, `AdminSession`, `AdminInvite`, `AdminAPIToken`, `AuthProvider`, `NotificationPrefs`, `AuditEntry` structs |
| `src/server/store/admin_store.go` | NEW — `AdminStore` iface + sqlite impl (CRUD, GetByUsername, GetByExternalID, session create/get/delete, invite, recovery, webauthn, prefs, audit append/list, setup get/complete) |
| `src/server/store/store.go` | Add `Admins AdminStore` + `Audit` to `DB`; wire in `sqlite.go` `Open()` |
| `src/server/store/sqlite.go` | Add tables; add one-time `is_admin → admins` seed migration (Decision 2) |
| `src/server/service/crypto` | Add Argon2id hash/verify + `needsRehash` (add `golang.org/x/crypto/argon2` — verify not already present) |

Compat: purely additive tables; seed migration is idempotent and guarded on
empty `admins`. `users.is_admin` untouched.
Tests: store CRUD round-trips; seed migration from a fixture users.db with an
is_admin row; Argon2id verify + rehash detection.

---

## P2 — Admin auth & session (separate from user auth)

| File | Change |
|------|--------|
| `src/server/handler/admin/auth.go` | NEW — login form + POST, logout; `admin_session` cookie (30d default, 90d remember-me); password policy (PART 17 auth section); lockout via `admins.login_attempts/locked_until` |
| `src/server/handler/admin/admin.go` (`requireAdmin`) | Rewrite: read `admin_session` cookie → `Admins.GetSessionByHash` → `Admins.Get`; drop `users.IsAdmin` path. Non-auth → admin login page (not `/login`) |
| `src/server/middleware/auth.go` | Ensure admin session cookie is isolated; admins treated as guest on `/**` and `/users/*` (PART 17 §Server Admin Behavior) |

Breaks: existing panel login via `cassonic_session`+is_admin stops working
for the panel; handled by P1 seed migration (existing admin re-logs in via
new admin login using migrated credentials). User-facing app login unchanged.
Tests (PART 17 §Testing Admin Routes — mandatory): unauth blocked; setup-token
→ create admin → login works; invalid creds rejected; session grants access;
lockout after N attempts.

---

## P3 — Nested route hierarchy

Restructure `admin.Routes()` to:
```
/                         dashboard
/{admin_username}/        profile, preferences, notifications (own account only)
/config/                  settings, branding, ssl, email, scheduler, logs,
                          logs/audit, backup, updates, info, metrics,
                          network/tor, network/geoip,
                          security/auth{,/oidc,/ldap,/saml}, security/tokens,
                          security/firewall, admins
```

| File | Change |
|------|--------|
| `src/server/handler/admin/admin.go` | Rebuild router; add `validateAdminRoute` guard (only `config` + `{admin_username}` under root); migrate existing handlers: `/system`→`/config/info`, `/library`→core (not admin config — keep or move to `/config/library` per IDEA), `/scheduler`→`/config/scheduler`, `/config`→`/config/settings`, `/logs`→`/config/logs`, `/backup`→`/config/backup` |
| `src/server/handler/admin/config_*.go` | Split large config handler into per-section files (ssl, email, network, security, backup, updates, info, metrics) |
| `src/server/handler/admin/self.go` | NEW — `{admin_username}` profile/preferences/notifications |
| `src/server/handler/admin/template/*` | New nav (sidebar groups per PART 17), breadcrumbs; sidebar conditionals for absent features OFF |
| `src/server/handler/api/...` | Mirror admin API tree under `/api/{api_version}/server/{admin_path}/...` (every web route needs matching API route — api-rules.md) |

Compat: old flat URLs 301→new (or removed — internal only, no external
consumers). admin_path hot re-mount (graceful reload) implemented here.
Tests: route-guard rejects invalid root segments; each page 200 with session,
redirect without; API mirror parity.

---

## P4 — First-run setup wizard

| File | Change |
|------|--------|
| `src/main.go` (~281–301 first-run block) | Generate 32-hex setup token when `admin_setup.setup_complete=false` AND no admin exists; print console banner (PART 17 layout); store only SHA-256 hash |
| `src/server/handler/admin/setup.go` | NEW — `/config/setup` wizard (6 steps) + verify-token gate; API `config/setup/{verify,account,token,config,security,services,complete}` |
| `src/server/store/admin_store.go` | Setup token verify (constant-time), single-use invalidate on complete |

Steps per spec: 1 admin account, 2 API token (shown once), 3 server config,
4 security (backup encryption pw, enable 2FA), 5 optional services (HTTPS
review; multi-user enable = **no-op/hidden**, PART 34 inactive), 6 complete.
Compat: app fully functional pre-setup (PART 17 §App Usability). Seed-migrated
installs (P1) start with setup_complete=true → wizard not shown.
Tests: token single-use; wrong token rejected; completing wizard creates
primary admin + invalidates token; pre-setup public routes still up.

---

## P5 — MFA (TOTP + WebAuthn/Passkey)

Deps to add: `github.com/pquerna/otp` (TOTP), `github.com/go-webauthn/webauthn`.

| File | Change |
|------|--------|
| `src/server/service/mfa/totp.go` | NEW — enroll (QR + secret), verify, 10 recovery codes |
| `src/server/service/mfa/webauthn.go` | NEW — registration + assertion (login or 2nd factor) |
| `src/server/handler/admin/self.go` | `/{admin_username}/profile/security` — enroll/disable TOTP (disable needs current code/recovery), register/revoke passkeys |
| `src/server/handler/admin/auth.go` | Insert 2FA prompt into login flow (PART 17 auth-flow diagram) |

Compat: MFA optional (support REQUIRED, usage recommended). Recovery codes
REQUIRED when MFA enabled.
Tests: TOTP enroll→verify→login-with-code; recovery code single-use; passkey
register + assert (virtual authenticator); disable requires proof.

---

## P6 — Admin invites

| File | Change |
|------|--------|
| `src/server/handler/admin/admins.go` | NEW — `/config/admins` (count-only view per Privacy), invite create (expiry 1h/6h/24h/48h/7d, max_uses default 1), delete/disable by-username; API `config/admins/{count,invite,{username},{username}/{enable,disable}}` |
| `src/server/handler/admin/invite_accept.go` | NEW — `/server/auth/invite/server/{token}` accept: new admin sets username/password/optional 2FA, gets API token once |

Compat: adheres to Server Admin Privacy (no listing/GET of other admins).
Primary admin undeletable (only `--maintenance setup`).
Tests: invite single-use + expiry; accept creates admin + audit entry;
enumerate/GET other admin → 404/forbidden.

---

## P7 — OIDC/LDAP/SAML admin sync

| File | Change |
|------|--------|
| `src/server/service/authprovider/{oidc,ldap,saml}.go` | NEW — provider clients; group fetch; test-connection |
| `src/server/handler/admin/auth_providers.go` | NEW — `/config/security/auth/{oidc,ldap,saml}` CRUD + test; SAML SP cert regen/upload |
| `src/server/store/admin_store.go` | Upsert admin by `external_id`+source; cache `groups`; `last_sync`; revoke on group removal |
| `src/server/handler/admin/auth.go` | External login path → map `admin_groups` → create/update local admin; fallback to cached local creds when provider down |

Deps: OIDC (`coreos/go-oidc`), LDAP (`go-ldap/ldap`), SAML
(`crewjam/saml`) — add only if provider enabled at build; all disabled by
default. Identity key = `external_id`+source (not username/email).
Compat: local admins unaffected; providers off by default.
Tests: group→admin mapping; removed-from-group revokes on next login;
provider-down uses cached local creds; external_id match survives username
change. (Provider integration behind mocks.)

---

## P8 — Notification prefs + account/notification email split

| File | Change |
|------|--------|
| `src/server/handler/admin/self.go` | `/{admin_username}/notifications` — categories (Security locked ON), delivery=email, SMTP-status gate |
| `src/server/handler/admin/self.go` | Email settings: account email (security; change needs pw+2FA) vs notification email (session only); both verified before use |
| `src/server/service/email` | Verification-token send; category-gated dispatch; if SMTP down → in-panel only |

Compat: notification email defaults to account email. All email needs working
SMTP (PART 18).
Tests: category toggle persists; security category cannot disable; account
email change requires pw(+2FA); unverified email not used; SMTP-down path.

---

## P9 — Absent-feature compliance gate (no code, verification only)

Per optional-rules.md, confirm ZERO tables/routes/conditionals/UI stubs for:

| Feature | Gate | Action |
|---------|------|--------|
| `config/users`, `config/users/invites` | PART 34 multi-user (inactive) | ABSENT — core Subsonic user CRUD stays in Subsonic/REST API |
| `config/orgs` | PART 35 (inactive) | ABSENT |
| `config/cluster/*` | PART 10 clustering (not activated) | ABSENT |
| `config/agents/*` | PART 33 agents — music server, not monitoring | ABSENT (no `src/agent/`) |

Sidebar sections for these MUST NOT render. Grep gate:
`grep -rniE "orgs|cluster|/agents|config/users" src/server/handler/admin`
returns only comments explaining intentional absence.

---

## Cross-cutting test/lint gate (every phase, before commit)

- `make test` green (Docker/Incus, PART 29 — never host).
- `go-lint` clean.
- Every user-facing string via `t()` translation keys, all 7 langs
  (testing-rules.md / PART 31).
- docs/ + mkdocs nav updated for new admin pages (PART 30).
- New behavior gets a test that fails before, passes after.

## Open questions for review

1. Migration default: seed `is_admin`→`admins` (continuity) vs force
   setup-token re-onboard (clean)? Plan assumes seed.
2. Copied legacy password hashes: store-as-is + rehash-on-login, or require
   password reset via invite? Plan assumes rehash-on-login.
3. `admin_path` hot re-mount vs restart-required — plan does graceful re-mount
   in P3; acceptable to defer to restart-required for v1?
4. `/library` admin page: keep under admin as core music admin, or move to
   `/config/library`? Not covered by PART 17 (music-server specific).
