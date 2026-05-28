# Features Rules (PART 18, 19, 20, 21, 22, 23)

Read: AI.md PART 18, 19, 20, 21, 22, 23

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## EMAIL (PART 18)
- ALL email templates must be customizable — defaults embedded in binary, custom in {config_dir}/template/email/
- Custom template exists → use it; else fall back to embedded default
- SMTP auto-detect on first run (loopback → Docker bridge → gateway → FQDN)
- No SMTP configured → disable ALL email features, hide email-dependent UI
- NEVER queue emails or attempt to send without valid working SMTP

## SCHEDULER (PART 19)
- Built-in scheduler ALWAYS RUNNING — never use external schedulers (cron, systemd timers, k8s CronJob, etc.)
- Required built-in tasks: ssl_renewal, geoip_update, blocklist_update, cve_update, session_cleanup, token_cleanup, log_rotation, backup_daily, backup_hourly, healthcheck_self, tor_health, cluster_heartbeat
- Scheduler state persistent in server.db; catch-up window for missed tasks on restart
- Cluster mode: global tasks run once per cluster (leader election); local tasks run on each node
- Lock timeout: 5 minutes (auto-release if node dies)

## GEOIP (PART 20)
- Built-in GeoIP via sapics/ip-location-db (no API key required)
- NEVER embed databases — download on first run, update via scheduler
- Go library: github.com/oschwald/maxminddb-golang NOT geoip2-golang (incompatible DB type strings)
- deny_countries OR allow_countries (mutually exclusive); both set → allow_countries wins
- Allowlisted IPs ALWAYS bypass country blocking; RFC 1918 IPs never blocked

## METRICS (PART 21)
- Prometheus-compatible metrics at /metrics (configurable)
- /metrics is INTERNAL ONLY — never expose publicly
- Metric prefix: {project_name}_ (snake_case, unit suffix, _total for counters)
- Optional Bearer token authentication

## BACKUP (PART 22)
- Backup: server.yml + server.db + users.db + custom templates/themes (SSL and data optional with flags)
- Filename: {project_name}_backup_YYYY-MM-DD_HHMMSS.tar.gz[.enc]
- Encryption: AES-256-GCM with Argon2id key derivation (optional; required if compliance mode enabled)
- NEVER store backup password — admin must remember it; no recovery path
- Unencrypted archive never touches disk when encryption enabled

## UPDATE (PART 23)
- --update check: check without installing; --update yes: in-place update with restart
- Branches: stable (v*.*.* tags), beta (*-beta), daily (YYYYMMDDHHMMSS)
- Verify SHA256 before replacing binary
- Unix: atomic rename (os.Rename); Windows: rename running binary, use .old suffix, re-exec

---
For complete details, see AI.md PART 18, 19, 20, 21, 22, 23
