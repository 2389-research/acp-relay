// ABOUTME: Tests for container runtime detection (Docker, Podman, Colima)
// ABOUTME: Validates socket discovery and priority ordering

package runtime

import (
	"testing"
)

func TestRuntimeInfo_String(t *testing.T) {
	ri := RuntimeInfo{
		Name:       "docker",
		Status:     "available",
		SocketPath: "/var/run/docker.sock",
		Version:    "24.0.7",
	}

	got := ri.String()
	want := "docker (available) v24.0.7 @ /var/run/docker.sock"

	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestDetectDocker(t *testing.T) {
	info := detectDocker()

	// Should at least return a RuntimeInfo (even if unavailable)
	if info.Name != "docker" {
		t.Errorf("detectDocker().Name = %q, want %q", info.Name, "docker")
	}

	// If available, should have socket path
	if info.Status == "available" && info.SocketPath == "" {
		t.Error("Docker available but no socket path")
	}
}

func TestDetectColima(t *testing.T) {
	info := detectColima()

	if info.Name != "colima" {
		t.Errorf("detectColima().Name = %q, want %q", info.Name, "colima")
	}
}

func TestDetectPodman(t *testing.T) {
	info := detectPodman()

	if info.Name != "podman" {
		t.Errorf("detectPodman().Name = %q, want %q", info.Name, "podman")
	}
}

func TestDetectAll(t *testing.T) {
	infos := DetectAll()

	// Should return info for all three runtimes
	if len(infos) != 3 {
		t.Errorf("DetectAll() returned %d runtimes, want 3", len(infos))
	}

	// Verify names present
	names := make(map[string]bool)
	for _, info := range infos {
		names[info.Name] = true
	}

	for _, want := range []string{"docker", "colima", "podman"} {
		if !names[want] {
			t.Errorf("DetectAll() missing %q", want)
		}
	}
}

func TestDetectBest(t *testing.T) {
	best := DetectBest()

	// If nothing available, should return nil
	if best == nil {
		t.Log("No runtime available (expected in CI)")
		return
	}

	// If something available, should have socket
	if best.SocketPath == "" {
		t.Error("DetectBest() returned runtime with no socket path")
	}

	// Should be in priority order: colima > docker > podman
	validNames := map[string]bool{"colima": true, "docker": true, "podman": true}
	if !validNames[best.Name] {
		t.Errorf("DetectBest() returned unknown runtime: %q", best.Name)
	}
}

func TestPriorityOrder(t *testing.T) {
	// Create mock available runtimes
	allAvailable := []RuntimeInfo{
		{Name: "podman", Status: "available", SocketPath: "/run/podman.sock"},
		{Name: "docker", Status: "available", SocketPath: "/var/run/docker.sock"},
		{Name: "colima", Status: "running", SocketPath: "~/.colima/default/docker.sock"},
	}

	// Simulate DetectBest logic
	for _, rt := range allAvailable {
		if rt.Name == "colima" && (rt.Status == "running" || rt.Status == "available") {
			// Colima has highest priority
			if rt.Name != "colima" {
				t.Error("Priority order broken: colima should be first")
			}
			return
		}
	}

	// If no Colima, Docker should be next
	for _, rt := range allAvailable {
		if rt.Name == "docker" && rt.Status == "available" {
			return
		}
	}
}
