package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime"
	"strings"

	"gopkg.in/yaml.v2"
)

// Transport mode for MCP server
type TransportMode string

const (
	SSETransport   TransportMode = "sse"
	StdioTransport TransportMode = "stdio"
)

// Common path configuration for all transport modes
type PathsConfig struct {
	SSE      string `yaml:"sse"`
	Messages string `yaml:"messages"`
}

// StdioConfig contains stdio-specific configuration
type StdioConfig struct {
	Enabled     bool     `yaml:"enabled"`
	UserCommand string   `yaml:"user_command"`   // The command provided by the user
	WorkDir     string   `yaml:"work_dir"`       // Working directory (optional)
	Args        []string `yaml:"args,omitempty"` // Additional arguments
	Env         []string `yaml:"env,omitempty"`  // Environment variables
}

type DemoConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	OrgName      string `yaml:"org_name"`
}

type AsgardeoConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	OrgName      string `yaml:"org_name"`
}

type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
}

type ParamConfig struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type ResponseConfig struct {
	Issuer                        string   `yaml:"issuer,omitempty"`
	JwksURI                       string   `yaml:"jwks_uri,omitempty"`
	AuthorizationEndpoint         string   `yaml:"authorization_endpoint,omitempty"`
	TokenEndpoint                 string   `yaml:"token_endpoint,omitempty"`
	RegistrationEndpoint          string   `yaml:"registration_endpoint,omitempty"`
	ResponseTypesSupported        []string `yaml:"response_types_supported,omitempty"`
	GrantTypesSupported           []string `yaml:"grant_types_supported,omitempty"`
	CodeChallengeMethodsSupported []string `yaml:"code_challenge_methods_supported,omitempty"`
}

type PathConfig struct {
	// For well-known endpoint
	Response *ResponseConfig `yaml:"response,omitempty"`

	// For authorization endpoint
	AddQueryParams []ParamConfig `yaml:"addQueryParams,omitempty"`

	// For token and register endpoints
	AddBodyParams []ParamConfig `yaml:"addBodyParams,omitempty"`
}

type DefaultConfig struct {
	BaseURL string                `yaml:"base_url,omitempty"`
	Path    map[string]PathConfig `yaml:"path,omitempty"`
	JWKSURL string                `yaml:"jwks_url,omitempty"`
}

type Config struct {
	AuthServerBaseURL string
	ListenPort        int    `yaml:"listen_port"`
	BaseURL           string `yaml:"base_url"`
	Port              int    `yaml:"port"`
	ExternalHost      string `yaml:"external_host"`
	JWKSURL           string
	TimeoutSeconds    int               `yaml:"timeout_seconds"`
	PathMapping       map[string]string `yaml:"path_mapping"`
	Mode              string            `yaml:"mode"`
	CORSConfig        CORSConfig        `yaml:"cors"`
	TransportMode     TransportMode     `yaml:"transport_mode"`
	Paths             PathsConfig       `yaml:"paths"`
	Stdio             StdioConfig       `yaml:"stdio"`

	// Nested config for Asgardeo
	Demo     DemoConfig     `yaml:"demo"`
	Asgardeo AsgardeoConfig `yaml:"asgardeo"`
	Default  DefaultConfig  `yaml:"default"`
}

// GetExternalHost returns the external host with environment variable taking precedence
func (c *Config) GetExternalHost() string {
	// Environment variable takes precedence
	if envHost := os.Getenv("EXTERNAL_HOST"); envHost != "" {
		return envHost
	}
	// Fallback to config file value
	return c.ExternalHost
}

// validateLocalURL ensures that the provided URL points to a local/localhost address
// and not to a remote host for security reasons when using stdio transport mode
func validateLocalURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}

	// Extract hostname and port
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must contain a hostname")
	}

	// Check if it's a local address
	if !isLocalAddress(hostname) {
		return fmt.Errorf("BaseURL must point to a local address (localhost, 127.x.x.x, ::1, 0.0.0.0) when using stdio transport mode, got: %s", hostname)
	}

	return nil
}

// isLocalAddress checks if the given hostname/IP is a local address
func isLocalAddress(hostname string) bool {
	// Convert to lowercase for case-insensitive comparison
	hostname = strings.ToLower(hostname)

	// Check for common localhost names
	localHostnames := []string{
		"localhost",
		"localhost.localdomain",
		"local",
	}

	for _, local := range localHostnames {
		if hostname == local {
			return true
		}
	}

	// Parse as IP address
	ip := net.ParseIP(hostname)
	if ip != nil {
		return isLocalIP(ip)
	}

	// If it's not a valid IP, it might be a hostname that resolves to local
	// For security, we'll be strict and only allow explicit local addresses
	return false
}

// isLocalIP checks if the given IP address is a local/loopback address
func isLocalIP(ip net.IP) bool {
	// Check for IPv4 loopback (127.x.x.x)
	if ip.IsLoopback() {
		return true
	}

	// Check for IPv6 loopback (::1)
	if ip.Equal(net.IPv6loopback) {
		return true
	}

	// Check for "any" addresses (0.0.0.0 for IPv4, :: for IPv6)
	if ip.IsUnspecified() {
		return true
	}

	// Check for IPv4 localhost range (127.0.0.0/8)
	if ip.To4() != nil {
		// 127.0.0.0/8 network
		if ip.To4()[0] == 127 {
			return true
		}
	}

	// Check for IPv6 localhost
	if ip.To16() != nil {
		// Check for ::1
		if ip.Equal(net.ParseIP("::1")) {
			return true
		}
	}

	return false
}

// validateBaseURLForTransportMode validates the BaseURL based on transport mode
func validateBaseURLForTransportMode(baseURL string, transportMode TransportMode) error {
	// Only validate for stdio transport mode
	if transportMode == StdioTransport {
		return validateLocalURL(baseURL)
	}

	// For other transport modes (like SSE), allow any URL
	return nil
}

// Validate checks if the config is valid based on transport mode
func (c *Config) Validate() error {
	// Validate based on transport mode
	if c.TransportMode == StdioTransport {
		if !c.Stdio.Enabled {
			return fmt.Errorf("stdio.enabled must be true in stdio transport mode")
		}
		if c.Stdio.UserCommand == "" {
			return fmt.Errorf("stdio.user_command is required in stdio transport mode")
		}

		// Validate that BaseURL points to localhost when using stdio
		if err := validateBaseURLForTransportMode(c.BaseURL, c.TransportMode); err != nil {
			return fmt.Errorf("BaseURL validation failed: %v", err)
		}
	}

	// Validate paths
	if c.Paths.SSE == "" {
		c.Paths.SSE = "/sse" // Default value
	}
	if c.Paths.Messages == "" {
		c.Paths.Messages = "/messages" // Default value
	}

	// Validate base URL
	if c.BaseURL == "" {
		if c.Port > 0 {
			c.BaseURL = fmt.Sprintf("http://localhost:%d", c.Port)
		} else {
			c.BaseURL = "http://localhost:8000" // Default value
		}
	}

	return nil
}

// GetMCPPaths returns the list of paths that should be proxied to the MCP server
func (c *Config) GetMCPPaths() []string {
	return []string{c.Paths.SSE, c.Paths.Messages}
}

// BuildExecCommand constructs the full command string for execution in stdio mode
func (c *Config) BuildExecCommand() string {
	if c.Stdio.UserCommand == "" {
		return ""
	}

	if runtime.GOOS == "windows" {
		// For Windows, we need to properly escape the inner command
		escapedCommand := strings.ReplaceAll(c.Stdio.UserCommand, `"`, `\"`)
		return fmt.Sprintf(
			`npx -y supergateway --header X-Accel-Buffering:no --stdio "%s" --port %d --baseUrl %s --ssePath %s --messagePath %s`,
			escapedCommand, c.Port, c.BaseURL, c.Paths.SSE, c.Paths.Messages,
		)
	}

	return fmt.Sprintf(
		`npx -y supergateway --header X-Accel-Buffering:no --stdio "%s" --port %d --baseUrl %s --ssePath %s --messagePath %s`,
		c.Stdio.UserCommand, c.Port, c.BaseURL, c.Paths.SSE, c.Paths.Messages,
	)
}

// LoadConfig reads a YAML config file into Config struct.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	// Set default values
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 15 // default
	}

	// Set default transport mode if not specified
	if cfg.TransportMode == "" {
		cfg.TransportMode = SSETransport // Default to SSE
	}

	// Set default port if not specified
	if cfg.Port == 0 {
		cfg.Port = 8000 // default
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
