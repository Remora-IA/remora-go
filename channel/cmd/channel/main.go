package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"channel/internal"
)

func main() {
	loadDotEnv()

	// Cloud Run inyecta PORT como env var (Axioma de deploy)
	defaultAddr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		defaultAddr = ":" + p
	}

	addr := flag.String("addr", defaultAddr, "Address to listen")
	baseDir := flag.String("base-dir", envOr("CHANNEL_BASE_DIR", "/workspace"), "Base directory for file operations")
	apiKeysFlag := flag.String("api-keys", os.Getenv("CHANNEL_API_KEYS"), "Comma-separated list of API keys (or env CHANNEL_API_KEYS)")
	flag.Parse()

	// Validar BASE_DIR (Axioma 4.3)
	if valid, errMsg := internal.ValidateBaseDir(*baseDir); !valid {
		log.Fatalf("Invalid BASE_DIR: %s", errMsg)
	}

	// Parsear API keys
	var keys []string
	for _, k := range strings.Split(*apiKeysFlag, ",") {
		if k = strings.TrimSpace(k); k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		log.Fatal("At least one API key is required (--api-keys or CHANNEL_API_KEYS)")
	}

	handler := internal.NewHandler(*baseDir, keys)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.Handle)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("Channel listening on %s (base_dir=%s, keys=%d)", *addr, *baseDir, len(keys))
	os.Setenv("CHANNEL_URL", "http://127.0.0.1"+*addr)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("Shutting down...")
		os.Exit(0)
	}()

	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadDotEnv() {
	candidates := []string{
		".env",
		"../.env",
		"../../.env",
		"../../../.env",
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		log.Printf(".env loaded from %s", path)
		return
	}
}
