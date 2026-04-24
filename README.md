# CS464 Card Game Backend

A multiplayer online card battle game backend built with Go. Players collect cards through packs, build decks, queue for matchmaking, and battle in real-time over WebSocket.

## Services

| Service | Port | Description |
|---------|------|-------------|
| **Auth** | 8000 | Registration, login, session management, bans |
| **Matchmaking** | 8001 | MMR-based queue and match pairing |
| **Gameplay** | 8002 | Real-time game engine over WebSocket (4 ticks/sec) |
| **Deck** | 8005 | Cards, decks, packs, leveling, crystals |
| **Friendlist** | 8003 | Friend management |
| **Replay** | 8004 | Game replay storage |
| **Cursor UDP** | 9001/UDP | Real-time cursor broadcast (Rust) |
| **MySQL** | 3306 | Persistent storage |
| **Redis** | 6379 | Session cache, queue state, match notifications |

## Project Structure

```
backend/
├── services/
│   ├── auth/           # Auth service
│   ├── matchmaking/    # Matchmaking service
│   ├── gameplay/       # Gameplay service (WebSocket)
│   ├── deck/           # Deck service
│   ├── friendlist/     # Friendlist service
│   ├── replay/         # Replay service
│   └── cursor-udp/     # Cursor UDP service (Rust)
├── db/
│   ├── migrations/     # SQL migration files (up/down)
│   ├── queries/        # sqlc query definitions
│   └── sqlc/           # Generated Go database bindings
├── shared/             # Shared Go types across services
├── docs/diagrams/      # Architecture diagrams (.drawio)
├── docker-compose.yml
├── Makefile
└── go.work             # Go workspace
```

## Prerequisites

- [Go](https://go.dev/dl/) 1.25+
- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [Make](https://www.gnu.org/software/make/)
- [golang-migrate](https://github.com/golang-migrate/migrate) (`brew install golang-migrate`)
- [sqlc](https://sqlc.dev/) (`brew install sqlc`)

## Getting Started

### 1. Set up environment variables

```bash
cp .env.example .env
```

Edit `.env` and set secure passwords for `MYSQL_PASSWORD`, `MYSQL_ROOT_PASSWORD`, and `REDIS_PASSWORD`.

### 2. Start MySQL and Redis

```bash
docker-compose up -d mysql redis
```

Wait for the health checks to pass (about 10-15 seconds).

### 3. Run database migrations

```bash
make migrate-up
```

### 4. Start the services

**With Docker (recommended):**

```bash
docker-compose up -d
```

**Locally (for development):**

```bash
# Run individual services
make run-auth
make run-matchmaking
make run-gameplay
make run-deck

# Or all at once
make run-all
```

### 5. Verify

Each service exposes a health check endpoint:

```bash
curl http://localhost:8000/health   # Auth
curl http://localhost:8001/health   # Matchmaking
curl http://localhost:8002/health   # Gameplay
curl http://localhost:8005/health   # Deck
```

## Development

### Running locally

Services need MySQL and Redis running. Start them with Docker, then run Go services directly for faster iteration:

```bash
# Start infra only
docker-compose up -d mysql redis

# Run the service you're working on
cd services/gameplay
go run .
```

### Database changes

1. Create a new migration:
   ```bash
   make migrate-create NAME=add_some_table
   ```
2. Write your SQL in the generated `db/migrations/XXXXXX_add_some_table.up.sql` and `.down.sql` files.
3. Run the migration:
   ```bash
   make migrate-up
   ```
4. If you changed queries, regenerate the Go bindings:
   ```bash
   make sqlc-generate
   ```

### Useful Make commands

| Command | Description |
|---------|-------------|
| `make build` | Build all services |
| `make test` | Run all tests |
| `make fmt` | Format code (gofumpt) |
| `make lint` | Lint code (golangci-lint) |
| `make tidy` | Tidy Go module dependencies |
| `make docker-up` | Start all containers |
| `make docker-down` | Stop all containers |
| `make migrate-up` | Run pending migrations |
| `make migrate-down` | Rollback last migration |
| `make migrate-reset` | Drop everything and re-run all migrations |
| `make sqlc-generate` | Regenerate Go code from SQL queries |

### Adding a new service

1. Create the directory: `mkdir -p services/my-service`
2. Init the module: `cd services/my-service && go mod init github.com/kKar1503/cs464-backend/services/my-service`
3. Add to `go.work`
4. Create a `Dockerfile` and add the service to `docker-compose.yml`
5. Add build/run targets to the `Makefile`

## Service Status

### Done

- [x] **Auth Service** - Registration, login/logout, bearer token sessions, Redis-cached validation, user bans
- [x] **Matchmaking Service** - MMR-based queue, expanding-range matching algorithm, match accept/reject, background matchmaker loop
- [x] **Gameplay Service** - WebSocket game engine, server-authoritative tick-based game loop, card placement, elixir system, auto-attack combat, card abilities/effects engine
- [x] **Deck Service** - Card collection, deck CRUD, deck validation, pack opening with crystal rewards, card leveling, disenchanting, starter content
- [x] **Database** - MySQL schema with 20+ migrations, sqlc-generated type-safe queries
- [x] **Infrastructure** - Docker Compose orchestration, Redis caching, health checks on all services

### Not Done

- [ ] **Friendlist Service** - Stub only (health check endpoint)
- [ ] **Replay Service** - Stub only (health check endpoint)
- [ ] **Cursor UDP Service** - Not implemented
- [ ] **Rate limiting** - No rate limiting on API endpoints
- [ ] **HTTPS** - Not enforced (would be handled by a reverse proxy in production)
- [ ] **API Gateway** - Services are accessed directly, no unified gateway

## Diagrams

Architecture diagrams are in [`docs/diagrams/`](docs/diagrams/). Open `.drawio` files with [draw.io](https://app.diagrams.net/) or the VS Code draw.io extension. See the [diagrams README](docs/diagrams/README.md) for descriptions.
