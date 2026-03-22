# devctl

Gerenciador de ambientes Docker para desenvolvimento local, com interface web, reverse proxy automático e SSL — similar ao Laravel Valet/Herd, mas para qualquer stack.

## O que é

O **devctl** cria e gerencia ambientes Docker multi-container a partir de templates de serviço (nginx, PHP-FPM, PostgreSQL, Redis, etc.). Cada projeto recebe um domínio `.local` com HTTPS automático via [mkcert](https://github.com/FiloSottile/mkcert) e roteamento pelo [Traefik](https://traefik.io/).

**Stack:** Go (backend) + React (frontend embutido no binário) + SQLite + Traefik

## Funcionalidades

- Dashboard web em `http://devctl.local` (porta 19800)
- Criação de projetos com múltiplos serviços Docker a partir de templates
- Domínio `.local` com SSL automático (mkcert) por projeto
- Reverse proxy automático com Traefik
- Autenticação de usuários com ownership de projetos
- Adição, remoção e ativação/desativação de serviços em projetos existentes
- Suporte a WSL: sincroniza `/etc/hosts` com o Windows automaticamente
- Integração com Claude Code via servidor MCP

## Pré-requisitos

- Docker e Docker Compose v2
- Go 1.21+ (para compilar)
- `mkcert` instalado (ou use `devctl init` para instalar automaticamente)

## Instalação

```bash
# Compilar o binário
go build -o devctl ./cmd/devctl/

# Inicializar (cria diretórios, instala mkcert, inicia Traefik, configura sudoers)
sudo devctl init

# Iniciar o servidor
devctl serve
```

Acesse o dashboard em [http://devctl.local:19800](http://devctl.local:19800).

## Comandos CLI

### `devctl init [--ssl]`

Inicializa o ambiente do devctl. Realiza as seguintes etapas:

- Cria o diretório `~/.devctl/` com subdiretórios (certs, templates, projects)
- Instala o mkcert e o CA raiz (em WSL, sincroniza o CA com o Windows via PowerShell)
- Cria a rede Docker `devctl-proxy`
- Inicia o container do Traefik como reverse proxy
- Configura `/etc/sudoers.d/devctl-hosts` para edição sem senha do `/etc/hosts`
- Copia os templates de serviço para `~/.devctl/templates/`

```bash
devctl init          # inicialização básica
devctl init --ssl    # habilita geração de certificados SSL
```

### `devctl serve`

Inicia o servidor da API REST e o dashboard web na porta **19800**.

```bash
devctl serve
```

### `devctl up <nome>`

Sobe todos os containers de um projeto. Gera o `docker-compose.yml`, aloca portas e executa `docker compose up -d`.

```bash
devctl up meu-projeto
```

### `devctl down <nome>`

Para todos os containers de um projeto.

```bash
devctl down meu-projeto
```

### `devctl status`

Exibe uma tabela com nome, domínio, status e quantidade de serviços de todos os projetos.

```bash
devctl status
```

### `devctl hosts-sync`

Re-sincroniza os domínios de todos os projetos com `/etc/hosts`. Em ambientes WSL, também atualiza o arquivo `hosts` do Windows.

```bash
devctl hosts-sync
```

### `devctl mcp`

Inicia um servidor MCP (Model Context Protocol) via stdio para integração com Claude Code.

```bash
devctl mcp
```

## Templates de serviço disponíveis

| Template | Categoria | Imagem padrão | Portas | Descrição |
|----------|-----------|---------------|--------|-----------|
| `nginx` | web | nginx:alpine | 80 | Servidor web Nginx |
| `php-fpm` | runtime | php:8.3-fpm | 9000 | PHP-FPM (versões 7.1–8.5) com extensões pré-instaladas |
| `supervisord` | runtime | — | 9000, 9001 | PHP-FPM + Supervisord para workers, filas e websockets |
| `node` | runtime | node:20-alpine | 3000 | Node.js |
| `postgres` | database | postgres:16-alpine | 5432 | PostgreSQL |
| `mysql` | database | mysql:8.0 | 3306 | MySQL |
| `mongo` | database | mongo:7 | 27017 | MongoDB |
| `redis` | cache | redis:7-alpine | 6379 | Redis |
| `rabbitmq` | queue | rabbitmq:3.13-management | 5672, 15672 | RabbitMQ com interface de gerenciamento |
| `mailpit` | email | axllent/mailpit | 1025, 8025 | SMTP fake para captura de e-mails em desenvolvimento |

## Integração Nginx + PHP-FPM

Quando um projeto possui os serviços `nginx` e `php-fpm`, o devctl gera automaticamente a configuração do Nginx com `fastcgi_pass` apontando para o container PHP-FPM, incluindo o mapeamento correto do document root entre os containers.

## Integração com Claude Code (MCP)

O devctl expõe um servidor [MCP (Model Context Protocol)](https://modelcontextprotocol.io) que permite ao Claude Code interagir com seus projetos diretamente pelo editor.

### Configurar no Claude Code

Adicione ao seu arquivo de configuração do Claude Code:

```json
{
  "mcpServers": {
    "devctl": {
      "command": "/caminho/para/devctl",
      "args": ["mcp"]
    }
  }
}
```

### Ferramentas disponíveis via MCP

| Ferramenta | Descrição |
|------------|-----------|
| `list_projects` | Lista todos os projetos com status |
| `project_status` | Retorna status detalhado de um projeto |
| `project_up` | Sobe os containers de um projeto |
| `project_down` | Para os containers de um projeto |
| `project_rebuild` | Reconstrói e reinicia os containers de um projeto |
| `list_templates` | Lista os templates de serviço disponíveis |

> **Auto-detecção:** todas as ferramentas aceitam o parâmetro `name` como opcional. Se omitido, o MCP procura o arquivo `.devctl` no diretório atual e nos diretórios ancestrais (similar ao `.git`) para detectar o projeto automaticamente.

## API REST

URL base: `http://localhost:19800/api`

### Autenticação

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `POST` | `/auth/register` | Registrar usuário |
| `POST` | `/auth/login` | Login (retorna cookie de sessão) |
| `POST` | `/auth/logout` | Logout |
| `GET` | `/auth/me` | Dados do usuário autenticado |

### Projetos

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET` | `/projects` | Listar projetos do usuário |
| `POST` | `/projects` | Criar projeto |
| `GET` | `/projects/{id}` | Detalhes do projeto |
| `PUT` | `/projects/{id}` | Atualizar projeto |
| `DELETE` | `/projects/{id}` | Deletar projeto |
| `POST` | `/projects/{id}/up` | Subir containers |
| `POST` | `/projects/{id}/down` | Parar containers |
| `POST` | `/projects/{id}/rebuild` | Reconstruir containers |
| `GET` | `/projects/{id}/logs` | Stream de logs (SSE) |

### Serviços

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `POST` | `/projects/{id}/services` | Adicionar serviço |
| `PUT` | `/projects/{id}/services/{sid}` | Atualizar serviço |
| `DELETE` | `/projects/{id}/services/{sid}` | Remover serviço |
| `PATCH` | `/projects/{id}/services/{sid}/toggle` | Ativar/desativar serviço |

### Templates e Sistema

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| `GET` | `/templates` | Listar templates disponíveis |
| `GET` | `/system/status` | Status do Docker |

## Exemplo de uso

```bash
# 1. Inicializar
sudo devctl init

# 2. Iniciar o dashboard
devctl serve

# 3. Abrir http://devctl.local:19800 no navegador
# 4. Criar um projeto pela interface:
#    - Nome: "meu-app"
#    - Caminho: "/home/user/projetos/meu-app"
#    - Serviços: Nginx + PHP-FPM + MySQL + Redis

# 5. Subir o projeto
devctl up meu-app

# 6. Acessar em https://meu-app.local
```

## Estrutura de dados em tempo de execução

```
~/.devctl/
├── devctl.db                    # Banco SQLite (modo WAL)
├── certs/                       # Certificados mkcert por domínio
├── traefik/dynamic/             # Configuração dinâmica do Traefik
├── templates/                   # Templates de serviço YAML
└── projects/{nome}/
    ├── docker-compose.yml
    ├── Dockerfile.<serviço>     # Apenas se o template tiver campo dockerfile
    └── nginx/<serviço>-default.conf

{caminho-do-projeto}/
└── .devctl                      # Marcador JSON com name/id/domain (usado pelo MCP)
```

## Desenvolvimento

```bash
# Compilar
go build -o devctl ./cmd/devctl/

# Testes
go test ./...

# Buildar o frontend (dentro de web/)
cd web && npm run build

# Lint do frontend
cd web && npm run lint
```

> O frontend React é embutido no binário via `embed.go`. É necessário fazer o build do frontend antes de compilar o Go para produção.
