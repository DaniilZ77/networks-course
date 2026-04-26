package main

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

const (
	DATA = 1
	ACK  = 2
	END  = 3
)

type Pkt struct {
	Type byte
	Seq  byte
	Data []byte
}

func enc(p Pkt) []byte {
	b := []byte{p.Type, p.Seq}
	return append(b, p.Data...)
}

func dec(b []byte) Pkt {
	if len(b) < 2 {
		return Pkt{}
	}
	return Pkt{b[0], b[1], b[2:]}
}

func loss(prob float64) bool {
	return rand.Float64() < prob
}

func sendRaw(c *net.UDPConn, addr *net.UDPAddr, b []byte, prob float64) {
	if loss(prob) {
		fmt.Println("DROP send")
		return
	}
	_, err := c.WriteToUDP(b, addr)
	if err != nil {
		fmt.Println("send err:", err)
	}
}

func sendFile(c *net.UDPConn, addr *net.UDPAddr, file string, chunk int, timeout time.Duration, prob float64) {
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Println("read err:", err)
		return
	}

	seq := byte(0)
	buf := make([]byte, 2048)

	for off := 0; off < len(data); {
		end := min(off+chunk, len(data))

		p := enc(Pkt{DATA, seq, data[off:end]})

		for {
			fmt.Println("SEND DATA", seq, off, end)
			sendRaw(c, addr, p, prob)

			_ = c.SetReadDeadline(time.Now().Add(timeout))
			n, _, err := c.ReadFromUDP(buf)
			if err != nil {
				fmt.Println("timeout, resend", seq)
				continue
			}

			r := dec(buf[:n])
			if r.Type == ACK && r.Seq == seq {
				fmt.Println("ACK", seq)
				break
			}
		}

		off = end
		seq ^= 1
	}

	for {
		fmt.Println("SEND END", seq)
		sendRaw(c, addr, enc(Pkt{END, seq, nil}), prob)

		_ = c.SetReadDeadline(time.Now().Add(timeout))
		n, _, err := c.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("timeout END")
			continue
		}

		r := dec(buf[:n])
		if r.Type == ACK && r.Seq == seq {
			fmt.Println("END ACK")
			break
		}
	}
}

func recvFile(c *net.UDPConn, out string, prob float64) *net.UDPAddr {
	_ = c.SetReadDeadline(time.Time{})

	var file bytes.Buffer
	expected := byte(0)
	buf := make([]byte, 65535)
	var peer *net.UDPAddr

	for {
		n, addr, err := c.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("recv err:", err)
			continue
		}
		peer = addr

		if loss(prob) {
			fmt.Println("DROP recv")
			continue
		}

		p := dec(buf[:n])

		if p.Type == DATA {
			fmt.Println("RECV DATA", p.Seq, "expected", expected)

			if p.Seq == expected {
				file.Write(p.Data)
				expected ^= 1
			}

			sendRaw(c, addr, enc(Pkt{ACK, p.Seq, nil}), prob)
		}

		if p.Type == END {
			fmt.Println("RECV END", p.Seq)
			sendRaw(c, addr, enc(Pkt{ACK, p.Seq, nil}), prob)

			err := os.WriteFile(out, file.Bytes(), 0644)
			if err != nil {
				fmt.Println("write err:", err)
			} else {
				fmt.Println("saved:", out)

				endSeq := p.Seq

				deadline := time.Now().Add(2 * time.Second)

				for time.Now().Before(deadline) {
					c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))

					n, addr, err := c.ReadFromUDP(buf)
					if err != nil {
						continue
					}
					peer = addr

					r := dec(buf[:n])

					if r.Type == END && r.Seq == endSeq {
						fmt.Println("RESEND ACK END")
						sendRaw(c, addr, enc(Pkt{ACK, endSeq, nil}), prob)
					}
				}
				return peer
			}
		}
	}
}

func main() {
	mode := flag.String("mode", "server", "server/client")
	addr := flag.String("addr", "127.0.0.1:9000", "server addr")
	in := flag.String("in", "in.txt", "input file")
	out := flag.String("out", "out.txt", "output file")
	chunk := flag.Int("chunk", 512, "packet size")
	timeout := flag.Int("timeout", 800, "timeout ms")
	lossP := flag.Float64("loss", 0.3, "loss probability")
	flag.Parse()

	if *mode == "server" {
		udpAddr, _ := net.ResolveUDPAddr("udp", *addr)
		conn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		fmt.Println("server listen", *addr)

		clientAddr := recvFile(conn, *out, *lossP)

		fmt.Println("send file back to client")
		sendFile(conn, clientAddr, *in, *chunk, time.Duration(*timeout)*time.Millisecond, *lossP)
		return
	}

	if *mode == "client" {
		serverAddr, _ := net.ResolveUDPAddr("udp", *addr)
		conn, err := net.ListenUDP("udp", nil)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		fmt.Println("client send file")
		sendFile(conn, serverAddr, *in, *chunk, time.Duration(*timeout)*time.Millisecond, *lossP)

		fmt.Println("client wait file from server")
		recvFile(conn, *out, *lossP)
		return
	}
}
