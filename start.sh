#!/bin/sh
set -e

echo "=== Starting Open MCP Auth Proxy ==="

# Create necessary directories
mkdir -p /tmp/app /tmp/app-home /tmp/app-tmp /tmp/app-tmp/.npm /tmp/logs

echo "Created directory structure"

# Ensure binary is available in /tmp/app
if [ ! -f /tmp/app/openmcpauthproxy ]; then
    echo "Copying auth proxy binary to /tmp/app/"
    if [ -f /usr/local/bin/openmcpauthproxy ]; then
        cp /usr/local/bin/openmcpauthproxy /tmp/app/
    else
        echo "Error: openmcpauthproxy binary not found!"
        exit 1
    fi
fi

# Ensure config is available - create default if not found
if [ ! -f /tmp/app/config.yaml ]; then
    echo "Creating default config in /tmp/app/"
    cat > /tmp/app/config.yaml << 'EOF'
listen_port: 8080
base_url: "http://localhost:8000"
port: 8000
timeout_seconds: 10

paths:
  sse: "/sse"
  messages: "/messages/"

transport_mode: "stdio"

stdio:
  enabled: true
  user_command: "npx -y @modelcontextprotocol/server-github"
  work_dir: ""

cors:
  allowed_origins:
    - "*"
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
    - "OPTIONS"
  allowed_headers:
    - "Authorization"
    - "Content-Type"
    - "mcp-protocol-version"
    - "Origin"
    - "Accept"
    - "X-Requested-With"
  allow_credentials: true

demo:
  org_name: "openmcpauthdemo"
  client_id: "N0U9e_NNGr9mP_0fPnPfPI0a6twa"
  client_secret: "qFHfiBp5gNGAO9zV4YPnDofBzzfInatfUbHyPZvM0jka"
EOF
fi

# Make sure the binary is executable
chmod +x /tmp/app/openmcpauthproxy

# Debug: List contents
echo "Contents of /tmp/app:"
ls -la /tmp/app/

# Set environment variables for the auth proxy
export HOME="/tmp/app-home"
export TMPDIR="/tmp/app-tmp"
export NODE_PATH="/usr/local/lib/node_modules"
export NPM_CONFIG_CACHE="/tmp/app-tmp/.npm"
export CONFIG_FILE="/tmp/app/config.yaml"

# Use environment variable for external host if provided
if [ ! -z "$EXTERNAL_HOST" ]; then
    echo "Using external host: $EXTERNAL_HOST"
fi

echo "Environment variables set:"
echo "  HOME=$HOME"
echo "  CONFIG_FILE=$CONFIG_FILE"
echo "  EXTERNAL_HOST=${EXTERNAL_HOST:-not set}"

# Function to handle shutdown
shutdown() {
    echo "=== Shutting down auth proxy ==="
    if [ ! -z "$PROXY_PID" ]; then
        echo "Stopping auth proxy (PID: $PROXY_PID)"
        kill $PROXY_PID 2>/dev/null || true
    fi
    echo "Waiting for process to exit..."
    wait 2>/dev/null || true
    echo "=== Shutdown complete ==="
}

# Trap signals
trap shutdown TERM INT EXIT

echo "=== Starting auth proxy directly on port 8080 ==="

# Start the auth proxy directly on port 8080
cd /tmp/app && ./openmcpauthproxy --demo --debug &
PROXY_PID=$!
echo "Auth proxy started with PID: $PROXY_PID"

# Give auth proxy a moment to start
sleep 3

# Test if auth proxy is responding
echo "Testing auth proxy health..."
if command -v wget >/dev/null 2>&1; then
    wget -q -O - http://localhost:8080/.well-known/oauth-authorization-server || echo "Auth proxy health check failed"
else
    echo "wget not available for health check"
fi

echo "=== Auth proxy started successfully ==="
echo "Auth Proxy PID: $PROXY_PID"
echo "=== Now monitoring (Ctrl+C to stop) ==="

# Wait for the auth proxy process
wait $PROXY_PID