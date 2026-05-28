# Config Rules (PART 5, 6, 12)

Read: AI.md PART 5, 6, 12

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Inline YAML comments — always above the setting
- Hardcode machine-specific values (hostname, IP, cores, memory)
- Write machine values to config files at build time
- Create config files in repo (runtime-generated only)
- Use strconv.ParseBool() — use config.ParseBool()

## CRITICAL - ALWAYS DO
- YAML comments above the setting, never inline
- Detect hostname, IP, cores, memory at runtime
- config.ParseBool() for all boolean parsing
- Mode detection: --mode flag > MODE env > default (production)
- Debug detection: --debug flag > DEBUG env > default (false)

## APPLICATION MODES
| Mode | Debug | Behavior |
|------|-------|----------|
| production | false | Minimal logs, no debug endpoints |
| production | true | Live debugging (temporary) |
| development | false | Verbose logs, hot reload |
| development | true | Full debugging, all features |

## BOOLEAN VALUES ACCEPTED
yes/no, true/false, 1/0, on/off, enable/disable (40+ variations)
Use config.ParseBool() — never strconv.ParseBool()

---
For complete details, see AI.md PART 5, 6, 12
