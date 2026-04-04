# Environment Variables Configuration

This project uses environment variables to configure database credentials, service ports, and other settings.

## Setup

### 1. Copy the example file

```bash
cp .env.example .env
```

### 2. Edit `.env` with your values

```bash
# Edit the file
nano .env
# or
vim .env
```

## Environment Variables

### MySQL Configuration

```bash
MYSQL_ROOT_PASSWORD=rootpassword      # Root password for MySQL
MYSQL_DATABASE=gamedb                 # Database name
MYSQL_USER=gameuser                   # Database user
MYSQL_PASSWORD=gamepassword           # Database password
MYSQL_PORT=3306                       # MySQL external port
```

### Redis Configuration

```bash
REDIS_PORT=6379                       # Redis external port
REDIS_MAX_MEMORY=256mb                # Redis max memory (configured in redis.conf)
```

### Service Ports

```bash
MATCHMAKING_PORT=8001                 # Matchmaking service port
GAMEPLAY_PORT=8002                    # Gameplay service port
FRIENDLIST_PORT=8003                  # Friendlist service port
REPLAY_PORT=8004                      # Replay service port
CURSOR_UDP_PORT=9001                  # Cursor UDP service port
```

## How Docker Compose Uses .env

Docker Compose automatically loads `.env` from the same directory as `docker-compose.yml`.

Variables are referenced in `docker-compose.yml` using `${VARIABLE_NAME}` syntax:

```yaml
services:
  mysql:
    ports:
      - "${MYSQL_PORT}:3306"
    environment:
      - MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}
      - MYSQL_DATABASE=${MYSQL_DATABASE}
```

## Different Environments

You can create different `.env` files for different environments:

```bash
.env                  # Default (local development)
.env.production       # Production settings
.env.staging          # Staging settings
.env.test             # Test settings
```

Use specific env files with:

```bash
docker-compose --env-file .env.production up
```

## Security Best Practices

1. **Never commit `.env` to git**
   - Already in `.gitignore`
   - Contains sensitive credentials

2. **Use strong passwords in production**
   ```bash
   # Generate a random password
   openssl rand -base64 32
   ```

3. **Different credentials per environment**
   - Dev: Simple passwords (convenience)
   - Staging: Strong passwords
   - Production: Very strong passwords + secrets management

4. **Use secrets management in production**
   - AWS Secrets Manager
   - HashiCorp Vault
   - Kubernetes Secrets
   - Docker Secrets (Swarm)

## Example Configurations

### Development (Default)

```bash
MYSQL_ROOT_PASSWORD=rootpassword
MYSQL_DATABASE=gamedb
MYSQL_USER=gameuser
MYSQL_PASSWORD=gamepassword
MYSQL_PORT=3306

REDIS_PORT=6379
```

### Production

```bash
MYSQL_ROOT_PASSWORD=<very-strong-generated-password>
MYSQL_DATABASE=gamedb_prod
MYSQL_USER=gameuser_prod
MYSQL_PASSWORD=<very-strong-generated-password>
MYSQL_PORT=3306

REDIS_PORT=6379
```

### Testing (Different Ports)

```bash
MYSQL_ROOT_PASSWORD=testroot
MYSQL_DATABASE=gamedb_test
MYSQL_USER=testuser
MYSQL_PASSWORD=testpass
MYSQL_PORT=3307              # Different port to avoid conflicts

REDIS_PORT=6380              # Different port

MATCHMAKING_PORT=8101        # Different ports
GAMEPLAY_PORT=8102
FRIENDLIST_PORT=8103
REPLAY_PORT=8104
CURSOR_UDP_PORT=9101
```

## Accessing Environment Variables in Go

Environment variables are passed to your containers and can be accessed in Go:

```go
package main

import (
    "fmt"
    "os"
)

func main() {
    dbHost := os.Getenv("DB_HOST")
    dbPort := os.Getenv("DB_PORT")
    dbUser := os.Getenv("DB_USER")
    dbPassword := os.Getenv("DB_PASSWORD")
    dbName := os.Getenv("DB_NAME")

    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
        dbUser, dbPassword, dbHost, dbPort, dbName)

    fmt.Println("Database DSN:", dsn)
}
```

## Validation

Check that your environment variables are loaded correctly:

```bash
# View all variables (Docker Compose will substitute them)
docker-compose config

# Check specific service environment
docker-compose config | grep -A 20 "matchmaking:"
```

## Troubleshooting

### Variables not being substituted

**Problem:** Seeing `${MYSQL_PASSWORD}` literally in logs

**Solution:**
1. Ensure `.env` file exists in the same directory as `docker-compose.yml`
2. Check `.env` file format (no spaces around `=`)
   ```bash
   MYSQL_PASSWORD=mypass    # Correct
   MYSQL_PASSWORD = mypass  # Wrong
   ```

### Can't connect to database

**Problem:** Services can't connect to MySQL/Redis

**Solution:**
1. Check service logs: `docker-compose logs mysql`
2. Verify credentials in `.env` match what you're using in code
3. Services use internal hostnames (`mysql`, `redis`) not `localhost`

### Port conflicts

**Problem:** `port is already allocated`

**Solution:**
Change the port in `.env`:
```bash
MYSQL_PORT=3307  # Instead of 3306
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Create .env file
  run: |
    echo "MYSQL_ROOT_PASSWORD=${{ secrets.MYSQL_ROOT_PASSWORD }}" >> .env
    echo "MYSQL_DATABASE=gamedb" >> .env
    echo "MYSQL_USER=gameuser" >> .env
    echo "MYSQL_PASSWORD=${{ secrets.MYSQL_PASSWORD }}" >> .env
```

### GitLab CI

```yaml
before_script:
  - cp .env.example .env
  - sed -i "s/MYSQL_PASSWORD=.*/MYSQL_PASSWORD=${MYSQL_PASSWORD}/" .env
```

## Default Values

If a variable is not set, Docker Compose will fail. You can provide defaults in `docker-compose.yml`:

```yaml
ports:
  - "${MYSQL_PORT:-3306}:3306"  # Use 3306 if MYSQL_PORT not set
```

However, for this project, all variables should be defined in `.env` for clarity.
