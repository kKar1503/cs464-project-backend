# Cursor UDP Service (Rust)

A high-performance UDP service for broadcasting real-time cursor movements between players.

## Setup

### Initialize the Rust Project

```bash
cd services/cursor-udp
cargo init --name cursor-udp
```

### Add Dependencies

Edit `Cargo.toml` to add your dependencies. Common ones for UDP networking:

```toml
[package]
name = "cursor-udp"
version = "0.1.0"
edition = "2021"

[dependencies]
tokio = { version = "1", features = ["full"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
```

### Example UDP Server

Here's a basic example for `src/main.rs`:

```rust
use std::net::UdpSocket;

fn main() -> std::io::Result<()> {
    let socket = UdpSocket::bind("0.0.0.0:9001")?;
    println!("Cursor UDP service listening on port 9001");

    let mut buf = [0; 1024];
    loop {
        let (amt, src) = socket.recv_from(&mut buf)?;
        let msg = &buf[..amt];
        println!("Received {} bytes from {}", amt, src);

        // Broadcast to all connected clients (implement your logic here)
    }
}
```

## Building

### Local Build

```bash
# From the service directory
cargo build --release

# Or from the root
make build-cursor-udp
```

### Docker Build

```bash
# From the root directory
docker-compose build cursor-udp
```

## Running

### Local Run

```bash
# From the service directory
cargo run --release

# Or from the root
make run-cursor-udp
```

### With Docker Compose

1. Uncomment the cursor-udp service in `docker-compose.yml`
2. Run:
```bash
make docker-up
```

## Testing

```bash
# From the service directory
cargo test

# Or from the root (runs all tests including Rust)
make test
```

## Why Rust + UDP?

- **Performance**: Rust's zero-cost abstractions and memory safety without garbage collection
- **Low Latency**: UDP avoids TCP's overhead, crucial for real-time cursor updates
- **Concurrency**: Tokio's async runtime handles thousands of concurrent connections efficiently
- **Reliability**: Rust's type system prevents common networking bugs

## Port

Default: `9001/udp`

Can be configured via the `SERVICE_PORT` environment variable in docker-compose.yml
