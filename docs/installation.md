# Installation

## Pre-built Binary

Download the latest binary for your platform from the [releases page](https://github.com/local/cassonic/releases).

```bash
# Linux amd64
curl -Lo cassonic https://github.com/local/cassonic/releases/latest/download/cassonic-linux-amd64
chmod +x cassonic
sudo mv cassonic /usr/local/bin/cassonic
```

## Docker

```bash
docker pull ghcr.io/local/cassonic:latest
```

See [docker/docker-compose.yml](https://github.com/local/cassonic/blob/main/docker/docker-compose.yml) for a full Compose example.

## Build from Source

Requires Docker (Go toolchain runs in a container).

```bash
git clone https://github.com/local/cassonic.git
cd cassonic
make local
```

## First Run

```bash
cassonic
```

On first run, cassonic auto-creates the configuration directory and generates a setup token displayed in the startup banner.
