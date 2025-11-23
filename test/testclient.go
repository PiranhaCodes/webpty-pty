package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const socketPath = "~/.webpty/pty.sock"

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

type Request struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

type Response struct {
	Ok   bool        `json:"ok"`
	Err  string      `json:"err,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

type SpawnResponse struct {
	ID string `json:"id"`
}

type WriteRequest struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type ListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
	Count    int           `json:"count"`
}

type SessionInfo struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func main() {
	log.Println("[TestClient] Starting test client...")

	// Expand socket path
	expandedSocketPath, err := expandPath(socketPath)
	if err != nil {
		log.Fatalf("[TestClient] Failed to expand socket path: %v", err)
	}

	// Connect to server
	conn, err := net.Dial("unix", expandedSocketPath)
	if err != nil {
		log.Fatalf("[TestClient] Failed to connect: %v", err)
	}
	defer conn.Close()

	log.Println("[TestClient] Connected to server")

	// Spawn a new session
	sessionID, err := spawnSession(conn)
	if err != nil {
		log.Fatalf("[TestClient] Failed to spawn session: %v", err)
	}

	log.Printf("[TestClient] Spawned session: %s", sessionID)

	// Open FIFO for reading (derive from socket path)
	sessionsDir := filepath.Join(filepath.Dir(expandedSocketPath), "sessions")
	fifoPath := filepath.Join(sessionsDir, fmt.Sprintf("%s.out", sessionID))
	
	// Wait a bit for FIFO to be created
	time.Sleep(200 * time.Millisecond)
	fifo, err := os.OpenFile(fifoPath, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		log.Fatalf("[TestClient] Failed to open FIFO: %v", err)
	}
	defer fifo.Close()

	log.Printf("[TestClient] Opened FIFO: %s", fifoPath)

	// Set up signal handler for graceful exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start reading from FIFO in background
	outputChan := make(chan []byte, 100)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := fifo.Read(buf)
			if err != nil {
				if err == io.EOF {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if netErr, ok := err.(*net.OpError); ok && netErr.Err.Error() == "resource temporarily unavailable" {
					// Non-blocking read returned EAGAIN, wait and retry
					time.Sleep(100 * time.Millisecond)
					continue
				}
				log.Printf("[TestClient] FIFO read error: %v", err)
				return
			}
			if n > 0 {
				outputChan <- buf[:n]
			}
		}
	}()

	// Send some commands
	commands := []string{
		"echo 'Hello from PTY!'\n",
		"pwd\n",
		"whoami\n",
		"echo 'Test complete'\n",
	}

	for i, cmd := range commands {
		log.Printf("[TestClient] Sending command %d: %q", i+1, cmd)
		if err := writeToSession(conn, sessionID, cmd); err != nil {
			log.Printf("[TestClient] Failed to write: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Read any available output
		select {
		case output := <-outputChan:
			fmt.Print(string(output))
		case <-time.After(100 * time.Millisecond):
			// No output yet
		}
	}

	// Wait a bit for all output
	time.Sleep(1 * time.Second)

	// Read remaining output
	for {
		select {
		case output := <-outputChan:
			fmt.Print(string(output))
		case <-time.After(500 * time.Millisecond):
			goto doneReading
		}
	}
doneReading:

	// List sessions
	log.Println("[TestClient] Listing sessions...")
	if err := listSessions(conn); err != nil {
		log.Printf("[TestClient] Failed to list sessions: %v", err)
	}

	// Wait for interrupt or kill session
	select {
	case <-sigChan:
		log.Println("[TestClient] Received interrupt signal")
	case <-time.After(5 * time.Second):
		log.Println("[TestClient] Timeout, killing session...")
	}

	// Kill session
	log.Printf("[TestClient] Killing session %s...", sessionID)
	if err := killSession(conn, sessionID); err != nil {
		log.Printf("[TestClient] Failed to kill session: %v", err)
	} else {
		log.Println("[TestClient] Session killed successfully")
	}

	log.Println("[TestClient] Test client exiting")
}

func spawnSession(conn net.Conn) (string, error) {
	req := Request{
		Action: "spawn",
		Data:   json.RawMessage("{}"),
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return "", err
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return "", err
	}

	if !resp.Ok {
		return "", fmt.Errorf("spawn failed: %s", resp.Err)
	}

	// Extract ID from data
	dataBytes, _ := json.Marshal(resp.Data)
	var spawnResp SpawnResponse
	if err := json.Unmarshal(dataBytes, &spawnResp); err != nil {
		return "", fmt.Errorf("failed to parse spawn response: %v", err)
	}

	return spawnResp.ID, nil
}

func writeToSession(conn net.Conn, sessionID, data string) error {
	writeReq := WriteRequest{
		ID:   sessionID,
		Data: data,
	}

	writeData, _ := json.Marshal(writeReq)
	req := Request{
		Action: "write",
		Data:   writeData,
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}

	if !resp.Ok {
		return fmt.Errorf("write failed: %s", resp.Err)
	}

	return nil
}

func listSessions(conn net.Conn) error {
	req := Request{
		Action: "list",
		Data:   json.RawMessage("{}"),
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}

	if !resp.Ok {
		return fmt.Errorf("list failed: %s", resp.Err)
	}

	dataBytes, _ := json.Marshal(resp.Data)
	var listResp ListResponse
	if err := json.Unmarshal(dataBytes, &listResp); err != nil {
		return fmt.Errorf("failed to parse list response: %v", err)
	}

	fmt.Printf("[TestClient] Active sessions: %d\n", listResp.Count)
	for _, sess := range listResp.Sessions {
		fmt.Printf("  - %s (%s)\n", sess.ID, sess.Status)
	}

	return nil
}

func killSession(conn net.Conn, sessionID string) error {
	killData, _ := json.Marshal(map[string]string{"id": sessionID})
	req := Request{
		Action: "kill",
		Data:   killData,
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}

	if !resp.Ok {
		return fmt.Errorf("kill failed: %s", resp.Err)
	}

	return nil
}
