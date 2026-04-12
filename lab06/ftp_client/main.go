package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/joho/godotenv"
)

func main() {
	var (
		command string
		srcPath string
		dstPath string
	)

	flag.StringVar(&command, "command", "", "FTP command to execute (list, download, upload)")
	flag.StringVar(&srcPath, "src", "", "Source path for the FTP command")
	flag.StringVar(&dstPath, "dst", "", "Destination path for the FTP command")

	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	login := os.Getenv("LOGIN")
	if login == "" {
		log.Fatal("LOGIN env variable is not set")
	}

	password := os.Getenv("PASSWORD")
	if password == "" {
		log.Fatal("PASSWORD env variable is not set")
	}

	c, err := ftp.Dial("localhost:21", ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Login(login, password)
	if err != nil {
		log.Fatal(err)
	}

	switch command {
	case "list":
		entries, err := c.List(srcPath)
		if err != nil {
			log.Fatal(err)
		}
		for _, entry := range entries {
			log.Printf("%s: %s", entry.Type, entry.Name)
		}
	case "download":
		r, err := c.Retr(srcPath)
		if err != nil {
			panic(err)
		}
		defer r.Close()

		f, err := os.Create(dstPath)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		if _, err := f.ReadFrom(r); err != nil {
			log.Fatal(err)
		}
	case "upload":
		f, err := os.Open(srcPath)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		err = c.Stor(dstPath, f)
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("Unsupported command: %s", command)
	}

	if err := c.Quit(); err != nil {
		log.Fatal(err)
	}
}
