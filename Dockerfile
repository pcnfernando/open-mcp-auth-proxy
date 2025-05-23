# Multi-stage build for Open MCP Auth Proxy
# Build stage
FROM golang:1.21-alpine AS builder

# Install git for go mod download
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o openmcpauthproxy \
    ./cmd/proxy

# Runtime stage
FROM node:20-alpine

# Install ca-certificates, create non-root user, and install global npm packages
RUN apk --no-cache add ca-certificates tzdata wget && \
    addgroup -g 10014 appgroup && \
    adduser -u 10014 -G appgroup -s /bin/sh -D appuser && \
    npm install -g supergateway \
        @modelcontextprotocol/server-filesystem \
        @modelcontextprotocol/server-github

# Create necessary directories with proper permissions
RUN mkdir -p /app /tmp/app-tmp /tmp/app-tmp/.npm && \
    chown -R 10014:10014 /app /tmp/app-tmp && \
    chmod -R 755 /app

# Copy the binary from builder stage
COPY --from=builder --chown=10014:10014 /app/openmcpauthproxy /app/openmcpauthproxy

# Copy config file to working directory (app expects config.yaml in current dir)
COPY --chown=10014:10014 config.yaml /app/config.yaml

# Set environment variables
ENV HOME=/tmp/app-tmp \
    TMPDIR=/tmp/app-tmp \
    PATH="/app:${PATH}" \
    NODE_PATH=/usr/local/lib/node_modules \
    NPM_CONFIG_CACHE=/tmp/app-tmp/.npm \
    EXTERNAL_HOST="" \
    PUBLIC_HOST="" \
    ADVERTISED_HOST="" \
    INGRESS_HOST="" \
    CHOREO_APP_URL=""

# Switch to non-root user
USER 10014:10014

# Set working directory
WORKDIR /app

# Expose the default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/.well-known/oauth-authorization-server || exit 1

# Default command - can be overridden with different flags
CMD ["./openmcpauthproxy", "--demo"]