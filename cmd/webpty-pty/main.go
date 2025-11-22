package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/PiranhaCodes/webpty-pty/internal/api"
	"github.com/PiranhaCodes/webpty-pty/internal/pty"
)

func main() {
	cfgpath := flag.String("config", "/etc/webpty/config.yml", "Path to configuration file")
	socketPath := flag.String("socket", "/run/webpty/pty.sock", "Path to Unix socket")
	flag.Parse()

	log.Printf("[PTY] Starting server with config: %s and socket: %s", *cfgpath, *socketPath)

	socketDir := filepath.Dir(*socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		log.Fatalf("[PTY] Failed to create socket directory: %v", err)
	}

	if err := os.MkdirAll("/run/webpty/sessions", 0755); err != nil {
		log.Fatalf("[PTY] Failed to create sessions directory: %v", err)
	}

	if err := os.MkdirAll("/var/log/webpty", 0755); err != nil {
		log.Fatalf("[PTY] Failed to create log directory: %v", err)
	}

	if _, err := os.Stat(*cfgpath); err == nil {
		log.Printf("[PTY] Config file found at %s (using defaults for now)", *cfgpath)
	} else {
		log.Printf("[PTY] Config file not found at %s, using defaults", *cfgpath)
	}

	server := api.NewServer(*socketPath)

	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("[PTY] Failed to start server: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[PTY] Shutting down server...")
	pty.CleanupAllSessions()
	server.Stop()
	log.Println("[PTY] Server shutdown complete")
}
