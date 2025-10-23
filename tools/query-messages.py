#!/usr/bin/env python3
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""
Query and display messages from the ACP relay SQLite database

Usage:
    python3 tools/query-messages.py [session_id]

If no session_id is provided, shows all sessions and their message counts.
"""

import sqlite3
import sys
import json
from datetime import datetime

DB_PATH = "./relay-messages.db"

def list_sessions(conn):
    """List all sessions with message counts"""
    cursor = conn.execute("""
        SELECT
            s.id,
            s.working_directory,
            s.created_at,
            s.closed_at,
            COUNT(m.id) as message_count
        FROM sessions s
        LEFT JOIN messages m ON s.id = m.session_id
        GROUP BY s.id
        ORDER BY s.created_at DESC
    """)

    print("\n📊 Sessions:")
    print("=" * 100)
    for row in cursor:
        session_id, working_dir, created_at, closed_at, msg_count = row
        status = "🟢 Active" if closed_at is None else "🔴 Closed"
        print(f"{status} {session_id}")
        print(f"   Working Dir: {working_dir}")
        print(f"   Created: {created_at}")
        if closed_at:
            print(f"   Closed: {closed_at}")
        print(f"   Messages: {msg_count}")
        print()

def show_session_messages(conn, session_id):
    """Show all messages for a session"""
    # Get session info
    cursor = conn.execute("""
        SELECT working_directory, created_at, closed_at
        FROM sessions WHERE id = ?
    """, (session_id,))

    row = cursor.fetchone()
    if not row:
        print(f"❌ Session {session_id} not found")
        return

    working_dir, created_at, closed_at = row
    print(f"\n📦 Session: {session_id}")
    print(f"   Working Dir: {working_dir}")
    print(f"   Created: {created_at}")
    if closed_at:
        print(f"   Closed: {closed_at}")
    print()

    # Get messages
    cursor = conn.execute("""
        SELECT id, direction, message_type, method, jsonrpc_id, raw_message, timestamp
        FROM messages
        WHERE session_id = ?
        ORDER BY timestamp ASC
    """, (session_id,))

    messages = cursor.fetchall()

    if not messages:
        print("   No messages found")
        return

    print(f"📨 Messages ({len(messages)} total):")
    print("=" * 100)

    for msg_id, direction, msg_type, method, jsonrpc_id, raw_msg, timestamp in messages:
        # Direction emoji
        dir_emoji = {
            "client_to_relay": "📥",
            "relay_to_agent": "➡️ ",
            "agent_to_relay": "⬅️ ",
            "relay_to_client": "📤"
        }.get(direction, "❓")

        # Parse JSON for better display
        try:
            msg_json = json.loads(raw_msg)
            msg_display = json.dumps(msg_json, indent=2)

            # Truncate long messages
            if len(msg_display) > 500:
                msg_display = msg_display[:500] + "\n  ... (truncated)"
        except:
            msg_display = raw_msg[:500]

        print(f"\n{dir_emoji} {direction} [{timestamp}]")
        if msg_type:
            print(f"   Type: {msg_type}")
        if method:
            print(f"   Method: {method}")
        if jsonrpc_id is not None:
            print(f"   ID: {jsonrpc_id}")
        print(f"   Message:")
        for line in msg_display.split('\n'):
            print(f"     {line}")

def main():
    if len(sys.argv) > 1:
        session_id = sys.argv[1]
    else:
        session_id = None

    try:
        conn = sqlite3.connect(DB_PATH)

        if session_id:
            show_session_messages(conn, session_id)
        else:
            list_sessions(conn)
            print("\n💡 Tip: Run `python3 tools/query-messages.py <session_id>` to see messages for a specific session")

        conn.close()
    except sqlite3.Error as e:
        print(f"❌ Database error: {e}")
        sys.exit(1)
    except FileNotFoundError:
        print(f"❌ Database not found: {DB_PATH}")
        sys.exit(1)

if __name__ == "__main__":
    main()
