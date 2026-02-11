# devctl — Docker Project Manager with GUI

## Stack
Go (backend/CLI) + React/Vite (frontend) + Traefik (reverse proxy) + SQLite + mkcert

## Database Schema (SQLite)

```sql
CREATE TABLE projects (
    id TEXT PRIMARY KEY, name TEXT UNIQUE NOT NULL,
    domain TEXT UNIQUE NOT NULL, status TEXT DEFAULT 'stopped',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE services (
    id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    template_name TEXT NOT NULL, name TEXT NOT NULL,
    enabled BOOLEAN DEFAULT 1, config TEXT DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE port_allocations (
    id TEXT PRIMARY KEY, service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    internal_port INTEGER NOT NULL, external_port INTEGER NOT NULL,
    protocol TEXT DEFAULT 'tcp', UNIQUE(external_port, protocol)
);
```

## Port Allocation
Deterministic offset +10000: 1st=base, 2nd=base+10000, 3rd=base+20000. Validates with net.Listen().

## API Endpoints
- `GET/POST /api/projects` — List/Create
- `GET/PUT/DELETE /api/projects/:id` — CRUD
- `POST /api/projects/:id/up|down` — Start/Stop
- `GET /api/projects/:id/logs` — SSE stream
- `POST/PUT/DELETE/PATCH /api/projects/:id/services/:sid` — Service CRUD + toggle
- `GET /api/templates` — List templates
- `GET /api/system/status` — Health check
