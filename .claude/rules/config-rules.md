# Configuration & Modes Rules (PART 5, 6, 12)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use JSON or TOML for the main config format
- Require a restart for settings other than listen address/port/DB driver
- Hide any config setting from the admin WebUI

## CRITICAL - ALWAYS DO

- Use server.yml as the canonical config format
- Support production/development mode detection per PART 6
- Make every server.yml setting live-reloadable and admin-UI editable

## Key Rules Summary

- Config file is always server.yml (never .yaml)
- Application modes: production and development, auto-detected per PART 6
- PART 12 defines the full server configuration schema and BuildURL rules

For complete details, see AI.md PART 5, 6, 12
