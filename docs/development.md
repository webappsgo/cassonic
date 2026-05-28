# Development

## Prerequisites

- Docker (Go toolchain runs inside a container)
- `make`
- `git`

## Quick Start

```bash
# Clone the repo
git clone https://github.com/local/cassonic.git
cd cassonic

# Quick dev build (to temp dir)
make dev

# Run unit tests
make test

# Build all platforms
make build
```

## Project Structure

```
src/                 # Go source code
src/main.go          # Server entry point
src/client/          # CLI companion
docker/              # Docker files
docker/Dockerfile    # Production Dockerfile
docker/rootfs/       # Container filesystem overlay
docs/                # MkDocs documentation
.github/workflows/   # GitHub Actions
Makefile             # Build targets
release.txt          # Version source of truth
```

## Spec

All implementation rules are in `AI.md`. Read the relevant PART before implementing any feature.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests for new behavior
4. Ensure `make test` passes
5. Submit a pull request
