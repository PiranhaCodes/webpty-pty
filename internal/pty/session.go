package pty

import (
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	ptylib "github.com/creack/pty"
)

// Session represents an active PTY session with its associated resources.
type Session struct {
	ID        string
	Cmd       *exec.Cmd
	Pty       *os.File
	logFile   *os.File
	fifoPath  string
	fifoWriter *os.File
	mu        sync.Mutex
	done      chan struct{}
}

// Write sends data to the PTY stdin.
func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Pty == nil {
		return 0, io.ErrClosedPipe
	}
	return s.Pty.Write(data)
}

// ReadLoop continuously reads from PTY and writes output to both FIFO and log file.
// It runs until the PTY is closed and then triggers cleanup.
func (s *Session) ReadLoop() {
	defer func() {
		close(s.done)
		CleanupSession(s)
	}()

	buf := make([]byte, 4096)
	for {
		n, err := s.Pty.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Printf("[PTY] Session %s: PTY closed (EOF)", s.ID)
				return
			}
			log.Printf("[PTY] Session %s: PTY read error: %v", s.ID, err)
			return
		}

		if n == 0 {
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		s.mu.Lock()
		if s.fifoWriter != nil {
			go func(d []byte) {
				if _, err := s.fifoWriter.Write(d); err != nil {
					log.Printf("[PTY] Session %s: FIFO write error (non-fatal): %v", s.ID, err)
				}
			}(data)
		}
		s.mu.Unlock()

		if s.logFile != nil {
			if _, err := s.logFile.Write(data); err != nil {
				log.Printf("[PTY] Session %s: Log write error: %v", s.ID, err)
			}
			s.logFile.Sync()
		}
	}
}

// Wait blocks until the session completes.
func (s *Session) Wait() {
	<-s.done
}

// Resize resizes the PTY terminal to the specified dimensions.
func (s *Session) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Pty == nil {
		return io.ErrClosedPipe
	}
	return ptylib.Setsize(s.Pty, &ptylib.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}
