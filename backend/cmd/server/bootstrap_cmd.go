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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/bootstrap"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// bootstrapSubcommand is the first positional argument that selects the in-process
// bootstrap one-shot instead of starting the long-running server.
const bootstrapSubcommand = "bootstrap"

// isBootstrapInvocation reports whether the process was started as the bootstrap
// one-shot (e.g. `thunderid bootstrap --admin-username ...`).
func isBootstrapInvocation() bool {
	return flag.Arg(0) == bootstrapSubcommand
}

// runBootstrap parses the bootstrap subcommand options, runs the in-process
// orchestrator, and tears down the shared resources. It does not start an HTTP
// listener. It returns an error describing why bootstrap failed, if it did, so the
// caller can log the reason before exiting.
func runBootstrap(ctx context.Context, logger *log.Logger, serverHome string,
	importSvc importer.ImportServiceInterface, cacheManager cache.CacheManagerInterface) error {
	opts, err := parseBootstrapOptions(serverHome, flag.Args()[1:])
	if err != nil {
		shutdownBootstrap(ctx, logger, cacheManager)
		return err
	}
	err = bootstrap.Run(ctx, importSvc, opts)
	if err == nil {
		printBootstrapSummary()
	}
	shutdownBootstrap(ctx, logger, cacheManager)
	return err
}

// printBootstrapSummary prints the admin credentials and role created by the bootstrap.
func printBootstrapSummary() {
	fmt.Println()
	fmt.Println("✅ Default resources setup completed successfully!")
	fmt.Println()
	fmt.Println("👤 Admin credentials:")
	fmt.Printf("   Username: %s\n", os.Getenv("ADMIN_USERNAME"))
	fmt.Printf("   Password: %s\n", os.Getenv("ADMIN_PASSWORD"))
	fmt.Println("   Role: Administrator (system permission via Administrators group)")
	fmt.Println()
}

// parseBootstrapOptions parses the bootstrap subcommand flags and exports the admin
// credentials and public URL to the environment, so the bundle's
// `{{ .ADMIN_USERNAME }}` / `{{ .ADMIN_PASSWORD }}` / `{{ .PUBLIC_URL }}` placeholders
// resolve at import time. Flags override the environment. The admin username defaults
// to "admin". The admin password has no default and no generation logic here: this
// subcommand never invents security material on its own, so callers (setup.sh/setup.ps1
// generate one if needed) must supply a non-empty ADMIN_PASSWORD, or bootstrap fails.
func parseBootstrapOptions(serverHome string, args []string) (bootstrap.Options, error) {
	fs := flag.NewFlagSet(bootstrapSubcommand, flag.ContinueOnError)
	adminUsername := fs.String("admin-username", "", "Username for the default admin user")
	adminPassword := fs.String("admin-password", "", "Password for the default admin user")
	consoleRedirectURIs := fs.String("console-redirect-uris", "",
		"Comma-separated extra redirect URIs for the Console application")
	defaultsDir := fs.String("defaults", "", "Path to the bootstrap resource definitions directory")
	// Flags are best-effort; unknown flags must not abort bootstrap.
	_ = fs.Parse(args)

	setEnv("ADMIN_USERNAME", firstNonEmpty(*adminUsername, os.Getenv("ADMIN_USERNAME"), "admin"))

	password := firstNonEmpty(*adminPassword, os.Getenv("ADMIN_PASSWORD"))
	if password == "" {
		return bootstrap.Options{}, fmt.Errorf(
			"no admin password supplied: set --admin-password or ADMIN_PASSWORD")
	}
	setEnv("ADMIN_PASSWORD", password)

	setEnv("PUBLIC_URL", firstNonEmpty(os.Getenv("PUBLIC_URL"),
		config.GetServerURL(&config.GetServerRuntime().Config.Server)))
	// The bundle ranges over CONSOLE_REDIRECT_URIS, which buildArrayFromEnvVars
	// reconstructs from the indexed CONSOLE_REDIRECT_URIS_0, _1, ... variables.
	idx := 0
	for _, uri := range strings.Split(*consoleRedirectURIs, ",") {
		if uri = strings.TrimSpace(uri); uri != "" {
			setEnv(fmt.Sprintf("CONSOLE_REDIRECT_URIS_%d", idx), uri)
			idx++
		}
	}

	dir := *defaultsDir
	if dir == "" {
		dir = path.Join(serverHome, "bootstrap")
	}
	return bootstrap.Options{DefaultsDir: dir}, nil
}

// setEnv sets an environment variable, ignoring the (practically impossible) error
// for a fixed, valid key.
func setEnv(key, value string) {
	_ = os.Setenv(key, value)
}

// shutdownBootstrap releases the shared resources used by the bootstrap one-shot.
func shutdownBootstrap(ctx context.Context, logger *log.Logger, cacheManager cache.CacheManagerInterface) {
	unregisterServices()

	if err := dbprovider.GetDBProviderCloser().Close(); err != nil {
		logger.Error(ctx, "Error closing database connections", log.Error(err))
	}

	if cacheManager != nil {
		cacheManager.Close()
	}
}

// firstNonEmpty returns the first non-empty string from the provided values.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
