#!/bin/bash
# Example script demonstrating ssh-agent-mux with 1Password

set -e

echo "=== SSH Agent Multiplexer Demo ==="
echo

# Start ssh-agent-mux
echo "1. Starting ssh-agent-mux..."
SOCKET_PATH="/tmp/ssh-agent-mux-demo.sock"
./ssh-agent-mux -socket "$SOCKET_PATH" -verbose &
AGENT_PID=$!

# Give it time to start
sleep 1

echo "2. Setting SSH_AUTH_SOCK..."
export SSH_AUTH_SOCK="$SOCKET_PATH"

echo "3. Listing keys (should be empty or show fallback keys)..."
ssh-add -l || echo "No keys yet"

echo
echo "4. Generating a temporary test key..."
ssh-keygen -t ed25519 -f /tmp/demo_key -N "" -C "demo-ephemeral-key" >/dev/null 2>&1

echo "5. Adding the ephemeral key..."
ssh-add /tmp/demo_key

echo
echo "6. Listing keys (should show the ephemeral key)..."
ssh-add -l

echo
echo "7. Removing the ephemeral key..."
ssh-add -d /tmp/demo_key

echo
echo "8. Listing keys again (should be back to original state)..."
ssh-add -l || echo "No keys"

echo
echo "=== Cleanup ==="
# Cleanup
kill $AGENT_PID 2>/dev/null || true
rm -f /tmp/demo_key /tmp/demo_key.pub "$SOCKET_PATH"
echo "Done!"
