# CS464 Game Backend - Monorepo

A microservices-based game backend built with Go, featuring matchmaking, gameplay, friend management, and replay systems.

## Project Structure

```
backend/
├── services/
│   ├── matchmaking/    # Matchmaking service (MMR-based matching)
│   ├── gameplay/       # Game state and validation service
│   ├── friendlist/     # Friend management service
│   ├── replay/         # Game replay service
│   └── cursor-udp/     # Rust-based UDP cursor broadcast service (TBD)
├── shared/             # Shared Go code and types
├── bin/                # Compiled binaries (gitignored)
├── go.work             # Go workspace configuration
├── docker-compose.yml  # Docker orchestration
├── Makefile           # Build automation
└── README.md
```

## Services Overview

### Matchmaking Service (Port 8001)
Responsible for matching players based on their MMR (Matchmaking Rating). When two players are matched, it creates a game session and hands it off to the gameplay service.

### Gameplay Service (Port 8002)
Handles game state management, validates player actions, and orchestrates the gameplay flow between matched players.

### Friendlist Service (Port 8003)
Manages player friend relationships, friend requests, and social features.

### Replay Service (Port 8004)
Stores and serves game replay data for post-game analysis.

### Cursor UDP Service (Port 9001/UDP)
Written in Rust, broadcasts real-time cursor movements using UDP for low-latency updates.

### MySQL Database (Port 3306)
Persistent storage for users, game sessions, friendships, and replays. See [DATABASE.md](DATABASE.md) for schema details.

### Redis Cache (Port 6379)
In-memory cache for sessions, matchmaking queues, and real-time game state.

## Prerequisites

- Go 1.23 or later
- Docker and Docker Compose (for containerized deployment)
- Make (for build automation)
- Rust (for cursor-udp service, when implemented)

## Getting Started

### Local Development

1. Clone the repository:
```bash
cd backend
```

2. Build all services:
```bash
make build
```

3. Run individual services:
```bash
make run-matchmaking
make run-gameplay
make run-friendlist
make run-replay
```

4. Run all services:
```bash
make run-all
```

### Using Docker Compose

1. Build Docker images:
```bash
make docker-build
```

2. Start all services:
```bash
make docker-up
```

3. Stop all services:
```bash
make docker-down
```

Or use docker-compose directly:
```bash
docker-compose up -d     # Start in detached mode
docker-compose logs -f   # View logs
docker-compose down      # Stop all services
```

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make help` | Show all available commands |
| `make build` | Build all Go services |
| `make build-<service>` | Build a specific service |
| `make run-<service>` | Run a specific service |
| `make run-all` | Run all services |
| `make test` | Run all tests |
| `make clean` | Clean build artifacts |
| `make docker-build` | Build Docker images |
| `make docker-up` | Start services with docker-compose |
| `make docker-down` | Stop services with docker-compose |
| `make tidy` | Tidy Go module dependencies |
| `make fmt` | Format Go code |
| `make lint` | Run linter (requires golangci-lint) |

## Security

### Generate Strong Passwords

Use the provided scripts to generate secure random passwords:

```bash
# Generate new passwords (display only)
./scripts/generate-passwords.sh

# Generate and update .env automatically
./scripts/update-env-passwords.sh

# Manual generation
openssl rand -base64 32
```

See [scripts/README.md](scripts/README.md) for more details.

## Service Ports

- Matchmaking: `8001`
- Gameplay: `8002`
- Friendlist: `8003`
- Replay: `8004`
- Cursor UDP: `9001/udp` (TBD)

## Development Workflow

### Adding a New Service

1. Create service directory:
```bash
mkdir -p services/new-service
```

2. Initialize Go module:
```bash
cd services/new-service
go mod init github.com/kKar1503/cs464-backend/services/new-service
```

3. Add to `go.work`:
```
use (
    ...
    ./services/new-service
)
```

4. Create Dockerfile and add to docker-compose.yml

5. Update Makefile with build/run targets

### Working with Shared Code

The `shared/` directory contains common types and utilities used across services. To use shared code in a service:

```go
import "github.com/kKar1503/cs464-backend/shared"
```

### Testing

Run tests for all services:
```bash
make test
```

Or test individual modules:
```bash
cd services/matchmaking
go test ./...
```

## Architecture

This monorepo uses Go workspaces to manage multiple services while sharing common code. Each service is independently deployable and can be scaled separately using Docker.

Services communicate via HTTP/REST APIs (and UDP for cursor movements). The architecture supports:

- Independent service deployment
- Shared code reuse via the `shared/` module
- Container orchestration via Docker Compose
- Simple build automation via Makefile

## Next Steps

- [ ] Implement matchmaking algorithm
- [ ] Add game state management in gameplay service
- [ ] Implement friend system
- [ ] Add replay recording and playback
- [ ] Create Rust-based cursor UDP service
- [ ] Add database integration (PostgreSQL/Redis)
- [ ] Implement authentication and authorization
- [ ] Add API gateway
- [ ] Set up monitoring and logging
- [ ] Write comprehensive tests

## License

[Add your license here]
