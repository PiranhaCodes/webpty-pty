// Package pty provides PTY session management, including spawning, reading, and cleanup.
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

	sessionsDirRaw := "~/.webpty/sessions"
	logDirRaw := "~/.webpty/log"

	sessionsDir, err := expandPath(sessionsDirRaw)
	if err != nil {
		ptyFile.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to expand sessions directory: %w", err)
	}

	logDir, err := expandPath(logDirRaw)
	if err != nil {
		ptyFile.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to expand log directory: %w", err)
	}

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

	// On macOS, FIFOs can't be opened for writing until a reader opens them.
	// We'll open it in a goroutine that retries, or defer opening until needed.
	// For now, we'll try to open it but don't fail if it's not immediately available.
	fifoWriter, err := os.OpenFile(fifoPath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		// On macOS, "device not configured" is expected if no reader is waiting.
		// We'll set fifoWriter to nil and handle it in the write path.
		// The FIFO will be opened when the relay service starts reading.
		log.Printf("[PTY] Session %s: FIFO not immediately available for writing (will retry on first write): %v", id, err)
		fifoWriter = nil
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
