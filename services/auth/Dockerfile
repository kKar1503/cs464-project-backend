# Build stage
FROM golang:1.25.7-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared ./shared

# Copy service module files first (for dependency caching)
COPY services/auth/go.mod services/auth/go.sum* ./services/auth/

# Download dependencies
WORKDIR /app/services/auth
RUN go mod download

# Copy service source code
COPY services/auth/. ./

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/auth .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/bin/auth .

EXPOSE 8000

CMD ["./auth"]
