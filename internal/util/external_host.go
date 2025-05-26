package util

import (
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/logging"
)

var (
	globalConfig *config.Config
	configMutex  sync.RWMutex
)

// SetGlobalConfig sets the global config instance for utility functions to use
func SetGlobalConfig(cfg *config.Config) {
	configMutex.Lock()
	defer configMutex.Unlock()
	globalConfig = cfg
}

// getGlobalConfig safely retrieves the global config
func getGlobalConfig() *config.Config {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalConfig
}

// GetExternalBaseURL determines the external URL that clients use to reach this service
// Priority: EXTERNAL_HOST env var > config.external_host > X-Forwarded headers > request headers
func GetExternalBaseURL(r *http.Request) string {
	cfg := getGlobalConfig()
	
	// Get external host with environment variable taking precedence over config
	var externalHost string
	if cfg != nil {
		externalHost = cfg.GetExternalHost()
	} else {
		// Fallback to environment variable only if no config is available
		externalHost = os.Getenv("EXTERNAL_HOST")
	}
	
	if externalHost != "" {
		logger.Debug("Using external host from config/env: %s", externalHost)
		
		// If external host already includes protocol, use as-is
		if strings.HasPrefix(externalHost, "http://") || strings.HasPrefix(externalHost, "https://") {
			return strings.TrimSuffix(externalHost, "/")
		}
		
		// Otherwise, default to HTTPS for external hosts
		return "https://" + externalHost
	}

	// Fallback: determine from request headers
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	// Check for forwarded protocol headers (from load balancers/proxies)
	if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = forwardedProto
		logger.Debug("Using X-Forwarded-Proto: %s", scheme)
	}
	if forwardedProto := r.Header.Get("X-Forwarded-Scheme"); forwardedProto != "" {
		scheme = forwardedProto
		logger.Debug("Using X-Forwarded-Scheme: %s", scheme)
	}

	// Determine the host
	host := r.Host

	// Check for forwarded host headers (from load balancers/proxies)
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
		logger.Debug("Using X-Forwarded-Host: %s", host)
	}
	if originalHost := r.Header.Get("X-Original-Host"); originalHost != "" {
		host = originalHost
		logger.Debug("Using X-Original-Host: %s", host)
	}

	baseURL := scheme + "://" + host
	logger.Debug("Constructed base URL from request: %s", baseURL)
	return baseURL
}

// GetExternalHost returns just the host part (without protocol) for use in SSE endpoint rewriting
// Priority: EXTERNAL_HOST env var > config.external_host > request host
func GetExternalHost(requestHost string) string {
	cfg := getGlobalConfig()
	
	// Get external host with environment variable taking precedence over config
	var externalHost string
	if cfg != nil {
		externalHost = cfg.GetExternalHost()
	} else {
		// Fallback to environment variable only if no config is available
		externalHost = os.Getenv("EXTERNAL_HOST")
	}
	
	if externalHost != "" {
		logger.Debug("Using external host from config/env: %s", externalHost)
		
		// Remove protocol if included - we just want the host part
		if strings.HasPrefix(externalHost, "http://") {
			externalHost = strings.TrimPrefix(externalHost, "http://")
		}
		if strings.HasPrefix(externalHost, "https://") {
			externalHost = strings.TrimPrefix(externalHost, "https://")
		}
		
		// Remove trailing slash if present
		externalHost = strings.TrimSuffix(externalHost, "/")
		
		return externalHost
	}

	// Fallback to request host
	logger.Debug("Using request host for host resolution: %s", requestHost)
	return requestHost
}