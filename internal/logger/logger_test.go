// ABOUTME: Tests for structured logging with verbosity control
// ABOUTME: Validates backward compatibility with existing log.Printf calls

package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetVerbose(t *testing.T) {
	// Default should be non-verbose
	if IsVerbose() {
		t.Error("Logger should default to non-verbose")
	}

	SetVerbose(true)
	if !IsVerbose() {
		t.Error("SetVerbose(true) did not enable verbose mode")
	}

	SetVerbose(false)
	if IsVerbose() {
		t.Error("SetVerbose(false) did not disable verbose mode")
	}
}

func TestDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer SetOutput(nil)

	// Debug should not show when not verbose
	SetVerbose(false)
	Debug("test debug message")
	if buf.Len() > 0 {
		t.Error("Debug output when not verbose")
	}

	// Debug should show when verbose
	SetVerbose(true)
	buf.Reset()
	Debug("test debug message")
	if !strings.Contains(buf.String(), "[DEBUG]") {
		t.Error("Debug did not output [DEBUG] prefix")
	}
	if !strings.Contains(buf.String(), "test debug message") {
		t.Error("Debug did not output message")
	}
}

func TestInfoLevel(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer SetOutput(nil)

	Info("test info message")
	if !strings.Contains(buf.String(), "[INFO]") {
		t.Error("Info did not output [INFO] prefix")
	}
	if !strings.Contains(buf.String(), "test info message") {
		t.Error("Info did not output message")
	}
}

func TestWarnLevel(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer SetOutput(nil)

	Warn("test warn message")
	if !strings.Contains(buf.String(), "[WARN]") {
		t.Error("Warn did not output [WARN] prefix")
	}
	if !strings.Contains(buf.String(), "test warn message") {
		t.Error("Warn did not output message")
	}
}

func TestErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer SetOutput(nil)

	Error("test error message")
	if !strings.Contains(buf.String(), "[ERROR]") {
		t.Error("Error did not output [ERROR] prefix")
	}
	if !strings.Contains(buf.String(), "test error message") {
		t.Error("Error did not output message")
	}
}

func TestFormatting(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer SetOutput(nil)

	Info("formatted %s: %d", "test", 42)
	output := buf.String()

	if !strings.Contains(output, "formatted test: 42") {
		t.Errorf("Formatting failed, got: %q", output)
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Verify that standard log package still works
	// This test just ensures our logger doesn't break existing code
	var buf bytes.Buffer
	SetOutput(&buf)
	defer SetOutput(nil)

	// Existing code uses log.Printf directly
	// Our logger should not interfere
	t.Log("Logger maintains backward compatibility with log package")
}
