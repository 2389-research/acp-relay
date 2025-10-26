#!/usr/bin/env python3
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "websockets",
#     "rich",
# ]
# ///
"""
Interactive WebSocket Chat with ACP Agent (Beautiful TUI Edition)

This script provides a beautiful terminal-based chat interface to interact
with an ACP agent through the relay server, using Rich for gorgeous formatting.

Usage:
    python3 examples/interactive_chat.py
"""

import asyncio
import websockets
import json
import sys
import os
from datetime import datetime
from pathlib import Path
from rich.console import Console
from rich.panel import Panel
from rich.markdown import Markdown
from rich.live import Live
from rich.layout import Layout
from rich.text import Text
from rich.box import ROUNDED
from rich.align import Align

# Configuration
RELAY_WS_URL = "ws://localhost:23891"
WORKING_DIR = "/tmp"
SESSION_FILE = Path.home() / ".acp-relay-session"

console = Console()

class ChatSession:
    def __init__(self, session_id=None):
        self.session_id = session_id
        self.messages = []
        self.current_response = ""
        self.is_typing = False

    def add_user_message(self, text):
        timestamp = datetime.now().strftime("%H:%M:%S")
        self.messages.append({
            "role": "user",
            "type": "text",
            "text": text,
            "timestamp": timestamp
        })

    def add_agent_message(self, text):
        timestamp = datetime.now().strftime("%H:%M:%S")
        self.messages.append({
            "role": "agent",
            "type": "text",
            "text": text,
            "timestamp": timestamp
        })

    def add_system_message(self, msg_type, data):
        """Add a system/update message with special rendering"""
        timestamp = datetime.now().strftime("%H:%M:%S")
        self.messages.append({
            "role": "system",
            "type": msg_type,
            "data": data,
            "timestamp": timestamp
        })

    def render_messages(self):
        """Render all messages with rich formatting"""
        output = []

        for msg in self.messages:
            if msg["role"] == "user":
                text = Text()
                text.append(f"[{msg['timestamp']}] ", style="dim")
                text.append("You: ", style="bold cyan")
                text.append(msg["text"], style="white")
                output.append(text)
                output.append("")  # Add spacing

            elif msg["role"] == "agent":
                text = Text()
                text.append(f"[{msg['timestamp']}] ", style="dim")
                text.append("Agent: ", style="bold green")
                text.append(msg["text"], style="white")
                output.append(text)
                output.append("")  # Add spacing

            elif msg["role"] == "system":
                # Render system messages based on type
                msg_type = msg["type"]
                data = msg["data"]

                if msg_type == "available_commands_update":
                    # Show available commands as a nice list
                    text = Text()
                    text.append(f"[{msg['timestamp']}] ", style="dim")
                    text.append("📋 ", style="")
                    text.append("Available Commands Updated", style="bold yellow")
                    output.append(text)

                    commands = data.get("availableCommands", [])
                    if commands and len(commands) <= 5:
                        # Show first few commands
                        for cmd in commands[:5]:
                            cmd_text = Text()
                            cmd_text.append("   • ", style="yellow")
                            cmd_text.append(f"/{cmd.get('name', 'unknown')}", style="bold yellow")
                            if cmd.get('description'):
                                desc = cmd['description']
                                if len(desc) > 50:
                                    desc = desc[:50] + "..."
                                cmd_text.append(f" - {desc}", style="dim")
                            output.append(cmd_text)
                        if len(commands) > 5:
                            output.append(Text(f"   ... and {len(commands) - 5} more", style="dim"))
                    else:
                        output.append(Text(f"   {len(commands)} commands available", style="dim"))
                    output.append("")

                elif msg_type == "permission_request":
                    # Show permission request
                    text = Text()
                    text.append(f"[{msg['timestamp']}] ", style="dim")
                    text.append("🔐 ", style="")
                    text.append("Permission Request: ", style="bold yellow")
                    text.append(data.get("tool", "unknown"), style="bold cyan")
                    output.append(text)

                    if data.get("arguments"):
                        output.append(Text(f"   Args: {json.dumps(data['arguments'], indent=2)}", style="dim"))
                    output.append("")

                elif msg_type == "permission_response":
                    # Show permission decision
                    allowed = data.get("allowed", False)
                    tool = data.get("tool", "unknown")
                    icon = "✅" if allowed else "❌"
                    status = "Allowed" if allowed else "Denied"

                    text = Text()
                    text.append(f"[{msg['timestamp']}] ", style="dim")
                    text.append(f"{icon} ", style="")
                    text.append(f"Permission {status}: ", style="bold green" if allowed else "bold red")
                    text.append(tool, style="cyan")
                    output.append(text)
                    output.append("")

                elif msg_type == "tool_use":
                    # Show tool usage
                    text = Text()
                    text.append(f"[{msg['timestamp']}] ", style="dim")
                    text.append("🔧 ", style="")
                    text.append("Tool Used: ", style="bold magenta")
                    text.append(data.get("tool", "unknown"), style="bold cyan")
                    output.append(text)
                    output.append("")

                elif msg_type == "thinking":
                    # Show agent thinking indicator
                    text = Text()
                    text.append(f"[{msg['timestamp']}] ", style="dim")
                    text.append("💭 ", style="")
                    text.append("Agent is thinking...", style="italic dim")
                    output.append(text)
                    output.append("")

                else:
                    # Generic system message
                    text = Text()
                    text.append(f"[{msg['timestamp']}] ", style="dim")
                    text.append("ℹ️  ", style="")
                    text.append(f"System: {msg_type}", style="dim italic")
                    output.append(text)
                    output.append("")

        # If agent is currently typing, show the in-progress response
        if self.is_typing and self.current_response:
            text = Text()
            text.append(f"[{datetime.now().strftime('%H:%M:%S')}] ", style="dim")
            text.append("Agent: ", style="bold green")
            text.append(self.current_response, style="white")
            text.append(" ▊", style="bold green blink")  # Typing cursor
            output.append(text)

        return output


def save_session_id(session_id):
    """Save session ID to disk for resumption"""
    try:
        SESSION_FILE.write_text(session_id)
        console.print(f"[dim]💾 Session ID saved to {SESSION_FILE}[/dim]")
    except Exception as e:
        console.print(f"[dim yellow]⚠️  Failed to save session: {e}[/dim yellow]")


def load_session_id():
    """Load saved session ID from disk"""
    try:
        if SESSION_FILE.exists():
            session_id = SESSION_FILE.read_text().strip()
            console.print(f"[dim]📂 Found saved session: {session_id}[/dim]")
            return session_id
    except Exception as e:
        console.print(f"[dim yellow]⚠️  Failed to load session: {e}[/dim yellow]")
    return None


def clear_session_id():
    """Clear saved session ID"""
    try:
        if SESSION_FILE.exists():
            SESSION_FILE.unlink()
            console.print(f"[dim]🗑️  Cleared saved session[/dim]")
    except Exception:
        pass


async def send_message(websocket, method, params, msg_id):
    """Send a JSON-RPC message via WebSocket"""
    message = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": msg_id
    }
    await websocket.send(json.dumps(message))


async def handle_permission_request(websocket, request_msg, chat_session, live):
    """Handle a permission request from the agent"""
    params = request_msg.get("params", {})
    tool_name = params.get("tool", "unknown")
    arguments = params.get("arguments", {})
    request_id = request_msg.get("id")

    # Add permission request to chat history
    chat_session.add_system_message(
        "permission_request",
        {"tool": tool_name, "arguments": arguments}
    )
    live.update(render_chat_ui(chat_session))

    # Show permission request to user
    live.stop()
    console.print(f"\n[bold yellow]🔐 Permission Request:[/bold yellow]")
    console.print(f"   Tool: [cyan]{tool_name}[/cyan]")
    console.print(f"   Arguments: [dim]{json.dumps(arguments, indent=2)}[/dim]")
    console.print("\n[bold]Allow this action? (y/n):[/bold] ", end="")

    response = await asyncio.get_event_loop().run_in_executor(
        None, sys.stdin.readline
    )
    response = response.strip().lower()
    live.start()

    allowed = response in ['y', 'yes']

    # Send permission response
    await send_message(
        websocket,
        "permission/response",
        {
            "allowed": allowed,
            "requestId": request_id
        },
        request_id
    )

    # Add permission response to chat history
    chat_session.add_system_message(
        "permission_response",
        {"tool": tool_name, "allowed": allowed}
    )
    live.update(render_chat_ui(chat_session))


async def receive_until_complete(websocket, expected_id, chat_session, live):
    """
    Receive messages from the agent until we get the final response.
    Updates the chat session with streaming text.
    """
    chat_session.current_response = ""
    chat_session.is_typing = True

    try:
        while True:
            try:
                raw_msg = await asyncio.wait_for(websocket.recv(), timeout=60.0)
                msg = json.loads(raw_msg)

                # Track if we handled this message
                handled = False

                # Handler: Permission request
                if "method" in msg and msg.get("method") == "permission/request":
                    await handle_permission_request(websocket, msg, chat_session, live)
                    handled = True

                # Handler: Session update notification
                elif "method" in msg and msg.get("method") == "session/update" and "id" not in msg:
                    if "params" in msg and "update" in msg["params"]:
                        update = msg["params"]["update"]
                        session_update_type = update.get("sessionUpdate")

                        # Handler: Agent message chunk (streaming text)
                        if session_update_type == "agent_message_chunk":
                            if "content" in update and "text" in update["content"]:
                                text = update["content"]["text"]
                                chat_session.current_response += text
                                live.update(render_chat_ui(chat_session))
                                handled = True

                        # Handler: Available commands update
                        elif session_update_type == "available_commands_update":
                            # Add to chat history with nice formatting
                            chat_session.add_system_message(
                                "available_commands_update",
                                {"availableCommands": update.get("availableCommands", [])}
                            )
                            live.update(render_chat_ui(chat_session))
                            handled = True

                        # Handler: Tool use notification
                        elif session_update_type == "tool_use":
                            tool_name = update.get("tool", {}).get("name", "unknown")
                            chat_session.add_system_message(
                                "tool_use",
                                {"tool": tool_name}
                            )
                            live.update(render_chat_ui(chat_session))
                            handled = True

                        # Handler: Thinking notification
                        elif session_update_type == "agent_thinking":
                            chat_session.add_system_message("thinking", {})
                            live.update(render_chat_ui(chat_session))
                            handled = True

                        # Handler: Other session updates (log them)
                        else:
                            chat_session.add_system_message(session_update_type, update)
                            live.update(render_chat_ui(chat_session))
                            handled = True

                # Handler: Final response (turn complete)
                elif "id" in msg and msg["id"] == expected_id:
                    chat_session.is_typing = False

                    if "error" in msg:
                        error_text = f"Error: {msg['error']}"
                        chat_session.add_agent_message(error_text)
                        live.update(render_chat_ui(chat_session))
                        return None

                    # Save the final response
                    final_text = chat_session.current_response
                    if final_text:  # Only add if there's text
                        chat_session.add_agent_message(final_text)
                    chat_session.current_response = ""
                    live.update(render_chat_ui(chat_session))
                    handled = True
                    return final_text

                # Handler: Response to other request (not ours)
                elif "id" in msg and msg["id"] != expected_id:
                    live.stop()
                    console.print(f"[dim]ℹ️  Received response for request {msg['id']} (expected {expected_id})[/dim]")
                    live.start()
                    handled = True

                # Log unhandled messages
                if not handled:
                    live.stop()
                    console.print("[bold yellow]⚠️  Unhandled message:[/bold yellow]")
                    console.print(f"[dim]{json.dumps(msg, indent=2)}[/dim]")
                    live.start()

            except asyncio.TimeoutError:
                chat_session.is_typing = False
                live.update(render_chat_ui(chat_session))
                return chat_session.current_response

    except Exception as e:
        chat_session.is_typing = False
        chat_session.add_agent_message(f"Error: {e}")
        live.update(render_chat_ui(chat_session))
        return None


def render_chat_ui(chat_session, show_input_prompt=False, session_id=None):
    """Render the complete chat UI with scrollable messages"""
    layout = Layout()

    # Use session_id from chat_session if not explicitly provided
    if session_id is None:
        session_id = chat_session.session_id

    # Create header
    header_text = Text()
    header_text.append("🤖 ACP Agent Chat", style="bold magenta")
    if session_id:
        header_text.append(f"\n[Session: {session_id}]", style="dim")

    header = Panel(
        Align.center(header_text, vertical="middle"),
        box=ROUNDED,
        style="bold blue"
    )

    # Create messages panel
    messages = chat_session.render_messages()
    messages_text = Text()
    for item in messages:
        if isinstance(item, Text):
            messages_text.append(item)
            messages_text.append("\n")
        else:
            messages_text.append(str(item) + "\n")

    messages_panel = Panel(
        messages_text,
        title="[bold]Conversation[/bold]",
        border_style="green",
        box=ROUNDED,
        height=console.height - 8  # Leave room for header and input
    )

    # Create input prompt panel
    if show_input_prompt:
        input_display = "[bold yellow]⬇️  Type your message at the prompt below ⬇️[/bold yellow]"
        border_style = "yellow"
    else:
        input_display = "[dim]🤖 Agent is typing...[/dim]"
        border_style = "green"

    input_panel = Panel(
        input_display,
        border_style=border_style,
        box=ROUNDED
    )

    # Combine layout
    layout.split_column(
        Layout(header, size=3),
        Layout(messages_panel),
        Layout(input_panel, size=3)
    )

    return layout


async def interactive_chat():
    """Main interactive chat session"""
    chat_session = ChatSession()  # Will set session_id once we have it

    try:
        # Show connection screen
        with console.status("[bold green]Connecting to relay server...") as status:
            websocket = await websockets.connect(RELAY_WS_URL)

            # Try to resume existing session first
            saved_session_id = load_session_id()
            session_id = None

            if saved_session_id:
                status.update(f"[bold green]Resuming session {saved_session_id[:13]}...")
                await send_message(
                    websocket,
                    "session/resume",
                    {"sessionId": saved_session_id},
                    1
                )

                # Wait for resume response
                try:
                    raw_msg = await asyncio.wait_for(websocket.recv(), timeout=5.0)
                    msg = json.loads(raw_msg)

                    if msg.get("id") == 1 and "result" in msg:
                        session_id = msg["result"].get("sessionId")
                        status.update(f"[bold green]✅ Session resumed: {session_id}")
                        console.print(f"[bold green]✨ Resumed existing session![/bold green]")
                    elif "error" in msg:
                        console.print(f"[dim yellow]⚠️  Resume failed: {msg['error'].get('message', 'unknown')}[/dim yellow]")
                        clear_session_id()
                except asyncio.TimeoutError:
                    console.print("[dim yellow]⚠️  Resume timed out[/dim yellow]")
                    clear_session_id()

            # If resume failed or no saved session, create new session
            if not session_id:
                status.update("[bold green]Creating new session...")
                await send_message(
                    websocket,
                    "session/new",
                    {"workingDirectory": WORKING_DIR},
                    2
                )

                # Get session ID
                while True:
                    raw_msg = await websocket.recv()
                    msg = json.loads(raw_msg)
                    if msg.get("id") == 2 and "result" in msg:
                        session_id = msg["result"].get("sessionId")
                        break

                if not session_id:
                    console.print("[bold red]❌ Failed to create session[/bold red]")
                    return

                # Save session ID for future resumption
                save_session_id(session_id)
                status.update(f"[bold green]✅ Session created: {session_id}")

            # Set session ID on chat session object
            chat_session.session_id = session_id
            await asyncio.sleep(1)

        # Start live display
        with Live(render_chat_ui(chat_session), console=console, refresh_per_second=10) as live:
            msg_id = 3  # Start at 3 (1=resume, 2=new session)

            while True:
                try:
                    # Show input prompt in the TUI
                    live.update(render_chat_ui(chat_session, show_input_prompt=True))

                    # Get user input (stop live to get clean input)
                    live.stop()
                    user_input = await asyncio.get_event_loop().run_in_executor(
                        None, input, "\n👤 You: "
                    )
                    user_input = user_input.strip()
                    live.start()

                    if not user_input:
                        continue

                    # Special command: clear session
                    if user_input.lower() in ['/clear', '/new']:
                        live.stop()
                        console.print("[bold yellow]🗑️  Clearing saved session and starting fresh...[/bold yellow]")
                        clear_session_id()
                        console.print("[bold green]Please restart the chat to create a new session.[/bold green]")
                        break

                    # Add user message to chat
                    chat_session.add_user_message(user_input)
                    live.update(render_chat_ui(chat_session))

                    # Send prompt to agent
                    await send_message(
                        websocket,
                        "session/prompt",
                        {
                            "sessionId": session_id,
                            "content": [
                                {
                                    "type": "text",
                                    "text": user_input
                                }
                            ]
                        },
                        msg_id
                    )

                    # Receive response
                    response_text = await receive_until_complete(
                        websocket, msg_id, chat_session, live
                    )

                    if response_text is None:
                        break

                    msg_id += 1

                except KeyboardInterrupt:
                    live.stop()
                    console.print("\n[bold yellow]👋 Goodbye![/bold yellow]")
                    break

        await websocket.close()

    except websockets.exceptions.ConnectionRefused:
        console.print("[bold red]❌ Cannot connect to relay server[/bold red]")
        console.print("   Make sure the relay server is running:")
        console.print("   [dim]$ go run cmd/relay/main.go --config config.yaml[/dim]")
        sys.exit(1)
    except Exception as e:
        console.print(f"[bold red]❌ Error: {e}[/bold red]")
        import traceback
        traceback.print_exc()
        sys.exit(1)


def main():
    """Entry point"""
    try:
        asyncio.run(interactive_chat())
    except KeyboardInterrupt:
        console.print("\n[bold yellow]👋 Goodbye![/bold yellow]")
        sys.exit(0)


if __name__ == "__main__":
    main()
