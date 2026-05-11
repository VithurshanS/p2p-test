# UDP Hole Punching P2P Tester

A minimal Go tool to test direct peer-to-peer (P2P) connectivity using UDP hole punching. This tool uses STUN only for public address discovery and does NOT use TURN or any relay servers.

## How UDP Hole Punching Works

1.  **Discovery:** Both Peer A and Peer B connect to a STUN server (`stun.l.google.com:19302`) using a local UDP socket. The STUN server tells each peer their public IP and port as seen from the internet.
2.  **Mapping:** The NAT (Network Address Translator) creates a "binding" (mapping) between the internal IP:port and the external IP:port.
3.  **Manual Signaling:** You manually copy Peer A's public address to Peer B and vice-versa.
4.  **Punching:** 
    *   Peer A sends UDP packets to Peer B's public address. These outgoing packets create a "hole" in Peer A's NAT, allowing incoming packets from Peer B's IP to pass through.
    *   Peer B simultaneously sends UDP packets to Peer A's public address.
5.  **Establishment:** If both NATs are "Cone NATs" (Full Cone, Restricted Cone, or Port-Restricted Cone), the first packets sent might be dropped, but once both sides have sent packets, the NATs recognize the traffic as "requested" and allow direct communication.
6.  **Symmetric NAT:** If one side is behind a Symmetric NAT, the public port might change when connecting to different destinations, making hole punching much harder (often impossible without port prediction).

## Features

*   **STUN Discovery:** Automatically finds your public IP/port.
*   **Socket Reuse:** Uses the exact same UDP port for STUN and P2P communication (critical for hole punching).
*   **NAT Type Detection:** Basic check to see if your local port matches your public port.
*   **Keepalive:** Periodically sends packets to keep the NAT binding alive.
*   **Timeout:** Automatically stops after 30 seconds if no connection is established.
*   **Auto-Send:** Optional flag to send test messages once connected.

## Installation

Ensure you have Go installed, then:

```bash
go build -o p2p-test
```

## Usage

### 1. Start on Peer A
```bash
./p2p-test --port 5000
```

### 2. Start on Peer B
```bash
./p2p-test --port 6000
```

### 3. Exchange Addresses
Peer A will show something like: `Public mapped address: 1.2.3.4:52341`
Peer B will show something like: `Public mapped address: 5.6.7.8:61002`

Paste Peer B's address into Peer A's prompt, and Peer A's address into Peer B's prompt.

### Optional: Auto-send messages
```bash
./p2p-test --port 5000 --auto-send "Hello from Peer A!"
```

## Testing Scenarios

| Scenario | Expected Outcome | Reason |
| :--- | :--- | :--- |
| **Same LAN** | **Success** | Direct local routing or Hairpin NAT. |
| **Home Broadband (Cone NAT)** | **Success** | Most home routers use Cone NAT which allows hole punching. |
| **Mobile Hotspot (CGNAT)** | **Likely Failure** | Carrier-Grade NAT is often Symmetric or highly restrictive. |
| **Corporate Firewall** | **Failure** | Often blocks all unsolicited or unknown UDP traffic. |

## Troubleshooting

*   **Timeout:** If it times out, one or both peers likely have a **Symmetric NAT**.
*   **NAT Type:** If the tool says `NAT Type: No NAT...` and you are still failing, check your local OS firewall (Windows Firewall, ufw, etc.).
*   **Public IP:** Ensure the public IP discovered by STUN is actually reachable (not behind another layer of NAT that doesn't support UDP).

## Security Note

This is a learning and testing tool. It does not implement encryption or authentication. Use it only for connectivity testing.
