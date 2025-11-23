package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/PiranhaCodes/webpty-pty/internal/api"
	"github.com/PiranhaCodes/webpty-pty/internal/pty"
)

// expandPath expands the tilde (~) character to the user's home directory.
func expandPath(path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}

	if path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		if len(path) == 1 {
			return homeDir, nil
		}
		if path[1] == '/' || path[1] == '\\' {
			return filepath.Join(homeDir, path[2:]), nil
		}
	}

	return path, nil
}

func main() {
	cfgpath := flag.String("config", "~/.webpty/config.yml", "Path to configuration file")
	socketPathRaw := flag.String("socket", "~/.webpty/pty.sock", "Path to Unix socket")
	flag.Parse()

	// Expand paths
	socketPath, err := expandPath(*socketPathRaw)
	if err != nil {
		log.Fatalf("[PTY] Failed to expand socket path: %v", err)
	}

	cfgPathExpanded, err := expandPath(*cfgpath)
	if err != nil {
		log.Fatalf("[PTY] Failed to expand config path: %v", err)
	}

	log.Printf("[PTY] Starting server with config: %s and socket: %s", cfgPathExpanded, socketPath)

	socketDir := filepath.Dir(socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		log.Fatalf("[PTY] Failed to create socket directory: %v", err)
	}

	// Expand session and log directories
	sessionsDir, err := expandPath("~/.webpty/sessions")
	if err != nil {
		log.Fatalf("[PTY] Failed to expand sessions directory: %v", err)
	}
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		log.Fatalf("[PTY] Failed to create sessions directory: %v", err)
	}

	logDir, err := expandPath("~/.webpty/log")
	if err != nil {
		log.Fatalf("[PTY] Failed to expand log directory: %v", err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("[PTY] Failed to create log directory: %v", err)
	}

	if _, err := os.Stat(cfgPathExpanded); err == nil {
		log.Printf("[PTY] Config file found at %s (using defaults for now)", cfgPathExpanded)
	} else {
		log.Printf("[PTY] Config file not found at %s, using defaults", cfgPathExpanded)
	}

	server := api.NewServer(socketPath)

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
