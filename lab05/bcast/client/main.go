package main

import (
	"fmt"
	"net"
)

func main() {
	addr := net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 9999,
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		fmt.Println("Failed to create UDP listener:", err)
		return
	}
	defer conn.Close()

	buf := make([]byte, 1024)

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Failed to read from UDP:", err)
			continue
		}

		fmt.Println("Current time from server:", string(buf[:n]))
	}
}
