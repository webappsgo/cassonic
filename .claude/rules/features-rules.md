# Email, Scheduler, GeoIP, Metrics, Backup & Update Rules (PART 18-23)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use cron or an external scheduler (PART 19 requires a built-in scheduler)
- Send email/notifications without a configurable provider
- Skip backup/restore or the update command

## CRITICAL - ALWAYS DO

- Implement notifications per PART 18 with admin-configurable providers
- Implement the built-in scheduler per PART 19
- Expose GeoIP (PART 20) and metrics (PART 21) as documented
- Implement backup/restore (PART 22) and the update command (PART 23)

## Key Rules Summary

- All background jobs run through the built-in scheduler, never OS cron
- Metrics follow the Prometheus-compatible format defined in PART 21

For complete details, see AI.md PART 18-23
