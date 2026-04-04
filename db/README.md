# Database Package

Type-safe database access using sqlc with golang-migrate for schema migrations.

## Directory Structure

```
db/
├── migrations/          # Database migration files
│   ├── 000001_create_users_table.up.sql
│   └── 000001_create_users_table.down.sql
├── queries/            # SQL query files for sqlc
│   └── users.sql
├── sqlc/              # Generated Go code (DO NOT EDIT)
│   ├── db.go
│   ├── models.go
│   ├── querier.go
│   └── users.sql.go
├── sqlc.yaml          # sqlc configuration
└── go.mod             # Go module for db package
```

## Prerequisites

Install required tools:

```bash
# Install sqlc
brew install sqlc

# Install golang-migrate
brew install golang-migrate
```

## Quick Start

### 1. Generate Type-Safe Go Code

After modifying SQL queries in `queries/`:

```bash
make sqlc-generate
```

This generates type-safe Go code in `sqlc/` directory.

### 2. Run Migrations

Apply all pending migrations:

```bash
make migrate-up
```

Rollback last migration:

```bash
make migrate-down
```

### 3. Create New Migration

```bash
make migrate-create NAME=add_email_to_users
```

This creates:
- `db/migrations/000002_add_email_to_users.up.sql`
- `db/migrations/000002_add_email_to_users.down.sql`

## Usage in Code

### Import the Package

```go
import (
    "database/sql"
    "github.com/kKar1503/cs464-backend/db/sqlc"
    _ "github.com/go-sql-driver/mysql"
)
```

### Connect to Database

```go
// Open database connection
database, err := sql.Open("mysql", "user:password@tcp(localhost:33061)/gamedb?parseTime=true")
if err != nil {
    log.Fatal(err)
}
defer database.Close()

// Create queries instance
queries := db.New(database)
```

### Create a User

```go
ctx := context.Background()

result, err := queries.CreateUser(ctx, db.CreateUserParams{
    Username:     "player1",
    PasswordHash: "$2a$10$hashedpassword...",
})
if err != nil {
    log.Fatal(err)
}

userID, _ := result.LastInsertId()
fmt.Printf("Created user with ID: %d\n", userID)
```

### Get User by ID

```go
user, err := queries.GetUserByID(ctx, userID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Username: %s\n", user.Username)
fmt.Printf("Created at: %s\n", user.CreatedAt)
```

### Get User by Username

```go
user, err := queries.GetUserByUsername(ctx, "player1")
if err != nil {
    if err == sql.ErrNoRows {
        fmt.Println("User not found")
    } else {
        log.Fatal(err)
    }
}
```

### List Users with Pagination

```go
users, err := queries.ListUsers(ctx, db.ListUsersParams{
    Limit:  10,
    Offset: 0,
})
if err != nil {
    log.Fatal(err)
}

for _, user := range users {
    fmt.Printf("%d: %s\n", user.ID, user.Username)
}
```

### Update User

```go
err = queries.UpdateUser(ctx, db.UpdateUserParams{
    ID:       userID,
    Username: "new_username",
})
if err != nil {
    log.Fatal(err)
}
```

### Update Password

```go
err = queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
    ID:           userID,
    PasswordHash: "$2a$10$newhashedpassword...",
})
if err != nil {
    log.Fatal(err)
}
```

### Delete User

```go
err = queries.DeleteUser(ctx, userID)
if err != nil {
    log.Fatal(err)
}
```

### Count Users

```go
count, err := queries.CountUsers(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Total users: %d\n", count)
```

## Writing Queries

### Query File Format

Create SQL files in `queries/` directory:

```sql
-- name: GetUserByID :one
SELECT id, username, password_hash, created_at, updated_at
FROM users
WHERE id = ?;

-- name: CreateUser :execresult
INSERT INTO users (username, password_hash)
VALUES (?, ?);

-- name: ListUsers :many
SELECT id, username, password_hash, created_at, updated_at
FROM users
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
```

### Query Annotations

- `:one` - Returns single row (error if not found)
- `:many` - Returns slice of rows
- `:exec` - Executes query, no return value
- `:execresult` - Returns `sql.Result` (for INSERT to get ID)

### Parameters

sqlc automatically generates parameter structs:

```go
type CreateUserParams struct {
    Username     string
    PasswordHash string
}

type ListUsersParams struct {
    Limit  int32
    Offset int32
}
```

## Migrations

### Migration File Format

**Up migration** (`*.up.sql`):
```sql
CREATE TABLE users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Down migration** (`*.down.sql`):
```sql
DROP TABLE IF EXISTS users;
```

### Migration Best Practices

1. **Always write both up and down migrations**
2. **Test rollbacks** - ensure down migrations work
3. **One change per migration** - easier to debug
4. **Never modify existing migrations** - create new ones
5. **Use transactions** when possible

### Migration Commands

```bash
# Apply all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Create new migration
make migrate-create NAME=add_profile_table

# Force migration version (if stuck)
make migrate-force VERSION=1
```

## Transactions

For operations requiring multiple queries:

```go
// Begin transaction
tx, err := database.Begin()
if err != nil {
    log.Fatal(err)
}
defer tx.Rollback() // Rollback if commit not called

// Create queries with transaction
qtx := queries.WithTx(tx)

// Use qtx for all operations
result, err := qtx.CreateUser(ctx, db.CreateUserParams{...})
if err != nil {
    return err // tx.Rollback() called by defer
}

// Another operation in same transaction
err = qtx.UpdateUser(ctx, db.UpdateUserParams{...})
if err != nil {
    return err
}

// Commit transaction
if err := tx.Commit(); err != nil {
    return err
}
```

## Configuration

### sqlc.yaml

```yaml
version: "2"
sql:
  - engine: "mysql"
    queries: "queries/"
    schema: "migrations/"
    gen:
      go:
        package: "db"
        out: "sqlc"
        emit_json_tags: true
        emit_interface: true
```

Options:
- `emit_json_tags` - Add JSON tags to structs
- `emit_interface` - Generate `Querier` interface
- `emit_empty_slices` - Return `[]` instead of `nil`
- `emit_pointers_for_null_types` - Use `*string` for NULL columns

## Testing

### Test with Mock Database

```go
func TestCreateUser(t *testing.T) {
    // Setup test database
    db, err := sql.Open("mysql", "test:test@tcp(localhost:33061)/test_db")
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    queries := sqlc.New(db)

    // Run migration
    // ... apply test migrations ...

    // Test CreateUser
    result, err := queries.CreateUser(context.Background(), sqlc.CreateUserParams{
        Username:     "testuser",
        PasswordHash: "hash",
    })

    assert.NoError(t, err)
    userID, _ := result.LastInsertId()
    assert.Greater(t, userID, int64(0))
}
```

## Common Patterns

### Password Hashing

```go
import "golang.org/x/crypto/bcrypt"

// Hash password before storing
func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
    return string(bytes), err
}

// Verify password
func CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

// Usage
hashedPassword, _ := HashPassword("user_password")
result, err := queries.CreateUser(ctx, db.CreateUserParams{
    Username:     "player1",
    PasswordHash: hashedPassword,
})
```

### Check if User Exists

```go
user, err := queries.GetUserByUsername(ctx, "player1")
if err == sql.ErrNoRows {
    // User doesn't exist
    fmt.Println("User not found")
} else if err != nil {
    // Database error
    log.Fatal(err)
} else {
    // User exists
    fmt.Printf("Found user: %s\n", user.Username)
}
```

## Troubleshooting

### "sqlc not found"

```bash
brew install sqlc
# or
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

### "migrate not found"

```bash
brew install golang-migrate
# or
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### Migration Error "Dirty database"

```bash
# Check current version
migrate -path db/migrations -database "$DB_URL" version

# Force to specific version
make migrate-force VERSION=1
```

### Regenerate All Code

```bash
# Delete generated code
rm -rf db/sqlc/*.go

# Regenerate
make sqlc-generate
```

## References

- [sqlc Documentation](https://docs.sqlc.dev/)
- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
- [MySQL Documentation](https://dev.mysql.com/doc/)
