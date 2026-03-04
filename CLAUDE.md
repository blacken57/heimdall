# Heimdall — Claude Code Guide

## Project Overview
Lightweight self-hosted uptime monitor. Polls URLs at a configurable interval, stores results in SQLite, serves a live dashboard via `html/template` + HTMX. Single binary, minimal Docker image, no CGO.

## Commands

### Build & Run
```bash
go build ./cmd/heimdall           # compile binary to ./heimdall
go build -o /tmp/heimdall ./cmd/heimdall  # compile to specific path

# Run locally (minimum config)
SERVICE_1_NAME=Test SERVICE_1_URL=https://example.com ./heimdall

# Run with all options
SERVICE_1_NAME=Test SERVICE_1_URL=https://example.com \
  POLL_INTERVAL=30 PORT=9090 DB_PATH=./test.db ./heimdall
```

### Testing
```bash
go vet ./...                      # lint all packages
go build ./...                    # ensure everything compiles
```

There are no automated tests yet. Verify manually:
1. Server starts and dashboard loads at `http://localhost:8080`
2. Rows appear in the `checks` table after one poll interval
3. `/partials/services` returns an HTML fragment (not a full page)
4. `/health` returns `ok` (no auth required even when basic auth is enabled)

### Docker
```bash
docker build -t heimdall .
docker run -p 8080:8080 \
  -e SERVICE_1_NAME=Test \
  -e SERVICE_1_URL=https://example.com \
  heimdall

# Multi-arch build (requires docker buildx)
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t heimdall .
```

## Architecture

### Package Layout
```
cmd/heimdall/main.go        — entry point: wires config→db→checker→api, graceful shutdown
internal/config/config.go   — env var parsing; Load() returns *Config or error
internal/db/db.go           — Open() SQLite with WAL mode + schema migrations
internal/db/queries.go      — UpsertService, InsertCheck, GetAllServiceSummaries, PurgeOldChecks
internal/checker/checker.go — one goroutine per service, immediate first poll + ticker
internal/api/server.go      — http.ServeMux, route registration, basicAuth middleware
internal/api/handlers.go    — full-page + HTMX partial handlers, serviceView mapping
web/templates/              — html/template files loaded from disk at request time
web/static/style.css        — dark theme, CSS variables, no build step
```

### Key Design Decisions
- **`CGO_ENABLED=0`** — pure Go SQLite via `modernc.org/sqlite`; essential for ARM cross-compile without a C toolchain
- **WAL mode** — allows concurrent reads (HTTP handlers) while checker goroutines write
- **One goroutine per service** — idiomatic Go; negligible overhead for typical service counts
- **Templates loaded from disk** — easy iteration during development; switch to `//go:embed` later for single-binary deploy
- **No JSON API** — HTMX partial (`GET /partials/services`) returns an HTML fragment; no JS framework needed
- **Basic auth is opt-in** — only enabled when both `HEIMDALL_USER` and `HEIMDALL_PASSWORD` are set; `/health` is always open

### Data Flow
```
config.Load() → db.Open() → db.UpsertService() × N
                          → checker.New().Run(ctx)  [goroutine per service]
                                 └─ poll() → db.InsertCheck()
                          → api.New() → http.ListenAndServe()
                                 ├─ GET /                → handleIndex (full page)
                                 ├─ GET /partials/services → handlePartialServices (fragment)
                                 ├─ GET /static/          → FileServer(web/static)
                                 └─ GET /health           → "ok"
```

## Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `SERVICE_N_NAME` | — | Service name (N = 1, 2, …; scanning stops at first gap) |
| `SERVICE_N_URL` | — | Service URL to poll |
| `POLL_INTERVAL` | `60` | Seconds between checks |
| `HTTP_TIMEOUT` | `10` | HTTP request timeout in seconds |
| `DB_PATH` | `./heimdall.db` | SQLite file path |
| `DATA_RETENTION_DAYS` | `90` | Days to keep check history |
| `PORT` | `8080` | Web UI port |
| `HEIMDALL_USER` | — | Basic auth username (both must be set to enable) |
| `HEIMDALL_PASSWORD` | — | Basic auth password |

## Git Commit Rules
- Do NOT add "Co-authored-by: Claude" to any commits in this project
