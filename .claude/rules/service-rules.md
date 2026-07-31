# Privilege Escalation & Service Rules (PART 24, 25)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Call sudo or escalate privilege at runtime unless explicitly authorized
- Install systemd/launchd/service files outside the locations PART 24/25 define

## CRITICAL - ALWAYS DO

- Follow the default scope and path rules for system integration files
- Support install/uninstall/status commands for the OS service manager

## Key Rules Summary

- PART 24 defines privilege escalation boundaries and service file placement
- PART 25 defines systemd/launchd/Windows service support requirements

For complete details, see AI.md PART 24, 25
