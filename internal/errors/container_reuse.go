// ABOUTME: Container reuse errors with LLM-optimized messaging
// ABOUTME: Used when existing container cannot be reused

package errors

import "fmt"

type ContainerReuseError struct {
	ContainerID string
	SessionID   string
	Reason      string
}

func NewContainerReuseError(containerID, sessionID, reason string) *ContainerReuseError {
	return &ContainerReuseError{
		ContainerID: containerID,
		SessionID:   sessionID,
		Reason:      reason,
	}
}

func (e *ContainerReuseError) Error() string {
	return fmt.Sprintf("cannot reuse container %s for session %s: %s", e.ContainerID, e.SessionID, e.Reason)
}

func (e *ContainerReuseError) ToJSONRPCError() JSONRPCError {
	return JSONRPCError{
		Code:    -32000,
		Message: e.Error(),
		Data: map[string]interface{}{
			"error_type":  "container_reuse_failed",
			"explanation": "Found an existing container for this session but could not reuse it.",
			"possible_causes": []string{
				"Container is in a corrupted state",
				"Container has insufficient permissions",
				"Container's workspace was deleted",
			},
			"suggested_actions": []string{
				fmt.Sprintf("Remove stale container: docker rm -f %s", e.ContainerID),
				"Check Docker permissions: docker ps",
				"Try creating a new session with a different session ID",
			},
			"relevant_state": map[string]interface{}{
				"container_id": e.ContainerID,
				"session_id":   e.SessionID,
				"reason":       e.Reason,
			},
			"recoverable": true,
		},
	}
}
