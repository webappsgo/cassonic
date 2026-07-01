# Optional Features Rules (PART 34, 35, 36)

⚠️ **These rules are OPTIONAL to implement, but NON-NEGOTIABLE once implemented.** ⚠️

## CRITICAL - ALWAYS DO (when implementing)

- PART 34 (Multi-User) must be complete before PART 35 (Organizations)
- All registration modes must be implemented if multi-user is enabled
- Never allow admin to set user passwords — admin can only issue invite/reset links
- Custom domains: always use Let's Encrypt DNS-01 for SSL
- Custom domains: always verify via TXT record ownership check
- Organizations: use `org`/`orgs` in routes, config, and DB regardless of user-facing label

## Multi-User (PART 34) — Optional

**cassonic is a music streaming server. It likely needs multi-user support.**

**Two modes:**
| Mode | Description |
|------|-------------|
| Admin-only | Default — no user accounts feature |
| Multi-user | End-user accounts, registration, profiles, API tokens |

**Registration modes (config: `users.registration.mode`):**
| Mode | Self-Register | Admin Invite | Admin Create | Default |
|------|--------------|--------------|--------------|---------|
| `open` | ✓ | Optional | Optional | YES |
| `invite` | ✗ | ✓ Required | ✗ | No |
| `admin_only` | ✗ | ✗ | ✓ Required | No |
| `disabled` | ✗ | ✗ | ✗ | No |

**Key rules:**
- `open` is the default when multi-user is enabled
- Registration mode controls NEW account creation only — existing users always log in
- Admin CANNOT set user passwords — only issue invite link or reset link
- Admin CAN: issue invites, create users (admin_only mode), send password reset, suspend/unsuspend, disable 2FA
- Invites: single-use, default 24h expiry, configurable (1h, 6h, 24h, 48h, 7d)
- External identity (OIDC/LDAP): first-login account creation respects `auto_register` and registration mode

## Organizations (PART 35) — Optional, requires PART 34

**Use organizations when users need to collaborate as teams with shared resources.**

| Use orgs when | Skip orgs when |
|---------------|----------------|
| Teams share repos/projects/files | Users work independently |
| Shared permissions across team | No collaboration needed |
| Billing at org/team level | Individual billing only |
| Agency/multi-tenant SaaS | Personal/self-hosted only |

**Terminology:** Internal code/routes/DB uses `org`/`orgs`. User-facing labels may use "team", "workspace", "group" if that fits the product better.

## Custom Domains (PART 36) — Optional

**Use custom domains when users need to present your service under their own brand/domain.**

| Use custom domains when | Skip when |
|------------------------|-----------|
| Users publish public content | Internal API service |
| White-label SaaS | Simple data APIs |
| Link-in-bio/landing pages | Personal self-hosted |
| E-commerce storefronts | Anonymous/ephemeral content |

**cassonic (music streaming) likely does NOT need custom domains** — personal media library served under admin-chosen domain.

**Implementation rules when enabled:**
- SSL: automatic via Let's Encrypt DNS-01
- Verification: TXT record ownership verification required
- Scope: per-user or per-org domain mapping

For complete details, see AI.md PART 34, 35, 36
