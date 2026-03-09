# Build stage
FROM golang:1.25.7-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared ./shared

# Copy service module files first (for dependency caching)
COPY services/matchmaking/go.mod services/matchmaking/go.sum* ./services/matchmaking/

# Download dependencies
WORKDIR /app/services/matchmaking
RUN go mod download

# Copy service source code
COPY services/matchmaking/. ./

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/matchmaking .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/bin/matchmaking .

EXPOSE 8001

CMD ["./matchmaking"]
