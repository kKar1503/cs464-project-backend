# Build stage
FROM golang:1.25.7-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared ./shared

# Copy service module files first (for dependency caching)
COPY services/friendlist/go.mod services/friendlist/go.sum* ./services/friendlist/

# Download dependencies
WORKDIR /app/services/friendlist
RUN go mod download

# Copy service source code
COPY services/friendlist/. ./

# Build the service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/friendlist .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/bin/friendlist .

EXPOSE 8003

CMD ["./friendlist"]
