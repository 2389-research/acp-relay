// ABOUTME: DatabaseClient for reading relay server SQLite database
// ABOUTME: Provides session history and message retrieval for TUI
package client

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DatabaseClient struct {
	dbPath string
	db     *sql.DB
}

type DBSession struct {
	ID               string
	WorkingDirectory string
	CreatedAt        time.Time
	ClosedAt         *time.Time
	IsActive         bool
}

// NewDatabaseClient creates a new database client
// If dbPath is empty, uses default path: ~/.local/share/acp-relay/db.sqlite.
func NewDatabaseClient(dbPath string) (*DatabaseClient, error) {
	if dbPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dbPath = filepath.Join(homeDir, ".local", "share", "acp-relay", "db.sqlite")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DatabaseClient{
		dbPath: dbPath,
		db:     db,
	}, nil
}

// GetAllSessions retrieves all sessions ordered by created_at DESC, limited to 20.
func (dc *DatabaseClient) GetAllSessions() ([]DBSession, error) {
	query := `SELECT id, working_directory, created_at, closed_at
	          FROM sessions
	          ORDER BY created_at DESC
	          LIMIT 20`

	rows, err := dc.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []DBSession
	for rows.Next() {
		var s DBSession
		var closedAt sql.NullTime

		err := rows.Scan(&s.ID, &s.WorkingDirectory, &s.CreatedAt, &closedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if closedAt.Valid {
			s.ClosedAt = &closedAt.Time
			s.IsActive = false
		} else {
			s.IsActive = true
		}

		sessions = append(sessions, s)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// GetSessionMessages retrieves all messages for a session, ordered by timestamp ASC.
func (dc *DatabaseClient) GetSessionMessages(sessionID string) ([]*Message, error) {
	query := `SELECT direction, message_type, method, raw_message, timestamp
	          FROM messages
	          WHERE session_id = ?
	          ORDER BY timestamp ASC`

	rows, err := dc.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []*Message
	for rows.Next() {
		var direction, rawMessage string
		var messageType, method sql.NullString
		var timestamp time.Time

		err := rows.Scan(&direction, &messageType, &method, &rawMessage, &timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		// Parse raw JSON message
		var rawMsg map[string]interface{}
		if err := json.Unmarshal([]byte(rawMessage), &rawMsg); err != nil {
			// Skip malformed messages
			continue
		}

		// Convert to Message struct based on direction and content
		msg := dc.convertToMessage(sessionID, direction, rawMsg, timestamp)
		if msg != nil {
			messages = append(messages, msg)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// convertToMessage converts a raw database message to a Message struct
//
//nolint:gocognit,funlen // complex message type parsing requiring extensive JSON parsing and type conversions
func (dc *DatabaseClient) convertToMessage(sessionID, direction string, rawMsg map[string]interface{}, timestamp time.Time) *Message {
	var msg *Message

	switch direction {
	case "client_to_relay":
		// User messages - look for prompt in params
		if params, ok := rawMsg["params"].(map[string]interface{}); ok {
			if prompt, ok := params["prompt"].(string); ok {
				msg = &Message{
					SessionID: sessionID,
					Type:      MessageTypeUser,
					Content:   prompt,
					Timestamp: timestamp,
				}
			}
		}

	case "relay_to_client":
		// Agent responses - look for chunks or updates
		method, _ := rawMsg["method"].(string)

		switch method {
		case "session/chunk":
			// Agent text chunks
			if params, ok := rawMsg["params"].(map[string]interface{}); ok {
				if chunk, ok := params["chunk"].(map[string]interface{}); ok {
					if text, ok := chunk["text"].(string); ok {
						msg = &Message{
							SessionID: sessionID,
							Type:      MessageTypeAgent,
							Content:   text,
							Timestamp: timestamp,
						}
					}
				}
			}

		case "session/update":
			// System updates
			if params, ok := rawMsg["params"].(map[string]interface{}); ok {
				if update, ok := params["update"].(map[string]interface{}); ok {
					// Convert update to readable text
					updateJSON, err := json.Marshal(update)
					if err != nil {
						updateJSON = []byte("{}")
					}
					msg = &Message{
						SessionID: sessionID,
						Type:      MessageTypeSystem,
						Content:   string(updateJSON),
						Timestamp: timestamp,
					}
				}
			}

		case "session/error":
			// Error messages
			if params, ok := rawMsg["params"].(map[string]interface{}); ok {
				if errMsg, ok := params["error"].(string); ok {
					msg = &Message{
						SessionID: sessionID,
						Type:      MessageTypeError,
						Content:   errMsg,
						Timestamp: timestamp,
					}
				}
			}
		}
	}

	return msg
}

// MarkSessionClosed marks a session as closed with current timestamp.
func (dc *DatabaseClient) MarkSessionClosed(sessionID string) error {
	query := `UPDATE sessions SET closed_at = ? WHERE id = ?`

	_, err := dc.db.Exec(query, time.Now(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to mark session closed: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (dc *DatabaseClient) Close() error {
	if dc.db != nil {
		return dc.db.Close()
	}
	return nil
}
