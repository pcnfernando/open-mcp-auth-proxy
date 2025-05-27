package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wso2/open-mcp-auth-proxy/internal/authz"
	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	"github.com/wso2/open-mcp-auth-proxy/internal/constants"
	"github.com/wso2/open-mcp-auth-proxy/internal/logging"
	"github.com/wso2/open-mcp-auth-proxy/internal/proxy"
	"github.com/wso2/open-mcp-auth-proxy/internal/subprocess"
	"github.com/wso2/open-mcp-auth-proxy/internal/util"
)

func main() {
	// Parse command line flags
	demoMode := flag.Bool("demo", false, "Use Asgardeo-based provider (demo).")
	asgardeoMode := flag.Bool("asgardeo", false, "Use Asgardeo-based provider (asgardeo).")
	debugMode := flag.Bool("debug", false, "Enable debug logging")
	stdioMode := flag.Bool("stdio", false, "Use stdio transport mode instead of SSE")
	flag.Parse()

	// Initialize logging
	logger.SetDebug(*debugMode)
	errorHandler := util.NewErrorHandler()

	// Load configuration
	cfg, err := loadConfiguration(errorHandler)
	if err != nil {
		os.Exit(1)
	}

	// Apply command line overrides
	if err := applyCommandLineOverrides(cfg, *stdioMode, errorHandler); err != nil {
		os.Exit(1)
	}

	// Set global config for utility functions
	util.SetGlobalConfig(cfg)

	// Log configuration summary
	logConfigurationSummary(cfg)

	// Start subprocess if needed
	procManager := startSubprocessIfNeeded(cfg)

	// Create authentication provider
	provider := createAuthProvider(cfg, *demoMode, *asgardeoMode)

	// Fetch JWKS if configured
	if err := fetchJWKSIfConfigured(cfg, errorHandler); err != nil {
		os.Exit(1)
	}

	// Start HTTP server
	srv := startHTTPServer(cfg, provider)

	// Wait for shutdown and cleanup
	waitForShutdownAndCleanup(srv, procManager)
}

// loadConfiguration loads and validates the configuration file
func loadConfiguration(errorHandler *util.ErrorHandler) (*config.Config, error) {
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config.yaml"
	}

	logger.Info("Loading config from: %s", configPath)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if errorHandler.IsValidationError(err.Error()) {
			errorHandler.LogConfigValidationError(err, "config_load")
		} else {
			logger.Error("Error loading config: %v", err)
		}
		return nil, err
	}

	logger.Info("Configuration loaded successfully")
	return cfg, nil
}

// applyCommandLineOverrides applies command line flag overrides to configuration
func applyCommandLineOverrides(cfg *config.Config, stdioMode bool, errorHandler *util.ErrorHandler) error {
	if !stdioMode {
		return nil
	}

	logger.Info("Overriding transport mode to stdio via command line flag")
	cfg.TransportMode = config.StdioTransport
	cfg.Stdio.Enabled = true

	// Re-validate configuration after changes
	if err := cfg.Validate(); err != nil {
		errorHandler.LogConfigValidationError(err, "stdio_flag")
		return err
	}

	return nil
}

// logConfigurationSummary logs key configuration details
func logConfigurationSummary(cfg *config.Config) {
	logger.Info("Using transport mode: %s", cfg.TransportMode)
	logger.Info("Using MCP server base URL: %s", cfg.BaseURL)
	logger.Info("Using MCP paths: SSE=%s, Messages=%s", cfg.Paths.SSE, cfg.Paths.Messages)

	if cfg.TransportMode == config.StdioTransport {
		logger.Info("BaseURL validation passed for stdio mode - using local address")
	}

	if externalHost := cfg.GetExternalHost(); externalHost != "" {
		logger.Info("Using external host: %s", externalHost)
	}
}

// startSubprocessIfNeeded starts a subprocess for stdio transport mode
func startSubprocessIfNeeded(cfg *config.Config) *subprocess.Manager {
	if cfg.TransportMode != config.StdioTransport || !cfg.Stdio.Enabled {
		logger.Info("Using SSE transport mode, not starting subprocess")
		return nil
	}

	logger.Info("Starting subprocess for stdio transport mode")

	// Check dependencies
	if err := subprocess.EnsureDependenciesAvailable(cfg.Stdio.UserCommand); err != nil {
		logger.Warn("%v", err)
		logger.Warn("Subprocess may fail to start due to missing dependencies")
	}

	// Start subprocess
	procManager := subprocess.NewManager()
	if err := procManager.Start(cfg); err != nil {
		util.NewErrorHandler().LogStartupError(err, "subprocess")
		// Don't exit here - continue without subprocess
	}

	return procManager
}

// createAuthProvider creates the appropriate authentication provider
func createAuthProvider(cfg *config.Config, demoMode, asgardeoMode bool) authz.Provider {
	var provider authz.Provider

	switch {
	case demoMode:
		cfg.Mode = "demo"
		cfg.AuthServerBaseURL = constants.ASGARDEO_BASE_URL + cfg.Demo.OrgName + "/oauth2"
		cfg.JWKSURL = constants.ASGARDEO_BASE_URL + cfg.Demo.OrgName + "/oauth2/jwks"
		provider = authz.NewAsgardeoProvider(cfg)
		logger.Info("Using demo mode with Asgardeo sandbox")

	case asgardeoMode:
		cfg.Mode = "asgardeo"
		cfg.AuthServerBaseURL = constants.ASGARDEO_BASE_URL + cfg.Asgardeo.OrgName + "/oauth2"
		cfg.JWKSURL = constants.ASGARDEO_BASE_URL + cfg.Asgardeo.OrgName + "/oauth2/jwks"
		provider = authz.NewAsgardeoProvider(cfg)
		logger.Info("Using Asgardeo mode with organization: %s", cfg.Asgardeo.OrgName)

	default:
		cfg.Mode = "default"
		cfg.JWKSURL = cfg.Default.JWKSURL
		cfg.AuthServerBaseURL = cfg.Default.BaseURL
		provider = authz.NewDefaultProvider(cfg)
		logger.Info("Using default provider mode")
	}

	return provider
}

// fetchJWKSIfConfigured fetches JWKS if a URL is configured
func fetchJWKSIfConfigured(cfg *config.Config, errorHandler *util.ErrorHandler) error {
	if cfg.JWKSURL == "" {
		return nil
	}

	logger.Info("Fetching JWKS from: %s", cfg.JWKSURL)
	if err := util.FetchJWKS(cfg.JWKSURL); err != nil {
		errorHandler.LogStartupError(err, "jwks")
		return err
	}

	logger.Info("JWKS fetched successfully")
	return nil
}

// startHTTPServer creates and starts the HTTP server
func startHTTPServer(cfg *config.Config, provider authz.Provider) *http.Server {
	mux := proxy.NewRouter(cfg, provider)
	listenAddress := fmt.Sprintf(":%d", cfg.ListenPort)

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: mux,
	}

	go func() {
		logger.Info("Server listening on %s", listenAddress)
		logger.Info("Auth proxy is ready to accept connections")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			util.NewErrorHandler().LogStartupError(err, "server")
			os.Exit(1)
		}
	}()

	return srv
}

// waitForShutdownAndCleanup waits for shutdown signal and performs cleanup
func waitForShutdownAndCleanup(srv *http.Server, procManager *subprocess.Manager) {
	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Shutting down...")

	// Terminate subprocess first
	if procManager != nil && procManager.IsRunning() {
		logger.Info("Terminating subprocess...")
		procManager.Shutdown()
	}

	// Shutdown HTTP server
	logger.Info("Shutting down HTTP server...")
	shutdownCtx, cancel := proxy.NewShutdownContext(5 * time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error: %v", err)
	}
	logger.Info("Stopped.")
}