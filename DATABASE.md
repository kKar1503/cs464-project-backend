# Database & Cache Setup

This project uses MySQL for persistent data storage and Redis for caching and session management.

## Services

### MySQL 8.0
- **Port**: 3306
- **Database**: gamedb
- **User**: gameuser
- **Password**: gamepassword (change in production!)

### Redis 7 (Alpine)
- **Port**: 6379
- **Max Memory**: 256MB
- **Eviction Policy**: allkeys-lru
- **Persistence**: AOF + RDB

## Quick Start

### Using Docker Compose

Start all services including database:
```bash
make docker-up
```

Stop all services:
```bash
make docker-down
```

### Local Development

If you want to run services locally without Docker:

**MySQL:**
```bash
# Install MySQL 8.0
brew install mysql@8.0  # macOS
# or use your system's package manager

# Start MySQL
mysql.server start

# Create database and user
mysql -u root -p < docker/mysql/init.sql
```

**Redis:**
```bash
# Install Redis
brew install redis  # macOS

# Start Redis with custom config
redis-server docker/redis/redis.conf
```

## Database Schema

### Users
Stores player information and MMR (Matchmaking Rating).

```sql
CREATE TABLE users (
    id VARCHAR(36) PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    mmr INT NOT NULL DEFAULT 1000,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### Game Sessions
Tracks active and completed game sessions.

```sql
CREATE TABLE game_sessions (
    id VARCHAR(36) PRIMARY KEY,
    player1_id VARCHAR(36) NOT NULL,
    player2_id VARCHAR(36) NOT NULL,
    status ENUM('pending', 'in_progress', 'completed', 'cancelled'),
    winner_id VARCHAR(36),
    started_at TIMESTAMP NULL,
    ended_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### Friendships
Manages friend relationships between users.

```sql
CREATE TABLE friendships (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    friend_id VARCHAR(36) NOT NULL,
    status ENUM('pending', 'accepted', 'blocked'),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### Game Replays
Stores replay data for completed games.

```sql
CREATE TABLE game_replays (
    id VARCHAR(36) PRIMARY KEY,
    game_session_id VARCHAR(36) NOT NULL,
    replay_data LONGTEXT NOT NULL,
    duration_seconds INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Matchmaking Queue
Temporary queue for players waiting for matches.

```sql
CREATE TABLE matchmaking_queue (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    mmr INT NOT NULL,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Redis Usage

### Cache Keys

Common cache key patterns:

```
user:{user_id}               - User data cache (5min TTL)
session:{session_id}         - Active game session (1hr TTL)
matchmaking:{user_id}        - Matchmaking queue entry
friend_list:{user_id}        - Cached friend list (10min TTL)
online_users                 - Set of online user IDs
game_state:{session_id}      - Real-time game state
```

### Pub/Sub Channels

```
matchmaking:found            - New match found notification
game:{session_id}:events     - Game events channel
cursor:{session_id}          - Cursor position updates
```

## Environment Variables

All services have access to these environment variables:

```bash
# Database
DB_HOST=mysql
DB_PORT=3306
DB_USER=gameuser
DB_PASSWORD=gamepassword
DB_NAME=gamedb

# Redis
REDIS_HOST=redis
REDIS_PORT=6379
```

## Connecting from Go Services

### MySQL Example

```go
import (
    "database/sql"
    "fmt"
    "os"

    _ "github.com/go-sql-driver/mysql"
)

func connectDB() (*sql.DB, error) {
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
        os.Getenv("DB_USER"),
        os.Getenv("DB_PASSWORD"),
        os.Getenv("DB_HOST"),
        os.Getenv("DB_PORT"),
        os.Getenv("DB_NAME"),
    )

    return sql.Open("mysql", dsn)
}
```

### Redis Example

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/redis/go-redis/v9"
)

func connectRedis() *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr: fmt.Sprintf("%s:%s",
            os.Getenv("REDIS_HOST"),
            os.Getenv("REDIS_PORT"),
        ),
        Password: os.Getenv("REDIS_PASSWORD"), // Auth password (empty string if no password)
        DB:       0,  // use default DB
    })
}

// For AWS ElastiCache compatibility
func connectRedisWithTLS() *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr:     fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
        Password: os.Getenv("REDIS_PASSWORD"),
        DB:       0,
        // TLSConfig: &tls.Config{}, // Uncomment for ElastiCache with TLS
    })
}
```

## Migrations

For database schema changes, create migration files in `docker/mysql/migrations/`:

```
docker/mysql/migrations/
├── 001_initial_schema.sql (already in init.sql)
├── 002_add_user_settings.sql
└── 003_add_game_stats.sql
```

Apply migrations manually or use a migration tool like:
- [golang-migrate](https://github.com/golang-migrate/migrate)
- [goose](https://github.com/pressly/goose)

## Data Persistence

### Docker Volumes

Data is persisted in Docker volumes:
- `mysql-data` - MySQL data directory
- `redis-data` - Redis persistence files

**List volumes:**
```bash
docker volume ls
```

**Backup MySQL:**
```bash
docker exec game-mysql mysqldump -u gameuser -pgamepassword gamedb > backup.sql
```

**Backup Redis:**
```bash
docker exec game-redis redis-cli BGSAVE
docker cp game-redis:/data/dump.rdb ./redis-backup.rdb
```

**Clean volumes** (WARNING: deletes all data):
```bash
docker-compose down -v
```

## Monitoring

### MySQL

Check database health:
```bash
docker exec -it game-mysql mysql -u gameuser -pgamepassword gamedb
```

Show tables:
```sql
SHOW TABLES;
```

Check connections:
```sql
SHOW PROCESSLIST;
```

### Redis

Connect to Redis:
```bash
docker exec -it game-redis redis-cli
```

Monitor commands:
```bash
redis-cli MONITOR
```

Check memory usage:
```bash
redis-cli INFO memory
```

Get all keys (dev only):
```bash
redis-cli KEYS '*'
```

## Production Considerations

1. **Change default passwords** in docker-compose.yml
2. **Enable Redis password** in redis.conf (uncomment requirepass)
3. **Set up regular backups** using cron jobs
4. **Configure SSL/TLS** for database connections
5. **Use connection pooling** in your Go services
6. **Monitor database performance** with tools like:
   - MySQL: Percona Monitoring and Management (PMM)
   - Redis: RedisInsight
7. **Set up replication** for high availability
8. **Configure proper indexes** based on query patterns
9. **Use environment-specific .env files** (don't commit secrets!)
10. **Implement proper error handling** and connection retries

## Troubleshooting

### MySQL connection refused
```bash
# Check if MySQL is healthy
docker exec game-mysql mysqladmin ping -h localhost

# Check logs
docker logs game-mysql
```

### Redis connection issues
```bash
# Test Redis connection
docker exec game-redis redis-cli ping

# Check logs
docker logs game-redis
```

### Can't connect from Go service
- Ensure service has `depends_on` with health checks
- Verify environment variables are set correctly
- Check network connectivity: `docker network inspect backend_game-network`
