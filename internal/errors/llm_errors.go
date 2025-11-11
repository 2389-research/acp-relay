// ABOUTME: LLM-optimized error messages with explanations and suggested actions
// ABOUTME: Provides verbose, actionable error context for AI agents

package errors

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/harper/acp-relay/internal/jsonrpc"
)

// JSONRPCError is a structured error type for JSON-RPC responses.
type JSONRPCError struct {
	Code    int
	Message string
	Data    map[string]interface{}
}

type LLMErrorData struct {
	ErrorType        string                 `json:"error_type"`
	Explanation      string                 `json:"explanation"`
	PossibleCauses   []string               `json:"possible_causes,omitempty"`
	SuggestedActions []string               `json:"suggested_actions,omitempty"`
	RelevantState    map[string]interface{} `json:"relevant_state,omitempty"`
	Recoverable      bool                   `json:"recoverable"`
	Details          string                 `json:"details,omitempty"`
}

func NewAgentConnectionError(agentURL string, attempts int, durationMs int, details string) *jsonrpc.Error {
	message := fmt.Sprintf(
		"I attempted to spawn the ACP agent process but it failed to start within %dms. "+
			"This typically means the agent command is incorrect, the agent binary is missing, "+
			"or the agent encountered an error during startup.",
		durationMs,
	)

	data := LLMErrorData{
		ErrorType:   "agent_connection_timeout",
		Explanation: "The relay server tried to start the agent subprocess but it did not become ready within the configured timeout.",
		PossibleCauses: []string{
			"The agent command path is incorrect or the binary doesn't exist",
			"The agent requires environment variables that aren't set",
			"The agent crashed immediately on startup",
			"The agent is waiting for input but the relay hasn't sent initialization",
		},
		SuggestedActions: []string{
			"Check that the agent command exists: ls -l /path/to/agent",
			"Verify the agent can run manually: /path/to/agent --help",
			"Check the relay's stderr logs for agent error messages",
			"Ensure required environment variables are set in config.yaml",
		},
		RelevantState: map[string]interface{}{
			"agent_url":  agentURL,
			"attempts":   attempts,
			"timeout_ms": durationMs,
		},
		Recoverable: true,
		Details:     details,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal error data: %v", err)
		dataBytes = []byte("{}")
	}

	return &jsonrpc.Error{
		Code:    jsonrpc.ServerError,
		Message: message,
		Data:    dataBytes,
	}
}

func NewSessionNotFoundError(sessionID string) *jsonrpc.Error {
	message := fmt.Sprintf(
		"The session '%s' does not exist. This means the session was never created, "+
			"it expired due to inactivity, or the agent process crashed and cleaned up.",
		sessionID,
	)

	data := LLMErrorData{
		ErrorType:   "session_not_found",
		Explanation: "The relay server doesn't have an active session with this ID.",
		PossibleCauses: []string{
			"The session was never created (missing session/new call)",
			"The session ID was mistyped or corrupted",
			"The session expired due to inactivity timeout",
			"The agent process crashed and the session was cleaned up",
		},
		SuggestedActions: []string{
			"Create a new session using session/new",
			"Verify you're using the correct session ID from the session/new response",
			"Check if the agent process is still running",
		},
		RelevantState: map[string]interface{}{
			"session_id": sessionID,
		},
		Recoverable: true,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal error data: %v", err)
		dataBytes = []byte("{}")
	}

	return &jsonrpc.Error{
		Code:    jsonrpc.ServerError,
		Message: message,
		Data:    dataBytes,
	}
}

func NewInvalidParamsError(paramName string, expectedType string, receivedValue string) *jsonrpc.Error {
	message := fmt.Sprintf(
		"The parameter '%s' is invalid. I expected a %s but received: %s. "+
			"Please check the API documentation and ensure all required parameters are included with correct types.",
		paramName, expectedType, receivedValue,
	)

	data := LLMErrorData{
		ErrorType:   "invalid_params",
		Explanation: "The request contained parameters that don't match the expected schema for this method.",
		PossibleCauses: []string{
			"The parameter value is missing or null when it's required",
			"The parameter has the wrong type (e.g., string instead of number)",
			"The parameter name is misspelled",
			"The JSON structure doesn't match the expected format",
		},
		SuggestedActions: []string{
			"Review the API documentation for the correct parameter schema",
			"Check that all required parameters are present",
			"Verify parameter types match what's expected",
			"Ensure parameter names are spelled correctly",
		},
		RelevantState: map[string]interface{}{
			"param_name":     paramName,
			"expected_type":  expectedType,
			"received_value": receivedValue,
		},
		Recoverable: true,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal error data: %v", err)
		dataBytes = []byte("{}")
	}

	return &jsonrpc.Error{
		Code:    jsonrpc.InvalidParams,
		Message: message,
		Data:    dataBytes,
	}
}

func NewParseError(details string) *jsonrpc.Error {
	message := fmt.Sprintf(
		"I couldn't parse the request as valid JSON. The JSON is malformed or contains syntax errors. "+
			"Details: %s",
		details,
	)

	data := LLMErrorData{
		ErrorType:   "parse_error",
		Explanation: "The request body is not valid JSON according to the JSON specification.",
		PossibleCauses: []string{
			"Missing quotes around strings",
			"Trailing commas in objects or arrays",
			"Unescaped special characters",
			"Incomplete JSON structure (missing closing braces or brackets)",
			"Invalid Unicode escape sequences",
		},
		SuggestedActions: []string{
			"Validate your JSON using a JSON linter (e.g., jsonlint.com)",
			"Check for common syntax errors: missing quotes, trailing commas, unmatched braces",
			"Ensure all strings are properly quoted",
			"Verify that special characters are properly escaped",
		},
		Recoverable: true,
		Details:     details,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal error data: %v", err)
		dataBytes = []byte("{}")
	}

	return &jsonrpc.Error{
		Code:    jsonrpc.ParseError,
		Message: message,
		Data:    dataBytes,
	}
}

func NewInvalidRequestError(details string) *jsonrpc.Error {
	message := fmt.Sprintf(
		"The request is not a valid JSON-RPC 2.0 request. "+
			"All requests must include 'jsonrpc': '2.0', 'method', and optionally 'params' and 'id'. "+
			"Details: %s",
		details,
	)

	data := LLMErrorData{
		ErrorType:   "invalid_request",
		Explanation: "The request doesn't conform to the JSON-RPC 2.0 specification structure.",
		PossibleCauses: []string{
			"Missing required 'jsonrpc' field",
			"Missing required 'method' field",
			"The 'jsonrpc' field is not '2.0'",
			"The request is not a JSON object",
			"Reserved field names are used incorrectly",
		},
		SuggestedActions: []string{
			"Ensure the request includes: {\"jsonrpc\": \"2.0\", \"method\": \"...\"}",
			"Add an 'id' field for requests that expect responses",
			"Review the JSON-RPC 2.0 specification",
			"Check that 'params' is an object or array if present",
		},
		Recoverable: true,
		Details:     details,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal error data: %v", err)
		dataBytes = []byte("{}")
	}

	return &jsonrpc.Error{
		Code:    jsonrpc.InvalidRequest,
		Message: message,
		Data:    dataBytes,
	}
}

func NewMethodNotFoundError(methodName string) *jsonrpc.Error {
	message := fmt.Sprintf(
		"The method '%s' is not supported by this relay server. "+
			"Available methods include: session/new, session/prompt. "+
			"Methods are forwarded to the agent after session creation.",
		methodName,
	)

	data := LLMErrorData{
		ErrorType:   "method_not_found",
		Explanation: "The requested method name doesn't match any handler in the relay server.",
		PossibleCauses: []string{
			"The method name is misspelled",
			"The method is not implemented on the relay server",
			"You meant to call a method on the agent but haven't created a session yet",
			"The API version you're targeting has different method names",
		},
		SuggestedActions: []string{
			"Check the method name spelling: it should be session/new or session/prompt",
			"Create a session first using session/new if you want to call agent methods",
			"Review the API documentation for available methods",
		},
		RelevantState: map[string]interface{}{
			"method_name": methodName,
		},
		Recoverable: true,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal error data: %v", err)
		dataBytes = []byte("{}")
	}

	return &jsonrpc.Error{
		Code:    jsonrpc.MethodNotFound,
		Message: message,
		Data:    dataBytes,
	}
}

func NewInternalError(details string) *jsonrpc.Error {
	message := fmt.Sprintf(
		"An internal server error occurred while processing your request. "+
			"This is likely a bug in the relay server. Details: %s",
		details,
	)

	data := LLMErrorData{
		ErrorType:   "internal_error",
		Explanation: "The relay server encountered an unexpected error during request processing.",
		PossibleCauses: []string{
			"A bug in the relay server code",
			"Resource exhaustion (out of memory, file descriptors)",
			"Filesystem permissions issues",
			"Unexpected agent behavior",
		},
		SuggestedActions: []string{
			"Check the relay server logs for stack traces or error details",
			"Ensure the relay has sufficient system resources",
			"Verify filesystem permissions for the working directory",
			"Report this error to the relay server maintainers",
			"Try the request again - it may be a transient issue",
		},
		Recoverable: false,
		Details:     details,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal error data: %v", err)
		dataBytes = []byte("{}")
	}

	return &jsonrpc.Error{
		Code:    jsonrpc.InternalError,
		Message: message,
		Data:    dataBytes,
	}
}
