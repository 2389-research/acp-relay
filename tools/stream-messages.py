#!/usr/bin/env python3
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""
Stream messages from the ACP relay SQLite database in real-time

Usage:
    python3 tools/stream-messages.py [session_id]

If no session_id is provided, streams all messages from all sessions.
Press Ctrl+C to stop.
"""

import sqlite3
import sys
import json
import time
from datetime import datetime
from pathlib import Path

DB_PATH = str(Path.home() / ".local" / "share" / "acp-relay" / "db.sqlite")
POLL_INTERVAL = 0.5  # seconds

def format_message(msg_id, session_id, direction, msg_type, method, jsonrpc_id, raw_msg, timestamp):
    """Format a message for display"""
    # Direction emoji
    dir_emoji = {
        "client_to_relay": "📥",
        "relay_to_agent": "➡️ ",
        "agent_to_relay": "⬅️ ",
        "relay_to_client": "📤"
    }.get(direction, "❓")

    # Session display
    session_display = session_id[:8] if session_id else "unknown"

    # Parse JSON for better display
    try:
        msg_json = json.loads(raw_msg)

        # Short summary based on message type
        if msg_type == "request" and method:
            summary = f"{method}"
        elif msg_type == "response":
            if "error" in msg_json:
                summary = f"ERROR: {msg_json.get('error', {}).get('message', 'unknown')}"
            else:
                summary = "response"
        elif msg_type == "notification" and method:
            summary = f"notification: {method}"
        else:
            summary = msg_type or "message"

        # For certain methods, add more detail
        if method == "session/prompt":
            params = msg_json.get("params", {})
            content = params.get("content", [])
            if content and len(content) > 0:
                text = content[0].get("text", "")
                if len(text) > 50:
                    text = text[:50] + "..."
                summary = f"prompt: {text}"
        elif method == "session/update":
            params = msg_json.get("params", {})
            update = params.get("update", {})
            update_type = update.get("sessionUpdate")
            if update_type == "agent_message_chunk":
                text = update.get("content", {}).get("text", "")
                if len(text) > 50:
                    text = text[:50] + "..."
                summary = f"agent chunk: {text}"
            elif update_type:
                summary = f"update: {update_type}"

    except:
        summary = "message"
        msg_json = None

    # Build the output line
    parts = [
        f"{dir_emoji}",
        f"[{timestamp}]",
        f"[{session_display}]",
        f"{direction}:",
        summary
    ]

    if jsonrpc_id is not None:
        parts.append(f"(id={jsonrpc_id})")

    return " ".join(parts)

def stream_messages(session_id=None):
    """Stream messages in real-time"""
    conn = sqlite3.connect(DB_PATH)
    last_msg_id = 0

    # Get the current max ID to start from
    cursor = conn.execute("SELECT COALESCE(MAX(id), 0) FROM messages")
    last_msg_id = cursor.fetchone()[0]

    if session_id:
        print(f"🔴 Streaming messages for session: {session_id}")
        print(f"   Starting from message #{last_msg_id + 1}")
    else:
        print(f"🔴 Streaming all messages")
        print(f"   Starting from message #{last_msg_id + 1}")

    print(f"   Poll interval: {POLL_INTERVAL}s")
    print(f"   Press Ctrl+C to stop\n")
    print("=" * 100)

    try:
        while True:
            # Query for new messages
            if session_id:
                cursor = conn.execute("""
                    SELECT id, session_id, direction, message_type, method, jsonrpc_id, raw_message, timestamp
                    FROM messages
                    WHERE id > ? AND session_id = ?
                    ORDER BY id ASC
                """, (last_msg_id, session_id))
            else:
                cursor = conn.execute("""
                    SELECT id, session_id, direction, message_type, method, jsonrpc_id, raw_message, timestamp
                    FROM messages
                    WHERE id > ?
                    ORDER BY id ASC
                """, (last_msg_id,))

            messages = cursor.fetchall()

            for msg in messages:
                msg_id = msg[0]
                output = format_message(*msg)
                print(output)
                last_msg_id = msg_id

            time.sleep(POLL_INTERVAL)

    except KeyboardInterrupt:
        print("\n\n👋 Stopped streaming")
    finally:
        conn.close()

def main():
    if len(sys.argv) > 1:
        session_id = sys.argv[1]
    else:
        session_id = None

    try:
        stream_messages(session_id)
    except sqlite3.Error as e:
        print(f"❌ Database error: {e}")
        sys.exit(1)
    except FileNotFoundError:
        print(f"❌ Database not found: {DB_PATH}")
        print(f"   Make sure the relay server is running and has created the database.")
        sys.exit(1)

if __name__ == "__main__":
    main()
