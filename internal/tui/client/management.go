// ABOUTME: HTTP client for relay management API
// ABOUTME: Provides session listing and status queries from relay server
package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ManagementSession represents a session from the management API.
type ManagementSession struct {
	ID               string
	AgentSessionID   string
	WorkingDirectory string
	CreatedAt        time.Time
	ClosedAt         *time.Time
	IsActive         bool
}

// managementSessionResponse matches the JSON structure from /api/sessions.
type managementSessionResponse struct {
	ID               string  `json:"id"`
	AgentSessionID   string  `json:"agentSessionId"`
	WorkingDirectory string  `json:"workingDirectory"`
	CreatedAt        string  `json:"createdAt"`
	ClosedAt         *string `json:"closedAt"`
	IsActive         bool    `json:"isActive"`
}

// GetSessionsFromManagementAPI queries the relay management API for all sessions.
func GetSessionsFromManagementAPI(baseURL string) ([]ManagementSession, error) {
	url := baseURL + "/api/sessions"

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sessions: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var rawSessions []managementSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&rawSessions); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to ManagementSession structs
	sessions := make([]ManagementSession, 0, len(rawSessions))
	for _, raw := range rawSessions {
		createdAt, err := time.Parse("2006-01-02 15:04:05", raw.CreatedAt)
		if err != nil {
			// Skip sessions with invalid timestamps
			continue
		}

		var closedAt *time.Time
		if raw.ClosedAt != nil {
			t, err := time.Parse("2006-01-02 15:04:05", *raw.ClosedAt)
			if err == nil {
				closedAt = &t
			}
		}

		sessions = append(sessions, ManagementSession{
			ID:               raw.ID,
			AgentSessionID:   raw.AgentSessionID,
			WorkingDirectory: raw.WorkingDirectory,
			CreatedAt:        createdAt,
			ClosedAt:         closedAt,
			IsActive:         raw.IsActive,
		})
	}

	return sessions, nil
}
