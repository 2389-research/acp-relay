#!/bin/bash
# ABOUTME: Helper script to run the container integration test
# ABOUTME: Starts the relay, runs the test, and cleans up

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== ACP Relay Container Integration Test ===${NC}"
echo

# Check if API key is set
if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo -e "${RED}ERROR: ANTHROPIC_API_KEY environment variable is not set${NC}"
    echo
    echo "Please set it before running this script:"
    echo "  export ANTHROPIC_API_KEY='sk-ant-your-key-here'"
    echo
    exit 1
fi

echo -e "${GREEN}✓ ANTHROPIC_API_KEY is set${NC}"

# Check if relay binary exists
if [ ! -f "./acp-relay" ]; then
    echo -e "${YELLOW}Building relay...${NC}"
    go build -o acp-relay ./cmd/relay
    echo -e "${GREEN}✓ Relay built${NC}"
fi

# Check if Docker image exists
if ! docker images | grep -q "acp-relay-runtime.*latest"; then
    echo -e "${YELLOW}Docker image not found. Building...${NC}"
    docker build -t acp-relay-runtime:latest .
    echo -e "${GREEN}✓ Docker image built${NC}"
fi

# Stop any existing relay
echo -e "${YELLOW}Stopping any existing relay...${NC}"
pkill -f "acp-relay" 2>/dev/null || true
sleep 1

# Clean up old containers
echo -e "${YELLOW}Cleaning up old containers...${NC}"
docker stop $(docker ps -q --filter "name=sess") 2>/dev/null || true
docker rm $(docker ps -aq --filter "name=sess") 2>/dev/null || true

# Start relay in background
echo -e "${GREEN}Starting relay...${NC}"
./acp-relay -config config-container-test.yaml > /tmp/acp-relay.log 2>&1 &
RELAY_PID=$!
echo "Relay PID: $RELAY_PID"

# Wait for relay to start
echo -e "${YELLOW}Waiting for relay to start...${NC}"
sleep 3

# Check if relay is running
if ! kill -0 $RELAY_PID 2>/dev/null; then
    echo -e "${RED}ERROR: Relay failed to start${NC}"
    echo "Logs from /tmp/acp-relay.log:"
    tail -20 /tmp/acp-relay.log
    exit 1
fi

echo -e "${GREEN}✓ Relay is running${NC}"
echo

# Run the integration test
echo -e "${GREEN}Running integration test...${NC}"
echo
python tests/integration_container_test.py
TEST_EXIT=$?

# Clean up
echo
echo -e "${YELLOW}Cleaning up...${NC}"
kill $RELAY_PID 2>/dev/null || true

if [ $TEST_EXIT -eq 0 ]; then
    echo -e "${GREEN}✓ Integration test PASSED${NC}"
else
    echo -e "${RED}✗ Integration test FAILED${NC}"
    echo
    echo "Relay logs from /tmp/acp-relay.log:"
    tail -50 /tmp/acp-relay.log
fi

exit $TEST_EXIT
