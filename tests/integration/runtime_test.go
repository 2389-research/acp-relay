// ABOUTME: Integration tests for runtime detection
// ABOUTME: Validates Docker/Colima/Podman detection in real environment

//nolint:goconst // test file uses repeated status strings for readability
package integration

import (
	"testing"

	"github.com/harper/acp-relay/internal/runtime"
)

func TestRuntimeDetection_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	all := runtime.DetectAll()
	if len(all) != 3 {
		t.Errorf("DetectAll() returned %d runtimes, want 3", len(all))
	}

	// At least one runtime should be available in CI
	availableCount := 0
	for _, rt := range all {
		if rt.Status == "available" || rt.Status == "running" {
			availableCount++

			// If available, should have socket
			if rt.SocketPath == "" {
				t.Errorf("%s is %s but has no socket path", rt.Name, rt.Status)
			}
		}
	}

	if availableCount == 0 {
		t.Log("No runtimes available (expected in minimal CI)")
	}
}

func TestRuntimeDetection_Priority(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	best := runtime.DetectBest()
	if best == nil {
		t.Skip("No runtime available")
	}

	t.Logf("Best runtime: %s (%s) @ %s", best.Name, best.Status, best.SocketPath)

	// Verify priority order
	all := runtime.DetectAll()
	colimaAvailable := false
	dockerAvailable := false

	for _, rt := range all {
		if rt.Name == "colima" && (rt.Status == "running" || rt.Status == "available") {
			colimaAvailable = true
		}
		if rt.Name == "docker" && rt.Status == "available" {
			dockerAvailable = true
		}
	}

	// If Colima available, it should be chosen
	if colimaAvailable && best.Name != "colima" {
		t.Errorf("Colima available but best=%s (priority violation)", best.Name)
	}

	// If only Docker available, it should be chosen
	if !colimaAvailable && dockerAvailable && best.Name != "docker" {
		t.Errorf("Only Docker available but best=%s (priority violation)", best.Name)
	}
}
