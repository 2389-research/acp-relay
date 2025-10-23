// ABOUTME: Stream demuxing for Docker stdout/stderr separation
// ABOUTME: Fixes bug where Docker multiplexes streams with 8-byte headers

package container

import (
	"io"
	"log"

	"github.com/docker/docker/pkg/stdcopy"
)

// demuxStreams separates Docker's multiplexed stdout/stderr stream
// Returns two readers: one for stdout, one for stderr (both are ReadCloser from io.Pipe)
func demuxStreams(multiplexed io.Reader) (stdout, stderr io.ReadCloser) {
	stdoutPipe, stdoutWriter := io.Pipe()
	stderrPipe, stderrWriter := io.Pipe()

	// Background goroutine to demux
	go func() {
		defer stdoutWriter.Close()
		defer stderrWriter.Close()

		// stdcopy.StdCopy handles Docker's 8-byte header protocol
		_, err := stdcopy.StdCopy(stdoutWriter, stderrWriter, multiplexed)
		if err != nil && err != io.EOF {
			// Log error but don't crash - container might be stopping
			log.Printf("stream demux error: %v", err)
		}
	}()

	return stdoutPipe, stderrPipe
}
