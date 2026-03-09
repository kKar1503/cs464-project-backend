# Helper Scripts

Utility scripts for managing your game backend.

## Testing Scripts

### Test Redis Connection

Test Redis with password authentication:

```bash
./scripts/test-redis.sh
```

This will test:
- PING command
- SET/GET operations
- Server information

### Test MySQL Connection

Test MySQL with password authentication:

```bash
./scripts/test-mysql.sh
```

This will test:
- Connection status
- Database access
- Table listing
- Basic queries

### Test Cursor UDP Service

Test the Rust UDP service for cursor movements:

```bash
./scripts/test-cursor-udp.sh
```

This will test:
- Container status
- UDP echo communication
- Service logs

**Note**: Requires `nc` (netcat) or `python3` for UDP testing.

## Password Generation

### Generate New Passwords

Display new random passwords without updating .env:

```bash
./scripts/generate-passwords.sh
```

Output:
```
=== Game Backend Password Generator ===

MySQL Root Password:
abcd1234...

MySQL User Password:
efgh5678...

Redis Password:
ijkl9012...
```

### Update .env Automatically

Generate AND update your .env file with new passwords:

```bash
./scripts/update-env-passwords.sh
```

This will:
1. Backup your current .env to .env.backup
2. Generate new random passwords
3. Update .env file automatically
4. Display the new passwords

**After updating, restart services:**
```bash
make docker-down
make docker-up
```

## Manual Password Generation

### Redis Password (32 chars, base64)
```bash
openssl rand -base64 32
```

### MySQL Password (24 chars, base64)
```bash
openssl rand -base64 24
```

### Hex Password (64 chars)
```bash
openssl rand -hex 32
```

### Alphanumeric Only (32 chars)
```bash
LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 32
```

## Security Tips

1. **Never use default passwords in production**
   ```bash
   # BAD
   REDIS_PASSWORD=password123

   # GOOD
   REDIS_PASSWORD=$(openssl rand -base64 32)
   ```

2. **Store passwords securely**
   - Use a password manager (1Password, LastPass, Bitwarden)
   - Use AWS Secrets Manager in production
   - Never commit .env to git

3. **Rotate passwords regularly**
   ```bash
   # Every 90 days in production
   ./scripts/update-env-passwords.sh
   ```

4. **Different passwords per environment**
   ```
   .env              # Local dev
   .env.staging      # Staging (different passwords)
   .env.production   # Production (different passwords)
   ```

## AWS Secrets Manager Integration

For production, use AWS Secrets Manager instead:

```bash
# Store password in AWS
aws secretsmanager create-secret \
  --name game/redis/password \
  --secret-string "$(openssl rand -base64 32)"

# Retrieve in your application
aws secretsmanager get-secret-value \
  --secret-id game/redis/password \
  --query SecretString \
  --output text
```

## Password Requirements

### Redis (ElastiCache)
- Length: 16-128 characters
- Must contain: uppercase, lowercase, numbers
- No special characters: `" @ / \ space`
- Recommendation: Use `openssl rand -base64 32`

### MySQL (RDS)
- Length: 8-41 characters
- Can contain: any printable ASCII
- Cannot contain: `"` `@` `/` `space`
- Recommendation: Use `openssl rand -base64 24`

## Quick Reference

```bash
# Generate one password
openssl rand -base64 32

# Generate multiple passwords
./scripts/generate-passwords.sh

# Update .env with new passwords
./scripts/update-env-passwords.sh

# Verify .env values
cat .env | grep PASSWORD

# Test docker-compose config
docker-compose config | grep PASSWORD
```

## Troubleshooting

### Script not executable
```bash
chmod +x scripts/*.sh
```

### openssl not found
```bash
# macOS
brew install openssl

# Ubuntu/Debian
apt-get install openssl

# Already installed on most systems
```

### .env.backup not created
The script creates it automatically. If missing, manually backup:
```bash
cp .env .env.backup
```
