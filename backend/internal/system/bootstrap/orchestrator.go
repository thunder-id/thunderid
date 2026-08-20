// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package bootstrap creates the default resources (organization unit, user types,
// admin user, system resource server, groups, roles, flows, the Console
// application, themes and translations) in-process at install time. It loads a
// templated YAML resource bundle and applies it through the existing import
// service, replacing the previous flow that started a temporary server with
// security disabled and seeded resources over unauthenticated HTTP.
package bootstrap

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// baselineIDPattern matches the fixed baseline ids the bootstrap bundle uses (a reserved prefix).
var baselineIDPattern = regexp.MustCompile(`01900000-0000-7000-8000-[0-9a-f]{12}`)

// resourceTypePattern extracts a bundle document's resource type.
var resourceTypePattern = regexp.MustCompile(`(?m)^resource_type:\s*(\w+)`)

// globalResourceTypes are resource types shared across every tenant rather than provisioned per
// tenant: their baseline ids are left untouched so every tenant upserts the same shared rows. All
// current resource types (including themes and layouts) are deployment-scoped, so this is empty; the
// mechanism remains for any future resource whose store is intentionally not tenant-partitioned.
var globalResourceTypes = map[string]bool{}

// localAdministratorTypes are the resources that exist only to make the built-in administrator work:
// the user itself, the group it belongs to, and the role granted to that group.
var localAdministratorTypes = map[string]bool{"user": true, "group": true, "role": true}

// omitLocalAdministrator drops the built-in administrator from a bundle, with the group and role that
// exist only to authorize it. They reference each other and nothing else references them, so they
// come out together or not at all.
func omitLocalAdministrator(content string) string {
	docs := strings.Split(content, "\n---")
	kept := make([]string, 0, len(docs))
	for _, doc := range docs {
		m := resourceTypePattern.FindStringSubmatch(doc)
		if len(m) > 1 && localAdministratorTypes[m[1]] {
			continue
		}
		kept = append(kept, doc)
	}
	return strings.Join(kept, "\n---")
}

// globalBaselineIDs returns the set of baseline ids that belong to global (shared) resource
// documents, so remapping can leave them - and any reference to them from tenant-scoped documents -
// pointing at the single shared row.
func globalBaselineIDs(content string) map[string]bool {
	ids := make(map[string]bool)
	for _, doc := range strings.Split(content, "\n---") {
		m := resourceTypePattern.FindStringSubmatch(doc)
		if len(m) < 2 || !globalResourceTypes[m[1]] {
			continue
		}
		for _, id := range baselineIDPattern.FindAllString(doc, -1) {
			ids[id] = true
		}
	}
	return ids
}

// remapBaselineIDs replaces every fixed baseline id in the bundle with a per-deployment id derived
// deterministically from the deployment id, applying the same replacement to every occurrence so
// cross-references within the bundle stay intact. Deterministic derivation keeps re-provisioning a
// tenant idempotent (upsert), and distinct deployment ids yield distinct baseline ids so tenants do
// not collide on the globally-unique primary keys. Ids of global (shared) resources are preserved so
// every tenant upserts the same shared row instead of creating a colliding copy.
func remapBaselineIDs(content, deploymentID string) string {
	global := globalBaselineIDs(content)
	seen := make(map[string]string)
	return baselineIDPattern.ReplaceAllStringFunc(content, func(orig string) string {
		if global[orig] {
			return orig
		}
		if v, ok := seen[orig]; ok {
			return v
		}
		v := deriveDeploymentScopedID(deploymentID, orig)
		seen[orig] = v
		return v
	})
}

// deriveDeploymentScopedID derives a stable, UUID-formatted id from a deployment id and an original
// id. It hashes the pair and formats 16 bytes as a version-7/RFC-4122-variant UUID string, so the
// result is a valid unique id without depending on any external UUID package.
func deriveDeploymentScopedID(deploymentID, originalID string) string {
	sum := sha256.Sum256([]byte(deploymentID + ":" + originalID))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x70 // version 7
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Options configures a bootstrap run.
type Options struct {
	// DefaultsDir is the directory holding the resource definition bundle.
	DefaultsDir string
	// DeploymentID scopes the seeded resources to a specific deployment (tenant). When set, the
	// bundle is imported under this deployment id instead of the server default - this is how a new
	// tenant's baseline is provisioned on a multi-tenant (token mode) Control Plane. Empty seeds the
	// server-configured deployment (single-tenant).
	DeploymentID string
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

	// Scope the seeded resources to a specific tenant when provisioning a deployment on a token-mode
	// Control Plane. Without this, stores resolve the deployment id from the (tenant-less) bootstrap
	// context and the baseline lands under the wrong deployment.
	if opts.DeploymentID != "" {
		ctx = deployment.WithID(ctx, opts.DeploymentID)
	}

	logger.Info(ctx, "Starting in-process bootstrap of default resources",
		log.String("defaultsDir", opts.DefaultsDir),
		log.String("deploymentId", opts.DeploymentID))

	content, err := loadBundle(opts.DefaultsDir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		logger.Warn(ctx, "No bootstrap resource definitions found; nothing to do",
			log.String("defaultsDir", opts.DefaultsDir))
		return nil
	}

	// A tenant gets no local administrator. Whoever administers one signs in against the trusted
	// issuer, which names the tenant in a token claim, so a local user with a password is an
	// unusable credential that would nonetheless be a real way in. A deployment provisioned without
	// a tenant is standalone, where that user is the only way in and is kept.
	//
	// This runs before the placeholders are resolved, so the credentials those documents reference
	// are no longer asked for at all.
	if opts.DeploymentID != "" {
		content = omitLocalAdministrator(content)
	}

	// Resolve `{{ .ENV_VAR }}` placeholders (e.g. PUBLIC_URL) from the environment before importing.
	resolved, err := utils.SubstituteEnvironmentVariables([]byte(content))
	if err != nil {
		return fmt.Errorf("failed to resolve bootstrap template variables: %w", err)
	}

	// The bundle uses fixed baseline ids, which would collide across tenants (ids are globally unique
	// primary keys). When provisioning a specific deployment, remap each baseline id to a value
	// derived deterministically from the deployment id, so every tenant gets its own baseline and
	// re-running upserts rather than duplicating. Nothing outside the bundle references these ids.
	if opts.DeploymentID != "" {
		resolved = []byte(remapBaselineIDs(string(resolved), opts.DeploymentID))
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
