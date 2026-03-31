package main

import (
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

func handler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	if path == "" {
		http.Error(w, "Usage: http://localhost:8888/<path>", 400)
		return
	}

	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		path = "http://" + path
	}

	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequest(r.Method, path, r.Body)
	if err != nil {
		http.Error(w, "Bad request: "+err.Error(), 400)
		slog.Error("bad request", "method", r.Method, "url", path, "err", err)
		return
	}

	for key, values := range r.Header {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Bad gateway: "+err.Error(), 502)
		slog.Error("bad gateway", "method", r.Method, "url", path, "err", err)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	slog.Info("proxied", "method", r.Method, "url", path, "status", resp.StatusCode)
}

func main() {
	logFile, err := os.OpenFile("proxy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	slog.Info("proxy started", "addr", "http://localhost:8888")

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8888", nil))
}
