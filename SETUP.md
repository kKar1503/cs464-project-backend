# Quick Setup Guide

Get your game backend up and running in 5 minutes.

## Prerequisites

- Docker & Docker Compose
- Go 1.23+ (for local development)
- Make
- OpenSSL (for password generation)

## Setup Steps

### 1. Clone & Navigate

```bash
cd backend
```

### 2. Generate Secure Passwords

The project now uses secure random passwords by default.

**Option A: Auto-generate (Recommended)**

```bash
./scripts/update-env-passwords.sh
```

This creates `.env` with secure random passwords automatically.

**Option B: Manual**

```bash
cp .env.example .env
./scripts/generate-passwords.sh
# Copy the passwords to .env
```

### 3. Start Services

```bash
make docker-up
```

This starts:
- MySQL (with password auth)
- Redis (with password auth)
- All Go services
- Rust cursor UDP service

### 4. Verify

```bash
# Check services are running
docker-compose ps

# Check logs
docker-compose logs -f

# Test connections
docker exec -it game-mysql mysql -u gameuser -p"$(grep MYSQL_PASSWORD .env | cut -d'=' -f2)" -e "SHOW DATABASES;"
docker exec -it game-redis redis-cli -a "$(grep REDIS_PASSWORD .env | cut -d'=' -f2)" ping
```

## Current Passwords

Your current passwords are stored in `.env`:

```bash
# View your passwords
cat .env
```

**⚠️ IMPORTANT: Keep these secure!**

```
MYSQL_ROOT_PASSWORD=Y4OEF54cduYZYyo0ucfBlxHiBsBJLxmb
MYSQL_PASSWORD=wJ+2ve4J+bPP8AhcmnZ4CqZkc7Fi+byB
REDIS_PASSWORD=vMIpkx6YQtbzXj8IUFlwZ/DIZuov6J4g38EXkEUwgPE=
```

## Local Development

### Build Services Locally

```bash
# Build all services
make build

# Run specific service
make run-matchmaking
```

### Connect to Databases

**MySQL:**
```bash
# From host
mysql -h 127.0.0.1 -P 3306 -u gameuser -p gamedb
# Password: <MYSQL_PASSWORD from .env>

# From container
docker exec -it game-mysql mysql -u gameuser -p gamedb
```

**Redis:**
```bash
# From host (if redis-cli installed)
redis-cli -h 127.0.0.1 -p 6379 -a <REDIS_PASSWORD>

# From container
docker exec -it game-redis redis-cli -a <REDIS_PASSWORD>
```

## Development Workflow

```bash
# Make changes to code

# Format code
make fmt

# Lint code
make lint

# Build
make build

# Test
make test

# Restart services
make docker-down
make docker-up
```

## Accessing Services

- Matchmaking: http://localhost:8001
- Gameplay: http://localhost:8002
- Friendlist: http://localhost:8003
- Replay: http://localhost:8004
- MySQL: localhost:3306
- Redis: localhost:6379
- Cursor UDP: localhost:9001/udp

## Common Tasks

### Change Passwords

```bash
# Generate new passwords
./scripts/update-env-passwords.sh

# Restart services
make docker-down
make docker-up
```

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker logs -f matchmaking-service
docker logs -f game-mysql
docker logs -f game-redis
```

### Reset Database

```bash
# Stop services
make docker-down

# Remove volumes (deletes all data)
docker volume rm backend_mysql-data backend_redis-data

# Start fresh
make docker-up
```

### Clean Everything

```bash
# Stop services
make docker-down

# Remove volumes
docker volume prune

# Clean build artifacts
make clean

# Start fresh
make docker-up
```

## Troubleshooting

### Services won't start

```bash
# Check for port conflicts
lsof -i :3306
lsof -i :6379
lsof -i :8001-8004

# Check Docker logs
docker-compose logs
```

### Can't connect to MySQL

```bash
# Verify password in .env matches
cat .env | grep MYSQL_PASSWORD

# Check MySQL is healthy
docker exec game-mysql mysqladmin ping -h localhost
```

### Can't connect to Redis

```bash
# Verify password in .env
cat .env | grep REDIS_PASSWORD

# Test Redis connection
docker exec game-redis redis-cli -a "$(grep REDIS_PASSWORD .env | cut -d'=' -f2)" ping
```

### Password authentication fails

```bash
# Verify services have restarted after password change
make docker-down
make docker-up

# Check environment variables are loaded
docker-compose config | grep PASSWORD
```

## Security Reminders

1. ✅ Passwords are randomly generated
2. ✅ `.env` is gitignored
3. ⚠️ Save passwords in a password manager
4. ⚠️ Use different passwords for staging/production
5. ⚠️ Never commit `.env` to git
6. ✅ Backup available: `.env.backup`

## Next Steps

1. [x] Setup complete
2. [ ] Implement matchmaking service
3. [ ] Implement gameplay service
4. [ ] Add authentication
5. [ ] Deploy to AWS (see [AWS_MIGRATION.md](AWS_MIGRATION.md))

## Quick Reference

```bash
# Generate passwords
./scripts/generate-passwords.sh

# Update .env with new passwords
./scripts/update-env-passwords.sh

# Start services
make docker-up

# Stop services
make docker-down

# View logs
docker-compose logs -f

# Rebuild and restart
make docker-down && make docker-build && make docker-up
```

## Support

- Database documentation: [DATABASE.md](DATABASE.md)
- Environment variables: [ENVIRONMENT.md](ENVIRONMENT.md)
- Redis auth: [REDIS_AUTH.md](REDIS_AUTH.md)
- AWS migration: [AWS_MIGRATION.md](AWS_MIGRATION.md)
- Linting: [LINTING.md](LINTING.md)
