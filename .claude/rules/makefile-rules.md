# Makefile Rules (PART 26)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Change Makefile target names/structure from spec (build, release, docker, test, dev, clean)
- Run go/cargo commands outside Docker from Makefile targets

## CRITICAL - ALWAYS DO

- Keep make dev/local/build/test/docker/clean targets working per PART 26
- Route all toolchain invocations through Docker containers

## Key Rules Summary

- Makefile targets and Docker command patterns are standardized across all projects — do not diverge

For complete details, see AI.md PART 26
