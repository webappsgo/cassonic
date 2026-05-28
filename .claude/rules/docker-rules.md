# Docker Rules (PART 27)

Read: AI.md PART 27

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Put Dockerfile in project root
- Modify ENTRYPOINT or CMD (use entrypoint.sh)
- Use LABEL blocks in Dockerfile (use OCI annotations)
- Pre-build binaries before Docker build
- Use docker-compose.yml in project root

## CRITICAL - ALWAYS DO
- Multi-stage Dockerfile: golang:alpine builder + alpine:latest runtime
- Location: `docker/Dockerfile`
- Required packages: git, curl, bash, tini, tor
- ENTRYPOINT: `["tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh"]`
- STOPSIGNAL: `SIGRTMIN+3`
- Internal port: always 80
- ENV MODE=development (container default)
- OCI annotations via docker/metadata-action (not LABEL)

## VOLUMES
```yaml
volumes:
  - './volumes/config:/config:z'
  - './volumes/data:/data:z'
```

## CONTAINER PATHS
| Path | Purpose |
|------|---------|
| `/config/cassonic/` | App config |
| `/data/cassonic/` | App data |
| `/data/db/sqlite/` | SQLite databases |
| `/data/log/cassonic/` | App logs |
| `/usr/local/bin/cassonic` | Binary |

---
For complete details, see AI.md PART 27
