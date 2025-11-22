package pty

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// DetectShell finds the first available shell in order of preference:
// 1. $SHELL environment variable
// 2. /bin/bash
// 3. /bin/zsh
// 4. /bin/sh
// Returns an error if none are found.
func DetectShell() (string, error) {
	if shell := os.Getenv("SHELL"); shell != "" {
		if isExecutable(shell) {
			return shell, nil
		}
	}

	candidates := []string{
		"/bin/bash",
		"/bin/zsh",
		"/bin/sh",
	}

	for _, candidate := range candidates {
		if isExecutable(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no shell found: checked $SHELL, /bin/bash, /bin/zsh, /bin/sh")
}

// isExecutable checks if a file exists and is executable.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	mode := info.Mode()
	if !mode.IsRegular() {
		return false
	}

	if mode&0111 != 0 {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return false
		}
		_, err = exec.LookPath(absPath)
		return err == nil
	}

	return false
}
