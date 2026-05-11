package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pion/stun/v2"
)

var (
	port     int
	autoSend string
	timeout  = 30 * time.Second
)

func init() {
	flag.IntVar(&port, "port", 5000, "Local UDP port to listen on")
	flag.StringVar(&autoSend, "auto-send", "", "Continuously send this message after connection established")
}

func main() {
	flag.Parse()

	// 1. Open UDP socket
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Fatalf("Failed to resolve local address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP: %v", err)
	}
	defer conn.Close()

	fmt.Printf("Local UDP: %s\n", conn.LocalAddr().String())

	// 2. Discover Public Address via STUN
	publicAddr, err := getPublicAddr(conn)
	if err != nil {
		log.Fatalf("Failed to get public address: %v", err)
	}
	fmt.Printf("Public mapped address: %s\n", publicAddr.String())

	// 3. Manual Input of Remote Peer Address
	fmt.Print("\nWaiting for remote peer address (e.g., 124.x.x.x:52341): ")
	reader := bufio.NewReader(os.Stdin)
	remoteAddrStr, _ := reader.ReadString('\n')
	remoteAddrStr = strings.TrimSpace(remoteAddrStr)

	if remoteAddrStr == "" {
		log.Fatal("Remote address cannot be empty")
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", remoteAddrStr)
	if err != nil {
		log.Fatalf("Invalid remote address: %v", err)
	}
	fmt.Printf("Remote peer: %s\n", remoteAddr.String())

	// 4. UDP Hole Punching
	fmt.Println("\nStarting UDP hole punching...")
	
	stopChan := make(chan struct{})
	successChan := make(chan net.Addr)
	
	// Packet counters
	sentCount := 0
	recvCount := 0

	// Receiver goroutine
	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-stopChan:
				return
			default:
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))
				n, raddr, err := conn.ReadFrom(buf)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					return
				}
				
				recvCount++
				// Print received packet info
				fmt.Printf("\r[RECV] Received %d bytes from %s (Total: %d)      ", n, raddr.String(), recvCount)
				
				// Once we receive a packet from the remote peer, we consider the hole punched
				if raddr.String() == remoteAddr.String() {
					select {
					case successChan <- raddr:
					default:
					}
				}
			}
		}
	}()

	// Sender loop (Keepalive / Hole punch)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	established := false

	for {
		select {
		case <-timeoutTimer.C:
			if !established {
				close(stopChan)
				fmt.Println("\n\nTimeout: Failed to establish direct P2P connection after 30 seconds.")
				fmt.Println("Possible reasons:")
				fmt.Println("- One or both peers are behind Symmetric NAT.")
				fmt.Println("- Firewall is dropping unknown UDP packets.")
				fmt.Println("- Incorrect public address entered.")
				return
			}

		case peerAddr := <-successChan:
			if !established {
				if !timeoutTimer.Stop() {
					select {
					case <-timeoutTimer.C:
					default:
					}
				}
				fmt.Printf("\n\nDirect P2P established with %s\n", peerAddr.String())
				established = true
				if autoSend == "" {
					fmt.Println("Press Ctrl+C to exit.")
				}
			}

		case <-ticker.C:
			msg := "punch"
			if established && autoSend != "" {
				msg = autoSend
			}
			
			_, err := conn.WriteToUDP([]byte(msg), remoteAddr)
			if err != nil {
				fmt.Printf("\nError sending packet: %v\n", err)
			} else {
				sentCount++
				if !established {
					fmt.Printf("\r[SENT] Sending hole-punching packet to %s (Total: %d)...          ", remoteAddr.String(), sentCount)
				} else if autoSend != "" {
					fmt.Printf("\r[SENT] Sending '%s' to %s (Total: %d)...          ", autoSend, remoteAddr.String(), sentCount)
				}
			}
		}
	}
}

func getPublicAddr(conn *net.UDPConn) (net.Addr, error) {
	// STUN server address
	stunServer := "stun.l.google.com:19302"
	
	// We need to use the same connection, so we can't use stun.Dial
	// Instead, we build the message and send it manually
	
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	
	raddr, err := net.ResolveUDPAddr("udp", stunServer)
	if err != nil {
		return nil, err
	}

	// Simple NAT type detection: check if port changes between multiple STUN requests
	// (Though we only do one for now to keep it minimal)
	
	_, err = conn.WriteToUDP(message.Raw, raddr)
	if err != nil {
		return nil, err
	}

	// Read response
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		return nil, err
	}

	res := &stun.Message{Raw: buf[:n]}
	if err := res.Decode(); err != nil {
		return nil, err
	}

	var xorAddr stun.XORMappedAddress
	if err := xorAddr.GetFrom(res); err != nil {
		return nil, err
	}

	// Basic NAT info
	localPort := conn.LocalAddr().(*net.UDPAddr).Port
	if xorAddr.Port == localPort {
		fmt.Println("NAT Type: No NAT or Full Cone NAT (Local port matches public port)")
	} else {
		fmt.Println("NAT Type: NAT detected (Local port is different from public port)")
	}

	return &net.UDPAddr{
		IP:   xorAddr.IP,
		Port: xorAddr.Port,
	}, nil
}
