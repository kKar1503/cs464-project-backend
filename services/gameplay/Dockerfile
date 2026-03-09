# Build stage
FROM golang:1.25.7-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared ./shared

# Copy service module files first (for dependency caching)
COPY services/gameplay/go.mod services/gameplay/go.sum* ./services/gameplay/

# Download dependencies
WORKDIR /app/services/gameplay
RUN go mod download

# Copy service source code
COPY services/gameplay/. ./

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/gameplay .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/bin/gameplay .

EXPOSE 8002

CMD ["./gameplay"]
