use std::io;
use std::net::UdpSocket;

fn main() -> io::Result<()> {
    // Get port from environment variable or use default
    let port = std::env::var("SERVICE_PORT").unwrap_or_else(|_| "9001".to_string());
    let bind_addr = format!("0.0.0.0:{}", port);

    println!("Cursor UDP Service starting...");
    println!("Binding to {}", bind_addr);

    let socket = UdpSocket::bind(&bind_addr)?;
    println!("Cursor UDP service listening on port {}", port);
    println!("Ready to receive cursor movement data");

    let mut buf = [0u8; 2048];

    loop {
        match socket.recv_from(&mut buf) {
            Ok((amt, src)) => {
                let data = &buf[..amt];
                println!(
                    "Received {} bytes from {}: {:?}",
                    amt,
                    src,
                    String::from_utf8_lossy(data)
                );

                // Echo back to sender for now (you can implement broadcast logic later)
                if let Err(e) = socket.send_to(data, src) {
                    eprintln!("Failed to send response to {}: {}", src, e);
                }
            }
            Err(e) => {
                eprintln!("Error receiving data: {}", e);
            }
        }
    }
}
