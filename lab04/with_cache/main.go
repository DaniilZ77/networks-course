package main

import (
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

const cacheDir = "cache"

type CacheEntry struct {
	StatusCode   int    `json:"status_code"`
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"`
}

func urlToKey(url string) string {
	r := strings.NewReplacer("://", "_", "/", "_", ":", "_", "?", "_")
	return r.Replace(url)
}

func loadCache(url string) (*CacheEntry, []byte) {
	key := urlToKey(url)
	meta, err1 := os.ReadFile(cacheDir + "/" + key + ".json")
	body, err2 := os.ReadFile(cacheDir + "/" + key + ".body")
	if err1 != nil || err2 != nil {
		return nil, nil
	}
	var entry CacheEntry
	json.Unmarshal(meta, &entry)
	return &entry, body
}

func saveCache(url string, resp *http.Response, body []byte) {
	key := urlToKey(url)
	entry := CacheEntry{
		StatusCode:   resp.StatusCode,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}
	meta, _ := json.Marshal(entry)
	os.WriteFile(cacheDir+"/"+key+".json", meta, 0644)
	os.WriteFile(cacheDir+"/"+key+".body", body, 0644)
}

func copyResponse(w http.ResponseWriter, resp *http.Response, body []byte) {
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

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

	if r.Method == http.MethodGet {
		entry, body := loadCache(path)
		if entry != nil {
			req, _ := http.NewRequest(http.MethodGet, path, nil)
			if entry.ETag != "" {
				req.Header.Set("If-None-Match", entry.ETag)
			}
			if entry.LastModified != "" {
				req.Header.Set("If-Modified-Since", entry.LastModified)
			}
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusNotModified {
					slog.Info("cache hit", "url", path)
					w.WriteHeader(entry.StatusCode)
					w.Write(body)
					return
				}
				body, _ = io.ReadAll(resp.Body)
				saveCache(path, resp, body)
				slog.Info("cache updated", "url", path)
				copyResponse(w, resp, body)
				return
			}
		}
	}

	req, err := http.NewRequest(r.Method, path, r.Body)
	if err != nil {
		http.Error(w, "Bad request: "+err.Error(), 400)
		slog.Error("bad request", "url", path, "err", err)
		return
	}
	for k, vals := range r.Header {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	req.Header.Del("Accept-Encoding")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Bad gateway: "+err.Error(), 502)
		slog.Error("bad gateway", "url", path, "err", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if r.Method == http.MethodGet && resp.StatusCode == http.StatusOK {
		saveCache(path, resp, body)
		slog.Info("cache miss", "url", path)
	} else {
		slog.Info("proxied", "method", r.Method, "url", path, "status", resp.StatusCode)
	}

	copyResponse(w, resp, body)
}

func main() {
	os.MkdirAll(cacheDir, 0755)

	logFile, err := os.OpenFile("proxy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	logger := slog.New(slog.NewTextHandler(io.MultiWriter(logFile, os.Stdout), nil))
	slog.SetDefault(logger)

	slog.Info("proxy started", "addr", "http://localhost:8888")
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8888", nil))
}
