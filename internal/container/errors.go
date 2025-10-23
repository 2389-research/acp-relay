// ABOUTME: Container-specific error types with helpful messages
// ABOUTME: Provides actionable error messages for Docker issues

package container

import "fmt"

type ContainerError struct {
	Type    string
	Message string
	Cause   error
}

func (e *ContainerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func NewDockerUnavailableError(cause error) *ContainerError {
	return &ContainerError{
		Type:    "docker_unavailable",
		Message: "Cannot connect to Docker daemon. Is Docker running? Check: docker ps",
		Cause:   cause,
	}
}

func NewImageNotFoundError(image string, cause error) *ContainerError {
	return &ContainerError{
		Type:    "image_not_found",
		Message: fmt.Sprintf("Docker image '%s' not found. Build it with:\n  docker build -t %s .", image, image),
		Cause:   cause,
	}
}

func NewAttachFailedError(cause error) *ContainerError {
	return &ContainerError{
		Type:    "attach_failed",
		Message: "Failed to attach to container stdio",
		Cause:   cause,
	}
}
