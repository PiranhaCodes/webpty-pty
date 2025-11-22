package pty

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	ptylib "github.com/creack/pty"
	"github.com/google/uuid"
)

// SpawnShell creates a new PTY session with an auto-detected shell.
// It creates the FIFO pipe and log file, and starts the read loop.
func SpawnShell() (*Session, error) {
	shellPath, err := DetectShell()
	if err != nil {
		return nil, fmt.Errorf("shell detection failed: %w", err)
	}

	id := uuid.New().String()
	cmd := exec.Command(shellPath)
	cmd.Env = os.Environ()

	ptyFile, err := ptylib.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	sessionsDir := "/run/webpty/sessions"
	logDir := "/var/log/webpty"

	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		ptyFile.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		ptyFile.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	fifoPath := filepath.Join(sessionsDir, id+".out")
	if err := os.Remove(fifoPath); err != nil && !os.IsNotExist(err) {
		ptyFile.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to remove existing FIFO: %w", err)
	}

	if err := syscall.Mkfifo(fifoPath, 0666); err != nil {
		ptyFile.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to create FIFO: %w", err)
	}

	fifoWriter, err := os.OpenFile(fifoPath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		os.Remove(fifoPath)
		ptyFile.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to open FIFO for writing: %w", err)
	}

	logPath := filepath.Join(logDir, id+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fifoWriter.Close()
		os.Remove(fifoPath)
		ptyFile.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	sess := &Session{
		ID:         id,
		Cmd:        cmd,
		Pty:        ptyFile,
		logFile:    logFile,
		fifoPath:   fifoPath,
		fifoWriter: fifoWriter,
		done:       make(chan struct{}),
	}

	DefaultManager.Add(id, sess)
	go sess.ReadLoop()

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("[PTY] Session %s: process exited with error: %v", id, err)
		}
	}()

	log.Printf("[PTY] Spawned session %s with shell %s", id, shellPath)
	return sess, nil
}
