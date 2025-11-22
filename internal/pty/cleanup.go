package pty

import (
	"log"
	"os"
	"syscall"
)

// CleanupSession performs complete cleanup of a PTY session including closing
// all file descriptors, removing FIFO files, killing subprocesses, and removing
// the session from the manager.
func CleanupSession(sess *Session) {
	if sess == nil {
		return
	}

	log.Printf("[PTY] Cleaning up session %s", sess.ID)

	if sess.Pty != nil {
		sess.Pty.Close()
	}

	if sess.logFile != nil {
		sess.logFile.Close()
	}

	if sess.fifoWriter != nil {
		sess.fifoWriter.Close()
	}

	if sess.fifoPath != "" {
		if err := os.Remove(sess.fifoPath); err != nil && !os.IsNotExist(err) {
			log.Printf("[PTY] Warning: failed to remove FIFO %s: %v", sess.fifoPath, err)
		}
	}

	if sess.Cmd != nil && sess.Cmd.Process != nil {
		if err := sess.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("[PTY] Warning: failed to send SIGTERM to process %d: %v", sess.Cmd.Process.Pid, err)
		}

		done := make(chan error, 1)
		go func() {
			done <- sess.Cmd.Wait()
		}()

		select {
		case <-done:
		default:
			if err := sess.Cmd.Process.Kill(); err != nil {
				log.Printf("[PTY] Warning: failed to kill process %d: %v", sess.Cmd.Process.Pid, err)
			}
			sess.Cmd.Wait()
		}
	}

	DefaultManager.Remove(sess.ID)
	log.Printf("[PTY] Session %s cleaned up", sess.ID)
}

// CleanupAllSessions cleans up all active sessions.
func CleanupAllSessions() {
	sessions := DefaultManager.List()
	for _, sess := range sessions {
		CleanupSession(sess)
	}
}
