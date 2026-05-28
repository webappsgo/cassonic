# Service Rules (PART 24, 25)

Read: AI.md PART 24, 25

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## PRIVILEGE ESCALATION (PART 24)
- User creation REQUIRES escalation; if cannot escalate → run as current user with user-level dirs
- Detection order (Linux): already root → sudo → su → pkexec → doas
- Detection order (macOS): already root → sudo → osascript
- Detection order (Windows): already Administrator → UAC → runas
- NEVER prompt for escalation if user cannot escalate — show informative error instead

## SERVICE INSTALL (PART 24)
- --service --install: detect platform init system (systemd/OpenRC/runit/s6/launchd/rc.d/Windows Service)
- Root/admin → system service; user → user service (systemd --user, launchctl user agent)
- Server binary handles user/group creation on normal startup, NOT at install time
- --service --uninstall: stop → disable → remove service file → DELETE ALL DATA → delete system user
- Confirm before destructive uninstall: "This will delete ALL data, configs, and the system user. Continue? [y/N]"

## SERVICE SUPPORT (PART 25)
- ALL projects MUST support ALL service managers
- Binary detects root/admin: bind any port, then DROP PRIVILEGES to {project_name} user
- User mode: bind ports >1024 only; no privilege drop
- Windows: Virtual Service Account (NT SERVICE\{internal_name}) — never Local System or Administrator

## SERVICE FILE PATHS
| Init System | Path |
|-------------|------|
| systemd | /etc/systemd/system/{internal_name}.service |
| OpenRC | /etc/init.d/{internal_name} |
| SysVinit | /etc/init.d/{internal_name} |
| launchd (system) | /Library/LaunchDaemons/us.{project_org}.{internal_name}.plist |
| launchd (user) | ~/Library/LaunchAgents/us.{project_org}.{internal_name}.plist |
| Windows | HKLM\SYSTEM\CurrentControlSet\Services\{internal_name} |

## KEY RULES
- ReadWritePaths in systemd unit: /etc/{project_org}/{internal_name}, /var/lib/{project_org}/{internal_name}, /var/cache/{project_org}/{internal_name}, /var/log/{project_org}/{internal_name}
- ProtectSystem=strict, ProtectHome=yes, PrivateTmp=yes in systemd
- Service user home dir: config_dir (user needs config access) or data_dir (data-heavy use cases)
- Home dir must exist BEFORE user creation; create dirs → create user → set ownership

---
For complete details, see AI.md PART 24, 25
