#!/bin/bash
# ssh-agent-mux demonstration script
# This script demonstrates the key features of ssh-agent-mux

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SOCKET_PATH="/tmp/ssh-agent-mux-demo.sock"
DEMO_KEY1="/tmp/demo_key_1"
DEMO_KEY2="/tmp/demo_key_2"
BINARY="./artifacts/build/release/$(uname -s | tr '[:upper:]' '[:lower:]')/$(uname -m)/ssh-agent-mux"

# Check if binary exists
if [ ! -f "$BINARY" ]; then
    echo -e "${RED}Error: ssh-agent-mux binary not found at $BINARY${NC}"
    echo -e "${YELLOW}Please build it first with: mage build${NC}"
    exit 1
fi

# Cleanup function
cleanup() {
    echo
    echo -e "${BLUE}=== Cleanup ===${NC}"
    if [ -n "$AGENT_PID" ] && kill -0 "$AGENT_PID" 2>/dev/null; then
        echo "Shutting down ssh-agent-mux..."
        "$BINARY" -c shutdown -s "$SOCKET_PATH" 2>/dev/null || kill "$AGENT_PID" 2>/dev/null || true
        sleep 1
    fi
    rm -f "$DEMO_KEY1" "${DEMO_KEY1}.pub" "$DEMO_KEY2" "${DEMO_KEY2}.pub" "$SOCKET_PATH"
    echo -e "${GREEN}Cleanup complete!${NC}"
}

# Set trap to cleanup on exit
trap cleanup EXIT INT TERM

echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   SSH Agent Multiplexer Demo          ║${NC}"
echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo

# Step 1: Start the agent
echo -e "${YELLOW}[Step 1] Starting ssh-agent-mux in foreground mode...${NC}"
"$BINARY" -s "$SOCKET_PATH" -f --debug --log-path /tmp/ssh-agent-mux-demo.log &
AGENT_PID=$!
echo "  → PID: $AGENT_PID"
echo "  → Socket: $SOCKET_PATH"
echo "  → Log: /tmp/ssh-agent-mux-demo.log"

# Give it time to start
sleep 2

# Step 2: Verify it's running
echo
echo -e "${YELLOW}[Step 2] Checking agent status...${NC}"
if "$BINARY" -c ping -s "$SOCKET_PATH" >/dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Agent is running${NC}"
else
    echo -e "  ${RED}✗ Agent failed to start${NC}"
    exit 1
fi

# Step 3: Get config
echo
echo -e "${YELLOW}[Step 3] Getting agent configuration...${NC}"
"$BINARY" -c config -s "$SOCKET_PATH" | sed 's/^/  /'

# Step 4: Set environment
echo
echo -e "${YELLOW}[Step 4] Setting SSH_AUTH_SOCK...${NC}"
export SSH_AUTH_SOCK="$SOCKET_PATH"
echo "  → export SSH_AUTH_SOCK=$SOCKET_PATH"

# Step 5: List initial keys
echo
echo -e "${YELLOW}[Step 5] Listing initial keys...${NC}"
if ssh-add -l 2>/dev/null; then
    echo -e "  ${GREEN}✓ Backend agent keys found${NC}"
else
    echo -e "  ${BLUE}ℹ No keys (agent is empty)${NC}"
fi

# Step 6: Generate first test key
echo
echo -e "${YELLOW}[Step 6] Generating first ephemeral key...${NC}"
ssh-keygen -t ed25519 -f "$DEMO_KEY1" -N "" -C "demo-ephemeral-key-1" >/dev/null 2>&1
echo -e "  ${GREEN}✓ Generated: ${DEMO_KEY1}${NC}"
KEY1_FINGERPRINT=$(ssh-keygen -lf "${DEMO_KEY1}.pub" | awk '{print $2}')
echo "  → Fingerprint: $KEY1_FINGERPRINT"

# Step 7: Add first key
echo
echo -e "${YELLOW}[Step 7] Adding first ephemeral key to agent...${NC}"
ssh-add "$DEMO_KEY1" 2>&1 | sed 's/^/  /'

# Step 8: List keys with first key
echo
echo -e "${YELLOW}[Step 8] Listing keys (should show first ephemeral key)...${NC}"
ssh-add -l 2>&1 | sed 's/^/  /'

# Step 9: Generate second test key
echo
echo -e "${YELLOW}[Step 9] Generating second ephemeral key...${NC}"
ssh-keygen -t ed25519 -f "$DEMO_KEY2" -N "" -C "demo-ephemeral-key-2" >/dev/null 2>&1
echo -e "  ${GREEN}✓ Generated: ${DEMO_KEY2}${NC}"
KEY2_FINGERPRINT=$(ssh-keygen -lf "${DEMO_KEY2}.pub" | awk '{print $2}')
echo "  → Fingerprint: $KEY2_FINGERPRINT"

# Step 10: Add second key
echo
echo -e "${YELLOW}[Step 10] Adding second ephemeral key to agent...${NC}"
ssh-add "$DEMO_KEY2" 2>&1 | sed 's/^/  /'

# Step 11: List all keys
echo
echo -e "${YELLOW}[Step 11] Listing all keys (should show both ephemeral keys)...${NC}"
KEY_COUNT=$(ssh-add -l 2>/dev/null | wc -l | tr -d ' ')
ssh-add -l 2>&1 | sed 's/^/  /'
echo -e "  ${GREEN}✓ Total keys: $KEY_COUNT${NC}"

# Step 12: Remove first key
echo
echo -e "${YELLOW}[Step 12] Removing first ephemeral key...${NC}"
ssh-add -d "$DEMO_KEY1" 2>&1 | sed 's/^/  /'

# Step 13: List keys after removal
echo
echo -e "${YELLOW}[Step 13] Listing keys (should show only second key)...${NC}"
ssh-add -l 2>&1 | sed 's/^/  /'

# Step 14: Remove all local keys
echo
echo -e "${YELLOW}[Step 14] Removing all local keys...${NC}"
ssh-add -D 2>&1 | sed 's/^/  /'

# Step 15: Final key list
echo
echo -e "${YELLOW}[Step 15] Listing keys (should be back to initial state)...${NC}"
if ssh-add -l 2>/dev/null; then
    ssh-add -l | sed 's/^/  /'
else
    echo -e "  ${BLUE}ℹ No keys${NC}"
fi

# Step 16: Test ping command
echo
echo -e "${YELLOW}[Step 16] Testing ping command...${NC}"
"$BINARY" -c ping -s "$SOCKET_PATH" 2>&1 | sed 's/^/  /'

# Summary
echo
echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   Demo Complete!                      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
echo
echo -e "${GREEN}Key Takeaways:${NC}"
echo "  1. ssh-agent-mux runs as a background daemon"
echo "  2. You can add/remove ephemeral keys with ssh-add"
echo "  3. Backend agent keys are available alongside local keys"
echo "  4. Management commands (ping, config, shutdown) are available"
echo "  5. Keys are stored in memory only (never on disk)"
echo
echo -e "${YELLOW}Log file available at: /tmp/ssh-agent-mux-demo.log${NC}"
echo
