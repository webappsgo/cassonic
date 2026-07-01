# Project Rules (PART 2, 3, 4)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use GPL/AGPL/LGPL licensed dependencies — they force the project to be GPL
- Hardcode `{project_name}` or `{project_org}` — always infer from git remote or path
- Change `{internal_name}` after first-time setup (it is FROZEN forever)
- Rename `internal_name` without a coordinated migration of all on-disk paths
- Place Dockerfile or docker-compose.yml in the project root
- Put non-ReadTheDocs files in `docs/` directory

## CRITICAL - ALWAYS DO

- Include `LICENSE.md` with MIT license text in project root
- Attribute all third-party dependencies in LICENSE.md (compact format for 10+ deps)
- Infer project_name from git remote: `git remote get-url origin | sed -E 's|.*/([^/]+)(\.git)?$|\1|'`
- Infer project_org from git remote: `git remote get-url origin | sed -E 's|.*/([^/]+)/[^/]+(\.git)?$|\1|'`
- Keep `{internal_name}` === `{project_name}` at first setup, then freeze it

## Project Identity

| Variable | Value | Notes |
|----------|-------|-------|
| project_name | cassonic | May change on rename |
| project_org | webappsgo | GitHub organization |
| internal_name | cassonic | FROZEN — never edit |
| plist_name | io.github.webappsgo.cassonic | Derived, not stored |

## Key Rules Summary

| Rule | Description |
|------|-------------|
| MIT License | ALL projects use MIT; include in LICENSE.md |
| Embedded licenses | ALL dependencies attributed in LICENSE.md |
| No GPL deps | Never use GPL/AGPL/LGPL dependencies |
| `internal_name` frozen | Set once, never changed after first setup |
| Git remote is source | Infer names from git remote, not directory |

## OS-Specific Paths (PART 4)

| Platform | Config | Data | Logs |
|----------|--------|------|------|
| Linux | `/etc/cassonic/` or `~/.config/cassonic/` | `/var/lib/cassonic/` | `/var/log/cassonic/` |
| macOS | `~/Library/Application Support/cassonic/` | `~/Library/Application Support/cassonic/data/` | `~/Library/Logs/cassonic/` |
| Windows | `%APPDATA%\cassonic\` | `%LOCALAPPDATA%\cassonic\` | `%LOCALAPPDATA%\cassonic\logs\` |
| Container | `/config/cassonic/` | `/data/cassonic/` | `/data/log/cassonic/` |

For complete details, see AI.md PART 2, 3, 4
