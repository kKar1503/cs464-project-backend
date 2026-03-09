# Build stage
FROM rust:1.83-alpine AS builder

# Install musl-dev for static linking
RUN apk add --no-cache musl-dev

WORKDIR /app

# Copy manifests
COPY services/cursor-udp/Cargo.toml services/cursor-udp/Cargo.lock* ./

# Create a dummy main.rs to cache dependencies
RUN mkdir src && \
    echo "fn main() {}" > src/main.rs && \
    cargo build --release && \
    rm -rf src

# Copy source code
COPY services/cursor-udp/src ./src

# Build the actual binary
RUN touch src/main.rs && cargo build --release

# Runtime stage
FROM alpine:latest

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/target/release/cursor-udp .

EXPOSE 9001/udp

CMD ["./cursor-udp"]
