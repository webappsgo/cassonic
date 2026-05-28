# Project Rules (PART 2, 3, 4)

Read: AI.md PART 2, 3, 4

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Use GPL/AGPL/LGPL licensed dependencies
- Hardcode {project_name} or {project_org} — always infer from git/path
- Create forbidden directories (config/, data/, logs/ in root)
- Create forbidden files (CHANGELOG.md, SUMMARY.md, COMPLIANCE.md, etc.)
- Put Dockerfile in project root
- Put docker-compose.yml in project root
- Create .env files

## CRITICAL - ALWAYS DO
- MIT License — all our code
- Attribute all third-party licenses in LICENSE.md
- Infer PROJECTNAME/PROJECTORG from git remote or directory
- Use singular directory names (handler/, model/, not handlers/, models/)
- All documentation UPPERCASE.md (README.md, LICENSE.md)
- All Go files lowercase snake_case

## PATHS
| Context | Config | Data | Logs |
|---------|--------|------|------|
| Linux privileged | `/etc/local/cassonic/` | `/var/lib/local/cassonic/` | `/var/log/local/cassonic/` |
| Linux user | `~/.config/local/cassonic/` | `~/.local/share/local/cassonic/` | `~/.local/log/local/cassonic/` |
| Container | `/config/cassonic/` | `/data/cassonic/` | `/data/log/cassonic/` |

## ALLOWED ROOT FILES
AI.md, IDEA.md, CLAUDE.md, README.md, LICENSE.md, Makefile, go.mod, go.sum, release.txt, .gitignore, .dockerignore, Jenkinsfile, mkdocs.yml, .readthedocs.yaml, renovate.json, .gitlab-ci.yml

---
For complete details, see AI.md PART 2, 3, 4
