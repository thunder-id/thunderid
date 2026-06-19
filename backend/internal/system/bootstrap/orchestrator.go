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

// Package bootstrap creates the default resources (organization unit, user types,
// admin user, system resource server, groups, roles, flows, the Console
// application, themes and translations) in-process at install time. It loads a
// templated YAML resource bundle and applies it through the existing import
// service, replacing the previous flow that started a temporary server with
// security disabled and seeded resources over unauthenticated HTTP.
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// Options configures a bootstrap run.
type Options struct {
	// DefaultsDir is the directory holding the resource definition bundle.
	DefaultsDir string
}

// Run creates the default resources in-process and idempotently by applying the
// YAML resource bundle through the import service.
//
// It runs under a runtime (privileged) context so that service-layer
// authorization grants the seeding operations without an authenticated subject —
// the same internal-privilege path used by flow executors. No HTTP server is
// started and no security middleware is involved. Upsert is enabled so re-running
// is safe; the run fails fast on the first resource error.
//
// Install-time values (admin credentials, public URL) are injected via the
// bundle's `{{ .ENV_VAR }}` placeholders, resolved from the environment with the
// same helper the server config and declarative resources use.
func Run(ctx context.Context, importSvc importer.ImportServiceInterface, opts Options) error {
	logger := log.GetLogger()
	ctx = security.WithRuntimeContext(ctx)

	logger.Info(ctx, "Starting in-process bootstrap of default resources",
		log.String("defaultsDir", opts.DefaultsDir))

	content, err := loadBundle(opts.DefaultsDir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		logger.Warn(ctx, "No bootstrap resource definitions found; nothing to do",
			log.String("defaultsDir", opts.DefaultsDir))
		return nil
	}

	// Resolve `{{ .ENV_VAR }}` placeholders (e.g. ADMIN_USERNAME, ADMIN_PASSWORD,
	// PUBLIC_URL) from the environment before importing.
	resolved, err := utils.SubstituteEnvironmentVariables([]byte(content))
	if err != nil {
		return fmt.Errorf("failed to resolve bootstrap template variables: %w", err)
	}

	upsert := true
	continueOnError := false
	request := &importer.ImportRequest{
		Content: string(resolved),
		Options: &importer.ImportOptions{
			Upsert:          &upsert,
			ContinueOnError: &continueOnError,
			Target:          "runtime",
		},
	}

	response, svcErr := importSvc.ImportResources(ctx, request)
	if svcErr != nil {
		return fmt.Errorf("bootstrap import failed [%s]: %s", svcErr.Code, svcErr.Error.DefaultValue)
	}

	if err := checkImportOutcome(ctx, logger, response); err != nil {
		return err
	}

	logger.Info(ctx, "In-process bootstrap completed",
		log.Int("imported", response.Summary.Imported))
	return nil
}

// checkImportOutcome returns an error if any resource document failed to import.
func checkImportOutcome(ctx context.Context, logger *log.Logger, response *importer.ImportResponse) error {
	if response == nil || response.Summary == nil {
		return fmt.Errorf("bootstrap import returned no result")
	}

	for _, result := range response.Results {
		logger.Debug(ctx, "Bootstrap resource processed",
			log.String("resourceType", result.ResourceType),
			log.String("resourceName", result.ResourceName),
			log.String("operation", result.Operation),
			log.String("status", result.Status))
	}

	if response.Summary.Failed == 0 {
		return nil
	}

	var failures []string
	for _, result := range response.Results {
		if result.Status != "success" {
			failures = append(failures,
				fmt.Sprintf("%s %q (%s): %s", result.ResourceType, result.ResourceName, result.Code, result.Message))
		}
	}
	return fmt.Errorf("bootstrap import failed for %d resource(s): %s",
		response.Summary.Failed, strings.Join(failures, "; "))
}

// loadBundle reads every YAML file under dir (recursively), in a stable order, and
// concatenates them into a single multi-document import payload. The import service
// orders documents by dependency, so file order only affects same-type sequencing.
func loadBundle(dir string) (string, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to access bootstrap resource directory %q: %w", dir, err)
	}

	var paths []string
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// JSON is valid YAML, so flow/theme definitions kept as .json files are
		// loaded as documents alongside the .yaml resource definitions.
		switch strings.ToLower(filepath.Ext(path)) {
		case ".yaml", ".yml", ".json":
			paths = append(paths, path)
		}
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("failed to scan bootstrap resource directory %q: %w", dir, walkErr)
	}
	sort.Strings(paths)

	var builder strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // paths come from the trusted server home
		if err != nil {
			return "", fmt.Errorf("failed to read bootstrap definition %q: %w", path, err)
		}
		if builder.Len() > 0 {
			builder.WriteString("\n---\n")
		}
		builder.Write(data)
	}

	return builder.String(), nil
}
