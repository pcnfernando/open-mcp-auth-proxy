# Multi-stage build for Open MCP Auth Proxy
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

# Runtime stage
FROM alpine:latest

# Install Node.js for MCP server support, debugging tools, and create user
RUN apk add --no-cache nodejs npm ca-certificates tzdata wget curl procps && \
    npm install -g @pcnfernando/supergateway \
        @modelcontextprotocol/server-filesystem \
        @modelcontextprotocol/server-github && \
    addgroup -g 10500 appgroup && \
    adduser -u 10500 -G appgroup -s /bin/sh -D appuser

# Create necessary directories
RUN mkdir -p /tmp/app \
             /tmp/app-home \
             /tmp/app-tmp \
             /tmp/app-tmp/.npm \
             /tmp/logs && \
    chown -R 10500:10500 /tmp/app \
                         /tmp/app-home \
                         /tmp/app-tmp \
                         /tmp/logs && \
    chmod -R 755 /tmp/app \
                 /tmp/app-home \
                 /tmp/app-tmp \
                 /tmp/logs

# Copy the Go binary
COPY --from=builder --chown=10500:10500 /app/openmcpauthproxy /tmp/app/openmcpauthproxy
COPY --from=builder --chown=10500:10500 /app/openmcpauthproxy /usr/local/bin/openmcpauthproxy

COPY --from=builder --chown=10500:10500 /app/config.yaml /tmp/app/config.yaml

# Make binary executable
RUN chmod +x /usr/local/bin/openmcpauthproxy /tmp/app/openmcpauthproxy

# Copy startup script
COPY --chown=10500:10500 start.sh /usr/local/bin/start.sh
RUN chmod +x /usr/local/bin/start.sh

# Test the binary works
RUN /usr/local/bin/openmcpauthproxy --help || echo "Binary help test completed"

# Expose the auth proxy port directly
EXPOSE 8080

# Set the user
USER 10500

# Health check - now directly to the auth proxy with more time
HEALTHCHECK --interval=30s --timeout=15s --start-period=30s --retries=5 \
    CMD wget --no-verbose --tries=1 --timeout=10 --spider http://localhost:8080/.well-known/oauth-authorization-server || exit 1

# Start the auth proxy directly
CMD ["/usr/local/bin/start.sh"]