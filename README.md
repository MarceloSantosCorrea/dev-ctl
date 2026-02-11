# devctl

Docker project manager with GUI. Manages multiple Docker projects with automatic port allocation, local domains with HTTPS, and a web dashboard.

## Prerequisites

- **Docker** (with Docker Compose v2)
- **mkcert** (for local HTTPS certificates)

## Installation

```bash
go build -o devctl ./cmd/devctl/
```

## CLI Commands

### `devctl init`

Initializes the environment: installs mkcert CA, creates the `devctl-proxy` Docker network, and starts the Traefik reverse proxy container.

```bash
devctl init
```

### `devctl serve`

Starts the API server and web dashboard on port **19800**.

```bash
devctl serve
```

Access the dashboard at [http://localhost:19800](http://localhost:19800).

### `devctl up <project-name>`

Starts all containers for a project. Generates the `docker-compose.yml`, allocates ports, and runs `docker compose up`.

```bash
devctl up my-project
```

### `devctl down <project-name>`

Stops all containers for a project.

```bash
devctl down my-project
```

### `devctl status`

Shows a table with name, domain, status, and service count for all projects.

```bash
devctl status
```

## Usage Example

```bash
# 1. Initialize the environment
devctl init

# 2. Start the dashboard
devctl serve

# 3. Open http://localhost:19800 in your browser
# 4. Create a new project via the dashboard:
#    - Set project name (e.g., "my-app")
#    - Set project path (e.g., "/home/user/projects/my-app")
#    - Select services: Nginx + PHP-FPM + MySQL + Redis
#    - Review and create

# 5. Start the project
devctl up my-app

# 6. Access at https://my-app.local
```

## REST API

Base URL: `http://localhost:19800/api`

### Projects

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/projects` | List all projects |
| `POST` | `/projects` | Create a new project |
| `GET` | `/projects/{id}` | Get project details |
| `PUT` | `/projects/{id}` | Update a project |
| `DELETE` | `/projects/{id}` | Delete a project |
| `POST` | `/projects/{id}/up` | Start project containers |
| `POST` | `/projects/{id}/down` | Stop project containers |
| `GET` | `/projects/{id}/logs` | Stream project logs (SSE) |

### Services

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/projects/{id}/services` | Add a service to a project |
| `PUT` | `/projects/{id}/services/{sid}` | Update a service |
| `DELETE` | `/projects/{id}/services/{sid}` | Remove a service |
| `PATCH` | `/projects/{id}/services/{sid}/toggle` | Enable/disable a service |

### Templates & System

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/templates` | List available service templates |
| `GET` | `/system/status` | Check Docker and system status |

### Create Project Example

```json
POST /api/projects
{
  "name": "my-app",
  "path": "/home/user/projects/my-app",
  "services": [
    { "template_name": "nginx", "name": "nginx" },
    { "template_name": "php-fpm", "name": "php-fpm" },
    { "template_name": "mysql", "name": "mysql" },
    { "template_name": "redis", "name": "redis" }
  ]
}
```

## Available Templates

| Template | Category | Default Image | Ports | Description |
|----------|----------|---------------|-------|-------------|
| `nginx` | web | nginx:alpine | 80 | Nginx web server and reverse proxy |
| `php-fpm` | runtime | php:8.3-fpm | 9000 | PHP FastCGI Process Manager |
| `node` | runtime | node:22-alpine | 3000 | Node.js JavaScript runtime |
| `mysql` | database | mysql:8.0 | 3306 | MySQL relational database |
| `postgres` | database | postgres:16-alpine | 5432 | PostgreSQL relational database |
| `mongo` | database | mongo:7 | 27017 | MongoDB document database |
| `redis` | cache | redis:7-alpine | 6379 | Redis in-memory data store |
| `rabbitmq` | messaging | rabbitmq:3-management-alpine | 5672, 15672 | RabbitMQ message broker |

### Project Path Mounting

Templates with `mount_project_path` (nginx, php-fpm, node) automatically mount the project's local directory as a bind volume when a project path is configured. Infrastructure services (mysql, postgres, redis, mongo, rabbitmq) use named Docker volumes for data persistence.

### Connection Info

Database and infrastructure templates include `connection_info` that is resolved and displayed in the dashboard's service cards, showing host, port, credentials, and a ready-to-use connection string.
