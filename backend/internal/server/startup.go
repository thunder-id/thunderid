// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/revocationcache"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// shutdownTimeout defines the timeout duration for graceful shutdown.
const shutdownTimeout = 5 * time.Second

var (
	netListen = net.Listen
	tlsListen = tls.Listen
)

// InitRevocationCache builds the Resource Server token-revocation enforcer and its background
// syncer. The first snapshot is loaded synchronously so enforcement is live before the first
// request; if that load fails the server still starts and the syncer repopulates on its next tick.
func InitRevocationCache(ctx context.Context, logger *log.Logger,
	cfg *config.Config) (revocationcache.EnforcerInterface, revocationcache.Syncer) {
	rc := cfg.Server.SecurityConfig.TokenRevocation
	enforcer, syncer, err := revocationcache.Initialize(revocationcache.Config{
		Enabled:      rc.IsEnabled(),
		Source:       rc.Source,
		SyncInterval: time.Duration(rc.SyncIntervalSeconds) * time.Second,
	})
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize token revocation cache", log.Error(err))
	}
	return enforcer, syncer
}

// ServerHome returns the server home directory, from the -thunderHome flag when given and the
// working directory otherwise.
func ServerHome(ctx context.Context, logger *log.Logger) string {
	// Parse project directory from command line arguments.
	projectHome := ""
	projectHomeFlag := flag.String("serverHome", "", "Path to ThunderID home directory")
	flag.Parse()

	if *projectHomeFlag != "" {
		logger.Info(ctx, "Using serverHome from command line argument",
			log.String("serverHome", *projectHomeFlag))
		projectHome = *projectHomeFlag
	} else {
		// If no command line argument is provided, use the current working directory.
		dir, dirErr := os.Getwd()
		if dirErr != nil {
			logger.Fatal(ctx, "Failed to get current working directory", log.Error(dirErr))
		}
		projectHome = dir
	}

	return projectHome
}

// LoadConfiguration reads deployment.yaml from the server home and initializes the server runtime
// from it.
func LoadConfiguration(ctx context.Context, logger *log.Logger, serverHome string) *config.Config {
	// Load the configurations.
	configFilePath := path.Join(serverHome, "deployment.yaml")
	defaultConfigPath := path.Join(serverHome, "config/default.json")
	cfg, err := config.LoadConfig(configFilePath, defaultConfigPath, serverHome)
	if err != nil {
		logger.Fatal(ctx, "Failed to load configurations", log.Error(err))
	}

	// Initialize runtime configurations.
	if err := config.InitializeServerRuntime(serverHome, cfg); err != nil {
		logger.Fatal(ctx, "Failed to initialize server runtime", log.Error(err))
	}

	return cfg
}

// LoadCertConfig builds the TLS configuration a plane serves with, from the runtime crypto
// provider's key material.
func LoadCertConfig(ctx context.Context, logger *log.Logger, runtimeSvc common.TLSConfigProvider) *tls.Config {
	mat, err := runtimeSvc.GetTLSMaterial(ctx)
	if err != nil {
		logger.Fatal(ctx, "Failed to load TLS material", log.Error(err))
	}
	// #nosec G402 -- MinVersion is set to TLS 1.2 or higher by GetTLSMaterial
	return &tls.Config{
		Certificates: []tls.Certificate{mat.Certificate},
		MinVersion:   mat.MinVersion,
	}
}

// accessLogExcludePaths returns the path prefixes excluded from the access log: the always-excluded
// Gate and Console frontend paths plus any configured extras. Empty and root ("/") entries are
// dropped because they match every request and would silently disable all access logging.
func accessLogExcludePaths(configured []string) []string {
	paths := []string{"/gate/", "/console/"}
	for _, prefix := range configured {
		if prefix != "" && prefix != "/" {
			paths = append(paths, prefix)
		}
	}
	return paths
}

// NewHTTPServer creates the HTTP server, wrapping the mux in the middleware chain every plane
// shares: correlation id, deployment id, security headers, access log, then the security gate.
func NewHTTPServer(ctx context.Context, logger *log.Logger, cfg *config.Config, mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface, revocationEnforcer revocationcache.EnforcerInterface) *http.Server {
	securityMiddleware := newSecurityMiddleware(ctx, logger, mux, jwtService, revocationEnforcer)

	// Build the middleware chain with proper execution order.
	// Request flow: CorrelationID (outermost) -> DeploymentID -> SecurityHeaders -> AccessLog ->
	// Security -> Route Handler (innermost)
	// Note: Middlewares are wrapped in reverse order - the last added will execute first.
	// The Gate and Console frontend paths are always excluded from the access log to keep it
	// focused on API traffic. Additional prefixes can be excluded via log.access.exclude_paths.
	handler := log.AccessLogHandler(logger, accessLogExcludePaths(cfg.Log.Access.ExcludePaths), securityMiddleware)
	handler = middleware.SecurityHeadersMiddleware()(handler)
	// Outside the security layer, so that every request carries the deployment id it acts for by the
	// time any store is reached.
	handler = middleware.DeploymentIDMiddleware(handler)
	handler = middleware.CorrelationIDMiddleware(handler)

	// Build the server address using hostname and port from the configurations.
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Hostname, cfg.Server.Port)

	server := &http.Server{
		Addr:              serverAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // Mitigate Slowloris attacks
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          log.NewServerErrorLog(logger),
	}

	return server
}

// NewListener opens the plain TCP listener, terminating startup if the address cannot be bound.
func NewListener(ctx context.Context, logger *log.Logger, server *http.Server) net.Listener {
	ln, err := netListen("tcp", server.Addr)
	if err != nil {
		logger.Fatal(ctx, "Failed to start HTTP listener", log.Error(err))
	}
	return ln
}

// NewTLSListener opens the TLS listener, terminating startup if the address cannot be bound.
func NewTLSListener(ctx context.Context, logger *log.Logger, server *http.Server,
	tlsConfig *tls.Config) net.Listener {
	ln, err := tlsListen("tcp", server.Addr, tlsConfig)
	if err != nil {
		logger.Fatal(ctx, "Failed to start TLS listener", log.Error(err))
	}
	return ln
}
func newSecurityMiddleware(ctx context.Context, logger *log.Logger, mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface, revocationEnforcer revocationcache.EnforcerInterface) http.Handler {
	middlewareFunc, err := security.Initialize(jwtService, revocationEnforcer)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize security middleware", log.Error(err))
	}
	return middlewareFunc(mux)
}

// GracefulShutdown stops the server and the components that need winding down, giving in-flight
// requests a bounded time to finish.
func GracefulShutdown(
	ctx context.Context,
	logger *log.Logger,
	server *http.Server,
	cacheManager cache.CacheManagerInterface,
	revocationSyncer revocationcache.Syncer,
) {
	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error(ctx, "Error during server shutdown", log.Error(err))
	} else {
		logger.Debug(ctx, "HTTP server shutdown completed")
	}

	// Stop the token-revocation cache syncer.
	revocationSyncer.Stop()

	// Shutdown services
	Unregister()

	// Close database connections
	dbCloser := provider.GetDBProviderCloser()
	if err := dbCloser.Close(); err != nil {
		logger.Error(ctx, "Error closing database connections", log.Error(err))
	} else {
		logger.Debug(ctx, "Database connections closed successfully")
	}

	if cacheManager != nil {
		cacheManager.Close()
		logger.Debug(ctx, "Cache manager closed successfully")
	}

	// Close the log file writer before the final shutdown log line.
	if err := logger.Close(); err != nil {
		logger.Error(ctx, "Error closing log file", log.Error(err))
	}

	logger.Info(ctx, "Server shutdown completed")
}

// StaticFileHandler serves a frontend application from a directory, falling back to index.html so a
// single-page app can route paths the filesystem does not have.
func StaticFileHandler(routePrefix, directory string, logger *log.Logger) (http.Handler, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	rootFS := root.FS()
	fileServer := http.FileServerFS(rootFS)

	// serveIndex serves index.html with no-cache headers, substituting the request's CSP nonce
	// (see SecurityHeadersMiddleware) for the placeholder in its <meta property="csp-nonce"> tag, so
	// the frontend can apply the same nonce to its own inline <style> tags. It reports whether
	// index.html existed and was served.
	serveIndex := func(w http.ResponseWriter, r *http.Request) bool {
		content, err := fs.ReadFile(rootFS, "index.html")
		if err != nil {
			return false
		}
		nonce := sysContext.GetCSPNonce(r.Context())
		content = bytes.ReplaceAll(content, []byte(constants.CSPNoncePlaceholder), []byte(nonce))

		w.Header().Set(constants.CacheControlHeaderName, constants.CacheControlNoCacheComposite)
		w.Header().Set(constants.PragmaHeaderName, constants.PragmaNoCache)
		w.Header().Set(constants.ExpiresHeaderName, constants.ExpiresZero)
		w.Header().Set(constants.ContentTypeHeaderName, constants.ContentTypeHTML)
		_, _ = w.Write(content)
		return true
	}

	return http.StripPrefix(routePrefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle the application root by explicitly serving index.html.
		if r.URL.Path == "/" || r.URL.Path == "" {
			if serveIndex(w, r) {
				return
			}
		}

		// Resolve the request against the served directory.
		name := strings.TrimPrefix(r.URL.Path, "/")
		isIndexHTML := name == "index.html"

		if name != "" {
			if _, err := root.Stat(name); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					// For SPA routing, serve index.html for non-existent in-bounds paths.
					logger.Debug(r.Context(), "Serving index.html for SPA routing",
						log.String("requested_path", r.URL.Path),
						log.String("route_prefix", routePrefix))
					if serveIndex(w, r) {
						return
					}
				} else {
					// The path escapes the served directory (or is otherwise invalid).
					logger.Warn(r.Context(), "Rejected request with out-of-bounds path",
						log.String("requested_path", r.URL.Path),
						log.String("route_prefix", routePrefix))
					http.NotFound(w, r)
					return
				}
			}
		}

		// Serve index.html directly with no-cache headers when requested.
		if isIndexHTML {
			if serveIndex(w, r) {
				return
			}
		}

		// Serve the requested file or directory listing.
		fileServer.ServeHTTP(w, r)
	})), nil
}
