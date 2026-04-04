package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type message struct {
	from        string
	to          string
	subject     string
	content     string
	attachments map[string][]byte
}

func newMessage() message {
	return message{
		attachments: map[string][]byte{},
	}
}

func (m *message) From(from string) {
	m.from = from
}

func (m *message) To(to string) {
	m.to = to
}

func (m *message) Subject(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	m.subject = string(data)
	return nil
}

func (m *message) Content(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	m.content = string(data)
	return nil
}

func (m *message) Attach(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	parts := strings.Split(filename, "/")
	if len(parts) == 0 {
		return fmt.Errorf("invalid filename: %v", filename)
	}

	m.attachments[parts[len(parts)-1]] = data
	return nil
}

func (m *message) Bytes() []byte {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintf(buf, "Subject: %s\r\n", m.subject)
	fmt.Fprintf(buf, "To: %s\r\n", m.to)
	buf.WriteString("MIME-Version: 1.0\r\n")

	writer := multipart.NewWriter(buf)
	boundary := writer.Boundary()

	if len(m.attachments) > 0 {
		fmt.Fprintf(buf, "Content-Type: multipart/mixed; boundary=%s\r\n", boundary)
		fmt.Fprintf(buf, "--%s\r\n", boundary)
	}

	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	buf.WriteString(m.content)
	if len(m.attachments) > 0 {
		for k, v := range m.attachments {
			fmt.Fprintf(buf, "\r\n--%s\r\n", boundary)
			fmt.Fprintf(buf, "Content-Type: %s\r\n", http.DetectContentType(v))
			buf.WriteString("Content-Transfer-Encoding: base64\r\n")
			fmt.Fprintf(buf, "Content-Disposition: attachment; filename=%s\r\n\r\n", k)

			b := make([]byte, base64.StdEncoding.EncodedLen(len(v)))
			base64.StdEncoding.Encode(b, v)
			buf.Write(b)
			fmt.Fprintf(buf, "\r\n--%s", boundary)
		}

		buf.WriteString("--")
	}

	return buf.Bytes()
}

type step struct {
	line string
	code int
	hide bool
}

var encodeBase64 = base64.StdEncoding.EncodeToString

func pipeline(
	r *bufio.Reader,
	w *bufio.Writer,
	steps []step,
) error {
	for _, s := range steps {
		req := s.line
		if s.hide {
			req = "###"
		}
		log.Printf("REQ: %v, EXP_CODE: %d\n", req, s.code)

		if _, err := w.WriteString(s.line + "\r\n"); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}

		resp, code, err := readResponse(r)
		if err != nil {
			return err
		}
		log.Printf("RESP: %s, RES_CODE: %d\n", resp, code)

		if code/100 != s.code {
			return fmt.Errorf("unexpected SMTP response: %d\n%s", code, resp)
		}
	}

	return nil
}

func readResponse(r *bufio.Reader) (string, int, error) {
	var lines []string
	var code int

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", 0, err
		}

		line = strings.TrimRight(line, "\r\n")
		lines = append(lines, line)

		if len(line) >= 3 {
			fmt.Sscanf(line[:3], "%d", &code)
		}

		if len(line) < 4 || line[3] != '-' {
			break
		}
	}

	return strings.Join(lines, "\n"), code, nil
}

func main() {
	var (
		smtpAddr    string = "smtp.gmail.com:587"
		from        string = "zolindaniil7705@gmail.com"
		to          string
		subjectPath string
		contentPath string
		attachPath  string
	)

	flag.StringVar(&to, "to", "", "recipient email address")
	flag.StringVar(&subjectPath, "subject", "", "path to subject file")
	flag.StringVar(&contentPath, "content", "", "path to content file")
	flag.StringVar(&attachPath, "attach", "", "file path to attach to mail")
	flag.Parse()

	if smtpAddr == "" || to == "" || subjectPath == "" || contentPath == "" {
		flag.Usage()
		return
	}

	m := newMessage()
	if err := m.Subject(subjectPath); err != nil {
		log.Fatal(err)
	}
	if err := m.Content(contentPath); err != nil {
		log.Fatal(err)
	}
	m.From(from)
	m.To(to)

	if attachPath != "" {
		if err := m.Attach(attachPath); err != nil {
			log.Fatal(err)
		}
	}

	msg := m.Bytes()

	conn, err := net.DialTimeout("tcp", smtpAddr, 10*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	password := os.Getenv("SMTP_PASSWORD")
	if password == "" {
		log.Fatal("SMTP_PASSWORD is empty")
	}

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	resp, code, err := readResponse(r)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("GREETING: %s, RES_CODE: %d\n", resp, code)
	if code != 220 {
		log.Fatalf("unexpected SMTP greeting: %d\n%s", code, resp)
	}

	if err := pipeline(r, w, []step{
		{"EHLO localhost", 2, false},
		{"STARTTLS", 2, false},
	}); err != nil {
		log.Fatal(err)
	}

	host := smtpAddr
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}

	tlsConn := tls.Client(conn, &tls.Config{
		ServerName: host,
	})
	if err := tlsConn.Handshake(); err != nil {
		log.Fatal(err)
	}

	r = bufio.NewReader(tlsConn)
	w = bufio.NewWriter(tlsConn)

	if err := pipeline(r, w, []step{
		{"EHLO localhost", 2, false},
		{"AUTH LOGIN", 3, false},
		{encodeBase64([]byte(from)), 3, true},
		{encodeBase64([]byte(password)), 2, true},
		{"MAIL FROM:<" + from + ">", 2, false},
		{"RCPT TO:<" + to + ">", 2, false},
		{"DATA", 3, false},
		{string(msg) + "\r\n.", 2, true},
		{"QUIT", 2, false},
	}); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Email sent successfully")
}
