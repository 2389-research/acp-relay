#!/usr/bin/env python3
"""
Integration test for container mode: Create a date stamp script via ACP agent.

This test:
1. Connects to the relay's WebSocket server
2. Creates a new container session
3. Sends a prompt to create a Python script that outputs a date stamp
4. Verifies the agent creates the file successfully
5. Cleans up the session

Prerequisites:
- Relay running: ANTHROPIC_API_KEY="your-key" ./acp-relay -config config-container-test.yaml
- Docker runtime image built: docker build -t acp-relay-runtime:latest .
- Python websockets: pip install websockets

Usage:
    python tests/integration_container_test.py
"""

import asyncio
import json
import sys
import time
from datetime import datetime
from pathlib import Path

try:
    import websockets
except ImportError:
    print("ERROR: websockets module not found")
    print("Install with: pip install websockets")
    sys.exit(1)


class ACPIntegrationTest:
    def __init__(self, ws_url="ws://localhost:8081", working_dir="/tmp/acp-test"):
        self.ws_url = ws_url
        self.working_dir = working_dir
        self.session_id = None
        self.msg_id = 0

    def next_id(self):
        """Get next message ID"""
        self.msg_id += 1
        return self.msg_id

    async def send_request(self, ws, method, params):
        """Send JSON-RPC request and return response"""
        msg_id = self.next_id()
        request = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
            "id": msg_id
        }

        print(f"→ Sending: {method} (id={msg_id})")
        await ws.send(json.dumps(request))

        # Wait for response with matching ID
        while True:
            response_text = await ws.recv()
            response = json.loads(response_text)

            # Check if this is our response (matching ID)
            if response.get("id") == msg_id:
                if "error" in response:
                    error = response["error"]
                    print(f"✗ Error: {error.get('message', 'Unknown error')}")
                    if "data" in error:
                        print(f"  Details: {error['data']}")
                    raise Exception(f"RPC error: {error}")

                print(f"← Received: {method} response")
                return response.get("result")
            else:
                # This is an agent notification/response, print and continue
                if "method" in response:
                    print(f"  [Agent notification: {response['method']}]")
                else:
                    print(f"  [Other message: {response}]")

    async def run_test(self):
        """Run the full integration test"""
        print("=" * 60)
        print("ACP Container Integration Test")
        print("=" * 60)
        print()

        # Ensure working directory exists
        Path(self.working_dir).mkdir(parents=True, exist_ok=True)

        try:
            print(f"Connecting to {self.ws_url}...")
            async with websockets.connect(self.ws_url) as ws:
                print("✓ Connected to relay WebSocket")
                print()

                # Step 1: Create new session
                print("Step 1: Creating new session...")
                result = await self.send_request(ws, "session/new", {
                    "workingDirectory": self.working_dir
                })
                self.session_id = result["sessionId"]
                print(f"✓ Session created: {self.session_id}")
                print()

                # Wait a moment for session to fully initialize
                await asyncio.sleep(2)

                # Step 2: Send prompt to create date stamp script
                print("Step 2: Sending prompt to create date stamp script...")
                prompt_content = {
                    "type": "text",
                    "text": """Create a Python script named 'datestamp.py' that outputs today's date
with seconds in this format: 'YYYY-MM-DD HH:MM:SS'. The script should use
datetime.datetime.now() and print the formatted date. Make it simple and
self-contained."""
                }

                result = await self.send_request(ws, "session/prompt", {
                    "sessionId": self.session_id,
                    "content": prompt_content
                })
                print(f"✓ Prompt sent, waiting for agent response...")

                # Wait for agent to process (give it time to create the file)
                print("  Waiting for agent to create the file...")
                await asyncio.sleep(5)

                # Step 3: Verify the file was created
                print()
                print("Step 3: Verifying datestamp.py was created...")
                expected_file = Path(self.working_dir) / "datestamp.py"

                if expected_file.exists():
                    print(f"✓ File created: {expected_file}")
                    print()
                    print("File contents:")
                    print("-" * 40)
                    print(expected_file.read_text())
                    print("-" * 40)
                    print()

                    # Try to run it
                    print("Step 4: Testing the script...")
                    import subprocess
                    result = subprocess.run(
                        ["python3", str(expected_file)],
                        capture_output=True,
                        text=True,
                        timeout=5
                    )

                    if result.returncode == 0:
                        output = result.stdout.strip()
                        print(f"✓ Script executed successfully!")
                        print(f"  Output: {output}")

                        # Verify it looks like a date stamp
                        if len(output) >= 10:  # At least YYYY-MM-DD
                            print(f"✓ Output looks like a date stamp")
                        else:
                            print(f"✗ Output doesn't look like a date stamp")
                            return False
                    else:
                        print(f"✗ Script failed with exit code {result.returncode}")
                        print(f"  Error: {result.stderr}")
                        return False
                else:
                    print(f"✗ File not created: {expected_file}")
                    print(f"  Files in directory: {list(Path(self.working_dir).iterdir())}")
                    return False

                print()
                print("=" * 60)
                print("✓ All tests passed!")
                print("=" * 60)
                return True

        except websockets.exceptions.WebSocketException as e:
            print(f"✗ WebSocket error: {e}")
            print()
            print("Make sure the relay is running:")
            print("  ./acp-relay -config config-container-test.yaml")
            return False
        except Exception as e:
            print(f"✗ Test failed: {e}")
            import traceback
            traceback.print_exc()
            return False


async def main():
    """Main entry point"""
    test = ACPIntegrationTest()
    success = await test.run_test()
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    asyncio.run(main())
