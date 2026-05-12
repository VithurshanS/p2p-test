# UDP Hole Punching Chat - Code Walkthrough

## What this program does

This Go CLI program creates a direct UDP peer-to-peer chat connection using UDP hole punching.

Flow:
1. Open local UDP socket.
2. Ask STUN server for public mapped address.
3. Accept remote peer public address manually.
4. Send periodic `punch` packets.
5. When packets are received from remote peer, switch to chat mode.

---

## Function-by-function explanation

### `init()`
Purpose:
- Registers CLI flag `-port`.

Why:
- Lets user choose local UDP listening port dynamically (default 5000).

Code:
```go
func init() {
    flag.IntVar(&port, "port", 5000, "Local UDP port to listen on")
}
```

---

### `main()`
Purpose:
- Orchestrates the entire program lifecycle.

Why:
- This is the runtime control center for socket setup, STUN lookup, peer input, hole punching, and chat.

Major responsibilities:
1. Parse flags.
2. Create UDP listener.
3. Discover public address via STUN.
4. Read and validate remote peer address.
5. Run concurrent receiver + control loop.
6. Start stdin chat sender after direct connectivity is confirmed.

---

### `getPublicAddr(conn *net.UDPConn) (net.Addr, error)`
Purpose:
- Sends STUN binding request and extracts XOR-mapped public address.

Why:
- In NAT environments, local socket address is not the public internet address. STUN provides the external mapped IP:port.

What it returns:
- `net.Addr` containing public mapped address.
- `error` if STUN interaction fails.

---

## Line-by-line explanation

### Package and imports
- **Line 1**: `package main` -> executable program package.
- **Lines 3-14**: Imports standard libs plus `github.com/pion/stun/v2` for STUN protocol.

### Global config
- **Lines 16-19**:
  - `port int` -> CLI-configurable local UDP port.
  - `timeout = 30 * time.Second` -> max wait for P2P establishment.

### init
- **Line 21**: `init` function starts.
- **Line 22**: Binds `-port` flag to `port` variable (default `5000`).

### main setup
- **Line 26**: Parses CLI flags.
- **Line 29**: Builds local UDP address `0.0.0.0:<port>`.
- **Lines 30-32**: Fatal exit if address resolve fails.
- **Line 34**: Opens UDP socket.
- **Lines 35-37**: Fatal exit if bind/listen fails.
- **Line 38**: Ensure UDP socket closes on exit.
- **Line 40**: Print local UDP bind address.

### STUN discovery
- **Line 43**: Call `getPublicAddr(conn)`.
- **Lines 44-46**: Fatal exit if STUN fails.
- **Line 47**: Print discovered public mapped address.

### Read remote peer address
- **Line 50**: Prompt user for peer public IP:port.
- **Line 51**: Create stdin reader.
- **Line 52**: Read one line.
- **Line 53**: Trim whitespace/newline.
- **Lines 55-57**: Reject empty peer address.
- **Line 59**: Resolve remote UDP address.
- **Lines 60-62**: Fatal exit if invalid address format.
- **Line 63**: Print resolved remote peer.

### Hole punching control channels
- **Line 68**: `stopChan` signals receiver goroutine shutdown.
- **Line 69**: `successChan` notifies when peer packet is received.

### Receiver goroutine
- **Line 72**: Start background receiver goroutine.
- **Line 73**: Allocate receive buffer.
- **Line 74**: Enter infinite receive loop.
- **Lines 75-78**: Exit if stop signal received.
- **Line 79**: Set 1-second read deadline (prevents blocking forever).
- **Line 80**: Read UDP packet.
- **Lines 81-86**:
  - Ignore timeout errors (loop again).
  - Return on other read errors.
- **Line 88**: Convert packet bytes to string.
- **Line 91**: Accept only packets from expected `remoteAddr`.
- **Lines 92-94**: If payload is not `punch`, display it as chat message.
- **Lines 96-99**: Non-blocking send to `successChan`.

### Sender/control loop
- **Line 106**: Ticker every 1 second for punch packets.
- **Line 107**: Ensure ticker cleanup.
- **Line 109**: Timeout timer using global timeout.
- **Line 110**: Ensure timer cleanup.
- **Line 112**: `established` state flag.
- **Line 114**: Infinite event loop.

#### Timeout case
- **Line 116**: Timeout event triggers.
- **Lines 117-121**: If not yet established, stop receiver, print timeout, exit.

#### Success case
- **Line 123**: Incoming success signal with peer address.
- **Line 124**: Only run setup once.
- **Lines 125-130**: Stop/drain timeout timer safely.
- **Line 131**: Print direct P2P established message.
- **Line 132**: Announce chat start.
- **Line 133**: Show prompt.
- **Line 134**: Mark connected state.
- **Line 135**: Stop punch ticker (no longer needed).
- **Line 138**: Start chat input goroutine.
- **Line 139**: Create scanner for stdin lines.
- **Line 140**: Loop on user input.
- **Line 141**: Read typed text.
- **Lines 142-144**: Skip empty messages.
- **Line 145**: Send text as UDP packet to `remoteAddr`.
- **Lines 146-150**: Print send error or prompt.

#### Periodic punch case
- **Line 155**: Ticker tick event.
- **Line 156**: Only punch while not established.
- **Line 157**: Send `"punch"` UDP packet.
- **Lines 158-162**: Print error or progress dot.

### STUN helper function details (`getPublicAddr`)
- **Line 169**: STUN server target (`stun.l.google.com:19302`).
- **Line 170**: Build STUN Binding Request.
- **Line 171**: Resolve STUN server UDP address.
- **Lines 172-174**: Return error if resolve fails.
- **Line 176**: Send STUN request over existing UDP socket.
- **Lines 177-179**: Return error if send fails.
- **Line 181**: Response buffer.
- **Line 182**: 5-second read deadline.
- **Line 183**: Read STUN response.
- **Lines 184-186**: Return error on read failure.
- **Line 188**: Build STUN message from raw bytes.
- **Lines 189-191**: Decode STUN message.
- **Line 193**: Declare XOR mapped address holder.
- **Lines 194-196**: Extract mapped public IP/port from response.
- **Line 198**: Read local UDP port from socket.
- **Lines 199-203**: Print simple NAT hint based on port comparison.
- **Lines 205-208**: Return mapped public address as `net.UDPAddr`.

---

## Architecture used

This is a **concurrent, event-driven CLI networking architecture** with a **single-process state machine**.

Key architectural elements:
1. **Single binary entrypoint** (`main`) for orchestration.
2. **Functional decomposition** (`getPublicAddr`) for STUN concern.
3. **Concurrency with goroutines**:
   - Receiver goroutine for incoming UDP.
   - Input goroutine for chat messages.
4. **Channel-based signaling**:
   - `stopChan` for cancellation.
   - `successChan` for connection-establishment event.
5. **Timer/Ticker-driven events**:
   - `ticker` for periodic punch packets.
   - `timeoutTimer` for fail-fast termination.
6. **State flag** (`established`) controlling behavior transitions.

You can think of runtime behavior as states:
- `Init -> DiscoverPublicAddr -> AwaitPeerAddress -> Punching -> EstablishedChat`.

This pattern is common in lightweight P2P networking tools where direct socket control and timing behavior matter.
