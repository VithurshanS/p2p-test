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
	timeout  = 30 * time.Second
)

func init() {
	flag.IntVar(&port, "port", 5000, "Local UDP port to listen on")
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

	// 4. UDP Hole Punching & Chat
	fmt.Println("\nStarting UDP hole punching...")
	
	stopChan := make(chan struct{})
	successChan := make(chan net.Addr)
	
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
				
				msg := string(buf[:n])
				
				// Once we receive a packet from the remote peer, we consider the hole punched
				if raddr.String() == remoteAddr.String() {
					if msg != "punch" {
						fmt.Printf("\r[PEER]: %s\n> ", msg)
					}
					
					select {
					case successChan <- raddr:
					default:
					}
				}
			}
		}
	}()

	// Sender / Control loop
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
				fmt.Println("CHAT START (Type message and press Enter)")
				fmt.Print("> ")
				established = true
				ticker.Stop()

				// Start the input goroutine for chat
				go func() {
					scanner := bufio.NewScanner(os.Stdin)
					for scanner.Scan() {
						text := scanner.Text()
						if text == "" {
							continue
						}
						_, err := conn.WriteToUDP([]byte(text), remoteAddr)
						if err != nil {
							fmt.Printf("\n[ERROR] Failed to send: %v\n> ", err)
						} else {
							fmt.Print("> ")
						}
					}
				}()
			}

		case <-ticker.C:
			if !established {
				_, err := conn.WriteToUDP([]byte("punch"), remoteAddr)
				if err != nil {
					fmt.Printf("\nError sending punch: %v\n", err)
				} else {
					fmt.Print(".")
				}
			}
		}
	}
}

func getPublicAddr(conn *net.UDPConn) (net.Addr, error) {
	stunServer := "stun.l.google.com:19302"
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	raddr, err := net.ResolveUDPAddr("udp", stunServer)
	if err != nil {
		return nil, err
	}

	_, err = conn.WriteToUDP(message.Raw, raddr)
	if err != nil {
		return nil, err
	}

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

	localPort := conn.LocalAddr().(*net.UDPAddr).Port
	if xorAddr.Port == localPort {
		fmt.Println("NAT Type: No NAT or Full Cone NAT")
	} else {
		fmt.Println("NAT Type: NAT detected")
	}

	return &net.UDPAddr{
		IP:   xorAddr.IP,
		Port: xorAddr.Port,
	}, nil
}
