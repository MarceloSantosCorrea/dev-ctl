# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Regras Gerais

- SEMPRE responder em portugues brasileiro (pt-BR).
- SEMPRE finalizar as respostas com: **Bora Marcelo !!!**

## Build & Test Commands

```bash
# Build the binary
go build -o devctl ./cmd/devctl/

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/docker/
go test ./internal/core/project/

# Run a single test
go test ./internal/docker/ -run TestGenerateCompose

# Build frontend (from web/ directory)
cd web && npm run build

# Lint frontend
cd web && npm run lint
```

The Go binary requires `source ~/.zshrc` to find the go binary at `~/.local/go/bin/go`.

## Architecture

**devctl** is a Docker project manager with a web GUI. It creates and manages multi-container Docker environments (similar to Laravel Valet/Herd) using a Go backend + React frontend + Traefik reverse proxy + SQLite.

### Request flow

```
CLI (Cobra) or Web Dashboard (React on :19800)
  → REST API (Chi router, internal/api/)
    → Project Service (internal/core/project/service.go) — central business logic
      → Template loader (internal/core/project/template.go) — YAML templates from ~/.devctl/templates/
      → Port Manager (internal/ports/) — deterministic slot-based allocation (+10000 offset)
      → Docker Client (internal/docker/) — compose generation & container management
      → SSL Manager (internal/ssl/) — mkcert certificates
      → Hosts Manager (internal/hosts/) — /etc/hosts with managed block markers
      → Traefik Manager (internal/traefik/) — reverse proxy container + TLS routing
    → SQLite (internal/database/) — projects, services, port_allocations tables
```

### Key design decisions

- **`project.Service`** (`internal/core/project/service.go`) is the central orchestrator. All project lifecycle operations (create, up, down, delete) flow through it. It coordinates port allocation, compose generation, SSL, hosts, and Traefik.
- **Templates** are YAML files in `templates/` that define Docker service blueprints (image, ports, env, volumes, healthcheck). They're copied to `~/.devctl/templates/` on `devctl init`. Templates with a `dockerfile` field generate a `Dockerfile.<service>` alongside docker-compose.yml and use `build:` instead of `image:`.
- **`ProjectUp`** is the core compose pipeline: load templates → allocate ports → write nginx config (if nginx+php-fpm) → write Dockerfiles (if template has `dockerfile`) → generate docker-compose.yml → `docker compose up -d`.
- **Frontend is embedded** into the Go binary via `embed.go` (`//go:embed web/dist/*`). The `api.SetDistFS()` call in `main.go` registers it.
- **Port allocation** uses deterministic 10,000-slot offsets (slot 0 = base port, slot 1 = base+10000, etc.) validated against both the DB and `net.Listen()`.
- **Web services** (templates with `category: "web"` or `"proxy"`) trigger SSL cert generation, /etc/hosts entry, and Traefik label injection in compose.

### Nginx + PHP-FPM integration

When both nginx and php-fpm services exist in a project, `buildComposeSpecs` detects the php-fpm service via `findEnabledPHPFPM()`, then generates an nginx config with `fastcgi_pass` pointing to the php-fpm container. The `mapContainerRoot()` function translates document root paths between the nginx and php-fpm container mount points.

### Data layout at runtime

```
~/.devctl/
├── devctl.db                          # SQLite (WAL mode)
├── certs/                             # mkcert certificates per domain
├── traefik/dynamic/                   # Traefik TLS config
├── templates/                         # Service template YAMLs
└── projects/{name}/
    ├── docker-compose.yml
    ├── Dockerfile.<service>           # Only if template has dockerfile field
    └── nginx/<service>-default.conf   # Only for nginx services
```

## Code Conventions

- Constructor-based dependency injection: `NewService(db, cfg, portMgr, ...)` pattern throughout.
- Errors are wrapped with context: `fmt.Errorf("action: %w", err)`.
- All DB/Docker operations take `context.Context`.
- Container names follow `devctl-{project}-{service}`, networks follow `devctl-{project}`.
- Handler structs per resource type in `internal/api/handlers/` with methods matching HTTP verbs.
- The project is written in Brazilian Portuguese comments/docs context, but code identifiers are in English.
