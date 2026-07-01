# Features Rules (PART 18-23)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Make telemetry opt-out — always opt-in only
- Block features for users without MFA — suggest, never force
- Overwrite non-empty user-edited fields in MusicBrainz auto-lookup
- Run backup operations that could corrupt live data
- Skip scrobble queue — always use queue + retry for external services
- Use `--update branch vbeta` — text versions NEVER get `v` prefix

## CRITICAL - ALWAYS DO

- Email/notifications: all transports are opt-in disabled by default
- Scrobbling: fan-out to ALL configured+enabled+verified services simultaneously
- Scrobbling: per-service queue + retry on failure
- MusicBrainz: opt-in nightly job; never overwrite non-empty user-edited fields
- Backup: idempotent, safe to run while server is live
- Update: check GitHub Releases API; `--update check` needs no privileges

## Email & Notifications (PART 18)

- All notification channels disabled by default
- SMTP only auto-enables if `SMTP_HOST` env var is set at startup
- Admin configures channels in admin panel
- Users configure per-channel preferences in user settings

## Scheduler (PART 19)

- Background job runner built into server binary
- Jobs: GeoIP update, MusicBrainz lookup, blocklist update, backup, scrobble retry
- Configurable intervals via `server.yml`
- Jobs run asynchronously — never block request handling

## GeoIP (PART 20)

- Provider: ip-location-db (free, no API key, CC0/PDDL)
- Data: ASN, Country, City, WHOIS MMDB files
- Location: `{data_dir}/security/geoip/`
- Update: daily via scheduler

## Metrics (PART 21)

- Prometheus metrics endpoint at `/api/{api_version}/server/metrics`
- Auth required unless explicitly public
- Aggregate only — never per-user detail in public metrics

## Backup & Restore (PART 22)

- `--backup` flag triggers backup
- Output: compressed archive to configured backup directory
- Safe to run live (no data corruption)
- `--restore` flag to restore from archive

## Update Command (PART 23)

- `--update` or `--update yes`: in-place update with restart
- `--update check`: check only, no install (no privileges needed)
- Branches: `stable` (default), `beta`, `daily`
- Version tags: numeric gets `v` prefix; text (`dev`, `beta`, `daily`) NEVER gets `v`

For complete details, see AI.md PART 18, 19, 20, 21, 22, 23
