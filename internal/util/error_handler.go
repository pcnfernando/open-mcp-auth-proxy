package util

import (
	"strings"

	logger "github.com/wso2/open-mcp-auth-proxy/internal/logging"
)

// ValidationErrorType represents different types of validation errors
type ValidationErrorType int

const (
	BaseURLValidationError ValidationErrorType = iota
	StdioConfigError
	GeneralConfigError
)

// ErrorHandler handles common error scenarios with consistent messaging
type ErrorHandler struct{}

// NewErrorHandler creates a new error handler instance
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{}
}

// IsValidationError checks if the error message contains validation-related keywords
func (eh *ErrorHandler) IsValidationError(errMsg string) bool {
	validationKeywords := []string{
		"BaseURL validation failed",
		"must point to a local address",
		"stdio transport mode",
		"stdio.enabled must be true",
		"stdio.user_command is required",
		"validation failed",
	}

	for _, keyword := range validationKeywords {
		if containsString(errMsg, keyword) {
			return true
		}
	}
	return false
}

// GetValidationErrorType determines the specific type of validation error
func (eh *ErrorHandler) GetValidationErrorType(errMsg string) ValidationErrorType {
	if containsString(errMsg, "BaseURL validation failed") ||
		containsString(errMsg, "must point to a local address") {
		return BaseURLValidationError
	}

	if containsString(errMsg, "stdio") {
		return StdioConfigError
	}

	return GeneralConfigError
}

// LogConfigValidationError logs detailed validation error messages with helpful guidance
func (eh *ErrorHandler) LogConfigValidationError(err error, context string) {
	errMsg := err.Error()
	logger.Error("Configuration validation failed: %v", err)
	logger.Error("")

	errorType := eh.GetValidationErrorType(errMsg)

	switch errorType {
	case BaseURLValidationError:
		eh.logBaseURLValidationHelp(context)
	case StdioConfigError:
		eh.logStdioConfigHelp(context)
	default:
		eh.logGeneralConfigHelp()
	}
}

// logBaseURLValidationHelp provides specific help for BaseURL validation errors
func (eh *ErrorHandler) logBaseURLValidationHelp(context string) {
	logger.Error("📍 BaseURL Configuration Issue:")
	logger.Error("")
	logger.Error("When using stdio transport mode, BaseURL must point to a local address for security.")
	logger.Error("")
	logger.Error("✅ Valid local addresses:")
	logger.Error("   • localhost")
	logger.Error("   • 127.x.x.x (any IP in 127.0.0.0/8 range)")
	logger.Error("   • ::1 (IPv6 localhost)")
	logger.Error("   • 0.0.0.0 (any interface)")
	logger.Error("")
	logger.Error("✅ Example valid BaseURLs:")
	logger.Error("   • http://localhost:8000")
	logger.Error("   • http://127.0.0.1:8000")
	logger.Error("   • http://0.0.0.0:8000")
	logger.Error("   • https://localhost:8443")
	logger.Error("")
	logger.Error("❌ Invalid for stdio mode:")
	logger.Error("   • http://example.com:8000 (remote domain)")
	logger.Error("   • http://192.168.1.100:8000 (private network)")
	logger.Error("   • http://api.service.com (external service)")
	logger.Error("")

	if context == "stdio_flag" {
		logger.Error("💡 Solutions:")
		logger.Error("   1. Update your config.yaml BaseURL to use localhost")
		logger.Error("   2. Remove the --stdio flag to use SSE mode for remote servers")
		logger.Error("   3. Use transport_mode: \"sse\" in config for remote connections")
	} else {
		logger.Error("💡 Solutions:")
		logger.Error("   1. Change BaseURL to localhost in your config.yaml")
		logger.Error("   2. Use transport_mode: \"sse\" for remote MCP servers")
		logger.Error("   3. Ensure MCP server is running locally if using stdio mode")
	}
}

// logStdioConfigHelp provides help for stdio configuration errors
func (eh *ErrorHandler) logStdioConfigHelp(context string) {
	logger.Error("📍 Stdio Configuration Issue:")
	logger.Error("")
	logger.Error("For stdio transport mode, the following are required:")
	logger.Error("   • stdio.enabled: true")
	logger.Error("   • stdio.user_command: \"<command to run MCP server>\"")
	logger.Error("   • BaseURL must point to localhost")
	logger.Error("")
	logger.Error("✅ Example stdio configuration:")
	logger.Error("   transport_mode: \"stdio\"")
	logger.Error("   base_url: \"http://localhost:8000\"")
	logger.Error("   stdio:")
	logger.Error("     enabled: true")
	logger.Error("     user_command: \"npx @modelcontextprotocol/server-filesystem\"")
	logger.Error("")
	logger.Error("💡 Alternatives:")
	logger.Error("   • Use transport_mode: \"sse\" for remote servers")
	logger.Error("   • Use --demo flag for quick testing")
}

// logGeneralConfigHelp provides general configuration guidance
func (eh *ErrorHandler) logGeneralConfigHelp() {
	logger.Error("📍 General Configuration Help:")
	logger.Error("")
	logger.Error("Common configuration issues:")
	logger.Error("   • Missing required fields in config.yaml")
	logger.Error("   • Invalid YAML syntax")
	logger.Error("   • Mismatched transport mode settings")
	logger.Error("")
	logger.Error("💡 Quick fixes:")
	logger.Error("   • Validate your YAML syntax")
	logger.Error("   • Check the example config in the repository")
	logger.Error("   • Use --demo flag for testing")
}

// LogStartupError logs startup-related errors with helpful context
func (eh *ErrorHandler) LogStartupError(err error, component string) {
	logger.Error("Failed to start %s: %v", component, err)
	logger.Error("")

	switch component {
	case "subprocess":
		logger.Error("💡 Subprocess startup help:")
		logger.Error("   • Ensure Node.js and npm/npx are installed")
		logger.Error("   • Check that the MCP server command is valid")
		logger.Error("   • Verify network connectivity if downloading packages")
		logger.Error("   • Try running the command manually first")
	case "jwks":
		logger.Error("💡 JWKS fetch help:")
		logger.Error("   • Check network connectivity")
		logger.Error("   • Verify the JWKS URL is correct")
		logger.Error("   • Ensure the identity provider is accessible")
		logger.Error("   • Check firewall and proxy settings")
	case "server":
		logger.Error("💡 Server startup help:")
		logger.Error("   • Check if the port is already in use")
		logger.Error("   • Verify sufficient permissions")
		logger.Error("   • Check network interface configuration")
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
