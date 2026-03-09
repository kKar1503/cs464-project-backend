# Build stage
FROM golang:1.25.7-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared ./shared

# Copy service module files first (for dependency caching)
COPY services/replay/go.mod services/replay/go.sum* ./services/replay/

# Download dependencies
WORKDIR /app/services/replay
RUN go mod download

# Copy service source code
COPY services/replay/. ./

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/replay .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/bin/replay .

EXPOSE 8004

CMD ["./replay"]
