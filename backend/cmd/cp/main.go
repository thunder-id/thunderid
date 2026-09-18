// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Command cp runs ThunderID as a control plane.
//
// A control plane serves the management APIs and nothing else. It holds no runtime traffic: no
// logins, no token issuance, no flow execution, no credential issuance. Those belong to a data
// plane, and this binary does not contain them, which is the point of there being two binaries
// rather than one with a setting. What decides it is which wiring this main calls, so a runtime
// surface cannot be reached here by configuration, only by editing this file.
package main

import (
	"context"
	"fmt"
	"mime"
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
	"github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/mcp"
	"github.com/thunder-id/thunderid/internal/system/security"

	appserver "github.com/thunder-id/thunderid/internal/server"
)

func main() {
	startupStartedAt := time.Now()

	ctx := context.Background()
	logger := log.GetLogger()

	serverHome := appserver.ServerHome(ctx, logger)

	cfg := appserver.LoadConfiguration(ctx, logger, serverHome)
	if cfg == nil {
		logger.Fatal(ctx, "Failed to initialize configurations")
	}

	// Apply the configured log level from deployment.yaml, now that config is loaded.
	if cfg.Log.Level != "" {
		if err := logger.SetLevel(cfg.Log.Level); err != nil {
			logger.Fatal(ctx, "Invalid log level in configuration", log.Error(err))
		}
	}

	// Apply the configured log output (console and/or rotating file), now that the
	// server home is known and the file path can be resolved.
	if err := logger.Configure(cfg.Log.BuildOutputOptions(serverHome)); err != nil {
		logger.Fatal(ctx, "Failed to configure log output", log.Error(err))
	}

	// Initialize the cache manager.
	cacheManager := cache.Initialize(cfg.Cache, cfg.Server.Identifier)

	// Initialize system permission strings before any service or middleware uses them.
	security.InitSystemPermissions(cfg.Server.SecurityConfig.SystemPermissionPrefix)

	// Create a new HTTP multiplexer.
	mux := http.NewServeMux()
	if mux == nil {
		logger.Fatal(ctx, "Failed to initialize multiplexer")
	}

	// The shared build, and only the shared build. There is no call to WireRuntime here, so this
	// binary mounts no runtime route and links no runtime package.
	svcs := appserver.BuildManagement(mux, cacheManager)

	// The management APIs accept tokens rather than issuing them, so the revocation cache is still
	// needed: a token this plane accepts can have been revoked by the plane that issued it.
	revocationEnforcer, revocationSyncer := appserver.InitRevocationCache(ctx, logger, cfg)
	revocationSyncer.Start(ctx)

	// Mount the MCP server's routes now that the revocation enforcer exists — DefaultGuard uses it
	// to authenticate MCP requests with the same verification and revocation logic as the REST gate.
	mcpGuard, mcpResourceMeta := mcp.DefaultGuard(svcs.JWTService, revocationEnforcer)
	mcp.Initialize(mux, svcs.MCPServer, mcpGuard, mcpResourceMeta)

	registerConsole(ctx, logger, mux, serverHome)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create the HTTP server.
	server := appserver.NewHTTPServer(ctx, logger, cfg, mux, svcs.JWTService, revocationEnforcer)
	var ln net.Listener
	if cfg.Server.HTTPOnly {
		logger.Info(ctx, "TLS is not enabled, starting server without TLS")
		ln = appserver.NewListener(ctx, logger, server)
	} else {
		tlsConfigProvider, ok := svcs.RuntimeCryptoSvc.(common.TLSConfigProvider)
		if !ok {
			logger.Fatal(ctx, "Runtime crypto provider does not support TLS material retrieval")
		}
		tlsConfig := appserver.LoadCertConfig(ctx, logger, tlsConfigProvider)
		ln = appserver.NewTLSListener(ctx, logger, server, tlsConfig)
	}

	serverURL := config.GetServerURL(&cfg.Server)
	consoleURL := fmt.Sprintf("%s/console", strings.TrimSuffix(serverURL, "/"))
	logger.Info(ctx, "ThunderID Control Plane URL", log.String("url", serverURL))
	logger.Info(ctx, "ThunderID Console URL", log.String("url", consoleURL))

	go func() {
		startupDuration := time.Since(startupStartedAt)
		logger.Info(ctx, "ThunderID Control Plane started",
			log.String("startup_time", startupDuration.String()))
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx, "Failed to serve requests", log.Error(err))
		}
	}()

	<-sigChan
	appserver.GracefulShutdown(ctx, logger, server, cacheManager, revocationSyncer)
}

// registerConsole serves the Console application.
//
// Only the Console. Gate is the login UI, and a control plane runs no login, so shipping it here
// would offer an interface for something this binary cannot do.
func registerConsole(ctx context.Context, logger *log.Logger, mux *http.ServeMux, serverHome string) {
	// Override the OS-level MIME mapping so .js/.mjs files are served as
	// application/javascript. Most proxies (Envoy, NGINX, Cloudflare) only
	// compress application/javascript in their default allowlists, not
	// text/javascript, which is Go's default on some systems.
	_ = mime.AddExtensionType(".js", "application/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".mjs", "application/javascript; charset=utf-8")

	consoleDir := path.Join(serverHome, "apps", "console")
	handler, err := appserver.StaticFileHandler("/console/", consoleDir, logger)
	if err != nil {
		logger.Warn(ctx, "Console application not registered",
			log.String("directory", consoleDir), log.Error(err))
		return
	}
	logger.Debug(ctx, "Registering static file handler for Console application",
		log.String("path", "/console/"), log.String("directory", consoleDir))
	mux.Handle("/console/", handler)
}
