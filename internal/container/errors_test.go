// ABOUTME: Unit tests for container error types and constructors
// ABOUTME: Tests error formatting with and without causes

package container

import (
	"errors"
	"strings"
	"testing"
)

func TestContainerError_Error_WithoutCause(t *testing.T) {
	err := &ContainerError{
		Type:    "test_type",
		Message: "test message",
		Cause:   nil,
	}

	got := err.Error()
	want := "test message"

	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestContainerError_Error_WithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := &ContainerError{
		Type:    "test_type",
		Message: "test message",
		Cause:   cause,
	}

	got := err.Error()
	want := "test message: underlying error"

	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestNewDockerUnavailableError(t *testing.T) {
	cause := errors.New("connection refused")
	err := NewDockerUnavailableError(cause)

	if err.Type != "docker_unavailable" {
		t.Errorf("Type = %q, want %q", err.Type, "docker_unavailable")
	}

	if !strings.Contains(err.Message, "Cannot connect to Docker daemon") {
		t.Errorf("Message should mention Docker daemon, got: %q", err.Message)
	}

	if !strings.Contains(err.Message, "docker ps") {
		t.Errorf("Message should suggest 'docker ps', got: %q", err.Message)
	}

	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}

	// Test Error() output includes both message and cause
	errorStr := err.Error()
	if !strings.Contains(errorStr, "Cannot connect to Docker daemon") {
		t.Errorf("Error() should include message, got: %q", errorStr)
	}
	if !strings.Contains(errorStr, "connection refused") {
		t.Errorf("Error() should include cause, got: %q", errorStr)
	}
}

func TestNewImageNotFoundError(t *testing.T) {
	imageName := "my-custom-image:latest"
	cause := errors.New("no such image")
	err := NewImageNotFoundError(imageName, cause)

	if err.Type != "image_not_found" {
		t.Errorf("Type = %q, want %q", err.Type, "image_not_found")
	}

	if !strings.Contains(err.Message, imageName) {
		t.Errorf("Message should mention image name %q, got: %q", imageName, err.Message)
	}

	if !strings.Contains(err.Message, "docker build") {
		t.Errorf("Message should suggest 'docker build', got: %q", err.Message)
	}

	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}

	// Test Error() output includes image name and cause
	errorStr := err.Error()
	if !strings.Contains(errorStr, imageName) {
		t.Errorf("Error() should include image name, got: %q", errorStr)
	}
	if !strings.Contains(errorStr, "no such image") {
		t.Errorf("Error() should include cause, got: %q", errorStr)
	}
}

func TestNewAttachFailedError(t *testing.T) {
	cause := errors.New("stream closed")
	err := NewAttachFailedError(cause)

	if err.Type != "attach_failed" {
		t.Errorf("Type = %q, want %q", err.Type, "attach_failed")
	}

	if !strings.Contains(err.Message, "Failed to attach") {
		t.Errorf("Message should mention attach failure, got: %q", err.Message)
	}

	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}

	// Test Error() output includes both message and cause
	errorStr := err.Error()
	if !strings.Contains(errorStr, "Failed to attach") {
		t.Errorf("Error() should include message, got: %q", errorStr)
	}
	if !strings.Contains(errorStr, "stream closed") {
		t.Errorf("Error() should include cause, got: %q", errorStr)
	}
}
