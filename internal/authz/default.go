package authz

import (
	"encoding/json"
	"net/http"

	"github.com/wso2/open-mcp-auth-proxy/internal/config"
	logger "github.com/wso2/open-mcp-auth-proxy/internal/logging"
	"github.com/wso2/open-mcp-auth-proxy/internal/util"
)

type defaultProvider struct {
	cfg *config.Config
}

// NewDefaultProvider initializes a Provider for default OAuth providers.
func NewDefaultProvider(cfg *config.Config) Provider {
	return &defaultProvider{cfg: cfg}
}

func (p *defaultProvider) WellKnownHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check if we have a custom response configuration
		if p.cfg.Default.Path != nil {
			pathConfig, exists := p.cfg.Default.Path["/.well-known/oauth-authorization-server"]
			if exists && pathConfig.Response != nil {
				// Use configured response values
				responseConfig := pathConfig.Response

				// Get current base URL using EXTERNAL_HOST priority
				baseURL := util.GetExternalBaseURL(r)
				logger.Info("Using base URL for OAuth endpoints: %s", baseURL)

				authorizationEndpoint := responseConfig.AuthorizationEndpoint
				if authorizationEndpoint == "" {
					authorizationEndpoint = baseURL + "/authorize"
				}
				tokenEndpoint := responseConfig.TokenEndpoint
				if tokenEndpoint == "" {
					tokenEndpoint = baseURL + "/token"
				}
				registrationEndpoint := responseConfig.RegistrationEndpoint
				if registrationEndpoint == "" {
					registrationEndpoint = baseURL + "/register"
				}

				// Build response from config
				response := map[string]interface{}{
					"issuer":                                responseConfig.Issuer,
					"authorization_endpoint":                authorizationEndpoint,
					"token_endpoint":                        tokenEndpoint,
					"jwks_uri":                              responseConfig.JwksURI,
					"response_types_supported":              responseConfig.ResponseTypesSupported,
					"grant_types_supported":                 responseConfig.GrantTypesSupported,
					"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
					"registration_endpoint":                 registrationEndpoint,
					"code_challenge_methods_supported":      responseConfig.CodeChallengeMethodsSupported,
				}

				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(response); err != nil {
					logger.Error("Error encoding well-known response: %v", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
				return
			}
		}
	}
}

func (p *defaultProvider) RegisterHandler() http.HandlerFunc {
	return nil
}
