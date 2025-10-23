-- ABOUTME: SQLite schema for ACP relay message logging
-- ABOUTME: Captures all messages flowing through sessions for debugging and analysis

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    agent_session_id TEXT,
    working_directory TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    closed_at DATETIME
);

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    direction TEXT NOT NULL CHECK(direction IN ('client_to_relay', 'relay_to_agent', 'agent_to_relay', 'relay_to_client')),
    message_type TEXT, -- 'request', 'response', 'notification'
    method TEXT,       -- e.g., 'session/new', 'session/prompt', 'session/request_permission'
    jsonrpc_id INTEGER, -- The 'id' field from JSON-RPC message
    raw_message TEXT NOT NULL, -- Full JSON message
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_method ON messages(method);
CREATE INDEX IF NOT EXISTS idx_sessions_created ON sessions(created_at);
