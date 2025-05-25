#!/bin/sh
set -e

echo "=== Starting Open MCP Auth Proxy with Debug Logging ==="

# Create all necessary directories first
mkdir -p /tmp/app /tmp/app-home /tmp/app-tmp /tmp/app-tmp/.npm
mkdir -p /tmp/nginx-temp/client_temp /tmp/nginx-temp/proxy_temp /tmp/nginx-temp/fastcgi_temp /tmp/nginx-temp/uwsgi_temp /tmp/nginx-temp/scgi_temp
mkdir -p /tmp/run /tmp/logs

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
listen_port: 8081
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
    - "https://1abe8483-9db5-4f7b-a457-787c98ad6593-dev.e1-us-east-azure.choreoapis.dev"
    - "http://127.0.0.1:6274"
    - "http://127.0.0.1:6274/"
    - "http://localhost:5173"
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
export CONFIG_FILE="/app/config.yaml"
export EXTERNAL_HOST="https://4e898286-2b6d-4a63-a5a6-5192df899ef1.e1-us-east-azure.choreoapps.dev"

echo "Environment variables set:"
echo "  HOME=$HOME"
echo "  CONFIG_FILE=$CONFIG_FILE"
echo "  EXTERNAL_HOST=$EXTERNAL_HOST"

# Test nginx configuration
echo "Testing nginx configuration..."
nginx -t
if [ $? -ne 0 ]; then
    echo "Nginx configuration test failed!"
    exit 1
fi
echo "Nginx configuration test passed"

# Function to handle shutdown
shutdown() {
    echo "=== Shutting down services ==="
    if [ ! -z "$NGINX_PID" ]; then
        echo "Stopping nginx (PID: $NGINX_PID)"
        kill $NGINX_PID 2>/dev/null || true
    fi
    if [ ! -z "$PROXY_PID" ]; then
        echo "Stopping auth proxy (PID: $PROXY_PID)"
        kill $PROXY_PID 2>/dev/null || true
    fi
    echo "Waiting for processes to exit..."
    wait 2>/dev/null || true
    echo "=== Shutdown complete ==="
}

# Trap signals
trap shutdown TERM INT EXIT

echo "=== Starting services with debug logging ==="

# Start nginx in foreground with debug logging
echo "Starting nginx on port 8080 with debug logging..."
nginx -g "daemon off;" 2>&1 | sed 's/^/[NGINX] /' &
NGINX_PID=$!
echo "Nginx started with PID: $NGINX_PID"

# Give nginx a moment to start
sleep 2

# Test if nginx is responding
echo "Testing nginx health..."
if command -v curl >/dev/null 2>&1; then
    curl -s http://localhost:8080/nginx-health || echo "Nginx health check failed (this is expected if auth proxy isn't ready)"
elif command -v wget >/dev/null 2>&1; then
    wget -q -O - http://localhost:8080/nginx-health || echo "Nginx health check failed (this is expected if auth proxy isn't ready)"
else
    echo "Neither curl nor wget available for health check"
fi

# Start the auth proxy on port 8081 in foreground with debug logging
echo "Starting auth proxy on port 8081 with debug logging..."
cd /tmp/app && ./openmcpauthproxy --demo --debug 2>&1 | sed 's/^/[PROXY] /' &
PROXY_PID=$!
echo "Auth proxy started with PID: $PROXY_PID"

# Give auth proxy a moment to start
sleep 3

# Test if auth proxy is responding
echo "Testing auth proxy health..."
if command -v curl >/dev/null 2>&1; then
    curl -s http://localhost:8081/.well-known/oauth-authorization-server || echo "Auth proxy health check failed"
elif command -v wget >/dev/null 2>&1; then
    wget -q -O - http://localhost:8081/.well-known/oauth-authorization-server || echo "Auth proxy health check failed"
else
    echo "Neither curl nor wget available for health check"
fi

# Test full stack through nginx
echo "Testing full stack through nginx..."
if command -v curl >/dev/null 2>&1; then
    echo "Testing OAuth well-known endpoint through nginx:"
    curl -v http://localhost:8080/.well-known/oauth-authorization-server || echo "Full stack test failed"
elif command -v wget >/dev/null 2>&1; then
    echo "Testing OAuth well-known endpoint through nginx:"
    wget -q -O - http://localhost:8080/.well-known/oauth-authorization-server || echo "Full stack test failed"
fi

echo "=== Services started successfully ==="
echo "Nginx PID: $NGINX_PID"
echo "Auth Proxy PID: $PROXY_PID"
echo "=== Now monitoring logs (Ctrl+C to stop) ==="

# Monitor both processes and log their output
while true; do
    # Check if nginx is still running
    if ! kill -0 $NGINX_PID 2>/dev/null; then
        echo "ERROR: Nginx process died!"
        break
    fi
    
    # Check if auth proxy is still running  
    if ! kill -0 $PROXY_PID 2>/dev/null; then
        echo "ERROR: Auth proxy process died!"
        break
    fi
    
    sleep 5
done

# If we get here, one of the processes died
echo "=== One or more services died, initiating shutdown ==="
shutdown