package errors

import (
	"encoding/json"
	"testing"
)

func TestAgentConnectionError(t *testing.T) {
	err := NewAgentConnectionError("ws://localhost:9000", 3, 5000, "connection timeout")

	data, jsonErr := json.Marshal(err.Data)
	if jsonErr != nil {
		t.Fatalf("failed to marshal: %v", jsonErr)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["error_type"] != "agent_connection_timeout" {
		t.Errorf("expected error_type agent_connection_timeout, got %v", parsed["error_type"])
	}

	explanation, ok := parsed["explanation"].(string)
	if !ok || explanation == "" {
		t.Error("expected explanation to be set")
	}

	suggestions, ok := parsed["suggested_actions"].([]interface{})
	if !ok || len(suggestions) == 0 {
		t.Error("expected suggested_actions to be set")
	}

	if parsed["recoverable"] != true {
		t.Error("expected recoverable to be true")
	}
}

func TestSessionNotFoundError(t *testing.T) {
	err := NewSessionNotFoundError("sess_12345")

	data, jsonErr := json.Marshal(err.Data)
	if jsonErr != nil {
		t.Fatalf("failed to marshal: %v", jsonErr)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["error_type"] != "session_not_found" {
		t.Errorf("expected error_type session_not_found, got %v", parsed["error_type"])
	}

	explanation, ok := parsed["explanation"].(string)
	if !ok || explanation == "" {
		t.Error("expected explanation to be set")
	}

	possibleCauses, ok := parsed["possible_causes"].([]interface{})
	if !ok || len(possibleCauses) == 0 {
		t.Error("expected possible_causes to be set")
	}

	suggestions, ok := parsed["suggested_actions"].([]interface{})
	if !ok || len(suggestions) == 0 {
		t.Error("expected suggested_actions to be set")
	}

	relevantState, ok := parsed["relevant_state"].(map[string]interface{})
	if !ok {
		t.Error("expected relevant_state to be set")
	}

	if relevantState["session_id"] != "sess_12345" {
		t.Errorf("expected session_id in relevant_state, got %v", relevantState["session_id"])
	}

	if parsed["recoverable"] != true {
		t.Error("expected recoverable to be true")
	}
}

func TestInvalidParamsError(t *testing.T) {
	err := NewInvalidParamsError("workingDirectory", "string", "null")

	data, jsonErr := json.Marshal(err.Data)
	if jsonErr != nil {
		t.Fatalf("failed to marshal: %v", jsonErr)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["error_type"] != "invalid_params" {
		t.Errorf("expected error_type invalid_params, got %v", parsed["error_type"])
	}

	if parsed["recoverable"] != true {
		t.Error("expected recoverable to be true")
	}
}

func TestParseError(t *testing.T) {
	err := NewParseError("unexpected token at position 15")

	data, jsonErr := json.Marshal(err.Data)
	if jsonErr != nil {
		t.Fatalf("failed to marshal: %v", jsonErr)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["error_type"] != "parse_error" {
		t.Errorf("expected error_type parse_error, got %v", parsed["error_type"])
	}

	if parsed["recoverable"] != true {
		t.Error("expected recoverable to be true")
	}
}
