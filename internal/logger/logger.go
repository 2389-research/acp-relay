// ABOUTME: Structured logging with verbosity control and level-based output
// ABOUTME: Backward compatible with existing log.Printf usage

package logger

import (
	"fmt"
	"io"
	"log"
	"os"
)

var (
	verbose = false
	output  io.Writer = os.Stderr
)

// SetVerbose enables or disables verbose (DEBUG) logging
func SetVerbose(v bool) {
	verbose = v
}

// IsVerbose returns current verbose setting
func IsVerbose() bool {
	return verbose
}

// SetOutput sets the output destination for logs
func SetOutput(w io.Writer) {
	if w == nil {
		output = os.Stderr
		log.SetOutput(os.Stderr)
	} else {
		output = w
		log.SetOutput(w)
	}
}

// Debug logs at DEBUG level (only shown when verbose)
func Debug(format string, args ...interface{}) {
	if verbose {
		msg := fmt.Sprintf(format, args...)
		log.Printf("[DEBUG] %s", msg)
	}
}

// Info logs at INFO level (always shown)
func Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[INFO] %s", msg)
}

// Warn logs at WARN level (always shown)
func Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[WARN] %s", msg)
}

// Error logs at ERROR level (always shown)
func Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[ERROR] %s", msg)
}
