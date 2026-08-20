package model

import "time"

// Admin represents a server admin account (AI.md PART 17 "Server Admin
// Accounts" / PART 12 admins table). Server admins are a distinct account
// type from regular Users — stored in their own admins table in users.db so
// they can never be conflated with the Subsonic/Ampache-facing
// users.is_admin privileged-user role. The Primary Admin (the one created by
// the setup wizard) is not a stored flag — it is the admin with the lowest
// ID, and cannot be deleted except via `--maintenance setup`.
type Admin struct {
	ID       int64  `db:"id"`
	Username string `db:"username"`
	// PasswordHash is the Argon2id hash; empty for externally synced admins
	// that authenticate exclusively via OIDC/LDAP/SAML.
	PasswordHash string `db:"password_hash"`
	Email        string `db:"email"`
	// Role is one of "superadmin", "admin", "readonly".
	Role    string `db:"role"`
	Enabled bool   `db:"enabled"`
	// APITokenHash is the SHA-256 hash of the admin's API token (prefix
	// "adm_"); empty until issued. Not yet wired to any handler — reserved
	// for the admin API token feature.
	APITokenHash string `db:"api_token_hash"`
	// Source is "local", or "oidc:{provider}" / "ldap:{provider}" /
	// "saml:{provider}" for externally synced admins.
	Source string `db:"source"`
	// ExternalID is the stable identity key for externally synced admins;
	// matching MUST use this, never the mutable username/email.
	ExternalID string `db:"external_id"`
	// Groups is a JSON array of the external provider's group memberships.
	Groups         string    `db:"groups"`
	LastSync       time.Time `db:"last_sync"`
	LastLogin      time.Time `db:"last_login"`
	FailedAttempts int       `db:"failed_attempts"`
	LockedUntil    time.Time `db:"locked_until"`
	// TOTPEnabled is the "enable 2FA for this admin" preference captured by
	// setup wizard step 4 (AI.md PART 17 "Security Settings") and by the
	// TOTP enrollment flow at
	// /server/{admin_path}/{admin_username}/profile/security. When true,
	// login requires a valid TOTP code (or backup code) after password
	// verification. WebAuthn/Passkey support is a separate future phase.
	TOTPEnabled bool      `db:"totp_enabled"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// IsLocked returns true if the admin account is currently locked out.
func (a *Admin) IsLocked() bool {
	if a.LockedUntil.IsZero() {
		return false
	}
	return time.Now().Before(a.LockedUntil)
}

// AdminSession represents an active admin panel login session, kept entirely
// separate from the regular-user sessions table (own cookie name, own table,
// own store) per AI.md PART 17 "Server Admin vs Regular User". Stored in
// server.db; admin_id is a logical FK to admins.id in users.db (cross-DB, not
// enforced).
type AdminSession struct {
	TokenHash  string    `db:"id"`
	AdminID    int64     `db:"admin_id"`
	IP         string    `db:"ip_address"`
	UserAgent  string    `db:"user_agent"`
	CreatedAt  time.Time `db:"created_at"`
	ExpiresAt  time.Time `db:"expires_at"`
	LastActive time.Time `db:"last_active"`
}

// IsExpired returns true if the session has passed its expiry time.
func (s *AdminSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// AdminPreferences holds one admin's WebUI settings (AI.md PART 12
// admin_preferences table): appearance, display format, and email
// notification toggles. EmailSecurity cannot be disabled by policy — that
// rule is enforced by the handler layer, not the store.
type AdminPreferences struct {
	AdminID       int64     `db:"admin_id"`
	Theme         string    `db:"theme"`
	FontSize      string    `db:"font_size"`
	ReduceMotion  bool      `db:"reduce_motion"`
	DateFormat    string    `db:"date_format"`
	TimeFormat    string    `db:"time_format"`
	EmailSecurity bool      `db:"email_security"`
	EmailServer   bool      `db:"email_server"`
	EmailBackups  bool      `db:"email_backups"`
	EmailUsers    bool      `db:"email_users"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// AuditEntry records a single admin action or security event (AI.md PART 12
// audit_log table, surfaced at
// "/server/{admin_path}/config/logs/audit"). Stored in server.db.
type AuditEntry struct {
	ID int64 `db:"id"`
	// Level is one of "info", "warning", "error", "security".
	Level string `db:"level"`
	// Category is one of "auth", "config", "admin", "api", "system".
	Category string `db:"category"`
	// Action is a short verb phrase: "login", "logout", "config_change", etc.
	Action string `db:"action"`
	// ActorType is "admin", "api_key", "system", or "anonymous".
	ActorType string `db:"actor_type"`
	ActorID   string `db:"actor_id"`
	ActorIP   string `db:"actor_ip"`
	// TargetType/TargetID identify what the action was performed on, e.g.
	// "user"/"42" or "config"/"server.admin_path".
	TargetType string `db:"target_type"`
	TargetID   string `db:"target_id"`
	// Detail is a JSON-encoded object with action-specific context.
	Detail    string    `db:"details"`
	Success   bool      `db:"success"`
	CreatedAt time.Time `db:"timestamp"`
}

// SetupToken represents the one-time first-run setup token (AI.md PART 17
// "Setup Token Rules" / PART 12 "setup_token" table). It is a single row in
// server.db; the raw token is never stored, only its SHA-256 hash. The row
// is created once, on first run with zero admins, and is marked Used when
// the setup wizard completes (never regenerated — a lost token requires
// resetting server.db).
type SetupToken struct {
	TokenHash string    `db:"token_hash"`
	CreatedAt time.Time `db:"created_at"`
	Used      bool      `db:"used"`
	UsedAt    time.Time `db:"used_at"`
}

// TOTPSecret represents one admin's TOTP enrollment (AI.md PART 17 "TOTP
// Two-Factor Authentication" / PART 12 "totp_secrets" table). Stored in
// server.db; UserID is a logical FK to admins.id in users.db (cross-DB, not
// enforced). Secret is AES-256-GCM encrypted with
// server.security.encryption_key before it is ever persisted. BackupCodes is
// a JSON array of SHA-256 hashes of the unused one-time recovery codes; raw
// codes are shown to the admin exactly once and never stored.
type TOTPSecret struct {
	ID          int64     `db:"id"`
	UserType    string    `db:"user_type"`
	UserID      int64     `db:"user_id"`
	Secret      string    `db:"secret"`
	Enabled     bool      `db:"enabled"`
	BackupCodes string    `db:"backup_codes"`
	CreatedAt   time.Time `db:"created_at"`
	LastUsed    time.Time `db:"last_used"`
}

// AdminMFAChallenge represents a short-lived "password verified, awaiting
// 2FA" login state (AI.md PART 17 "/server/{admin_path} Authentication
// Flow"), mirroring AdminSession's token-hash pattern. Stored in server.db;
// AdminID is a logical FK to admins.id in users.db. Remember preserves the
// "remember me" choice across the two-step login so the eventual real
// session gets the correct duration.
type AdminMFAChallenge struct {
	TokenHash string    `db:"id"`
	AdminID   int64     `db:"admin_id"`
	IP        string    `db:"ip_address"`
	UserAgent string    `db:"user_agent"`
	Remember  bool      `db:"remember"`
	CreatedAt time.Time `db:"created_at"`
	ExpiresAt time.Time `db:"expires_at"`
}

// IsExpired returns true if the MFA challenge has passed its expiry time.
func (c *AdminMFAChallenge) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}
