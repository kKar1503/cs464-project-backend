module github.com/kKar1503/cs464-backend/services/matchmaking

go 1.25.7

require (
	github.com/go-sql-driver/mysql v1.9.3
	github.com/google/uuid v1.6.0
	github.com/kKar1503/cs464-backend/db v0.0.0
	github.com/redis/go-redis/v9 v9.18.0
)

replace github.com/kKar1503/cs464-backend/db => ../../db

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	go.uber.org/atomic v1.11.0 // indirect
)
