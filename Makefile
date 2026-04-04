.PHONY: help build build-auth build-matchmaking build-gameplay build-friendlist build-replay build-cursor-udp clean test run-auth run-matchmaking run-gameplay run-friendlist run-replay run-cursor-udp run-all docker-build docker-up docker-down sqlc-generate migrate-up migrate-down migrate-create migrate-force migrate-drop migrate-reset

# Database configuration
DB_URL := "mysql://$(shell grep MYSQL_USER .env | cut -d'=' -f2):$(shell grep MYSQL_PASSWORD .env | cut -d'=' -f2 | sed 's/+/%2B/g')@tcp(localhost:$(shell grep MYSQL_PORT .env | cut -d'=' -f2))/$(shell grep MYSQL_DATABASE .env | cut -d'=' -f2)"

# Default target
help:
	@echo "Available targets:"
	@echo "  build              - Build all Go services"
	@echo "  build-auth         - Build auth service"
	@echo "  build-matchmaking  - Build matchmaking service"
	@echo "  build-gameplay     - Build gameplay service"
	@echo "  build-friendlist   - Build friendlist service"
	@echo "  build-replay       - Build replay service"
	@echo "  build-cursor-udp   - Build cursor UDP service (Rust)"
	@echo "  run-auth           - Run auth service"
	@echo "  run-matchmaking    - Run matchmaking service"
	@echo "  run-gameplay       - Run gameplay service"
	@echo "  run-friendlist     - Run friendlist service"
	@echo "  run-replay         - Run replay service"
	@echo "  run-cursor-udp     - Run cursor UDP service (Rust)"
	@echo "  run-all            - Run all services (in separate terminals recommended)"
	@echo "  test               - Run all tests"
	@echo "  clean              - Clean build artifacts"
	@echo "  fmt                - Format all code (gofumpt + rustfmt)"
	@echo "  lint               - Lint all code (golangci-lint + clippy)"
	@echo "  tidy               - Tidy Go module dependencies"
	@echo "  docker-build       - Build all Docker images"
	@echo "  docker-up          - Start all services with docker-compose"
	@echo "  docker-down        - Stop all services with docker-compose"
	@echo ""
	@echo "Database targets:"
	@echo "  sqlc-generate      - Generate Go code from SQL queries"
	@echo "  migrate-up         - Run all pending migrations"
	@echo "  migrate-down       - Rollback last migration"
	@echo "  migrate-create     - Create a new migration (NAME=migration_name)"
	@echo "  migrate-force      - Force set migration version (VERSION=N)"
	@echo "  migrate-drop       - Drop everything in the database (full nuke)"
	@echo "  migrate-reset      - Drop everything and re-run all migrations"

# Build targets
build: build-auth build-matchmaking build-gameplay build-friendlist build-replay build-cursor-udp
	@echo "All services built successfully"

build-auth:
	@echo "Building auth service..."
	cd services/auth && go build -o ../../bin/auth .

build-matchmaking:
	@echo "Building matchmaking service..."
	cd services/matchmaking && go build -o ../../bin/matchmaking .

build-gameplay:
	@echo "Building gameplay service..."
	cd services/gameplay && go build -o ../../bin/gameplay .

build-friendlist:
	@echo "Building friendlist service..."
	cd services/friendlist && go build -o ../../bin/friendlist .

build-replay:
	@echo "Building replay service..."
	cd services/replay && go build -o ../../bin/replay .

build-cursor-udp:
	@echo "Building cursor UDP service..."
	@if [ -f services/cursor-udp/Cargo.toml ]; then \
		cd services/cursor-udp && cargo build --release && \
		cp target/release/cursor-udp ../../bin/cursor-udp; \
	else \
		echo "Cursor UDP service not initialized yet. Run 'cargo init' in services/cursor-udp first."; \
		exit 1; \
	fi

# Run targets
run-auth: build-auth
	@echo "Running auth service..."
	./bin/auth

run-matchmaking: build-matchmaking
	@echo "Running matchmaking service..."
	./bin/matchmaking

run-gameplay: build-gameplay
	@echo "Running gameplay service..."
	./bin/gameplay

run-friendlist: build-friendlist
	@echo "Running friendlist service..."
	./bin/friendlist

run-replay: build-replay
	@echo "Running replay service..."
	./bin/replay

run-cursor-udp: build-cursor-udp
	@echo "Running cursor UDP service..."
	./bin/cursor-udp

run-all: build
	@echo "Starting all services..."
	@echo "Note: This runs services sequentially. Use docker-compose for proper orchestration."
	./bin/matchmaking & ./bin/gameplay & ./bin/friendlist & ./bin/replay & wait

# Test target
test:
	@echo "Running tests..."
	cd services/auth && go test ./...
	cd services/matchmaking && go test ./...
	cd services/gameplay && go test ./...
	cd services/friendlist && go test ./...
	cd services/replay && go test ./...
	cd shared && go test ./...
	@if [ -f services/cursor-udp/Cargo.toml ]; then \
		cd services/cursor-udp && cargo test; \
	fi

# Clean target
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	find . -name "*.test" -type f -delete
	find . -name "*.out" -type f -delete
	@if [ -d services/cursor-udp/target ]; then \
		cd services/cursor-udp && cargo clean; \
	fi

# Docker targets
docker-build:
	@echo "Building Docker images..."
	docker-compose build

docker-up:
	@echo "Starting services with docker-compose..."
	docker-compose up -d

docker-down:
	@echo "Stopping services with docker-compose..."
	docker-compose down

# Development helpers
tidy:
	@echo "Tidying Go modules..."
	cd services/auth && go mod tidy
	cd services/matchmaking && go mod tidy
	cd services/gameplay && go mod tidy
	cd services/friendlist && go mod tidy
	cd services/replay && go mod tidy
	cd shared && go mod tidy

fmt:
	@echo "Formatting code..."
	@echo "Formatting Go code with gofumpt..."
	@if command -v gofumpt >/dev/null 2>&1; then \
		gofumpt -l -w services/auth services/matchmaking services/gameplay services/friendlist services/replay shared; \
	else \
		echo "gofumpt not found, using go fmt instead..."; \
		cd services/auth && go fmt ./...; \
		cd ../matchmaking && go fmt ./...; \
		cd ../gameplay && go fmt ./...; \
		cd ../friendlist && go fmt ./...; \
		cd ../replay && go fmt ./...; \
		cd ../../shared && go fmt ./...; \
	fi
	@echo "Formatting Rust code..."
	@if [ -f services/cursor-udp/Cargo.toml ]; then \
		cd services/cursor-udp && cargo fmt; \
	fi
	@echo "All code formatted!"

lint:
	@echo "Running linters..."
	@echo "Linting Go code with golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		cd services/auth && golangci-lint run ./...; \
		cd ../matchmaking && golangci-lint run ./...; \
		cd ../gameplay && golangci-lint run ./...; \
		cd ../friendlist && golangci-lint run ./...; \
		cd ../replay && golangci-lint run ./...; \
		cd ../../shared && golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install with: brew install golangci-lint"; \
		exit 1; \
	fi
	@echo ""
	@echo "Linting Rust code with clippy..."
	@if [ -f services/cursor-udp/Cargo.toml ]; then \
		cd services/cursor-udp && cargo clippy -- -D warnings; \
	fi
	@echo ""
	@echo "All linting passed!"

# Database targets
sqlc-generate:
	@echo "Generating Go code from SQL queries..."
	@if command -v sqlc >/dev/null 2>&1; then \
		sqlc -f db/sqlc.yaml generate; \
		echo "sqlc generation complete!"; \
	else \
		echo "sqlc not found. Install with: brew install sqlc"; \
		exit 1; \
	fi

migrate-up:
	@echo "Running database migrations..."
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path db/migrations -database $(DB_URL) up; \
	else \
		echo "golang-migrate not found."; \
		echo "Install with: brew install golang-migrate"; \
		exit 1; \
	fi

migrate-down:
	@echo "Rolling back last migration..."
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path db/migrations -database $(DB_URL) down 1; \
	else \
		echo "golang-migrate not found."; \
		echo "Install with: brew install golang-migrate"; \
		exit 1; \
	fi

migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Usage: make migrate-create NAME=your_migration_name"; \
		exit 1; \
	fi
	@echo "Creating new migration: $(NAME)..."
	@if command -v migrate >/dev/null 2>&1; then \
		migrate create -ext sql -dir db/migrations -seq $(NAME); \
	else \
		echo "golang-migrate not found."; \
		echo "Install with: brew install golang-migrate"; \
		exit 1; \
	fi

migrate-force:
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is required. Usage: make migrate-force VERSION=N"; \
		exit 1; \
	fi
	@echo "Forcing migration version to $(VERSION)..."
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path db/migrations -database $(DB_URL) force $(VERSION); \
	else \
		echo "golang-migrate not found."; \
		echo "Install with: brew install golang-migrate"; \
		exit 1; \
	fi

migrate-drop:
	@echo "Dropping all tables and data..."
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path db/migrations -database $(DB_URL) drop -f; \
	else \
		echo "golang-migrate not found."; \
		echo "Install with: brew install golang-migrate"; \
		exit 1; \
	fi

migrate-reset: migrate-drop migrate-up
	@echo "Database reset complete!"
