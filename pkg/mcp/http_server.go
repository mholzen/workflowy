package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mholzen/workflowy/pkg/workflowy"
)

// HTTPConfig extends Config with HTTP-specific settings.
type HTTPConfig struct {
	Config

	// Addr is the address to listen on (e.g., ":8080" or "localhost:8080").
	Addr string

	// BaseURL is the canonical URL of this server (used for OAuth resource indicator).
	// If empty, defaults to http://localhost:{port}
	BaseURL string

	// TLSCertFile is the path to the TLS certificate file (for HTTPS).
	TLSCertFile string

	// TLSKeyFile is the path to the TLS key file (for HTTPS).
	TLSKeyFile string

	// OAuth configuration for authentication.
	OAuth *OAuthConfig

	// EndpointPath is the path for the MCP endpoint (default: "/mcp").
	EndpointPath string

	// EnableCORS enables CORS headers for browser-based clients.
	EnableCORS bool

	// AllowedOrigins is a list of allowed CORS origins (if EnableCORS is true).
	// If empty, allows all origins.
	AllowedOrigins []string
}

// RunHTTPServer starts the MCP server over streamable HTTP transport.
func RunHTTPServer(ctx context.Context, cfg HTTPConfig) error {
	expose := cfg.Expose
	if expose == "" {
		expose = "read"
	}

	toolsToEnable, err := ParseExposeList(expose)
	if err != nil {
		return err
	}

	option, err := workflowy.ResolveAPIKey(cfg.APIKeyFile, cfg.DefaultAPIKeyFile)
	if err != nil {
		return fmt.Errorf("cannot load API key: %w", err)
	}

	client := workflowy.NewWorkflowyClient(option)

	// Resolve write-root-id if provided
	writeRootID := cfg.WriteRootID
	if workflowy.IsWriteRestricted(writeRootID) {
		resolvedID, err := workflowy.ResolveNodeIDToUUID(ctx, client, writeRootID)
		if err != nil {
			return fmt.Errorf("cannot resolve write-root-id: %w", err)
		}
		writeRootID = resolvedID
		slog.Info("write restrictions enabled", "write_root_id", writeRootID)
	}

	builder := NewToolBuilder(client, writeRootID)
	serverTools, err := builder.BuildTools(toolsToEnable)
	if err != nil {
		return err
	}

	hooks := &mcpserver.Hooks{}
	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcptypes.MCPMethod, message any) {
		msgJSON, _ := json.Marshal(message)
		slog.Debug("mcp request", "id", id, "method", method, "message", string(msgJSON))
	})
	hooks.AddOnSuccess(func(ctx context.Context, id any, method mcptypes.MCPMethod, message any, result any) {
		resultJSON, _ := json.Marshal(result)
		slog.Debug("mcp success", "id", id, "method", method, "result", string(resultJSON))
	})
	hooks.AddOnError(func(ctx context.Context, id any, method mcptypes.MCPMethod, message any, err error) {
		slog.Debug("mcp error", "id", id, "method", method, "error", err)
	})

	mcpServer := mcpserver.NewMCPServer(
		"workflowy",
		cfg.Version,
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithLogging(),
		mcpserver.WithHooks(hooks),
	)

	for _, tool := range serverTools {
		mcpServer.AddTool(tool.Tool, tool.Handler)
	}

	// Set up endpoint path
	endpointPath := cfg.EndpointPath
	if endpointPath == "" {
		endpointPath = "/mcp"
	}

	// Create the streamable HTTP server
	httpServer := mcpserver.NewStreamableHTTPServer(
		mcpServer,
		mcpserver.WithEndpointPath(endpointPath),
		mcpserver.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			// Pass through any OAuth claims from middleware
			if claims := TokenClaimsFromContext(r.Context()); claims != nil {
				return contextWithTokenClaims(ctx, claims)
			}
			return ctx
		}),
	)

	// Set up HTTP mux
	mux := http.NewServeMux()

	// Add OAuth protected resource metadata endpoint if OAuth is configured
	if cfg.OAuth != nil {
		wellKnownPath := "/.well-known/oauth-protected-resource"
		mux.HandleFunc(wellKnownPath, ProtectedResourceMetadataHandler(*cfg.OAuth))
		slog.Info("OAuth protected resource metadata enabled", "path", wellKnownPath)
	}

	// Add MCP endpoint with optional OAuth middleware
	var handler http.Handler = httpServer
	if cfg.OAuth != nil {
		handler = OAuthMiddleware(*cfg.OAuth)(httpServer)
	}

	// Add CORS middleware if enabled
	if cfg.EnableCORS {
		handler = corsMiddleware(cfg.AllowedOrigins)(handler)
	}

	mux.Handle(endpointPath, handler)

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // No timeout for SSE streaming
		IdleTimeout:  120 * time.Second,
	}

	// Log server configuration
	baseURL := cfg.BaseURL
	if baseURL == "" {
		protocol := "http"
		if cfg.TLSCertFile != "" {
			protocol = "https"
		}
		baseURL = fmt.Sprintf("%s://localhost%s", protocol, cfg.Addr)
	}
	slog.Info("starting MCP HTTP server",
		"addr", cfg.Addr,
		"endpoint", endpointPath,
		"base_url", baseURL,
		"tls", cfg.TLSCertFile != "",
		"oauth", cfg.OAuth != nil,
	)

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		slog.Info("shutting down MCP HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("error shutting down server", "error", err)
		}
	}()

	// Start the server
	var serverErr error
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		serverErr = server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
	} else {
		serverErr = server.ListenAndServe()
	}

	if serverErr != nil && serverErr != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", serverErr)
	}

	return nil
}

// corsMiddleware adds CORS headers to responses.
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := len(allowedOrigins) == 0 // Allow all if empty
			for _, o := range allowedOrigins {
				if o == origin || o == "*" {
					allowed = true
					break
				}
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Session-Id")
				w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
