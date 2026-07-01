# Service Rules (PART 24, 25)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Run `--service --uninstall` without confirmation prompt ("This will delete ALL data...")
- Prompt user to escalate if they CANNOT escalate (check first)
- Keep privilege elevation after port binding on Unix (must drop)
- Use permanent root without explicit requirement in IDEA.md
- Run service as root permanently when user privilege drop is possible

## CRITICAL - ALWAYS DO

- Check escalation capability BEFORE prompting (see escalation detection below)
- Drop privileges after port binding (Unix-like systems)
- `--service --install`: install, enable, and start in one command
- `--service --disable`: stop + disable, keep data and service file
- `--service --uninstall`: stop + disable + delete everything (binary remains)
- Windows: run as Virtual Service Account (`NT SERVICE\{internal_name}`)

## Privilege Escalation Detection (PART 24)

### Linux (in order)
1. Already root (EUID == 0) → no prompt
2. sudo (if in sudoers/wheel group)
3. su (if knows root password)
4. pkexec (PolicyKit, if available)
5. doas (if configured)

### macOS (in order)
1. Already root (EUID == 0) → no prompt
2. sudo (must be in admin group)
3. osascript with administrator privileges (GUI prompt)

### BSD (in order)
1. Already root (EUID == 0) → no prompt
2. doas (OpenBSD default)
3. sudo (if installed)
4. su

### Windows (in order)
1. Already Administrator (elevated token) → no prompt
2. UAC prompt (requires GUI)
3. runas (requires admin password)

## Service Install Logic

```
--service --install:
1. Detect platform and init system
   Linux: systemd, OpenRC, runit, s6
   macOS: launchd
   BSD: rc.d
   Windows: Windows Service
2. If can escalate → system service (any port)
3. If cannot → user service (ports >1024 only)
4. Write service file → enable → start
```

## Service Manager Templates (PART 25)

| Platform | Path |
|----------|------|
| systemd (Linux) | `/etc/systemd/system/{internal_name}.service` |
| OpenRC (Alpine/Gentoo) | `/etc/init.d/{internal_name}` |
| macOS launchd | `/Library/LaunchDaemons/{project_org}.{internal_name}.plist` |
| Windows Service | `NT SERVICE\{internal_name}` |

**systemd:** `ProtectSystem=strict`, `ProtectHome=yes`, `PrivateTmp=yes`, explicit `ReadWritePaths` only

**Run mode summary:**

| Mode | Who Runs | Port Restriction | Privilege Drop |
|------|----------|-----------------|----------------|
| Service (escalated) | root/admin | Any port | Yes (after binding) |
| User mode ($USER) | Calling user | >1024 only | No |

For complete details, see AI.md PART 24, 25
