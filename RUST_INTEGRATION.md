# Adding the Rust Cursor UDP Service

This guide explains how to integrate the Rust cursor-udp service into the monorepo.

## Quick Start

```bash
cd services/cursor-udp
cargo init --name cursor-udp
```

Then implement your UDP server in `src/main.rs`.

## Integration Checklist

### ✅ Already Done

- [x] Created `services/cursor-udp/` directory
- [x] Created Dockerfile with multi-stage build
- [x] Added to `.gitignore` (Rust artifacts)
- [x] Updated Makefile with Rust targets
- [x] Configured docker-compose.yml (commented out, ready to enable)

### 🔲 To Do When Ready

1. **Initialize Rust project**
   ```bash
   cd services/cursor-udp
   cargo init --name cursor-udp
   ```

2. **Add dependencies to Cargo.toml**
   ```toml
   [dependencies]
   tokio = { version = "1", features = ["full"] }
   serde = { version = "1", features = ["derive"] }
   serde_json = "1"
   ```

3. **Implement your UDP server in src/main.rs**

4. **Enable in docker-compose.yml**
   - Uncomment the `cursor-udp` service section
   - Adjust ports/environment as needed

5. **Build and test**
   ```bash
   make build-cursor-udp
   make run-cursor-udp
   ```

## Available Make Commands

Once initialized:

- `make build-cursor-udp` - Build the Rust service
- `make run-cursor-udp` - Run the service locally
- `make test` - Run all tests (including Rust tests)
- `make clean` - Clean all build artifacts (including Rust target/)

## Docker Integration

The Dockerfile uses a two-stage build:

1. **Builder stage**: Compiles Rust code with cargo
2. **Runtime stage**: Minimal Alpine image with just the binary

This keeps the final image small (~10-20MB vs ~1GB+ with the full Rust toolchain).

## Service Communication

### From Go Services to Rust

The Go services can send UDP packets to the cursor service:

```go
import "net"

conn, err := net.Dial("udp", "cursor-udp:9001")
if err != nil {
    // handle error
}
defer conn.Close()

conn.Write([]byte("cursor data"))
```

### Port Configuration

- Local development: `localhost:9001`
- Docker Compose: `cursor-udp:9001` (service name as hostname)
- External: Exposed on host port `9001/udp`

## Example UDP Server (Rust)

See `services/cursor-udp/README.md` for a complete example.

Basic structure:

```rust
use std::net::UdpSocket;

fn main() -> std::io::Result<()> {
    let socket = UdpSocket::bind("0.0.0.0:9001")?;
    println!("Cursor UDP service listening on port 9001");

    let mut buf = [0; 1024];
    loop {
        let (amt, src) = socket.recv_from(&mut buf)?;
        // Handle cursor data and broadcast
    }
}
```

## Testing UDP Service

Use netcat to test:

```bash
# Send test data
echo "test cursor data" | nc -u localhost 9001

# Or use a simple Go/Rust client
```

## Why This Setup Works

1. **Polyglot-friendly**: Makefile abstracts the differences between Go and Rust builds
2. **Docker-native**: Each service gets its own optimized Dockerfile
3. **Network isolation**: Docker Compose provides a shared network for inter-service communication
4. **Independent scaling**: Can scale Rust and Go services separately

## Troubleshooting

### Cargo not found

Install Rust:
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

### Permission denied on UDP port

Ensure port 9001 is not already in use:
```bash
lsof -i :9001
```

### Docker build fails

Make sure `Cargo.toml` exists before building Docker image.
