package jsonrpc

import (
	"encoding/json"
	"testing"
)

func TestParseRequest(t *testing.T) {
	data := []byte(`{
		"jsonrpc": "2.0",
		"method": "session/new",
		"params": {"workingDirectory": "/tmp/test"},
		"id": 1
	}`)

	var req Request
	err := json.Unmarshal(data, &req)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", req.JSONRPC)
	}

	if req.Method != "session/new" {
		t.Errorf("expected method session/new, got %s", req.Method)
	}

	if req.ID == nil {
		t.Error("expected id to be set")
	}
}

func TestParseResponse(t *testing.T) {
	data := []byte(`{
		"jsonrpc": "2.0",
		"result": {"sessionId": "sess_123"},
		"id": 1
	}`)

	var resp Response
	err := json.Unmarshal(data, &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Result == nil {
		t.Error("expected result to be set")
	}
}

func TestParseError(t *testing.T) {
	data := []byte(`{
		"jsonrpc": "2.0",
		"error": {
			"code": -32600,
			"message": "Invalid request",
			"data": {"detail": "test"}
		},
		"id": 1
	}`)

	var resp Response
	err := json.Unmarshal(data, &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error to be set")
	}

	if resp.Error.Code != -32600 {
		t.Errorf("expected code -32600, got %d", resp.Error.Code)
	}
}
