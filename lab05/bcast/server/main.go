package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	addr := net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: 9999,
	}

	conn, err := net.DialUDP("udp", nil, &addr)
	if err != nil {
		fmt.Println("Failed to create UDP connection:", err)
		return
	}
	defer conn.Close()

	for {
		now := time.Now().Format("15:04:05")
		_, err = conn.Write([]byte(now))
		if err != nil {
			log.Println("Failed to send time:", err)
		}
		time.Sleep(1 * time.Second)
	}
}
