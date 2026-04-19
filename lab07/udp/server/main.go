package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"
)

const (
	listenAddress = ":8888"
	lossRate      = 0.2
	bufferSize    = 1024
)

func main() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	conn, err := net.ListenPacket("udp", listenAddress)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v\n", listenAddress, err)
	}
	defer conn.Close()

	log.Printf("UDP ping server is listening on %s\n", listenAddress)

	buffer := make([]byte, bufferSize)
	for {
		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			log.Printf("read error: %v\n", err)
			continue
		}

		message := string(buffer[:n])
		if rng.Float64() < lossRate {
			log.Printf("simulated packet loss for %q from %s\n", strings.TrimSpace(message), addr.String())
			continue
		}

		response := strings.ToUpper(message)
		if _, err := conn.WriteTo([]byte(response), addr); err != nil {
			log.Printf("write error to %s: %v\n", addr.String(), err)
			continue
		}

		fmt.Printf("handled request from %s: %s\n", addr.String(), response)
	}
}
