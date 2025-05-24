#!/bin/sh
# Create nginx temp directories with proper permissions
mkdir -p /tmp/nginx-temp/client_temp /tmp/nginx-temp/proxy_temp /tmp/nginx-temp/fastcgi_temp /tmp/nginx-temp/uwsgi_temp /tmp/nginx-temp/scgi_temp

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
cd /tmp/app && ./openmcpauthproxy --demo &
PROXY_PID=$!

# Wait for either process to exit
wait $NGINX_PID $PROXY_PID