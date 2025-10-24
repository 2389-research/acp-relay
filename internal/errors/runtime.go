// ABOUTME: Runtime detection errors with LLM-optimized messaging
// ABOUTME: Used when container runtime is not found or misconfigured

package errors

import "fmt"

type RuntimeNotFoundError struct {
	RequestedRuntime  string
	AvailableRuntimes []string
}

func NewRuntimeNotFoundError(requested string, available []string) *RuntimeNotFoundError {
	return &RuntimeNotFoundError{
		RequestedRuntime:  requested,
		AvailableRuntimes: available,
	}
}

func (e *RuntimeNotFoundError) Error() string {
	if len(e.AvailableRuntimes) > 0 {
		return fmt.Sprintf("requested runtime %q not found, available: %v", e.RequestedRuntime, e.AvailableRuntimes)
	}
	return fmt.Sprintf("requested runtime %q not found, no runtimes available", e.RequestedRuntime)
}

func (e *RuntimeNotFoundError) ToJSONRPCError() JSONRPCError {
	causes := []string{
		"The requested runtime is not installed on this system",
		"The runtime daemon is not running",
		"The runtime socket path is incorrect in config",
	}

	actions := []string{
		fmt.Sprintf("Install %s: https://docs.docker.com/get-docker/", e.RequestedRuntime),
		"Run 'acp-relay setup' to detect available runtimes",
	}

	if len(e.AvailableRuntimes) > 0 {
		actions = append(actions, fmt.Sprintf("Use one of the available runtimes: %v", e.AvailableRuntimes))
	}

	return JSONRPCError{
		Code:    -32000,
		Message: e.Error(),
		Data: map[string]interface{}{
			"error_type":       "runtime_not_found",
			"explanation":      "The container runtime specified in your config is not available.",
			"possible_causes":  causes,
			"suggested_actions": actions,
			"relevant_state": map[string]interface{}{
				"requested":  e.RequestedRuntime,
				"available":  e.AvailableRuntimes,
			},
			"recoverable": true,
		},
	}
}
