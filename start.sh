#!/bin/sh
set -e

echo "=== Starting Open MCP Auth Proxy ==="

# Set environment variables for the auth proxy
export HOME="/app"
export TMPDIR="/tmp"
export NODE_PATH="/usr/local/lib/node_modules"
export NPM_CONFIG_CACHE="/tmp/.npm"

# Create tmpfs directories if they don't exist
mkdir -p /tmp/.npm

# Use environment variable for external host if provided
if [ ! -z "$EXTERNAL_HOST" ]; then
    echo "Using external host: $EXTERNAL_HOST"
    export EXTERNAL_HOST="$EXTERNAL_HOST"
fi

echo "Environment variables set:"
echo "  HOME=$HOME"
echo "  CONFIG_FILE=${CONFIG_FILE:-/app/config.yaml}"
echo "  EXTERNAL_HOST=${EXTERNAL_HOST:-not set}"

# Function to handle shutdown - simplified to avoid double execution
shutdown() {
    if [ "$SHUTDOWN_CALLED" = "1" ]; then
        return
    fi
    export SHUTDOWN_CALLED=1
    
    echo "=== Shutting down auth proxy ==="
    if [ ! -z "$PROXY_PID" ] && kill -0 "$PROXY_PID" 2>/dev/null; then
        echo "Stopping auth proxy (PID: $PROXY_PID)"
        kill "$PROXY_PID" 2>/dev/null || true
        # Wait for graceful shutdown
        sleep 2
        # Force kill if still running
        if kill -0 "$PROXY_PID" 2>/dev/null; then
            echo "Force killing auth proxy"
            kill -9 "$PROXY_PID" 2>/dev/null || true
        fi
    fi
    echo "=== Shutdown complete ==="
}

# Trap signals - ensure single execution
trap shutdown TERM INT EXIT

echo "=== Starting auth proxy directly on port 8080 ==="

# # Set the config file path if not already set
# if [ -z "$CONFIG_FILE" ]; then
#     export CONFIG_FILE="/app/config.yaml"
# fi

# Verify the binary and config exist
if [ ! -f "/app/openmcpauthproxy" ]; then
    echo "ERROR: openmcpauthproxy binary not found at /app/openmcpauthproxy"
    exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo "WARNING: Config file not found at $CONFIG_FILE, using default"
fi

# Start the auth proxy directly on port 8080 in background
cd /app && exec ./openmcpauthproxy --demo --debug &
PROXY_PID=$!
echo "Auth proxy started with PID: $PROXY_PID"

# Give auth proxy time to start
echo "Waiting for auth proxy to start..."
sleep 5

# Test if auth proxy is responding with retries
echo "Testing auth proxy health..."
for i in 1 2 3 4 5; do
    if wget -q -T 5 -O - http://localhost:8080/.well-known/oauth-authorization-server >/dev/null 2>&1; then
        echo "Auth proxy health check passed on attempt $i"
        HEALTH_OK=1
        break
    else
        echo "Auth proxy health check failed on attempt $i, retrying..."
        sleep 2
    fi
done

if [ "$HEALTH_OK" != "1" ]; then
    echo "ERROR: Auth proxy failed to start properly after multiple attempts"
    # Check if process is still running
    if kill -0 "$PROXY_PID" 2>/dev/null; then
        echo "Process is running but not responding to HTTP requests"
        # Show recent logs if available
        echo "=== Checking process status ==="
        ps aux | grep openmcpauthproxy | grep -v grep || true
    else
        echo "Process has died"
        exit 1
    fi
else
    echo "=== Auth proxy started successfully ==="
fi

echo "Auth Proxy PID: $PROXY_PID"
echo "=== Now monitoring (Ctrl+C to stop) ==="

# Wait for the auth proxy process and handle its exit
wait "$PROXY_PID"
PROXY_EXIT_CODE=$?

echo "Auth proxy exited with code: $PROXY_EXIT_CODE"

# If we reach here, the process exited on its own
if [ $PROXY_EXIT_CODE -ne 0 ]; then
    echo "ERROR: Auth proxy exited with non-zero code"
    exit $PROXY_EXIT_CODE
fi