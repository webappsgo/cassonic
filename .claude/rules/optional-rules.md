# Optional Features Rules (PART 34, 35, 36)

Read: AI.md PART 34, 35, 36

⚠️ **These PARTs are OPTIONAL to implement but NON-NEGOTIABLE once implemented.** ⚠️

## WHEN TO IMPLEMENT

### PART 34: MULTI-USER
- Required when: app needs end-user accounts, registration, profiles, API tokens
- NOT required for: simple APIs (jokes, quotes, data services), admin-only tools
- Server Admin accounts (PART 17) are ALWAYS required; Regular User accounts are optional
- Storage: Server Admin → `admins` table; Regular Users → `users` table

### PART 35: ORGANIZATIONS
- Requires PART 34: MULTI-USER first
- Required when: users need to collaborate as teams with shared resources, team billing, or agency/company workflows
- NOT required for: personal tools, individual-use apps, consumer social, simple APIs
- Use canonical internal term `org`/`orgs` in code/routes/DB; UI label can be "team"/"workspace"/"group"

### PART 36: CUSTOM DOMAINS
- Required when: users need to present content under their own brand/domain
- NOT required for: internal tools, private data, API-only services, personal self-hosted apps, git hosting
- Verification: TXT record ownership verification; SSL: automatic via Let's Encrypt DNS-01

## NON-NEGOTIABLE RULES WHEN IMPLEMENTED

### Multi-User (PART 34) — if implemented:
- Registration modes: open (default), invite, admin_only, disabled
- Admin CANNOT set user passwords (only user can, via invite link or reset)
- Admin CANNOT view passwords, 2FA secrets, or private user data
- Email verification respects SMTP availability (PART 18 rules apply)
- User data in users.db (separate from server.db)
- Full CRUD + moderation in admin panel
- API tokens: SHA-256 hashed; scopes enforced; never stored in plaintext

### Organizations (PART 35) — if implemented:
- Orgs use canonical term `org`/`orgs` in code, routes, and DB
- Org owns resources (not individual users); member roles: Owner, Admin, Member
- Transfer ownership, public profile, org-level audit trail all required
- Requires multi-user mode to be enabled

### Custom Domains (PART 36) — if implemented:
- TXT record DNS verification before activation
- Automatic SSL per custom domain via Let's Encrypt DNS-01
- Domain validation prevents subdomain takeover
- Custom domain routing independent of tenant path structure

---
For complete details, see AI.md PART 34, 35, 36
