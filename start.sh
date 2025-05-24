#!/bin/sh
# Create all necessary directories first
mkdir -p /tmp/app /tmp/app-home /tmp/app-tmp /tmp/app-tmp/.npm
mkdir -p /tmp/nginx-temp/client_temp /tmp/nginx-temp/proxy_temp /tmp/nginx-temp/fastcgi_temp /tmp/nginx-temp/uwsgi_temp /tmp/nginx-temp/scgi_temp
mkdir -p /tmp/nginx-logs /tmp/supervisor-logs /tmp/run

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
export CONFIG_FILE="/tmp/app/config.yaml"
export EXTERNAL_HOST="https://1abe8483-9db5-4f7b-a457-787c98ad6593-dev.e1-us-east-azure.choreoapis.dev"

# Start nginx in background (runs on port 8080, doesn't need root)
nginx -g "daemon off;" &
NGINX_PID=$!

# Start the auth proxy on port 8081
echo "Starting auth proxy..."
cd /tmp/app && ./openmcpauthproxy --demo --debug &
PROXY_PID=$!

# Wait for either process to exit
wait $NGINX_PID $PROXY_PID