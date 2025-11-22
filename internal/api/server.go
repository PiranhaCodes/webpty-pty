package api

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/PiranhaCodes/webpty-pty/internal/pty"
)

// Server handles UNIX socket connections and PTY session management.
type Server struct {
	socketPath string
	listener   net.Listener
	stopChan   chan struct{}
}

// NewServer creates a new server instance.
func NewServer(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
		stopChan:   make(chan struct{}),
	}
}

// Start starts the server and begins accepting connections.
func (s *Server) Start() error {
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}

	s.listener = listener
	log.Printf("[PTY] Server listening on %s", s.socketPath)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("[PTY] Received shutdown signal, cleaning up...")
		pty.CleanupAllSessions()
		s.Stop()
		os.Exit(0)
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return nil
			default:
				return err
			}
		}
		go s.handleConn(conn)
	}
}

// Stop stops the server and closes the listener.
func (s *Server) Stop() {
	close(s.stopChan)
	if s.listener != nil {
		s.listener.Close()
	}
	log.Println("[PTY] Server stopped")
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		encoder.Encode(Response{Ok: false, Err: "invalid request: " + err.Error()})
		return
	}

	switch req.Action {
	case "spawn":
		s.handleSpawn(req.Data, encoder)
	case "write":
		s.handleWrite(req.Data, encoder)
	case "resize":
		s.handleResize(req.Data, encoder)
	case "kill":
		s.handleKill(req.Data, encoder)
	case "list":
		s.handleList(encoder)
	default:
		encoder.Encode(Response{Ok: false, Err: "unknown action: " + req.Action})
	}
}

func (s *Server) handleSpawn(data json.RawMessage, encoder *json.Encoder) {
	var req SpawnRequest
	if len(data) > 0 {
		if err := json.Unmarshal(data, &req); err != nil {
			encoder.Encode(Response{Ok: false, Err: "invalid spawn request: " + err.Error()})
			return
		}
	}

	sess, err := pty.SpawnShell()
	if err != nil {
		encoder.Encode(Response{Ok: false, Err: err.Error()})
		return
	}

	encoder.Encode(Response{
		Ok: true,
		Data: SpawnResponse{ID: sess.ID},
	})
}

func (s *Server) handleWrite(data json.RawMessage, encoder *json.Encoder) {
	var req WriteRequest
	if err := json.Unmarshal(data, &req); err != nil {
		encoder.Encode(Response{Ok: false, Err: "invalid write request: " + err.Error()})
		return
	}

	if req.ID == "" {
		encoder.Encode(Response{Ok: false, Err: "session ID is required"})
		return
	}

	sess := pty.DefaultManager.Get(req.ID)
	if sess == nil {
		encoder.Encode(Response{Ok: false, Err: "session not found"})
		return
	}

	_, err := sess.Write([]byte(req.Data))
	if err != nil {
		encoder.Encode(Response{Ok: false, Err: err.Error()})
		return
	}

	encoder.Encode(Response{Ok: true})
}

func (s *Server) handleResize(data json.RawMessage, encoder *json.Encoder) {
	var req ResizeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		encoder.Encode(Response{Ok: false, Err: "invalid resize request: " + err.Error()})
		return
	}

	if req.ID == "" {
		encoder.Encode(Response{Ok: false, Err: "session ID is required"})
		return
	}

	if req.Cols <= 0 || req.Rows <= 0 {
		encoder.Encode(Response{Ok: false, Err: "cols and rows must be positive"})
		return
	}

	sess := pty.DefaultManager.Get(req.ID)
	if sess == nil {
		encoder.Encode(Response{Ok: false, Err: "session not found"})
		return
	}

	err := sess.Resize(req.Cols, req.Rows)
	if err != nil {
		encoder.Encode(Response{Ok: false, Err: err.Error()})
		return
	}

	encoder.Encode(Response{Ok: true})
}

func (s *Server) handleKill(data json.RawMessage, encoder *json.Encoder) {
	var req KillRequest
	if err := json.Unmarshal(data, &req); err != nil {
		encoder.Encode(Response{Ok: false, Err: "invalid kill request: " + err.Error()})
		return
	}

	if req.ID == "" {
		encoder.Encode(Response{Ok: false, Err: "session ID is required"})
		return
	}

	sess := pty.DefaultManager.Get(req.ID)
	if sess == nil {
		encoder.Encode(Response{Ok: false, Err: "session not found"})
		return
	}

	pty.CleanupSession(sess)
	encoder.Encode(Response{Ok: true})
}

func (s *Server) handleList(encoder *json.Encoder) {
	sessions := pty.DefaultManager.List()
	infos := make([]SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		status := "active"
		if sess.Cmd != nil && sess.Cmd.Process != nil {
			if err := sess.Cmd.Process.Signal(syscall.Signal(0)); err != nil {
				status = "exiting"
			}
		}
		infos = append(infos, SessionInfo{
			ID:     sess.ID,
			Status: status,
		})
	}

	encoder.Encode(Response{
		Ok: true,
		Data: ListResponse{
			Sessions: infos,
			Count:    len(infos),
		},
	})
}
