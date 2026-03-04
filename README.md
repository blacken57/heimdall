# Heimdall

A lightweight, self-hosted uptime monitor built for Dokploy + Raspberry Pi (ARM). Polls your services on a configurable interval, stores results in SQLite, and serves a live dashboard — no JavaScript framework, no external dependencies, single binary.

## Features

- **Simple config** — define services as numbered env var pairs; no YAML or config files
- **Live dashboard** — HTMX auto-refreshes every 30 seconds without a full page reload
- **30-day history bar** — per-day uptime segments with hover tooltips, UptimeRobot-style
- **ARM-native** — pure Go SQLite (`CGO_ENABLED=0`) cross-compiles to arm64/armv7 without a C toolchain
- **Persistent history** — SQLite with WAL mode; configurable retention window
- **Optional basic auth** — set two env vars to password-protect the dashboard
- **Tiny footprint** — ~15 MB Docker image, <10 MB binary

## Quick Start

### Local binary
```bash
git clone https://github.com/blacken57/heimdall
cd heimdall
go build -o heimdall ./cmd/heimdall

SERVICE_1_NAME="My Site" SERVICE_1_URL="https://example.com" ./heimdall
# Dashboard → http://localhost:8080
```

### Docker
```bash
docker build -t heimdall .

docker run -p 8080:8080 \
  -e SERVICE_1_NAME="My Site" \
  -e SERVICE_1_URL="https://example.com" \
  heimdall
```

### Docker Compose (Dokploy-ready)
```bash
cp .env.example .env
# edit .env with your services
docker compose up -d
```

## Configuration

All configuration is via environment variables. Copy `.env.example` to `.env` and edit as needed.

### Services

Define any number of services as sequential numbered pairs. Scanning stops at the first gap.

```env
SERVICE_1_NAME=My Website
SERVICE_1_URL=https://example.com

SERVICE_2_NAME=API Server
SERVICE_2_URL=https://api.example.com/health

SERVICE_3_NAME=Raspberry Pi
SERVICE_3_URL=http://raspberrypi.local:8080/health
```

### All Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVICE_N_NAME` | — | Display name for service N |
| `SERVICE_N_URL` | — | URL to poll for service N |
| `POLL_INTERVAL` | `60` | Seconds between checks |
| `HTTP_TIMEOUT` | `10` | Per-request timeout in seconds |
| `DB_PATH` | `./heimdall.db` | SQLite database file path |
| `DATA_RETENTION_DAYS` | `90` | Days of check history to retain |
| `PORT` | `8080` | Port for the web UI |
| `HEIMDALL_USER` | — | Basic auth username *(both must be set to enable)* |
| `HEIMDALL_PASSWORD` | — | Basic auth password |

### Basic Auth

Set both `HEIMDALL_USER` and `HEIMDALL_PASSWORD` to enable HTTP Basic Auth on the dashboard. The `/health` endpoint is always unauthenticated.

```env
HEIMDALL_USER=admin
HEIMDALL_PASSWORD=changeme
```

## Deployment on Dokploy + Raspberry Pi

1. Push to a Git repo Dokploy can access (GitHub, Gitea, etc.)
2. Create a new service in Dokploy, point it at this repo
3. Dokploy will build the Docker image — `CGO_ENABLED=0` means it cross-compiles cleanly for ARM with no extra setup
4. Set environment variables in the Dokploy UI
5. Attach a persistent volume at `/data` for the SQLite database

For a manual multi-arch build:
```bash
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t heimdall:latest \
  --push .
```

## Project Structure

```
heimdall/
├── cmd/heimdall/main.go              # Entry point, graceful shutdown
├── internal/
│   ├── config/config.go              # Env var parsing
│   ├── db/
│   │   ├── db.go                     # SQLite open, WAL mode, migrations
│   │   └── queries.go                # CRUD + aggregated stats
│   ├── checker/checker.go            # HTTP polling goroutines
│   └── api/
│       ├── server.go                 # Routing + basic auth middleware
│       └── handlers.go               # Template rendering
├── web/
│   ├── templates/                    # html/template files
│   └── static/style.css             # Dark theme, pure CSS
├── Dockerfile                        # Multi-stage alpine build
├── docker-compose.yml
└── .env.example
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Full dashboard page |
| `GET` | `/partials/services` | HTMX partial — service grid HTML fragment |
| `GET` | `/static/*` | Static assets |
| `GET` | `/health` | Health check — returns `ok` (always open) |

## Tech Stack

- **Go 1.25** — stdlib `net/http`, `html/template`
- **SQLite** — [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **HTMX 2.0.4** — partial page refresh, no build step
- **Docker** — multi-stage alpine build, non-root user

## License

MIT
