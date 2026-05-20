/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package main is the entry point for starting the server.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pkiservice"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// shutdownTimeout defines the timeout duration for graceful shutdown.
const shutdownTimeout = 5 * time.Second

var (
	netListen = net.Listen
	tlsListen = tls.Listen
)

func main() {
	startupStartedAt := time.Now()
	logger := log.GetLogger()

	serverHome := getThunderHome(logger)

	cfg := initThunderConfigurations(logger, serverHome)
	if cfg == nil {
		logger.Fatal("Failed to initialize configurations")
	}

	// Install the CORS allowed-origins matcher used by the HTTP middleware.
	// Compilation errors are already surfaced by config validation; this call
	// rebuilds the rules and installs them as the cors package singleton.
	if err := cors.InitializeMatcher(cfg.CORS.AllowedOrigins); err != nil {
		logger.Fatal("Failed to initialize CORS matcher", log.Error(err))
	}

	// Initialize the cache manager.
	cacheManager := cache.Initialize()

	// Initialize system permission strings before any service or middleware uses them.
	security.InitSystemPermissions(cfg.Resource.SystemResourceServer.Handle)

	// Create a new HTTP multiplexer.
	mux := http.NewServeMux()
	if mux == nil {
		logger.Fatal("Failed to initialize multiplexer")
	}

	// Register the services.
	jwtService := registerServices(mux, cacheManager)

	// Register static file handlers for frontend applications.
	registerStaticFileHandlers(logger, mux, serverHome)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create the HTTP server.
	server := createHTTPServer(logger, cfg, mux, jwtService)
	var ln net.Listener
	if cfg.Server.HTTPOnly {
		logger.Info("TLS is not enabled, starting server without TLS")
		ln = createListener(logger, server)
	} else {
		tlsConfig := loadCertConfig(logger, cfg, serverHome)
		ln = createTLSListener(logger, server, tlsConfig)
	}

	serverURL := config.GetServerURL(&cfg.Server)
	consoleURL := fmt.Sprintf("%s/console", strings.TrimSuffix(serverURL, "/"))
	logger.Info("ThunderID Server URL", log.String("url", serverURL))
	logger.Info("ThunderID Console URL", log.String("url", consoleURL))

	// Start server in a goroutine
	go func() {
		startupDuration := time.Since(startupStartedAt)
		logger.Info("ThunderID Server started", log.String("startup_time", startupDuration.String()))
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to serve requests", log.Error(err))
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutting down server...")
	gracefulShutdown(logger, server, cacheManager)
}

// getThunderHome retrieves and return the home directory.
func getThunderHome(logger *log.Logger) string {
	// Parse project directory from command line arguments.
	projectHome := ""
	projectHomeFlag := flag.String("serverHome", "", "Path to ThunderID home directory")
	flag.Parse()

	if *projectHomeFlag != "" {
		logger.Info("Using serverHome from command line argument", log.String("serverHome", *projectHomeFlag))
		projectHome = *projectHomeFlag
	} else {
		// If no command line argument is provided, use the current working directory.
		dir, dirErr := os.Getwd()
		if dirErr != nil {
			logger.Fatal("Failed to get current working directory", log.Error(dirErr))
		}
		projectHome = dir
	}

	return projectHome
}

// initThunderConfigurations initializes the configurations.
func initThunderConfigurations(logger *log.Logger, serverHome string) *config.Config {
	// Load the configurations.
	configFilePath := path.Join(serverHome, "repository/conf/deployment.yaml")
	defaultConfigPath := path.Join(serverHome, "repository/resources/conf/default.json")
	cfg, err := config.LoadConfig(configFilePath, defaultConfigPath, serverHome)
	if err != nil {
		logger.Fatal("Failed to load configurations", log.Error(err))
	}

	// Initialize runtime configurations.
	if err := config.InitializeServerRuntime(serverHome, cfg); err != nil {
		logger.Fatal("Failed to initialize server runtime", log.Error(err))
	}

	return cfg
}

// loadCertConfig loads the certificate configuration and extracts the Key ID (kid).
func loadCertConfig(logger *log.Logger, cfg *config.Config, serverHome string) *tls.Config {
	// Build full paths for certificate and key files
	certFilePath := path.Join(serverHome, cfg.TLS.CertFile)
	keyFilePath := path.Join(serverHome, cfg.TLS.KeyFile)

	// Load TLS configuration
	tlsConfig, err := pkiservice.LoadTLSConfig(cfg, certFilePath, keyFilePath)
	if err != nil {
		logger.Fatal("Failed to load TLS configuration", log.Error(err))
	}
	return tlsConfig
}

// createHTTPServer creates and configures an HTTP server with common settings.
func createHTTPServer(logger *log.Logger, cfg *config.Config, mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface) *http.Server {
	securityMiddleware := createSecurityMiddleware(logger, mux, jwtService)

	// Build the middleware chain with proper execution order.
	// Request flow: CorrelationID (outermost) -> AccessLog -> Security -> Route Handler (innermost)
	// Note: Middlewares are wrapped in reverse order - the last added will execute first.
	handler := log.AccessLogHandler(logger, securityMiddleware)
	handler = middleware.CorrelationIDMiddleware(handler)

	// Build the server address using hostname and port from the configurations.
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Hostname, cfg.Server.Port)

	server := &http.Server{
		Addr:              serverAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // Mitigate Slowloris attacks
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return server
}

// createListener creates and returns a listener for the HTTP server.
func createListener(logger *log.Logger, server *http.Server) net.Listener {
	ln, err := netListen("tcp", server.Addr)
	if err != nil {
		logger.Fatal("Failed to start HTTP listener", log.Error(err))
	}
	return ln
}

// createTLSListener creates and returns a TLS listener for the HTTPS server.
func createTLSListener(logger *log.Logger, server *http.Server, tlsConfig *tls.Config) net.Listener {
	ln, err := tlsListen("tcp", server.Addr, tlsConfig)
	if err != nil {
		logger.Fatal("Failed to start TLS listener", log.Error(err))
	}
	return ln
}

func createSecurityMiddleware(logger *log.Logger, mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface) http.Handler {
	middlewareFunc, err := security.Initialize(jwtService)
	if err != nil {
		logger.Fatal("Failed to initialize security middleware", log.Error(err))
	}
	return middlewareFunc(mux)
}

// gracefulShutdown handles the graceful shutdown of all components.
func gracefulShutdown(
	logger *log.Logger,
	server *http.Server,
	cacheManager cache.CacheManagerInterface,
) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Error during server shutdown", log.Error(err))
	} else {
		logger.Debug("HTTP server shutdown completed")
	}

	// Shutdown services
	unregisterServices()

	// Close database connections
	dbCloser := provider.GetDBProviderCloser()
	if err := dbCloser.Close(); err != nil {
		logger.Error("Error closing database connections", log.Error(err))
	} else {
		logger.Debug("Database connections closed successfully")
	}

	if cacheManager != nil {
		cacheManager.Close()
		logger.Debug("Cache manager closed successfully")
	}

	logger.Info("Server shutdown completed")
}

// registerStaticFileHandlers registers static file handlers for frontend applications.
func registerStaticFileHandlers(logger *log.Logger, mux *http.ServeMux, serverHome string) {
	// Serve gate application from /gate
	gateDir := path.Join(serverHome, "apps", "gate")
	if directoryExists(gateDir) {
		logger.Debug("Registering static file handler for Gate application",
			log.String("path", "/gate/"), log.String("directory", gateDir))
		mux.Handle("/gate/", createStaticFileHandler("/gate/", gateDir, logger))
	} else {
		logger.Warn("Gate application directory not found", log.String("directory", gateDir))
	}

	// Serve console application from /console
	consoleDir := path.Join(serverHome, "apps", "console")
	if directoryExists(consoleDir) {
		logger.Debug("Registering static file handler for Console application",
			log.String("path", "/console/"), log.String("directory", consoleDir))
		mux.Handle("/console/", createStaticFileHandler("/console/", consoleDir, logger))
	} else {
		logger.Warn("Console application directory not found", log.String("directory", consoleDir))
	}
}

// createStaticFileHandler creates a handler for serving static files with SPA fallback.
func createStaticFileHandler(routePrefix, directory string, logger *log.Logger) http.Handler {
	fileServer := http.FileServer(http.Dir(directory))

	return http.StripPrefix(routePrefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle root path "/" by explicitly serving index.html
		if r.URL.Path == "/" || r.URL.Path == "" {
			indexPath := path.Join(directory, "index.html")
			if fileExists(indexPath) {
				// Set no-cache headers for index.html
				w.Header().Set(constants.CacheControlHeaderName, constants.CacheControlNoCacheComposite)
				w.Header().Set(constants.PragmaHeaderName, constants.PragmaNoCache)
				w.Header().Set(constants.ExpiresHeaderName, constants.ExpiresZero)
				http.ServeFile(w, r, indexPath)
				return
			}
		}

		// Get the file path
		filePath := path.Join(directory, r.URL.Path)

		// Check if the requested file is index.html
		isIndexHTML := r.URL.Path == "/index.html" || path.Base(filePath) == "index.html"

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			// For SPA routing, serve index.html for non-existent files
			indexPath := path.Join(directory, "index.html")
			if fileExists(indexPath) {
				logger.Debug("Serving index.html for SPA routing",
					log.String("requested_path", r.URL.Path),
					log.String("route_prefix", routePrefix))
				// Set no-cache headers for index.html
				w.Header().Set(constants.CacheControlHeaderName, constants.CacheControlNoCacheComposite)
				w.Header().Set(constants.PragmaHeaderName, constants.PragmaNoCache)
				w.Header().Set(constants.ExpiresHeaderName, constants.ExpiresZero)
				http.ServeFile(w, r, indexPath)
				return
			}
		}

		// Serve index.html directly with no-cache headers when requested
		if isIndexHTML {
			indexPath := path.Join(directory, "index.html")
			if fileExists(indexPath) {
				// Set no-cache headers for index.html
				w.Header().Set(constants.CacheControlHeaderName, constants.CacheControlNoCacheComposite)
				w.Header().Set(constants.PragmaHeaderName, constants.PragmaNoCache)
				w.Header().Set(constants.ExpiresHeaderName, constants.ExpiresZero)
				http.ServeFile(w, r, indexPath)
				return
			}
		}

		// Serve the requested file or directory listing
		fileServer.ServeHTTP(w, r)
	}))
}

// directoryExists checks if a directory exists.
func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
