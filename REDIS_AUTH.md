# Redis Authentication Setup

Redis authentication is now configured and ready for both local development and AWS ElastiCache migration.

## How It Works

### Without Password (Local Development - Default)

```bash
# .env
REDIS_PASSWORD=
```

Redis runs **without authentication** - perfect for local development.

### With Password (Production/AWS)

```bash
# .env
REDIS_PASSWORD=your-strong-password-here
```

Redis automatically enables `requirepass` authentication.

## Local Development

### Current Setup (No Password)

```bash
# .env
REDIS_PASSWORD=
```

Connect without auth:
```bash
redis-cli ping
# PONG
```

### Testing with Password Locally

To test auth locally before AWS:

```bash
# Edit .env
REDIS_PASSWORD=local-test-password

# Restart services
make docker-down
make docker-up

# Connect with auth
docker exec -it game-redis redis-cli -a local-test-password ping
# PONG
```

## Go Code - Works for Both!

Your Go code automatically adapts:

```go
import (
    "fmt"
    "os"
    "github.com/redis/go-redis/v9"
)

func connectRedis() *redis.Client {
    password := os.Getenv("REDIS_PASSWORD")

    return redis.NewClient(&redis.Options{
        Addr:     fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
        Password: password, // Empty string = no auth, Non-empty = auth enabled
        DB:       0,
    })
}
```

The go-redis library handles it:
- Empty password → No AUTH command sent
- Non-empty password → Sends AUTH command

## AWS ElastiCache Migration

### Step 1: Create ElastiCache with Auth

When creating your ElastiCache cluster:
- ✅ Enable "Auth token" (also called AUTH)
- ✅ Enable "Encryption in-transit" (TLS)
- Copy the auth token

### Step 2: Update .env

```bash
# .env (production)
REDIS_HOST=your-cluster.abc123.0001.use1.cache.amazonaws.com
REDIS_PORT=6379
REDIS_PASSWORD=your-elasticache-auth-token
```

### Step 3: Deploy (No Code Changes!)

That's it! Your application automatically uses the password.

## Security Best Practices

### Local Development
```bash
REDIS_PASSWORD=  # Empty - convenience
```

### Staging
```bash
REDIS_PASSWORD=staging-redis-pass-2024
```

### Production
```bash
REDIS_PASSWORD=<generated-strong-password>
# Use: openssl rand -base64 32
```

### AWS Secrets Manager (Recommended for Production)

Store in AWS Secrets Manager instead of .env:

```go
import (
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/secretsmanager"
)

func getRedisPassword() string {
    // Fetch from AWS Secrets Manager
    sess := session.Must(session.NewSession())
    svc := secretsmanager.New(sess)

    result, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
        SecretId: aws.String("game/redis/password"),
    })
    if err != nil {
        return os.Getenv("REDIS_PASSWORD") // Fallback to env var
    }

    return *result.SecretString
}
```

## Testing Connection

### Without Password
```bash
docker exec -it game-redis redis-cli ping
# PONG
```

### With Password
```bash
docker exec -it game-redis redis-cli -a "your-password" ping
# PONG

# Or authenticate after connecting
docker exec -it game-redis redis-cli
127.0.0.1:6379> AUTH your-password
OK
127.0.0.1:6379> PING
PONG
```

### Test from Go Service
```bash
# Check service logs
docker logs matchmaking-service

# Should see successful Redis connection
```

## Troubleshooting

### Error: NOAUTH Authentication required

**Problem:** Service trying to connect without password when password is required

**Solution:**
- Check `.env` has `REDIS_PASSWORD=your-password`
- Restart services: `make docker-down && make docker-up`

### Error: ERR invalid password

**Problem:** Wrong password configured

**Solution:**
- Verify password in `.env` matches Redis configuration
- Check for trailing spaces in `.env`

### Health Check Failing

The health check automatically adapts to password:

```yaml
healthcheck:
  test: >
    sh -c "
    if [ -n \"$$REDIS_PASSWORD\" ]; then
      redis-cli -a \"$$REDIS_PASSWORD\" ping
    else
      redis-cli ping
    fi
    "
```

## Monitoring

### Check if Auth is Enabled

```bash
# Connect to Redis
docker exec -it game-redis redis-cli

# Try command without auth
127.0.0.1:6379> PING
# If password set: (error) NOAUTH Authentication required.
# If no password: PONG
```

### View Redis Configuration

```bash
docker exec -it game-redis redis-cli CONFIG GET requirepass
# If password set: 1) "requirepass" 2) "your-password"
# If no password: 1) "requirepass" 2) ""
```

## Migration Path

### Phase 1: Local Development (Current)
```bash
REDIS_PASSWORD=  # No password
```

### Phase 2: Pre-production Testing
```bash
REDIS_PASSWORD=test-password  # Test with password locally
```

### Phase 3: AWS ElastiCache
```bash
REDIS_HOST=elasticache-endpoint.amazonaws.com
REDIS_PASSWORD=elasticache-auth-token
```

### Phase 4: AWS with TLS (Most Secure)
```go
import "crypto/tls"

client := redis.NewClient(&redis.Options{
    Addr:      os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT"),
    Password:  os.Getenv("REDIS_PASSWORD"),
    TLSConfig: &tls.Config{},  // Enable TLS
})
```

## Summary

✅ **Local Dev:** No password (REDIS_PASSWORD empty)
✅ **Production:** Password required (REDIS_PASSWORD set)
✅ **AWS Ready:** Works with ElastiCache auth token
✅ **No Code Changes:** Same Go code works for both
✅ **Health Checks:** Automatically adapt to password
✅ **Secure:** Easy to add TLS for ElastiCache

You're now ready to migrate to AWS ElastiCache with zero code changes! 🚀
