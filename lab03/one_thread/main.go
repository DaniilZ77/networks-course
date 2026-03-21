package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		log.Println("Usage: <server.exe> server_port")
		return
	}

	port := os.Args[1]
	address := ":" + port

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Println("Failed to start server:", err)
		return
	}
	defer lis.Close()
	log.Printf("Server is listening on port %s...\n", port)

	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("Failed to accept connection:", err)
			continue
		}

		handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	request, err := reader.ReadString('\n')
	if err != nil {
		log.Println("Failed to read request:", err)
		return
	}

	request = strings.TrimSpace(request)
	log.Println("Request:", request)

	parts := strings.Split(request, " ")
	path := parts[1]

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Println("Failed to read request:", err)
			}
			break
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	path = strings.TrimPrefix(path, "/")
	path = filepath.Clean(path)

	data, err := os.ReadFile(path)
	if err != nil {
		sendResponse(conn, "404 Not Found", []byte("404 Not Found"))
		return
	}

	sendResponse(conn, "200 OK", data)
}

func sendResponse(conn net.Conn, status string, body []byte) {
	headers := fmt.Sprintf(
		"HTTP/1.1 %s\r\nContent-Length: %d\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n",
		status,
		len(body),
	)

	conn.Write([]byte(headers))
	conn.Write(body)
}
