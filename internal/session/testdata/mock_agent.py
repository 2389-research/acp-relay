#!/usr/bin/env python3
"""
ABOUTME: Mock ACP agent for testing that implements initialize handshake
ABOUTME: Reads JSON-RPC requests from stdin and writes responses to stdout
"""
import sys
import json

def main():
    while True:
        try:
            line = sys.stdin.readline()
            if not line:
                break

            request = json.loads(line)
            method = request.get("method")
            req_id = request.get("id")

            if method == "initialize":
                # Respond to initialize with proper ACP response
                response = {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "protocolVersion": 1,
                        "serverInfo": {
                            "name": "mock-agent",
                            "version": "0.1.0"
                        },
                        "capabilities": {}
                    }
                }
            elif method == "session/new":
                # Respond to session/new with a mock session ID
                # Expecting params: { cwd: string, mcpServers: object }
                response = {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "sessionId": "mock_sess_12345"
                    }
                }
            else:
                # Echo back other requests
                response = {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "echo": request
                    }
                }

            sys.stdout.write(json.dumps(response) + "\n")
            sys.stdout.flush()

        except Exception as e:
            sys.stderr.write(f"Error: {e}\n")
            sys.stderr.flush()
            break

if __name__ == "__main__":
    main()
