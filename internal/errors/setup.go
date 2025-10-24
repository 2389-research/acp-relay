// ABOUTME: Setup required errors with LLM-optimized messaging
// ABOUTME: Used when first-time setup is needed

package errors

type SetupRequiredError struct {
	MissingConfig  bool
	InvalidRuntime bool
	NoRuntimeFound bool
}

func NewSetupRequiredError(missingConfig, invalidRuntime, noRuntimeFound bool) *SetupRequiredError {
	return &SetupRequiredError{
		MissingConfig:  missingConfig,
		InvalidRuntime: invalidRuntime,
		NoRuntimeFound: noRuntimeFound,
	}
}

func (e *SetupRequiredError) Error() string {
	if e.MissingConfig {
		return "config file not found, run 'acp-relay setup' for first-time configuration"
	}
	if e.InvalidRuntime {
		return "configured runtime is invalid, run 'acp-relay setup' to reconfigure"
	}
	if e.NoRuntimeFound {
		return "no container runtime found, run 'acp-relay setup' to detect and configure"
	}
	return "setup required"
}

func (e *SetupRequiredError) ToJSONRPCError() JSONRPCError {
	var causes []string
	var actions []string

	if e.MissingConfig {
		causes = append(causes, "Config file does not exist")
		actions = append(actions, "Run: acp-relay setup")
	}
	if e.InvalidRuntime {
		causes = append(causes, "Configured runtime is not available")
		actions = append(actions, "Run: acp-relay setup")
		actions = append(actions, "Check runtime installation: docker version")
	}
	if e.NoRuntimeFound {
		causes = append(causes, "No container runtime (Docker/Podman/Colima) found")
		actions = append(actions, "Install Docker: https://docs.docker.com/get-docker/")
		actions = append(actions, "Or install Colima: brew install colima")
	}

	return JSONRPCError{
		Code:    -32000,
		Message: e.Error(),
		Data: map[string]interface{}{
			"error_type":        "setup_required",
			"explanation":       "acp-relay requires initial setup before it can run.",
			"possible_causes":   causes,
			"suggested_actions": actions,
			"relevant_state": map[string]interface{}{
				"missing_config":   e.MissingConfig,
				"invalid_runtime":  e.InvalidRuntime,
				"no_runtime_found": e.NoRuntimeFound,
			},
			"recoverable": true,
		},
	}
}
