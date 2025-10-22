#!/usr/bin/env python3
"""
Hello World example for ACP Relay Server

This script demonstrates how to:
1. Create a session with the relay server
2. Send a prompt to an ACP agent
3. Receive and display the response

Usage:
    python3 examples/hello_world.py
"""

import requests
import json
import sys

# Configuration
RELAY_URL = "http://localhost:8080"
WORKING_DIR = "/tmp"

def pretty_print(label, data):
    """Pretty print JSON data"""
    print(f"\n{'='*60}")
    print(f"{label}")
    print(f"{'='*60}")
    print(json.dumps(data, indent=2))
    print()

def create_session():
    """Create a new session with the relay server"""
    print("📝 Creating new session...")

    request = {
        "jsonrpc": "2.0",
        "method": "session/new",
        "params": {
            "workingDirectory": WORKING_DIR
        },
        "id": 1
    }

    response = requests.post(
        f"{RELAY_URL}/session/new",
        json=request,
        headers={"Content-Type": "application/json"}
    )

    if response.status_code != 200:
        print(f"❌ Failed to create session: {response.status_code}")
        print(response.text)
        sys.exit(1)

    data = response.json()
    pretty_print("Session Created", data)

    if "error" in data:
        print("❌ Error creating session:")
        print(f"   Code: {data['error']['code']}")
        print(f"   Message: {data['error']['message']}")
        if 'data' in data['error']:
            error_data = json.loads(data['error']['data']) if isinstance(data['error']['data'], str) else data['error']['data']
            if 'explanation' in error_data:
                print(f"   Explanation: {error_data['explanation']}")
            if 'suggested_actions' in error_data:
                print("   Suggested actions:")
                for action in error_data['suggested_actions']:
                    print(f"     • {action}")
        sys.exit(1)

    session_id = data["result"]["sessionId"]
    print(f"✅ Session created: {session_id}")
    return session_id

def send_prompt(session_id, prompt_text):
    """Send a prompt to the agent via the relay server"""
    print(f"\n💬 Sending prompt: '{prompt_text}'...")

    request = {
        "jsonrpc": "2.0",
        "method": "session/prompt",
        "params": {
            "sessionId": session_id,
            "content": [
                {
                    "type": "text",
                    "text": prompt_text
                }
            ]
        },
        "id": 2
    }

    response = requests.post(
        f"{RELAY_URL}/session/prompt",
        json=request,
        headers={"Content-Type": "application/json"},
        timeout=30
    )

    if response.status_code != 200:
        print(f"❌ Failed to send prompt: {response.status_code}")
        print(response.text)
        return None

    data = response.json()
    pretty_print("Agent Response", data)

    if "error" in data:
        print("❌ Error from agent:")
        print(f"   Code: {data['error']['code']}")
        print(f"   Message: {data['error']['message']}")
        return None

    return data.get("result")

def check_health():
    """Check relay server health"""
    print("🏥 Checking relay server health...")

    try:
        response = requests.get("http://localhost:8082/api/health")
        if response.status_code == 200:
            health = response.json()
            print(f"✅ Relay server is {health.get('status', 'unknown')}")
            print(f"   Agent command: {health.get('agent_command', 'unknown')}")
            return True
        else:
            print(f"❌ Health check failed: {response.status_code}")
            return False
    except requests.exceptions.ConnectionError:
        print("❌ Cannot connect to relay server at localhost:8082")
        print("   Make sure the relay server is running:")
        print("   $ go run cmd/relay/main.go --config config.yaml")
        return False

def main():
    """Main function"""
    print("\n🚀 ACP Relay Server - Hello World Example\n")

    # Check if relay server is running
    if not check_health():
        sys.exit(1)

    # Create a session
    try:
        session_id = create_session()
    except requests.exceptions.ConnectionError:
        print("❌ Cannot connect to relay server at localhost:8080")
        print("   Make sure the relay server is running:")
        print("   $ go run cmd/relay/main.go --config config.yaml")
        sys.exit(1)

    # Send a hello world prompt
    prompt = "Hello! Can you introduce yourself?"
    result = send_prompt(session_id, prompt)

    if result:
        print("✅ Successfully communicated with ACP agent via relay!")
    else:
        print("⚠️  Note: This example uses a test echo agent.")
        print("   For real responses, configure an actual ACP agent in config.yaml")

    print("\n" + "="*60)
    print("🎉 Hello World example complete!")
    print("="*60 + "\n")

if __name__ == "__main__":
    main()
