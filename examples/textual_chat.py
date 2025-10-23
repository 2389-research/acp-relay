#!/usr/bin/env python3
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "websockets",
#     "textual",
# ]
# ///
"""
Interactive WebSocket Chat with ACP Agent (Textual Edition)

A beautiful terminal UI for chatting with an ACP agent through the relay server.

Usage:
    python3 examples/textual_chat.py
"""

import asyncio
import websockets
import json
import sqlite3
from datetime import datetime
from textual.app import App, ComposeResult
from textual.containers import Container, Vertical, ScrollableContainer
from textual.widgets import Header, Footer, Input, Static, RichLog, Button, ListView, ListItem, Label, ProgressBar
from textual.binding import Binding
from textual.message import Message
from textual.screen import ModalScreen

# Configuration
RELAY_WS_URL = "ws://localhost:8081"
WORKING_DIR = "/tmp"
DB_PATH = "./relay-messages.db"


class ChatMessage(Static):
    """A single chat message widget"""

    def __init__(self, role: str, text: str, timestamp: str, msg_type: str = "text", data: dict = None):
        self.role = role
        self.text = text
        self.timestamp = timestamp
        self.msg_type = msg_type
        self.data = data or {}
        super().__init__()

    def compose(self) -> ComposeResult:
        if self.role == "user":
            yield Static(f"[dim]{self.timestamp}[/dim] [bold cyan]You:[/bold cyan] {self.text}")

        elif self.role == "agent":
            yield Static(f"[dim]{self.timestamp}[/dim] [bold green]Agent:[/bold green] {self.text}")

        elif self.role == "system":
            # Handle different system message types
            if self.msg_type == "available_commands_update":
                # Show available commands nicely
                commands = self.data.get("availableCommands", [])
                commands_text = f"[dim]{self.timestamp}[/dim] 📋 [bold yellow]Available Commands Updated[/bold yellow]\n"

                if commands and len(commands) <= 5:
                    for cmd in commands[:5]:
                        name = cmd.get("name", "unknown")
                        desc = cmd.get("description", "")
                        if len(desc) > 50:
                            desc = desc[:50] + "..."
                        commands_text += f"   [yellow]•[/yellow] [bold yellow]/{name}[/bold yellow]"
                        if desc:
                            commands_text += f" [dim]- {desc}[/dim]"
                        commands_text += "\n"
                    if len(commands) > 5:
                        commands_text += f"   [dim]... and {len(commands) - 5} more[/dim]\n"
                else:
                    commands_text += f"   [dim]{len(commands)} commands available[/dim]\n"

                yield Static(commands_text)

            elif self.msg_type == "permission_request":
                # Show permission request
                tool = self.data.get("tool", "unknown")
                args = self.data.get("arguments", {})
                perm_text = f"[dim]{self.timestamp}[/dim] 🔐 [bold yellow]Permission Request:[/bold yellow] [bold cyan]{tool}[/bold cyan]\n"
                if args:
                    perm_text += f"   [dim]Args: {json.dumps(args, indent=2)}[/dim]\n"
                yield Static(perm_text)

            elif self.msg_type == "permission_response":
                # Show permission decision
                tool = self.data.get("tool", "unknown")
                allowed = self.data.get("allowed", False)
                icon = "✅" if allowed else "❌"
                status_style = "bold green" if allowed else "bold red"
                status = "Allowed" if allowed else "Denied"
                yield Static(f"[dim]{self.timestamp}[/dim] {icon} [{status_style}]Permission {status}:[/{status_style}] [cyan]{tool}[/cyan]")

            elif self.msg_type == "tool_use":
                # Show tool usage
                tool = self.data.get("tool", "unknown")
                yield Static(f"[dim]{self.timestamp}[/dim] 🔧 [bold magenta]Tool Used:[/bold magenta] [bold cyan]{tool}[/bold cyan]")

            elif self.msg_type == "thinking":
                # Show thinking indicator
                yield Static(f"[dim]{self.timestamp}[/dim] 💭 [dim italic]Agent is thinking...[/dim italic]")

            else:
                # Generic system message
                yield Static(f"[dim]{self.timestamp}[/dim] ℹ️ [bold yellow]System:[/bold yellow] {self.text}")

        elif self.role == "unhandled":
            yield Static(f"[dim]{self.timestamp}[/dim] [bold red]Unhandled Message:[/bold red]\n{self.text}")


class AgentTyping(Static):
    """Widget showing the agent is typing"""

    def __init__(self, text: str = ""):
        self.typing_text = text
        super().__init__()

    def update_text(self, text: str):
        self.typing_text = text
        self.update(f"[dim]{datetime.now().strftime('%H:%M:%S')}[/dim] [bold green]Agent:[/bold green] {text} [blink]▊[/blink]")

    def compose(self) -> ComposeResult:
        yield Static(f"[dim]{datetime.now().strftime('%H:%M:%S')}[/dim] [bold green]Agent:[/bold green] {self.typing_text} [blink]▊[/blink]")


class SessionSelectionScreen(ModalScreen):
    """Modal screen for selecting or creating a session"""

    CSS = """
    SessionSelectionScreen {
        align: center middle;
    }

    #session-dialog {
        padding: 1 2;
        width: 80;
        height: auto;
        border: thick $background 80%;
        background: $surface;
    }

    #session-title {
        width: 100%;
        content-align: center middle;
        text-style: bold;
    }

    #session-list {
        height: 15;
        width: 100%;
        margin: 1 0;
    }

    #button-container {
        width: 100%;
        height: auto;
        align: center middle;
    }

    Button {
        margin: 0 1;
    }
    """

    def __init__(self, sessions: list):
        self.sessions = sessions
        super().__init__()

    def compose(self) -> ComposeResult:
        with Container(id="session-dialog"):
            yield Static("🔄 Resume or Create Session", id="session-title")
            yield Static("\n[dim]Active Sessions:[/dim]")

            if self.sessions:
                session_list = "\n".join([
                    f"[bold cyan]{i+1}.[/bold cyan] {s['id'][:12]}... [dim]({s['working_directory']}) - {s['created_at']}[/dim]"
                    for i, s in enumerate(self.sessions[:10])
                ])
                yield Static(session_list, id="session-list")
            else:
                yield Static("\n[dim italic]No active sessions found[/dim italic]", id="session-list")

            with Vertical(id="button-container"):
                yield Button("Create New Session", id="new-session", variant="primary")
                if self.sessions:
                    yield Input(placeholder="Enter session number to resume (1-10)...", id="session-input")
                    yield Button("Resume Selected", id="resume-session", variant="success")

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "new-session":
            self.dismiss({"action": "new"})
        elif event.button.id == "resume-session":
            # Get the session number from input
            input_widget = self.query_one("#session-input", Input)
            try:
                session_num = int(input_widget.value.strip())
                if 1 <= session_num <= len(self.sessions):
                    selected_session = self.sessions[session_num - 1]
                    self.dismiss({"action": "resume", "session": selected_session})
                else:
                    input_widget.value = ""
                    input_widget.placeholder = f"Please enter 1-{len(self.sessions)}"
            except ValueError:
                input_widget.value = ""
                input_widget.placeholder = "Please enter a valid number"

    def on_input_submitted(self, event: Input.Submitted) -> None:
        # Allow Enter key to resume session
        try:
            session_num = int(event.value.strip())
            if 1 <= session_num <= len(self.sessions):
                selected_session = self.sessions[session_num - 1]
                self.dismiss({"action": "resume", "session": selected_session})
        except ValueError:
            pass


class ACPChatApp(App):
    """A Textual app for chatting with an ACP agent."""

    CSS = """
    Screen {
        background: $background;
    }

    #chat-container {
        height: 1fr;
        border: solid $primary;
        background: $surface;
    }

    #messages {
        height: 1fr;
        padding: 1;
    }

    #input-container {
        height: auto;
        background: $panel;
        border-top: solid $primary;
        padding: 1;
    }

    Input {
        width: 100%;
    }

    ChatMessage {
        width: 100%;
        height: auto;
        padding: 0 1;
    }

    AgentTyping {
        width: 100%;
        height: auto;
        padding: 0 1;
    }

    #status {
        background: $accent;
        color: $text;
        padding: 0 1;
        height: 1;
    }

    #agent-progress {
        height: 1;
        display: none;
    }

    #agent-progress.visible {
        display: block;
    }
    """

    BINDINGS = [
        Binding("ctrl+c", "quit", "Quit", priority=True),
    ]

    def __init__(self):
        super().__init__()
        self.websocket = None
        self.session_id = None
        self.msg_id = 2
        self.agent_typing_widget = None
        self.current_response = ""

    def compose(self) -> ComposeResult:
        """Create child widgets for the app."""
        yield Header()
        with Container(id="chat-container"):
            yield ScrollableContainer(id="messages")
        with Vertical(id="input-container"):
            yield Static("", id="status")
            yield ProgressBar(id="agent-progress", show_eta=False, show_percentage=False)
            yield Input(placeholder="Type your message here...", id="chat-input")
        yield Footer()

    async def on_mount(self) -> None:
        """Initialize the app with session selection"""
        self.title = "🤖 ACP Agent Chat"
        self.sub_title = "Starting..."

        # Get active sessions from database
        sessions = get_active_sessions()

        # Show session selection modal using call_later to ensure we're in worker context
        async def show_session_selector():
            result = await self.push_screen_wait(SessionSelectionScreen(sessions))
            if result["action"] == "new":
                # Create new session
                await self.create_new_session()
            elif result["action"] == "resume":
                # Resume existing session
                await self.resume_session(result["session"])

        self.call_later(show_session_selector)

    async def create_new_session(self):
        """Create a new session"""
        self.sub_title = "Creating session..."
        input_widget = self.query_one("#chat-input", Input)
        input_widget.focus()

        try:
            # Connect to relay server
            self.websocket = await websockets.connect(RELAY_WS_URL)
            self.update_status("Creating session...")

            # Create session
            await self.send_message("session/new", {"workingDirectory": WORKING_DIR}, 1)

            # Get session ID
            while True:
                raw_msg = await self.websocket.recv()
                msg = json.loads(raw_msg)
                if msg.get("id") == 1 and "result" in msg:
                    self.session_id = msg["result"].get("sessionId")
                    break

            if not self.session_id:
                self.update_status("❌ Failed to create session")
                return

            self.sub_title = f"Session: {self.session_id[:8]}"
            self.update_status("✅ Ready to chat! (Ctrl+C to exit)")

            # Start message receiver
            asyncio.create_task(self.receive_messages())

        except Exception as e:
            self.update_status(f"❌ Error: {e}")
            self.notify(f"Failed to connect: {e}", severity="error")

    async def resume_session(self, session: dict):
        """Resume an existing session"""
        self.session_id = session["id"]
        self.sub_title = f"Resuming {self.session_id[:8]}..."

        input_widget = self.query_one("#chat-input", Input)
        input_widget.focus()

        try:
            # Connect to relay server
            self.websocket = await websockets.connect(RELAY_WS_URL)
            self.update_status(f"Resuming session {self.session_id[:8]}...")

            # Load previous messages from database
            await self.load_session_history()

            self.sub_title = f"Session: {self.session_id[:8]}"
            self.update_status("✅ Session resumed! (Ctrl+C to exit)")

            # Start message receiver
            asyncio.create_task(self.receive_messages())

        except Exception as e:
            self.update_status(f"❌ Error: {e}")
            self.notify(f"Failed to resume session: {e}", severity="error")

    async def load_session_history(self):
        """Load previous messages from the database"""
        try:
            conn = sqlite3.connect(DB_PATH)
            cursor = conn.execute("""
                SELECT direction, message_type, method, raw_message, timestamp
                FROM messages
                WHERE session_id = ?
                ORDER BY timestamp ASC
            """, (self.session_id,))

            for row in cursor:
                direction, msg_type, method, raw_msg, timestamp = row

                try:
                    msg = json.loads(raw_msg)

                    # Replay messages in the UI
                    if direction == "client_to_relay" and method == "session/prompt":
                        # User message
                        params = msg.get("params", {})
                        content = params.get("content", [])
                        if content and len(content) > 0:
                            text = content[0].get("text", "")
                            if text:
                                timestamp_str = datetime.fromisoformat(timestamp).strftime("%H:%M:%S")
                                messages = self.query_one("#messages", ScrollableContainer)
                                messages.mount(ChatMessage("user", text, timestamp_str))

                    elif direction == "relay_to_client" and "method" in msg:
                        # Session updates - show system messages
                        if msg.get("method") == "session/update":
                            params = msg.get("params", {})
                            update = params.get("update", {})
                            session_update_type = update.get("sessionUpdate")

                            timestamp_str = datetime.fromisoformat(timestamp).strftime("%H:%M:%S")

                            if session_update_type == "agent_message_chunk":
                                # Collect agent message chunks
                                # For history, we'll just show the final text
                                pass
                            elif session_update_type == "available_commands_update":
                                commands = update.get("availableCommands", [])
                                self.add_system_message(
                                    "",
                                    msg_type="available_commands_update",
                                    data={"availableCommands": commands}
                                )

                except json.JSONDecodeError:
                    pass

            conn.close()

            # Scroll to bottom
            messages = self.query_one("#messages", ScrollableContainer)
            messages.scroll_end(animate=False)

        except (sqlite3.Error, FileNotFoundError) as e:
            self.notify(f"Could not load history: {e}", severity="warning")

    async def send_message(self, method: str, params: dict, msg_id: int):
        """Send a JSON-RPC message"""
        message = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
            "id": msg_id
        }
        await self.websocket.send(json.dumps(message))

    def update_status(self, text: str):
        """Update the status bar"""
        status = self.query_one("#status", Static)
        status.update(text)

    def show_progress(self):
        """Show progress bar when agent is working"""
        progress = self.query_one("#agent-progress", ProgressBar)
        progress.add_class("visible")
        progress.update(total=100, progress=0)

    def hide_progress(self):
        """Hide progress bar when agent is done"""
        progress = self.query_one("#agent-progress", ProgressBar)
        progress.remove_class("visible")

    def advance_progress(self, amount: float = 5.0):
        """Advance the progress bar by a small amount"""
        progress = self.query_one("#agent-progress", ProgressBar)
        if progress.has_class("visible"):
            # Advance progress, wrapping around if it exceeds 100
            progress.advance(amount)

    def add_user_message(self, text: str):
        """Add a user message to the chat"""
        timestamp = datetime.now().strftime("%H:%M:%S")
        messages = self.query_one("#messages", ScrollableContainer)
        messages.mount(ChatMessage("user", text, timestamp))
        messages.scroll_end(animate=False)

    def add_agent_message(self, text: str):
        """Add an agent message to the chat"""
        timestamp = datetime.now().strftime("%H:%M:%S")
        messages = self.query_one("#messages", ScrollableContainer)
        messages.mount(ChatMessage("agent", text, timestamp))
        messages.scroll_end(animate=False)

    def add_system_message(self, text: str, msg_type: str = "text", data: dict = None):
        """Add a system message to the chat"""
        timestamp = datetime.now().strftime("%H:%M:%S")
        messages = self.query_one("#messages", ScrollableContainer)
        messages.mount(ChatMessage("system", text, timestamp, msg_type, data))
        messages.scroll_end(animate=False)

    def add_unhandled_message(self, msg: dict):
        """Add an unhandled message to the chat for debugging"""
        timestamp = datetime.now().strftime("%H:%M:%S")

        # Format the message nicely
        msg_type = msg.get("method", f"response:{msg.get('id', 'unknown')}")
        formatted = f"Type: {msg_type}\n{json.dumps(msg, indent=2)}"

        messages = self.query_one("#messages", ScrollableContainer)
        messages.mount(ChatMessage("unhandled", formatted, timestamp))
        messages.scroll_end(animate=False)

    def start_agent_typing(self):
        """Show agent typing indicator"""
        messages = self.query_one("#messages", ScrollableContainer)
        self.agent_typing_widget = AgentTyping("")
        messages.mount(self.agent_typing_widget)
        messages.scroll_end(animate=False)

    def update_agent_typing(self, text: str):
        """Update the agent's streaming response"""
        if self.agent_typing_widget:
            self.agent_typing_widget.update_text(text)
            messages = self.query_one("#messages", ScrollableContainer)
            messages.scroll_end(animate=False)

    def stop_agent_typing(self):
        """Remove typing indicator and finalize message"""
        if self.agent_typing_widget:
            self.agent_typing_widget.remove()
            self.agent_typing_widget = None
            # Add the final message
            if self.current_response:
                self.add_agent_message(self.current_response)
                self.current_response = ""

    async def handle_permission_request(self, msg):
        """Handle permission request from agent"""
        params = msg.get("params", {})
        request_id = msg.get("id")

        # Extract tool details from the correct structure
        tool_call = params.get("toolCall", {})
        tool_id = tool_call.get("toolCallId", "unknown")
        raw_input = tool_call.get("rawInput", {})

        # Parse the tool name and arguments
        arguments = {}
        if "file_path" in raw_input:
            arguments["file_path"] = raw_input["file_path"]
        if "content" in raw_input:
            content = raw_input["content"]
            if len(content) > 100:
                arguments["content"] = content[:100] + "..."
            else:
                arguments["content"] = content

        # Show permission request in chat with nice formatting
        self.add_system_message(
            "",
            msg_type="permission_request",
            data={"tool": tool_id, "arguments": arguments}
        )

        # For now, auto-approve all permissions
        # Response format per ACP TypeScript SDK example:
        # { outcome: { outcome: "selected", optionId: "allow" } }
        response = {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {
                "outcome": {
                    "outcome": "selected",
                    "optionId": "allow"  # Could also be "allow_always" or "reject"
                }
            }
        }
        await self.websocket.send(json.dumps(response))

        # Show permission response in chat with nice formatting
        self.add_system_message(
            "",
            msg_type="permission_response",
            data={"tool": tool_id, "allowed": True}
        )
        self.update_status(f"✅ Auto-approved")

    async def receive_messages(self):
        """Receive and process messages from the agent"""
        try:
            while True:
                raw_msg = await self.websocket.recv()
                msg = json.loads(raw_msg)

                # Track if we handled this message
                handled = False

                # Handler: Permission request
                if "method" in msg and msg.get("method") == "session/request_permission":
                    await self.handle_permission_request(msg)
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
                                self.current_response += text
                                self.update_agent_typing(self.current_response)
                                self.advance_progress(2.0)  # Advance progress bar slightly with each chunk
                                handled = True

                        # Handler: Available commands update
                        elif session_update_type == "available_commands_update":
                            commands = update.get("availableCommands", [])
                            self.add_system_message(
                                "",
                                msg_type="available_commands_update",
                                data={"availableCommands": commands}
                            )
                            self.update_status(f"ℹ️  Agent updated {len(commands)} commands")
                            handled = True

                        # Handler: Tool use notification
                        elif session_update_type == "tool_use":
                            tool_name = update.get("tool", {}).get("name", "unknown")
                            self.add_system_message(
                                "",
                                msg_type="tool_use",
                                data={"tool": tool_name}
                            )
                            self.update_status(f"🔧 Tool: {tool_name}")
                            handled = True

                        # Handler: Thinking notification
                        elif session_update_type == "agent_thinking":
                            self.add_system_message(
                                "",
                                msg_type="thinking",
                                data={}
                            )
                            self.update_status("💭 Agent is thinking...")
                            handled = True

                        # Handler: Other session updates
                        else:
                            self.add_system_message(f"Session update: {session_update_type}")
                            self.update_status(f"ℹ️  Session update: {session_update_type}")
                            handled = True

                # Handler: Final response (turn complete)
                elif "id" in msg and msg["id"] >= 2:  # Response to our prompt
                    self.stop_agent_typing()
                    self.hide_progress()
                    self.update_status("✅ Ready to chat! (Ctrl+C to exit)")

                    if "error" in msg:
                        self.notify(f"Error: {msg['error']}", severity="error")
                    handled = True

                # Handler: Response to session creation
                elif "id" in msg and msg["id"] == 1:
                    # Already handled in on_mount
                    handled = True

                # Show unhandled messages in the chat
                if not handled:
                    self.add_unhandled_message(msg)
                    self.update_status(f"⚠️ Unhandled: {msg.get('method', msg.get('id', 'unknown'))}")

        except websockets.exceptions.ConnectionClosed:
            self.update_status("❌ Connection closed")
        except Exception as e:
            self.update_status(f"❌ Error: {e}")
            self.log(f"Error in receive_messages: {e}")

    async def on_input_submitted(self, event: Input.Submitted) -> None:
        """Handle user input submission"""
        user_input = event.value.strip()
        if not user_input:
            return

        # Clear input
        event.input.value = ""

        # Add user message to chat
        self.add_user_message(user_input)

        # Update status and show progress bar
        self.update_status("🤖 Agent is thinking...")
        self.show_progress()

        # Start agent typing indicator
        self.current_response = ""
        self.start_agent_typing()

        # Send prompt to agent
        await self.send_message(
            "session/prompt",
            {
                "sessionId": self.session_id,
                "content": [
                    {
                        "type": "text",
                        "text": user_input
                    }
                ]
            },
            self.msg_id
        )

        self.msg_id += 1


def get_active_sessions():
    """Get list of active sessions from the database"""
    try:
        conn = sqlite3.connect(DB_PATH)
        cursor = conn.execute("""
            SELECT id, working_directory, created_at, closed_at
            FROM sessions
            WHERE closed_at IS NULL
            ORDER BY created_at DESC
            LIMIT 10
        """)
        sessions = []
        for row in cursor:
            sessions.append({
                "id": row[0],
                "working_directory": row[1],
                "created_at": row[2],
                "closed_at": row[3]
            })
        conn.close()
        return sessions
    except (sqlite3.Error, FileNotFoundError):
        # If database doesn't exist or has errors, return empty list
        return []


def main():
    """Entry point"""
    app = ACPChatApp()
    app.run()


if __name__ == "__main__":
    main()
