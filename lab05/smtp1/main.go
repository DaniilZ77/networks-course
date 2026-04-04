package main

import (
	"flag"
	"log"
	"net/smtp"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	var (
		smtpAddr    string = "smtp.gmail.com:587"
		from        string = "zolindaniil7705@gmail.com"
		to          string
		subjectPath string
		contentPath string
	)

	flag.StringVar(&to, "to", "", "recipient email address")
	flag.StringVar(&subjectPath, "subject", "", "path to subject file")
	flag.StringVar(&contentPath, "content", "", "path to content file, must be one of .txt or .html")
	flag.Parse()

	if smtpAddr == "" ||
		to == "" ||
		subjectPath == "" ||
		contentPath == "" ||
		!strings.HasSuffix(contentPath, ".txt") && !strings.HasSuffix(contentPath, ".html") {
		flag.Usage()
		return
	}

	subjectBytes, err := os.ReadFile(subjectPath)
	if err != nil {
		log.Fatal(err)
	}
	subject := strings.TrimSpace(string(subjectBytes))

	contentBytes, err := os.ReadFile(contentPath)
	if err != nil {
		log.Fatal(err)
	}

	contentType := "text/plain"
	if strings.HasSuffix(contentPath, ".html") {
		contentType = "text/html"
	}

	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: " + contentType + "; charset=UTF-8",
	}

	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	auth := smtp.PlainAuth("", from, os.Getenv("SMTP_PASSWORD"), strings.Split(smtpAddr, ":")[0])

	msg := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + string(contentBytes))
	if err := smtp.SendMail(
		smtpAddr,
		auth,
		from,
		[]string{to},
		msg,
	); err != nil {
		log.Fatal(err)
	}
}
