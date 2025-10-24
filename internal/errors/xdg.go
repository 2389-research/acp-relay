// ABOUTME: XDG path errors with LLM-optimized messaging
// ABOUTME: Used when XDG directories cannot be created

package errors

import "fmt"

type XDGPathError struct {
	Variable      string
	AttemptedPath string
	UnderlyingErr error
}

func NewXDGPathError(variable, path string, err error) *XDGPathError {
	return &XDGPathError{
		Variable:      variable,
		AttemptedPath: path,
		UnderlyingErr: err,
	}
}

func (e *XDGPathError) Error() string {
	return fmt.Sprintf("cannot create %s directory at %s: %v", e.Variable, e.AttemptedPath, e.UnderlyingErr)
}

func (e *XDGPathError) ToJSONRPCError() JSONRPCError {
	return JSONRPCError{
		Code:    -32000,
		Message: e.Error(),
		Data: map[string]interface{}{
			"error_type":  "xdg_path_error",
			"explanation": "Could not create required XDG directories for acp-relay.",
			"possible_causes": []string{
				"Insufficient permissions in parent directory",
				"Disk is full",
				"Path already exists as a file (not directory)",
			},
			"suggested_actions": []string{
				fmt.Sprintf("Check permissions: ls -ld %s", e.AttemptedPath),
				"Check disk space: df -h",
				fmt.Sprintf("Manually create directory: mkdir -p %s", e.AttemptedPath),
			},
			"relevant_state": map[string]interface{}{
				"variable":       e.Variable,
				"attempted_path": e.AttemptedPath,
				"error":          e.UnderlyingErr.Error(),
			},
			"recoverable": true,
		},
	}
}
