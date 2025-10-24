// ABOUTME: Container runtime detection for Docker, Podman, and Colima
// ABOUTME: Provides socket discovery and availability checking

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RuntimeInfo contains detected runtime information
type RuntimeInfo struct {
	Name       string // "docker", "podman", "colima"
	Status     string // "available", "cli-only", "unavailable", "running", "stopped"
	SocketPath string // e.g., "/var/run/docker.sock"
	Version    string // e.g., "24.0.7"
}

func (r RuntimeInfo) String() string {
	return fmt.Sprintf("%s (%s) v%s @ %s", r.Name, r.Status, r.Version, r.SocketPath)
}

// DetectAll finds all available container runtimes
func DetectAll() []RuntimeInfo {
	return []RuntimeInfo{
		detectDocker(),
		detectColima(),
		detectPodman(),
	}
}

// DetectBest returns the best available runtime (priority: Colima > Docker > Podman)
func DetectBest() *RuntimeInfo {
	all := DetectAll()

	// Priority 1: Colima (if running)
	for _, rt := range all {
		if rt.Name == "colima" && (rt.Status == "running" || rt.Status == "available") {
			return &rt
		}
	}

	// Priority 2: Docker
	for _, rt := range all {
		if rt.Name == "docker" && rt.Status == "available" {
			return &rt
		}
	}

	// Priority 3: Podman
	for _, rt := range all {
		if rt.Name == "podman" && rt.Status == "available" {
			return &rt
		}
	}

	return nil
}

// getHome returns HOME with fallback to current directory
func getHome() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func detectDocker() RuntimeInfo {
	info := RuntimeInfo{Name: "docker"}

	// Check CLI presence
	version, err := exec.Command("docker", "version", "--format", "{{.Client.Version}}").Output()
	if err != nil {
		info.Status = "unavailable"
		return info
	}
	info.Version = strings.TrimSpace(string(version))

	// Check socket
	socketPath := "/var/run/docker.sock"
	if _, err := os.Stat(socketPath); err == nil {
		info.Status = "available"
		info.SocketPath = socketPath
	} else {
		info.Status = "cli-only"
	}

	return info
}

func detectColima() RuntimeInfo {
	info := RuntimeInfo{Name: "colima"}

	// Check CLI presence
	version, err := exec.Command("colima", "version").Output()
	if err != nil {
		info.Status = "unavailable"
		return info
	}

	// Parse version from output like "colima version 0.6.6"
	parts := strings.Fields(string(version))
	if len(parts) >= 3 {
		info.Version = parts[2]
	}

	// Check if running
	statusOut, err := exec.Command("colima", "status").Output()
	if err != nil {
		info.Status = "stopped"
		return info
	}

	if strings.Contains(string(statusOut), "colima is running") {
		info.Status = "running"

		// Find socket
		home := getHome()
		socketPath := filepath.Join(home, ".colima", "default", "docker.sock")
		if _, err := os.Stat(socketPath); err == nil {
			info.SocketPath = socketPath
		}
	} else {
		info.Status = "stopped"
	}

	return info
}

func detectPodman() RuntimeInfo {
	info := RuntimeInfo{Name: "podman"}

	// Check CLI presence
	version, err := exec.Command("podman", "version", "--format", "{{.Client.Version}}").Output()
	if err != nil {
		info.Status = "unavailable"
		return info
	}
	info.Version = strings.TrimSpace(string(version))

	// Check socket
	socketPath := "/var/run/podman/podman.sock"
	if _, err := os.Stat(socketPath); err == nil {
		info.Status = "available"
		info.SocketPath = socketPath
	} else {
		info.Status = "cli-only"
	}

	return info
}
