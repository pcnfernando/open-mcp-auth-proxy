package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Transport mode for MCP server
type TransportMode string

const (
	SSETransport   TransportMode = "sse"
	StdioTransport TransportMode = "stdio"
)

// AsgardeoConfig groups all Asgardeo-specific fields
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
	AuthServerBaseURL  string
	MCPServerBaseURL   string       `yaml:"mcp_server_base_url"`
	ListenPort         int          `yaml:"listen_port"`
	JWKSURL            string
	TimeoutSeconds     int               `yaml:"timeout_seconds"`
	MCPPaths           []string          `yaml:"mcp_paths"`
	PathMapping        map[string]string `yaml:"path_mapping"`
	Mode               string            `yaml:"mode"`
	CORSConfig         CORSConfig        `yaml:"cors"`
	TransportMode      TransportMode     `yaml:"transport_mode"`

	// Nested config for Asgardeo
	Demo     DemoConfig     `yaml:"demo"`
	Asgardeo AsgardeoConfig `yaml:"asgardeo"`
	Default  DefaultConfig  `yaml:"default"`
	Command  Command        `yaml:"command"` // Command to run
}

// Command struct with explicit configuration for all relevant paths
type Command struct {
	Enabled     bool     `yaml:"enabled"`
	UserCommand string   `yaml:"user_command"` // Only the part provided by the user
	BaseUrl     string   `yaml:"base_url"`     // Base URL for the MCP server
	Port        int      `yaml:"port"`         // Port for the MCP server
	SsePath     string   `yaml:"sse_path"`     // SSE endpoint path
	MessagePath string   `yaml:"message_path"` // Messages endpoint path
	WorkDir     string   `yaml:"work_dir"`     // Working directory
	Args        []string `yaml:"args,omitempty"` // Additional arguments
	Env         []string `yaml:"env,omitempty"`  // Environment variables
}

// Validate checks if the command config is valid based on transport mode
func (c *Command) Validate(transportMode TransportMode) error {
	if transportMode == StdioTransport {
		if !c.Enabled {
			return fmt.Errorf("command must be enabled in stdio transport mode")
		}
		if c.UserCommand == "" {
			return fmt.Errorf("user_command is required in stdio transport mode")
		}
	}
	return nil
}

// GetBaseURL returns the base URL for the MCP server
func (c *Command) GetBaseURL() string {
	if c.BaseUrl != "" {
		return c.BaseUrl
	}
	if c.Port > 0 {
		return fmt.Sprintf("http://localhost:%d", c.Port)
	}
	return "http://localhost:8000" // default
}

// GetPaths returns the SSE and message paths
func (c *Command) GetPaths() []string {
	var paths []string
	
	// Add SSE path
	ssePath := c.SsePath
	if ssePath == "" {
		ssePath = "/sse" // default
	}
	paths = append(paths, ssePath)
	
	// Add message path
	messagePath := c.MessagePath
	if messagePath == "" {
		messagePath = "/messages" // default
	}
	paths = append(paths, messagePath)
	
	return paths
}

// BuildExecCommand constructs the full command string for execution
func (c *Command) BuildExecCommand() string {
	if c.UserCommand == "" {
		return ""
	}

	// Apply defaults if not specified
	port := c.Port
	if port == 0 {
		port = 8000
	}

	baseUrl := c.BaseUrl
	if baseUrl == "" {
		baseUrl = fmt.Sprintf("http://localhost:%d", port)
	}

	ssePath := c.SsePath
	if ssePath == "" {
		ssePath = "/sse"
	}

	messagePath := c.MessagePath
	if messagePath == "" {
		messagePath = "/messages"
	}

	// Construct the full command
	return fmt.Sprintf(
		`npx -y supergateway --stdio "%s" --port %d --baseUrl %s --ssePath %s --messagePath %s`,
		c.UserCommand, port, baseUrl, ssePath, messagePath,
	)
}

// GetExec returns the complete command string for execution
func (c *Command) GetExec() string {
	if c.UserCommand == "" {
		return ""
	}
	return c.BuildExecCommand()
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
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 15 // default
	}
	
	// Set default transport mode if not specified
	if cfg.TransportMode == "" {
		cfg.TransportMode = SSETransport // Default to SSE
	}
	
	// Validate command config based on transport mode
	if err := cfg.Command.Validate(cfg.TransportMode); err != nil {
		return nil, err
	}
	
	// In stdio mode, use command.base_url for MCPServerBaseURL if it's not explicitly set
	if cfg.TransportMode == StdioTransport && cfg.MCPServerBaseURL == "" {
		cfg.MCPServerBaseURL = cfg.Command.GetBaseURL()
	} else if cfg.TransportMode == SSETransport && cfg.MCPServerBaseURL == "" {
		return nil, fmt.Errorf("mcp_server_base_url is required in SSE transport mode")
	}
	
	// In stdio mode, set the MCPPaths from the command configuration
	if cfg.TransportMode == StdioTransport && cfg.Command.Enabled {
		// Override MCPPaths with paths from command configuration
		cfg.MCPPaths = cfg.Command.GetPaths()
	} else if cfg.TransportMode == SSETransport && len(cfg.MCPPaths) == 0 {
		return nil, fmt.Errorf("mcp_paths are required in SSE transport mode")
	}
	
	return &cfg, nil
}