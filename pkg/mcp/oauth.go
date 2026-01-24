package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// OAuthConfig holds the OAuth configuration for the MCP server.
// The MCP server acts as a Resource Server per RFC 9728 - it validates tokens
// but delegates authentication to an external Authorization Server.
type OAuthConfig struct {
	// AuthorizationServers is a list of authorization server URLs that can issue tokens.
	// Per RFC 9728, this is returned in the Protected Resource Metadata.
	AuthorizationServers []string

	// Resource is the canonical URI of this MCP server (resource indicator per RFC 8707).
	// This should match the URL clients use to access this server.
	Resource string

	// ResourceName is an optional human-readable name for this resource.
	ResourceName string

	// TokenValidator validates bearer tokens and returns claims.
	// If nil, tokens are not validated (useful for testing or delegated validation).
	TokenValidator TokenValidator

	// RequireAuth when true requires valid tokens for all requests.
	// When false, unauthenticated requests are allowed but won't have user context.
	RequireAuth bool

	// Scopes lists the scopes this resource server accepts.
	Scopes []string
}

// TokenValidator is an interface for validating OAuth tokens.
type TokenValidator interface {
	// ValidateToken validates a bearer token and returns claims if valid.
	// Returns an error if the token is invalid, expired, or cannot be validated.
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
}

// TokenClaims holds the validated claims from a token.
type TokenClaims struct {
	// Subject is the user/principal identifier (sub claim).
	Subject string

	// Scopes are the granted scopes from the token.
	Scopes []string

	// Issuer is the token issuer (iss claim).
	Issuer string

	// Audience is the intended audience (aud claim).
	Audience []string

	// ExpiresAt is the token expiration time as Unix timestamp.
	ExpiresAt int64

	// Extra holds additional claims from the token.
	Extra map[string]any
}

// HasScope checks if the claims include a specific scope.
func (c *TokenClaims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// ProtectedResourceMetadata is the response for /.well-known/oauth-protected-resource
// as defined in RFC 9728.
type ProtectedResourceMetadata struct {
	// Resource is the canonical URI of this resource server.
	Resource string `json:"resource"`

	// AuthorizationServers is a list of authorization server URLs.
	AuthorizationServers []string `json:"authorization_servers"`

	// ResourceName is an optional human-readable name.
	ResourceName string `json:"resource_name,omitempty"`

	// ScopesSupported lists the scopes this resource accepts.
	ScopesSupported []string `json:"scopes_supported,omitempty"`

	// BearerMethodsSupported indicates how tokens can be passed.
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
}

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	// tokenClaimsKey is the context key for storing validated token claims.
	tokenClaimsKey contextKey = "oauth_token_claims"
)

// TokenClaimsFromContext retrieves validated token claims from the context.
// Returns nil if no claims are present (unauthenticated request).
func TokenClaimsFromContext(ctx context.Context) *TokenClaims {
	claims, _ := ctx.Value(tokenClaimsKey).(*TokenClaims)
	return claims
}

// contextWithTokenClaims adds token claims to a context.
func contextWithTokenClaims(ctx context.Context, claims *TokenClaims) context.Context {
	return context.WithValue(ctx, tokenClaimsKey, claims)
}

// OAuthMiddleware creates HTTP middleware that handles OAuth authentication.
// It validates bearer tokens and returns 401 with WWW-Authenticate header
// when authentication is required but not provided.
func OAuthMiddleware(config OAuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Extract bearer token from Authorization header
			token := extractBearerToken(r)

			// If we have a token and a validator, validate it
			if token != "" && config.TokenValidator != nil {
				claims, err := config.TokenValidator.ValidateToken(ctx, token)
				if err != nil {
					slog.Debug("token validation failed", "error", err)
					writeUnauthorized(w, config, "invalid_token", "Token validation failed")
					return
				}
				// Add claims to context for downstream handlers
				ctx = contextWithTokenClaims(ctx, claims)
				r = r.WithContext(ctx)
			} else if config.RequireAuth && token == "" {
				writeUnauthorized(w, config, "", "Bearer token required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractBearerToken extracts a bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	// Check for "Bearer " prefix (case-insensitive)
	if len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
		return strings.TrimSpace(auth[7:])
	}

	return ""
}

// writeUnauthorized writes a 401 response with appropriate WWW-Authenticate header.
func writeUnauthorized(w http.ResponseWriter, config OAuthConfig, errorCode, errorDesc string) {
	// Build WWW-Authenticate header per RFC 6750
	wwwAuth := "Bearer"

	// Add resource_metadata parameter pointing to our PRM endpoint
	if config.Resource != "" {
		wwwAuth += fmt.Sprintf(` resource_metadata="%s/.well-known/oauth-protected-resource"`, config.Resource)
	}

	if errorCode != "" {
		wwwAuth += fmt.Sprintf(`, error="%s"`, errorCode)
	}
	if errorDesc != "" {
		wwwAuth += fmt.Sprintf(`, error_description="%s"`, errorDesc)
	}

	w.Header().Set("WWW-Authenticate", wwwAuth)
	w.WriteHeader(http.StatusUnauthorized)
}

// ProtectedResourceMetadataHandler returns an HTTP handler for the
// /.well-known/oauth-protected-resource endpoint per RFC 9728.
func ProtectedResourceMetadataHandler(config OAuthConfig) http.HandlerFunc {
	metadata := ProtectedResourceMetadata{
		Resource:               config.Resource,
		AuthorizationServers:   config.AuthorizationServers,
		ResourceName:           config.ResourceName,
		ScopesSupported:        config.Scopes,
		BearerMethodsSupported: []string{"header"},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")

		if err := json.NewEncoder(w).Encode(metadata); err != nil {
			slog.Error("failed to encode protected resource metadata", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// SimpleTokenValidator is a basic token validator that uses a callback function.
// This is useful for integrating with external token validation services.
type SimpleTokenValidator struct {
	ValidateFn func(ctx context.Context, token string) (*TokenClaims, error)
}

// ValidateToken implements TokenValidator.
func (v *SimpleTokenValidator) ValidateToken(ctx context.Context, token string) (*TokenClaims, error) {
	return v.ValidateFn(ctx, token)
}

// StaticTokenValidator validates tokens against a static map of token->claims.
// This is primarily useful for testing and development.
type StaticTokenValidator struct {
	Tokens map[string]*TokenClaims
}

// ValidateToken implements TokenValidator.
func (v *StaticTokenValidator) ValidateToken(ctx context.Context, token string) (*TokenClaims, error) {
	claims, ok := v.Tokens[token]
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
