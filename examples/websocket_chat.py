#!/usr/bin/env python3
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "websockets",
# ]
# ///
"""
WebSocket Chat example for ACP Relay Server

This script demonstrates how to:
1. Connect to the relay server via WebSocket
2. Create a session with the ACP agent
3. Send prompts and receive streaming responses
4. Handle multiple messages in a turn (notifications + final response)

Usage:
    python3 examples/websocket_chat.py
"""

import asyncio
import websockets
import json
import sys

# Configuration
RELAY_WS_URL = "ws://localhost:8081"
WORKING_DIR = "/tmp"

async def send_message(websocket, method, params, msg_id):
    """Send a JSON-RPC message via WebSocket"""
    message = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": msg_id
    }
    print(f"\n📤 Sending: {method}")
    print(f"   Params: {json.dumps(params, indent=2)}")
    await websocket.send(json.dumps(message))

async def receive_messages(websocket, expected_id):
    """
    Receive messages from the agent until we get the final response.

    Per ACP spec, a turn consists of:
    - Multiple session/update notifications (no id field)
    - Final response (with id field matching request)
    """
    messages = []
    print(f"\n📨 Waiting for messages from agent...")

    while True:
        try:
            raw_msg = await asyncio.wait_for(websocket.recv(), timeout=30.0)
            msg = json.loads(raw_msg)
            messages.append(msg)

            # Check if this is a notification (session/update) or response
            if "method" in msg and "id" not in msg:
                # This is a notification
                print(f"\n🔔 Notification: {msg.get('method')}")
                if "params" in msg:
                    print(f"   {json.dumps(msg['params'], indent=2)}")
            elif "id" in msg:
                # This is a response
                msg_id = msg["id"]
                print(f"\n✅ Response received (id: {msg_id})")

                if msg_id == expected_id:
                    # This is the response to our request - turn is complete
                    if "error" in msg:
                        print(f"❌ Error: {msg['error']}")
                    elif "result" in msg:
                        result = msg["result"]
                        print(f"   Result: {json.dumps(result, indent=2)}")

                        # Check for stopReason to confirm turn is complete
                        if isinstance(result, dict) and "stopReason" in result:
                            print(f"   Stop reason: {result['stopReason']}")

                    return messages
                else:
                    print(f"   (Response to different request: {msg_id})")
            else:
                print(f"\n⚠️  Unknown message type: {json.dumps(msg, indent=2)}")

        except asyncio.TimeoutError:
            print("\n⏱️  Timeout waiting for agent response")
            return messages
        except json.JSONDecodeError as e:
            print(f"\n❌ Failed to parse message: {e}")
            print(f"   Raw: {raw_msg}")
        except Exception as e:
            print(f"\n❌ Error receiving message: {e}")
            return messages

async def chat_session():
    """Main chat session"""
    print("\n🚀 ACP Relay Server - WebSocket Chat Example\n")

    try:
        async with websockets.connect(RELAY_WS_URL) as websocket:
            print(f"✅ Connected to relay server at {RELAY_WS_URL}")

            # Step 1: Create a session
            print("\n" + "="*60)
            print("STEP 1: Creating Session")
            print("="*60)

            await send_message(
                websocket,
                "session/new",
                {"workingDirectory": WORKING_DIR},
                1
            )

            session_msgs = await receive_messages(websocket, 1)

            # Extract session ID from response
            session_id = None
            for msg in session_msgs:
                if msg.get("id") == 1 and "result" in msg:
                    session_id = msg["result"].get("sessionId")
                    break

            if not session_id:
                print("❌ Failed to create session")
                return

            print(f"\n✅ Session created: {session_id}")

            # Step 2: Send a prompt
            print("\n" + "="*60)
            print("STEP 2: Sending Prompt")
            print("="*60)

            prompt_text = "Hello! Can you introduce yourself and tell me what you can do?"

            await send_message(
                websocket,
                "session/prompt",
                {
                    "sessionId": session_id,
                    "content": [
                        {
                            "type": "text",
                            "text": prompt_text
                        }
                    ]
                },
                2
            )

            prompt_msgs = await receive_messages(websocket, 2)

            print(f"\n📊 Received {len(prompt_msgs)} message(s) in this turn")

            # Step 3: Interactive chat (optional - could be extended)
            print("\n" + "="*60)
            print("Chat session complete!")
            print("="*60)
            print("\n💡 To extend this example, you could:")
            print("   • Add a loop to send multiple prompts")
            print("   • Process session/update notifications in real-time")
            print("   • Display agent status updates as they arrive")
            print("   • Handle user input for interactive chat")

    except websockets.exceptions.ConnectionRefused:
        print("❌ Cannot connect to relay server at ws://localhost:8081")
        print("   Make sure the relay server is running:")
        print("   $ go run cmd/relay/main.go --config config.yaml")
        sys.exit(1)
    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

def main():
    """Entry point"""
    asyncio.run(chat_session())

if __name__ == "__main__":
    main()
