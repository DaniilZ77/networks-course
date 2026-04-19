package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

const (
	defaultServerAddress = "127.0.0.1:8888"
	requestCount         = 10
	timeout              = time.Second
	bufferSize           = 1024
)

func main() {
	serverAddress := defaultServerAddress
	if len(os.Args) > 1 {
		serverAddress = os.Args[1]
	}

	conn, err := net.Dial("udp", serverAddress)
	if err != nil {
		log.Fatalf("failed to connect to %s: %v", serverAddress, err)
	}
	defer conn.Close()

	buffer := make([]byte, bufferSize)

	for sequence := 1; sequence <= requestCount; sequence++ {
		sentAt := time.Now()
		message := fmt.Sprintf("Ping %d %s", sequence, sentAt.Format(time.RFC3339Nano))

		if _, err := conn.Write([]byte(message)); err != nil {
			log.Fatalf("failed to send request %d: %v\n", sequence, err)
		}

		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			log.Fatalf("failed to set deadline: %v\n", err)
		}

		n, err := conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Printf("Request timed out (seq=%d)\n", sequence)
				continue
			}
			log.Fatalf("failed to receive reply for request %d: %v\n", sequence, err)
		}

		rtt := time.Since(sentAt).Seconds()
		fmt.Printf("Reply from %s: %s | RTT = %.6f s\n", serverAddress, string(buffer[:n]), rtt)

		time.Sleep(time.Second)
	}
}
