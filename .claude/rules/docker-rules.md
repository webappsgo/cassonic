# Docker Rules (PART 27)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Place Dockerfile or docker-compose.yml in project root (always in `docker/`)
- Use pre-built binaries in Dockerfile — always multi-stage build
- Modify ENTRYPOINT or CMD in Dockerfile — always use `entrypoint.sh`
- Mount `./volumes/` or `./docker/rootfs/` from the project directory at runtime
- Use `docker-compose.yml` or `docker-compose.dev.yml` as AI — human only
- Create runtime/test data in the project directory
- Create `docker/Dockerfile.build` for Go projects (forbidden; use `casjaysdev/go:latest`)
- Skip OCI labels — all labels are REQUIRED
- Use LABEL blocks for multi-arch metadata — use OCI annotations only

## CRITICAL - ALWAYS DO

- Multi-stage build: builder=`casjaysdev/go:latest`, runtime=`alpine:latest`
- Dockerfile at `docker/Dockerfile`, not project root
- Entrypoint at `docker/rootfs/usr/local/bin/entrypoint.sh`
- Use `tini` as init system
- Expose port `80` internally
- Set `ENV MODE=development` in the image
- Install: git, curl, bash, tini, tor (in runtime image)
- All OCI labels on every Dockerfile
- Volume mounts: `./volumes/config:/config:z` and `./volumes/data:/data:z` only
- SQLite databases in `/data/db/sqlite/` (never scattered)
- Database names: always `server.db` and `users.db`
- `STOPSIGNAL SIGRTMIN+3`
- For AI testing: use `docker-compose.test.yml` only, in a temp dir

## Docker Directory Structure

```
docker/
├── Dockerfile              # Production (multi-stage)
├── Dockerfile.dev          # Devel image (:devel tag)
├── docker-compose.yml      # Production — HUMAN ONLY
├── docker-compose.dev.yml  # Development — HUMAN ONLY
├── docker-compose.test.yml # Automated testing — AI USE ONLY
└── rootfs/
    └── usr/local/bin/
        └── entrypoint.sh  # Container entrypoint (REQUIRED)
```

## Container Path Layout

| Path | Purpose |
|------|---------|
| `/config/{project_name}/` | App config (server.yml, ssl/, tor/) |
| `/data/{project_name}/` | App data (uploads, cache, tor/) |
| `/data/db/sqlite/` | SQLite databases (server.db, users.db) |
| `/data/db/postgres/` | PostgreSQL data |
| `/data/db/valkey/` | Valkey/Redis data |
| `/data/log/{project_name}/` | App logs |
| `/data/backups/{project_name}/` | Backup archives |
| `/usr/local/bin/{project_name}` | Application binary |

## Volume Mounts (all compose files)

```yaml
volumes:
  - './volumes/config:/config:z'
  - './volumes/data:/data:z'
```

## AI Testing Workflow

```bash
mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX")
mkdir -p "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
cp docker/docker-compose.test.yml "$TEMP_DIR/docker-compose.yml"
cd "$TEMP_DIR" && docker compose up -d
# run tests...
docker compose -f "$TEMP_DIR/docker-compose.yml" down
rm -rf "$TEMP_DIR"
```

## OCI Multi-Arch Annotations

Use `docker/metadata-action` in GitHub Actions — annotations on the manifest index, NOT LABEL blocks.

```yaml
- uses: docker/metadata-action@80c7e94dd9b9319bd5eb7a0e0fe9291e23a2a2e9  # v6.1.0
```

For complete details, see AI.md PART 27
