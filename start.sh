#!/bin/sh
# Create all necessary directories first
mkdir -p /tmp/app /tmp/app-home /tmp/app-tmp /tmp/app-tmp/.npm
mkdir -p /tmp/nginx-temp/client_temp /tmp/nginx-temp/proxy_temp /tmp/nginx-temp/fastcgi_temp /tmp/nginx-temp/uwsgi_temp /tmp/nginx-temp/scgi_temp
mkdir -p /tmp/nginx-logs /tmp/supervisor-logs /tmp/run

# Ensure binary and config are available in /tmp/app
if [ ! -f /tmp/app/openmcpauthproxy ]; then
    echo "Copying auth proxy binary to /tmp/app/"
    if [ -f /usr/local/bin/openmcpauthproxy ]; then
        cp /usr/local/bin/openmcpauthproxy /tmp/app/
    else
        echo "Error: openmcpauthproxy binary not found in expected locations!"
        exit 1
    fi
fi

if [ ! -f /tmp/app/config.yaml ]; then
    echo "Copying config to /tmp/app/"
    if [ -f /tmp/config.yaml ]; then
        cp /tmp/config.yaml /tmp/app/
    else
        echo "Error: config.yaml not found!"
        exit 1
    fi
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
cd /tmp/app && ./openmcpauthproxy --demo &
PROXY_PID=$!

# Wait for either process to exit
wait $NGINX_PID $PROXY_PID