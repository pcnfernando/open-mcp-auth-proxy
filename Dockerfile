# Multi-stage build for Open MCP Auth Proxy with Nginx
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o openmcpauthproxy \
    ./cmd/proxy

# Runtime stage with nginx
FROM nginx:alpine

# Install Node.js and create user in the 10000-20000 range
RUN apk add --no-cache nodejs npm ca-certificates tzdata wget supervisor && \
    npm install -g supergateway \
        @modelcontextprotocol/server-filesystem \
        @modelcontextprotocol/server-github && \
    addgroup -g 10500 appgroup && \
    adduser -u 10500 -G appgroup -s /bin/sh -D appuser

# Create all necessary directories in /tmp for readonly filesystem
# Note: These will be recreated by the startup script to ensure they exist
RUN mkdir -p /tmp/app \
             /tmp/app-home \
             /tmp/app-tmp \
             /tmp/app-tmp/.npm \
             /tmp/nginx-cache \
             /tmp/nginx-temp \
             /tmp/nginx-logs \
             /tmp/supervisor-logs \
             /tmp/run && \
    chown -R 10500:10500 /tmp/app \
                         /tmp/app-home \
                         /tmp/app-tmp \
                         /tmp/nginx-cache \
                         /tmp/nginx-temp \
                         /tmp/nginx-logs \
                         /tmp/supervisor-logs \
                         /tmp/run && \
    chmod -R 755 /tmp/app \
                 /tmp/app-home \
                 /tmp/app-tmp \
                 /tmp/nginx-cache \
                 /tmp/nginx-temp \
                 /tmp/nginx-logs \
                 /tmp/supervisor-logs \
                 /tmp/run

# Copy the Go binary and config to /tmp (writable location)
COPY --from=builder --chown=10500:10500 /app/openmcpauthproxy /tmp/app/openmcpauthproxy
COPY --chown=10500:10500 config.yaml /tmp/app/config.yaml

# Update config.yaml to use port 8081 for the Go app
RUN sed -i 's/listen_port: 8080/listen_port: 8081/' /tmp/app/config.yaml

# Copy nginx configuration and startup script
COPY --chown=root:root nginx.conf /etc/nginx/nginx.conf
COPY --chown=10500:10500 start.sh /usr/local/bin/start.sh

# Make startup script executable
RUN chmod +x /usr/local/bin/start.sh

# Expose the nginx port (now 8080, not 80)
EXPOSE 8080

# Explicitly set the user to 10500 (like the reference)
USER 10500

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Start with the custom script (container starts as root, script handles user switching)
CMD ["/usr/local/bin/start.sh"]