package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "valid bearer token",
			header:   "Bearer abc123",
			expected: "abc123",
		},
		{
			name:     "bearer lowercase",
			header:   "bearer abc123",
			expected: "abc123",
		},
		{
			name:     "bearer mixed case",
			header:   "BeArEr abc123",
			expected: "abc123",
		},
		{
			name:     "empty header",
			header:   "",
			expected: "",
		},
		{
			name:     "no bearer prefix",
			header:   "Basic abc123",
			expected: "",
		},
		{
			name:     "bearer with extra spaces",
			header:   "Bearer   token-with-spaces   ",
			expected: "token-with-spaces",
		},
		{
			name:     "just bearer no token",
			header:   "Bearer",
			expected: "",
		},
		{
			name:     "bearer with one space no token",
			header:   "Bearer ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			result := extractBearerToken(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTokenClaims_HasScope(t *testing.T) {
	claims := &TokenClaims{
		Scopes: []string{"read", "write", "admin"},
	}

	assert.True(t, claims.HasScope("read"))
	assert.True(t, claims.HasScope("write"))
	assert.True(t, claims.HasScope("admin"))
	assert.False(t, claims.HasScope("delete"))
	assert.False(t, claims.HasScope(""))
}

func TestTokenClaimsFromContext(t *testing.T) {
	t.Run("no claims in context", func(t *testing.T) {
		ctx := context.Background()
		claims := TokenClaimsFromContext(ctx)
		assert.Nil(t, claims)
	})

	t.Run("claims in context", func(t *testing.T) {
		expected := &TokenClaims{
			Subject: "user123",
			Scopes:  []string{"read"},
		}
		ctx := contextWithTokenClaims(context.Background(), expected)
		claims := TokenClaimsFromContext(ctx)
		assert.Equal(t, expected, claims)
	})
}

func TestStaticTokenValidator(t *testing.T) {
	validator := &StaticTokenValidator{
		Tokens: map[string]*TokenClaims{
			"valid-token": {
				Subject: "user123",
				Scopes:  []string{"read", "write"},
			},
		},
	}

	t.Run("valid token", func(t *testing.T) {
		claims, err := validator.ValidateToken(context.Background(), "valid-token")
		require.NoError(t, err)
		assert.Equal(t, "user123", claims.Subject)
		assert.Equal(t, []string{"read", "write"}, claims.Scopes)
	})

	t.Run("invalid token", func(t *testing.T) {
		claims, err := validator.ValidateToken(context.Background(), "invalid-token")
		assert.Error(t, err)
		assert.Nil(t, claims)
	})
}

func TestOAuthMiddleware(t *testing.T) {
	validator := &StaticTokenValidator{
		Tokens: map[string]*TokenClaims{
			"valid-token": {
				Subject: "user123",
				Scopes:  []string{"read"},
			},
		},
	}

	t.Run("valid token passes through", func(t *testing.T) {
		config := OAuthConfig{
			TokenValidator: validator,
			RequireAuth:    true,
			Resource:       "https://example.com",
		}

		var capturedClaims *TokenClaims
		handler := OAuthMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedClaims = TokenClaimsFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, capturedClaims)
		assert.Equal(t, "user123", capturedClaims.Subject)
	})

	t.Run("missing token returns 401 when required", func(t *testing.T) {
		config := OAuthConfig{
			TokenValidator: validator,
			RequireAuth:    true,
			Resource:       "https://example.com",
		}

		handler := OAuthMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Header().Get("WWW-Authenticate"), "Bearer")
		assert.Contains(t, rr.Header().Get("WWW-Authenticate"), "resource_metadata")
	})

	t.Run("missing token allowed when not required", func(t *testing.T) {
		config := OAuthConfig{
			TokenValidator: validator,
			RequireAuth:    false,
		}

		var capturedClaims *TokenClaims
		handler := OAuthMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedClaims = TokenClaimsFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Nil(t, capturedClaims)
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		config := OAuthConfig{
			TokenValidator: validator,
			RequireAuth:    true,
			Resource:       "https://example.com",
		}

		handler := OAuthMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Header().Get("WWW-Authenticate"), "invalid_token")
	})
}

func TestProtectedResourceMetadataHandler(t *testing.T) {
	config := OAuthConfig{
		AuthorizationServers: []string{"https://auth.example.com"},
		Resource:             "https://mcp.example.com",
		ResourceName:         "Test MCP Server",
		Scopes:               []string{"mcp.read", "mcp.write"},
	}

	handler := ProtectedResourceMetadataHandler(config)

	t.Run("returns metadata on GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

		var metadata ProtectedResourceMetadata
		err := json.NewDecoder(rr.Body).Decode(&metadata)
		require.NoError(t, err)

		assert.Equal(t, "https://mcp.example.com", metadata.Resource)
		assert.Equal(t, []string{"https://auth.example.com"}, metadata.AuthorizationServers)
		assert.Equal(t, "Test MCP Server", metadata.ResourceName)
		assert.Equal(t, []string{"mcp.read", "mcp.write"}, metadata.ScopesSupported)
		assert.Contains(t, metadata.BearerMethodsSupported, "header")
	})

	t.Run("rejects non-GET methods", func(t *testing.T) {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			req := httptest.NewRequest(method, "/.well-known/oauth-protected-resource", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		}
	})
}

func TestSimpleTokenValidator(t *testing.T) {
	validator := &SimpleTokenValidator{
		ValidateFn: func(ctx context.Context, token string) (*TokenClaims, error) {
			if token == "valid" {
				return &TokenClaims{Subject: "test-user"}, nil
			}
			return nil, assert.AnError
		},
	}

	t.Run("valid token", func(t *testing.T) {
		claims, err := validator.ValidateToken(context.Background(), "valid")
		require.NoError(t, err)
		assert.Equal(t, "test-user", claims.Subject)
	})

	t.Run("invalid token", func(t *testing.T) {
		claims, err := validator.ValidateToken(context.Background(), "invalid")
		assert.Error(t, err)
		assert.Nil(t, claims)
	})
}
