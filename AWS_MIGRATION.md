# AWS Migration Guide

This guide helps you migrate from local Docker setup to AWS managed services.

## Redis Migration: Docker → AWS ElastiCache

### Current Setup (Docker)

```bash
# .env (local)
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=
```

### AWS ElastiCache Setup

#### 1. Create ElastiCache Redis Cluster

**Console:**
- Go to AWS ElastiCache Console
- Click "Create"
- Choose Redis
- Select cluster mode: Disabled (for single shard) or Enabled (for multi-shard)
- Instance type: cache.t3.micro (dev) or cache.r6g.large (prod)
- **Enable "Auth token"** - This generates a password
- **Enable encryption in-transit** (TLS)
- Save the Auth token - you'll need it

**CLI:**
```bash
aws elasticache create-replication-group \
  --replication-group-id game-redis-prod \
  --replication-group-description "Game backend Redis" \
  --engine redis \
  --cache-node-type cache.t3.micro \
  --num-cache-clusters 2 \
  --auth-token "YOUR_STRONG_PASSWORD_HERE" \
  --transit-encryption-enabled \
  --at-rest-encryption-enabled
```

#### 2. Update .env for AWS

```bash
# .env (AWS)
REDIS_HOST=game-redis-prod.abc123.0001.use1.cache.amazonaws.com
REDIS_PORT=6379
REDIS_PASSWORD=your-elasticache-auth-token
```

#### 3. Security Group Configuration

Allow inbound Redis traffic from your ECS/EC2 instances:

```
Type: Custom TCP
Port: 6379
Source: sg-xxxxxxxx (your application security group)
```

#### 4. Update Go Code (No Changes Required!)

Your existing code already supports Redis auth:

```go
client := redis.NewClient(&redis.Options{
    Addr:     fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
    Password: os.Getenv("REDIS_PASSWORD"), // Works with ElastiCache auth token
    DB:       0,
})
```

For TLS (recommended for production):

```go
import "crypto/tls"

client := redis.NewClient(&redis.Options{
    Addr:      fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
    Password:  os.Getenv("REDIS_PASSWORD"),
    TLSConfig: &tls.Config{},
    DB:        0,
})
```

## MySQL Migration: Docker → AWS RDS

### Current Setup (Docker)

```bash
# .env (local)
DB_HOST=mysql
DB_PORT=3306
DB_USER=gameuser
DB_PASSWORD=gamepassword
DB_NAME=gamedb
```

### AWS RDS Setup

#### 1. Create RDS MySQL Instance

**Console:**
- Go to AWS RDS Console
- Click "Create database"
- Engine: MySQL 8.0
- Template: Production (or Dev/Test)
- DB instance identifier: game-db-prod
- Master username: admin
- Master password: Generate or set strong password
- Instance type: db.t3.micro (dev) or db.r6g.large (prod)
- Storage: 20 GB with autoscaling
- **Enable encryption**
- Database name: gamedb

**CLI:**
```bash
aws rds create-db-instance \
  --db-instance-identifier game-db-prod \
  --db-instance-class db.t3.micro \
  --engine mysql \
  --engine-version 8.0.35 \
  --master-username admin \
  --master-user-password "YOUR_STRONG_PASSWORD" \
  --allocated-storage 20 \
  --storage-encrypted \
  --db-name gamedb \
  --backup-retention-period 7 \
  --publicly-accessible false
```

#### 2. Update .env for AWS

```bash
# .env (AWS)
DB_HOST=game-db-prod.abc123.us-east-1.rds.amazonaws.com
DB_PORT=3306
DB_USER=admin
DB_PASSWORD=your-rds-password
DB_NAME=gamedb
```

#### 3. Security Group Configuration

```
Type: MySQL/Aurora
Port: 3306
Source: sg-xxxxxxxx (your application security group)
```

#### 4. Import Schema

```bash
# From your local machine (if RDS is publicly accessible)
mysql -h game-db-prod.abc123.us-east-1.rds.amazonaws.com \
      -u admin -p gamedb < docker/mysql/init.sql

# Or from an EC2 instance in the same VPC
```

## Complete AWS Environment Variables

### Production .env

```bash
# MySQL (RDS)
MYSQL_ROOT_PASSWORD=not-used-in-rds
MYSQL_DATABASE=gamedb
MYSQL_USER=admin
MYSQL_PASSWORD=<RDS_MASTER_PASSWORD>
MYSQL_PORT=3306

# Redis (ElastiCache)
REDIS_PORT=6379
REDIS_PASSWORD=<ELASTICACHE_AUTH_TOKEN>

# Service Ports (if using ECS)
MATCHMAKING_PORT=8001
GAMEPLAY_PORT=8002
FRIENDLIST_PORT=8003
REPLAY_PORT=8004
CURSOR_UDP_PORT=9001
```

### Using AWS Secrets Manager (Recommended)

Store sensitive data in AWS Secrets Manager:

```bash
# Create secrets
aws secretsmanager create-secret \
  --name game/db/password \
  --secret-string "your-rds-password"

aws secretsmanager create-secret \
  --name game/redis/password \
  --secret-string "your-elasticache-auth-token"
```

Retrieve in your application:

```go
import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/secretsmanager"
)

func getSecret(secretName string) (string, error) {
    sess := session.Must(session.NewSession())
    svc := secretsmanager.New(sess)

    result, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
        SecretId: aws.String(secretName),
    })
    if err != nil {
        return "", err
    }

    return *result.SecretString, nil
}

// Usage
dbPassword, _ := getSecret("game/db/password")
redisPassword, _ := getSecret("game/redis/password")
```

## Deployment Options

### Option 1: ECS (Elastic Container Service)

Deploy your Docker containers to ECS:

1. Push images to ECR (Elastic Container Registry)
2. Create ECS Task Definitions
3. Create ECS Service
4. Use ALB (Application Load Balancer) for routing

### Option 2: EC2 with Docker Compose

Run docker-compose on EC2 instances:

```bash
# On EC2 instance
git clone <your-repo>
cd backend

# Update .env with RDS/ElastiCache endpoints
nano .env

# Run services
docker-compose up -d
```

### Option 3: Kubernetes (EKS)

Deploy to Amazon EKS using Kubernetes manifests.

## Cost Optimization

### Development/Testing

- RDS: db.t3.micro ($15-20/month)
- ElastiCache: cache.t3.micro ($12-15/month)
- **Total: ~$30/month**

### Production (Minimal)

- RDS: db.t3.small with Multi-AZ ($60-80/month)
- ElastiCache: cache.r6g.large with replica ($150-200/month)
- **Total: ~$250/month**

## Monitoring

### CloudWatch Metrics

Both RDS and ElastiCache automatically send metrics to CloudWatch:

- CPU utilization
- Memory usage
- Database connections
- Cache hit rate
- Network throughput

### Alarms

Create CloudWatch alarms for:
- High CPU usage (>80%)
- Low memory (<20%)
- Connection count approaching max
- Cache evictions increasing

## Backup & Disaster Recovery

### RDS Automated Backups

- Enabled by default (7-day retention)
- Can extend to 35 days
- Automated snapshots
- Point-in-time recovery

### ElastiCache Backups

- Enable automatic backups
- Set backup retention (1-35 days)
- Manual snapshots for major changes

## Migration Checklist

- [ ] Create ElastiCache Redis cluster with auth token
- [ ] Create RDS MySQL instance
- [ ] Update security groups for both
- [ ] Import database schema to RDS
- [ ] Update .env with AWS endpoints
- [ ] Test Redis connection with auth
- [ ] Test MySQL connection
- [ ] Deploy application to AWS (ECS/EC2)
- [ ] Update DNS/Load Balancer
- [ ] Set up CloudWatch monitoring
- [ ] Configure automated backups
- [ ] Test disaster recovery procedure

## Rollback Plan

If issues occur:

1. Keep local Docker setup running
2. Update DNS to point back to local/old setup
3. Investigate AWS issues
4. Fix and re-test before switching again

## Testing Connection from Local

```bash
# Test Redis (with auth)
redis-cli -h <elasticache-endpoint> -p 6379 -a <auth-token> ping

# Test MySQL
mysql -h <rds-endpoint> -u admin -p gamedb
```

## Additional Resources

- [AWS ElastiCache Redis Best Practices](https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/BestPractices.html)
- [AWS RDS MySQL Best Practices](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_BestPractices.html)
- [AWS Secrets Manager](https://docs.aws.amazon.com/secretsmanager/)
