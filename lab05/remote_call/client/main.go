package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter command: ")
	command, _ := reader.ReadString('\n')

	fmt.Fprint(conn, command)

	r := bufio.NewReader(conn)
	for {
		resp, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("Connection closed by server.")
			} else {
				fmt.Println("Error reading:", err)
			}
			return
		}
		fmt.Print(resp)
	}
}
