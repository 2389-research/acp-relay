#!/usr/bin/env python3
# ABOUTME: Simple test agent that echoes JSON-RPC messages back for testing
# ABOUTME: Used in integration tests to verify relay server functionality

import sys
import json

# Ensure unbuffered I/O
sys.stdout.reconfigure(line_buffering=True)
sys.stderr.reconfigure(line_buffering=True)

for line in sys.stdin:
    # Echo the line back immediately
    sys.stdout.write(line)
    sys.stdout.flush()
