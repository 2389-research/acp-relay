// ABOUTME: Unit tests for Docker stream demuxing functionality
// ABOUTME: Tests separation of multiplexed stdout/stderr streams

package container

import (
	"bytes"
	"io"
	"testing"

	"github.com/docker/docker/pkg/stdcopy"
)

// readStreamsConcurrently reads both stdout and stderr concurrently to avoid deadlock
func readStreamsConcurrently(stdout, stderr io.Reader) (stdoutData, stderrData []byte, err error) {
	type result struct {
		data []byte
		err  error
	}

	stdoutChan := make(chan result, 1)
	stderrChan := make(chan result, 1)

	go func() {
		data, err := io.ReadAll(stdout)
		stdoutChan <- result{data, err}
	}()

	go func() {
		data, err := io.ReadAll(stderr)
		stderrChan <- result{data, err}
	}()

	stdoutResult := <-stdoutChan
	stderrResult := <-stderrChan

	if stdoutResult.err != nil {
		return nil, nil, stdoutResult.err
	}
	if stderrResult.err != nil {
		return nil, nil, stderrResult.err
	}

	return stdoutResult.data, stderrResult.data, nil
}

func TestDemuxStreams_Stdout(t *testing.T) {
	// Create multiplexed stream with only stdout
	var buf bytes.Buffer
	stdoutData := []byte("stdout content\n")

	_, err := stdcopy.NewStdWriter(&buf, stdcopy.Stdout).Write(stdoutData)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	// Demux the stream
	stdout, stderr := demuxStreams(&buf)

	// Read both streams concurrently to avoid deadlock
	stdoutContent, stderrContent, err := readStreamsConcurrently(stdout, stderr)
	if err != nil {
		t.Fatalf("Failed to read streams: %v", err)
	}

	if !bytes.Equal(stdoutContent, stdoutData) {
		t.Errorf("stdout = %q, want %q", stdoutContent, stdoutData)
	}

	if len(stderrContent) != 0 {
		t.Errorf("stderr should be empty, got: %q", stderrContent)
	}
}

func TestDemuxStreams_Stderr(t *testing.T) {
	// Create multiplexed stream with only stderr
	var buf bytes.Buffer
	stderrData := []byte("stderr content\n")

	_, err := stdcopy.NewStdWriter(&buf, stdcopy.Stderr).Write(stderrData)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	// Demux the stream
	stdout, stderr := demuxStreams(&buf)

	// Read both streams concurrently to avoid deadlock
	stdoutContent, stderrContent, err := readStreamsConcurrently(stdout, stderr)
	if err != nil {
		t.Fatalf("Failed to read streams: %v", err)
	}

	if len(stdoutContent) != 0 {
		t.Errorf("stdout should be empty, got: %q", stdoutContent)
	}

	if !bytes.Equal(stderrContent, stderrData) {
		t.Errorf("stderr = %q, want %q", stderrContent, stderrData)
	}
}

func TestDemuxStreams_Both(t *testing.T) {
	// Create multiplexed stream with both stdout and stderr
	var buf bytes.Buffer
	stdoutData := []byte("stdout line 1\n")
	stderrData := []byte("stderr line 1\n")

	// Write stdout
	_, err := stdcopy.NewStdWriter(&buf, stdcopy.Stdout).Write(stdoutData)
	if err != nil {
		t.Fatalf("Failed to write stdout: %v", err)
	}

	// Write stderr
	_, err = stdcopy.NewStdWriter(&buf, stdcopy.Stderr).Write(stderrData)
	if err != nil {
		t.Fatalf("Failed to write stderr: %v", err)
	}

	// Demux the stream
	stdout, stderr := demuxStreams(&buf)

	// Read both streams concurrently to avoid deadlock
	stdoutContent, stderrContent, err := readStreamsConcurrently(stdout, stderr)
	if err != nil {
		t.Fatalf("Failed to read streams: %v", err)
	}

	if !bytes.Equal(stdoutContent, stdoutData) {
		t.Errorf("stdout = %q, want %q", stdoutContent, stdoutData)
	}

	if !bytes.Equal(stderrContent, stderrData) {
		t.Errorf("stderr = %q, want %q", stderrContent, stderrData)
	}
}

func TestDemuxStreams_InterleavedStreams(t *testing.T) {
	// Create multiplexed stream with interleaved stdout/stderr
	var buf bytes.Buffer

	stdoutLine1 := []byte("stdout line 1\n")
	stderrLine1 := []byte("error line 1\n")
	stdoutLine2 := []byte("stdout line 2\n")
	stderrLine2 := []byte("error line 2\n")

	// Write in interleaved order
	_, err := stdcopy.NewStdWriter(&buf, stdcopy.Stdout).Write(stdoutLine1)
	if err != nil {
		t.Fatalf("Failed to write stdout line 1: %v", err)
	}

	_, err = stdcopy.NewStdWriter(&buf, stdcopy.Stderr).Write(stderrLine1)
	if err != nil {
		t.Fatalf("Failed to write stderr line 1: %v", err)
	}

	_, err = stdcopy.NewStdWriter(&buf, stdcopy.Stdout).Write(stdoutLine2)
	if err != nil {
		t.Fatalf("Failed to write stdout line 2: %v", err)
	}

	_, err = stdcopy.NewStdWriter(&buf, stdcopy.Stderr).Write(stderrLine2)
	if err != nil {
		t.Fatalf("Failed to write stderr line 2: %v", err)
	}

	// Demux the stream
	stdout, stderr := demuxStreams(&buf)

	// Read both streams concurrently to avoid deadlock
	stdoutContent, stderrContent, err := readStreamsConcurrently(stdout, stderr)
	if err != nil {
		t.Fatalf("Failed to read streams: %v", err)
	}

	expectedStdout := append(stdoutLine1, stdoutLine2...)
	if !bytes.Equal(stdoutContent, expectedStdout) {
		t.Errorf("stdout = %q, want %q", stdoutContent, expectedStdout)
	}

	expectedStderr := append(stderrLine1, stderrLine2...)
	if !bytes.Equal(stderrContent, expectedStderr) {
		t.Errorf("stderr = %q, want %q", stderrContent, expectedStderr)
	}
}

func TestDemuxStreams_EmptyInput(t *testing.T) {
	// Create empty multiplexed stream
	var buf bytes.Buffer

	// Demux the stream
	stdout, stderr := demuxStreams(&buf)

	// Read both streams concurrently to avoid deadlock
	stdoutContent, stderrContent, err := readStreamsConcurrently(stdout, stderr)
	if err != nil {
		t.Fatalf("Failed to read streams: %v", err)
	}

	if len(stdoutContent) != 0 {
		t.Errorf("stdout should be empty, got: %q", stdoutContent)
	}

	if len(stderrContent) != 0 {
		t.Errorf("stderr should be empty, got: %q", stderrContent)
	}
}
