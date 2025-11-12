// ABOUTME: Debug logging utilities for TUI development
// ABOUTME: Provides file-based logging that doesn't interfere with TUI display
package tui

import (
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	debugEnabled bool
	debugWriter  io.Writer
	debugMu      sync.Mutex
)

// EnableDebug enables debug logging to the specified writer.
func EnableDebug(w io.Writer) {
	debugMu.Lock()
	defer debugMu.Unlock()
	debugEnabled = true
	debugWriter = w
}

// DebugLog writes a debug message with timestamp.
func DebugLog(format string, args ...interface{}) {
	debugMu.Lock()
	defer debugMu.Unlock()

	if !debugEnabled || debugWriter == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(debugWriter, "[%s] %s\n", timestamp, message)
}
