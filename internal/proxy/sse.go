package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
	
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/logging"
	"github.com/wso2/open-mcp-auth-proxy/internal/util"
)

// HandleSSE sets up a go-routine to wait for context cancellation
// and flushes the response if possible.
func HandleSSE(w http.ResponseWriter, r *http.Request, rp *httputil.ReverseProxy) {
	ctx := r.Context()
	done := make(chan struct{})

	go func() {
		<-ctx.Done()
		logger.Info("SSE connection closed from %s (path: %s)", r.RemoteAddr, r.URL.Path)
		close(done)
	}()

	rp.ServeHTTP(w, r)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	<-done
}

// NewShutdownContext is a little helper to gracefully shut down
func NewShutdownContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// sseTransport is a custom http.RoundTripper that intercepts and modifies SSE responses
type sseTransport struct {
	Transport  http.RoundTripper
	proxyHost  string
	targetHost string
	config     *config.Config
}

func (t *sseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Call the underlying transport
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	
	// Check if this is an SSE response
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		return resp, nil
	}
	
	logger.Info("Intercepting SSE response to modify endpoint events")
	
	// Determine the actual proxy host to use (considering EXTERNAL_HOST)
	actualProxyHost := util.GetExternalHost(t.proxyHost)
	
	// Create a response wrapper that modifies the response body
	originalBody := resp.Body
	pr, pw := io.Pipe()
	
	go func() {
		defer originalBody.Close()
		defer func() {
			if err := pw.Close(); err != nil {
				logger.Debug("Error closing pipe writer: %v", err)
			}
		}()
		
		scanner := bufio.NewScanner(originalBody)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		
		for scanner.Scan() {
			line := scanner.Text()
			
			// Check if this line contains an endpoint event
			if strings.HasPrefix(line, "event: endpoint") {
				// Read the data line
				if scanner.Scan() {
					dataLine := scanner.Text()
					if strings.HasPrefix(dataLine, "data: ") {
						// Extract the endpoint URL
						endpoint := strings.TrimPrefix(dataLine, "data: ")
						
						// Rewrite the endpoint to use proxy paths
						modifiedEndpoint := t.rewriteEndpoint(endpoint, actualProxyHost)
						
						// Write the modified event lines
						if _, err := fmt.Fprintln(pw, line); err != nil {
							logger.Error("Error writing event line: %v", err)
							return
						}
						if _, err := fmt.Fprintln(pw, "data: "+modifiedEndpoint); err != nil {
							logger.Error("Error writing data line: %v", err)
							return
						}
						continue
					}
				}
			}
			
			// Write the original line for non-endpoint events
			if _, err := fmt.Fprintln(pw, line); err != nil {
				logger.Error("Error writing line: %v", err)
				return
			}
		}
		
		if err := scanner.Err(); err != nil {
			logger.Error("Error reading SSE stream: %v", err)
		}
	}()
	
	// Replace the response body with our modified pipe
	resp.Body = pr
	return resp, nil
}

// rewriteEndpoint rewrites endpoint URLs to use generic proxy paths
func (t *sseTransport) rewriteEndpoint(endpoint, proxyHost string) string {
	// Add nil check for config
	if t.config == nil {
		logger.Error("Config is nil in sseTransport")
		return endpoint
	}
	
	// If the endpoint is already a full URL, parse and modify it
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		// For full URLs, replace the host part
		if strings.Contains(endpoint, t.targetHost) {
			return strings.Replace(endpoint, t.targetHost, proxyHost, 1)
		}
		return endpoint
	}
	
	// For relative URLs from MCP server, rewrite to generic proxy paths
	// Extract just the query parameters if any
	var queryParams string
	if idx := strings.Index(endpoint, "?"); idx != -1 {
		queryParams = endpoint[idx:]
	}
	
	// Map remote MCP server paths to configured proxy paths
	// SSE "endpoint" events contain the message endpoint URL
	proxyPath := t.config.Paths.Messages
	
	// Determine the protocol based on the proxy host
	protocol := "http" // Default to HTTP for localhost
	cleanProxyHost := proxyHost
	
	// Check if proxyHost already includes protocol
	if strings.HasPrefix(proxyHost, "http://") {
		protocol = "http"
		cleanProxyHost = strings.TrimPrefix(proxyHost, "http://")
	} else if strings.HasPrefix(proxyHost, "https://") {
		protocol = "https"
		cleanProxyHost = strings.TrimPrefix(proxyHost, "https://")
	} else {
		// For bare hostnames, determine protocol based on the host
		if strings.HasPrefix(cleanProxyHost, "localhost") || strings.HasPrefix(cleanProxyHost, "127.0.0.1") {
			protocol = "http" // Use HTTP for localhost
		} else {
			protocol = "https" // Use HTTPS for external hosts
		}
	}
	
	// Remove trailing slash from host to avoid double slashes
	cleanProxyHost = strings.TrimSuffix(cleanProxyHost, "/")
	
	// Construct the full proxy URL with configured path
	result := fmt.Sprintf("%s://%s%s%s", protocol, cleanProxyHost, proxyPath, queryParams)
	logger.Debug("Endpoint rewrite: %s -> %s", endpoint, result)
	return result
}
