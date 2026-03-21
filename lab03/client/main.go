package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: <client.exe> server_host server_port filename")
		return
	}

	host := os.Args[1]
	port := os.Args[2]
	file := os.Args[3]

	address := host + ":" + port

	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("Failed to connect:", err)
		return
	}
	defer conn.Close()

	req := fmt.Sprintf("GET /%s HTTP/1.1\r\nHost: %s\r\n\r\n", file, host)
	_, err = conn.Write([]byte(req))
	if err != nil {
		fmt.Println("Failed to write:", err)
		return
	}

	fmt.Printf("Http request: %s", req)

	fmt.Println("Http response:")
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			fmt.Print(line)
		}
		if err != nil {
			break
		}
	}
}
