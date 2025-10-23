#!/usr/bin/env python3
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "websockets",
# ]
# ///
"""
Minimal permission testing script - iteratively fix permission handling
"""

import asyncio
import websockets
import json

RELAY_WS_URL = "ws://localhost:8081"
WORKING_DIR = "/tmp"


async def test_permission():
    """Test permission request/response flow"""
    print("🔌 Connecting to relay...")
    websocket = await websockets.connect(RELAY_WS_URL)

    # Create session
    print("📝 Creating session...")
    create_msg = {
        "jsonrpc": "2.0",
        "method": "session/new",
        "params": {"workingDirectory": WORKING_DIR},
        "id": 1
    }
    await websocket.send(json.dumps(create_msg))

    # Get session ID
    session_id = None
    while True:
        raw_msg = await websocket.recv()
        msg = json.loads(raw_msg)
        print(f"📥 Received: {json.dumps(msg, indent=2)[:200]}...")

        if msg.get("id") == 1 and "result" in msg:
            session_id = msg["result"].get("sessionId")
            print(f"✅ Session created: {session_id}")
            break

    # Send prompt
    print("\n💬 Sending prompt: 'can you make a file named test.txt'")
    prompt_msg = {
        "jsonrpc": "2.0",
        "method": "session/prompt",
        "params": {
            "sessionId": session_id,
            "content": [
                {
                    "type": "text",
                    "text": "can you make a file named test.txt"
                }
            ]
        },
        "id": 2
    }
    await websocket.send(json.dumps(prompt_msg))

    # Listen for messages and handle permissions
    print("\n👂 Listening for messages...\n")
    while True:
        try:
            raw_msg = await asyncio.wait_for(websocket.recv(), timeout=60.0)
            msg = json.loads(raw_msg)

            # Check message type
            method = msg.get("method", "")
            msg_id = msg.get("id")

            # Permission request
            if method == "session/request_permission":
                print("=" * 60)
                print("🔐 PERMISSION REQUEST RECEIVED:")
                print(json.dumps(msg, indent=2))
                print("=" * 60)

                # Extract details
                params = msg.get("params", {})
                tool_call = params.get("toolCall", {})
                options = params.get("options", [])

                print(f"\n📋 Available options: {[opt.get('optionId') for opt in options]}")
                print(f"\n📋 Full options:")
                for opt in options:
                    print(json.dumps(opt, indent=2))

                # Send response with outcome wrapper!
                # Based on TypeScript SDK example:
                # { outcome: { outcome: "selected", optionId: "allow" } }
                response = {
                    "jsonrpc": "2.0",
                    "id": msg_id,
                    "result": {
                        "outcome": {
                            "outcome": "selected",
                            "optionId": "allow"
                        }
                    }
                }
                print(f"\n✅ Sending approval response:")
                print(json.dumps(response, indent=2))
                await websocket.send(json.dumps(response))
                print("📤 Response sent!\n")

                # Now watch for any immediate response or error
                print("👀 Watching for immediate response from agent...\n")

            # Session update (agent message)
            elif method == "session/update":
                update = msg.get("params", {}).get("update", {})
                update_type = update.get("sessionUpdate")

                if update_type == "agent_message_chunk":
                    text = update.get("content", {}).get("text", "")
                    if text:
                        print(f"🤖 Agent: {text}", end="", flush=True)

            # Final response
            elif msg_id == 2:
                print(f"\n\n🏁 Final response received:")
                print(json.dumps(msg, indent=2))

                if "error" in msg:
                    print(f"\n❌ ERROR: {msg['error']}")
                else:
                    print(f"\n✅ SUCCESS!")
                break

            # Other messages
            else:
                if method:
                    print(f"ℹ️  Other message: {method}")

        except asyncio.TimeoutError:
            print("\n⏱️  Timeout waiting for response")
            break

    await websocket.close()
    print("\n👋 Done!")


if __name__ == "__main__":
    asyncio.run(test_permission())
