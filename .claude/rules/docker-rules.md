# Docker Rules (PART 27)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use a Dockerfile.build unless the casjaysdev image cannot satisfy a genuine need
- Skip multi-stage builds or entrypoint.sh conventions

## CRITICAL - ALWAYS DO

- Keep docker/Dockerfile, docker-compose.yml, docker-compose.dev.yml, docker-compose.test.yml in sync with PART 27
- Verify docker/rootfs entrypoint matches spec exactly

## Key Rules Summary

- Docker is the mandatory build environment; never build the toolchain on host
- Toolchain image: casjaysdev/go:latest for this Go project unless the project declares otherwise

For complete details, see AI.md PART 27
