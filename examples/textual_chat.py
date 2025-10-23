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
from datetime import datetime
from textual.app import App, ComposeResult
from textual.containers import Container, Vertical, ScrollableContainer
from textual.widgets import Header, Footer, Input, Static, RichLog
from textual.binding import Binding
from textual.message import Message

# Configuration
RELAY_WS_URL = "ws://localhost:8081"
WORKING_DIR = "/tmp"


class ChatMessage(Static):
    """A single chat message widget"""

    def __init__(self, role: str, text: str, timestamp: str):
        self.role = role
        self.text = text
        self.timestamp = timestamp
        super().__init__()

    def compose(self) -> ComposeResult:
        if self.role == "user":
            yield Static(f"[dim]{self.timestamp}[/dim] [bold cyan]You:[/bold cyan] {self.text}")
        elif self.role == "agent":
            yield Static(f"[dim]{self.timestamp}[/dim] [bold green]Agent:[/bold green] {self.text}")
        elif self.role == "system":
            yield Static(f"[dim]{self.timestamp}[/dim] [bold yellow]System:[/bold yellow] {self.text}")
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
            yield Input(placeholder="Type your message here...", id="chat-input")
        yield Footer()

    async def on_mount(self) -> None:
        """Initialize the WebSocket connection"""
        self.title = "🤖 ACP Agent Chat"
        self.sub_title = "Connecting..."

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

    def add_system_message(self, text: str):
        """Add a system message to the chat"""
        timestamp = datetime.now().strftime("%H:%M:%S")
        messages = self.query_one("#messages", ScrollableContainer)
        messages.mount(ChatMessage("system", text, timestamp))
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
        file_path = raw_input.get("file_path", "")
        content = raw_input.get("content", "")

        # Show permission request in chat
        tool_summary = f"Tool: {tool_id}\nFile: {file_path}"
        if content:
            preview = content[:100] + "..." if len(content) > 100 else content
            tool_summary += f"\nContent preview: {preview}"

        self.add_system_message(f"🔐 Permission requested:\n{tool_summary}")

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

        self.add_system_message(f"✅ Auto-approved: {tool_id}")
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
                                handled = True

                        # Handler: Available commands update
                        elif session_update_type == "available_commands_update":
                            self.update_status("ℹ️ Agent updated available commands")
                            handled = True

                        # Handler: Other session updates
                        else:
                            self.update_status(f"ℹ️ Session update: {session_update_type}")
                            handled = True

                # Handler: Final response (turn complete)
                elif "id" in msg and msg["id"] >= 2:  # Response to our prompt
                    self.stop_agent_typing()
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

        # Update status
        self.update_status("🤖 Agent is thinking...")

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


def main():
    """Entry point"""
    app = ACPChatApp()
    app.run()


if __name__ == "__main__":
    main()
