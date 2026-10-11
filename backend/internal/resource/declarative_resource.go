// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeResourceServer = "resource_server"
	paramTypeResourceServer    = "ResourceServer"
)

// resourceServerExporter implements declarativeresource.ResourceExporter for resource servers.
type resourceServerExporter struct {
	service ResourceServiceInterface
}

// newResourceServerExporter creates a new resource server exporter.
func newResourceServerExporter(service ResourceServiceInterface) *resourceServerExporter {
	return &resourceServerExporter{service: service}
}

// newResourceServerExporterForTest creates a new resource server exporter for testing purposes.
func newResourceServerExporterForTest(service ResourceServiceInterface) *resourceServerExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newResourceServerExporter(service)
}

// GetResourceType returns the resource type for resource servers.
func (e *resourceServerExporter) GetResourceType() string {
	return resourceTypeResourceServer
}

// GetParameterizerType returns the parameterizer type for resource servers.
func (e *resourceServerExporter) GetParameterizerType() string {
	return paramTypeResourceServer
}

// GetAllResourceIDs retrieves all resource server IDs.
// In composite mode, this excludes declarative (YAML-based) resource servers.
func (e *resourceServerExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	ids := make([]string, 0)
	offset := 0
	for {
		servers, err := e.service.GetResourceServerList(ctx, serverconst.MaxPageSize, offset)
		if err != nil {
			return nil, err
		}
		if len(servers.ResourceServers) == 0 {
			break
		}
		for _, server := range servers.ResourceServers {
			if !e.service.IsResourceServerDeclarative(server.ID) {
				ids = append(ids, server.ID)
			}
		}
		offset += len(servers.ResourceServers)
		if offset >= servers.TotalResults {
			break
		}
	}
	return ids, nil
}

// GetResourceByID retrieves a resource server with all its nested resources and actions by its ID.
func (e *resourceServerExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError,
) {
	// Get the resource server
	server, err := e.service.GetResourceServer(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// Build providers.ResourceServer with nested structure
	rs := &providers.ResourceServer{
		ID:                  server.ID,
		Name:                server.Name,
		Description:         server.Description,
		Identifier:          server.Identifier,
		Type:                server.Type,
		OUID:                server.OUID,
		Delimiter:           server.Delimiter,
		AuthorizationEngine: server.AuthorizationEngine,
		Resources:           []providers.Resource{},
	}

	allResources, err := e.service.GetAllResourceList(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// First pass: build an ID-to-path map so parents can be referenced regardless of order. The
	// parent is written as its full path rather than its bare handle, since a handle is unique only
	// under its own parent.
	idToPathMap := make(map[string]string)
	for _, res := range allResources {
		idToPathMap[res.ID] = res.Permission
	}

	// Second pass: build declarative resources with resolved parent paths and actions
	for _, res := range allResources {
		resource := providers.Resource{
			Name:        res.Name,
			Handle:      res.Handle,
			Description: res.Description,
			Actions:     []providers.Action{},
		}

		if res.Parent != nil && *res.Parent != "" {
			if parentPath, ok := idToPathMap[*res.Parent]; ok {
				resource.ParentHandle = parentPath
			}
		}

		// Get actions for this resource
		actOffset := 0
		for {
			actions, actErr := e.service.GetActionList(ctx, id, &res.ID, "", serverconst.MaxPageSize, actOffset)
			if actErr != nil {
				return nil, "", actErr
			}
			if len(actions.Actions) == 0 {
				break
			}
			for _, action := range actions.Actions {
				resource.Actions = append(resource.Actions, providers.Action{
					Name:        action.Name,
					Handle:      action.Handle,
					Description: action.Description,
					Kind:        action.Kind,
				})
			}
			actOffset += len(actions.Actions)
			if actOffset >= actions.TotalResults {
				break
			}
		}

		rs.Resources = append(rs.Resources, resource)
	}

	return rs, server.Name, nil
}

// ValidateResource validates a resource server resource.
func (e *resourceServerExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	server, ok := resource.(*providers.ResourceServer)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeResourceServer, id)
	}

	if err := declarativeresource.ValidateResourceName(ctx,
		server.Name, resourceTypeResourceServer, id, "RS_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return server.Name, nil
}

// GetResourceRules returns the parameterization rules for resource servers.
// Resource servers have no fields that need to be parameterized as template variables,
// so nil is returned to use the standard YAML encoder path which preserves literal values
// and correctly quotes fields tagged with yamlfmt:"quoted" (e.g. Delimiter).
func (e *resourceServerExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return nil
}

// loadDeclarativeResources loads resource server resources from declarative files.
// Works in both declarative-only and composite modes:
// - In declarative mode: resourceStore is a fileBasedResourceStore
// - In composite mode: resourceStore is a compositeResourceStore (contains both file and DB stores)
func loadDeclarativeResources(resourceStore resourceStoreInterface, resourceService ResourceServiceInterface) error {
	var fileStore resourceStoreInterface
	var dbStore resourceStoreInterface

	// Determine store type and extract appropriate stores
	switch store := resourceStore.(type) {
	case *compositeResourceStore:
		// Composite mode: both file and DB stores available
		fileStore = store.fileStore
		dbStore = store.dbStore
	case *fileBasedResourceStore:
		// Declarative-only mode: only file store available
		fileStore = store
		dbStore = nil
	default:
		return fmt.Errorf("invalid store type for loading declarative resources")
	}

	// Type assert to access Storer interface for resource loading
	fileBasedStoreImpl, ok := fileStore.(*fileBasedResourceStore)
	if !ok {
		return fmt.Errorf("failed to assert fileStore to *fileBasedResourceStore")
	}

	// Use a custom loader for resource servers
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "ResourceServer",
		DirectoryName: "resource_servers",
		Parser:        parseAndValidateResourceServerWrapper(resourceService),
		Validator: func(data interface{}) error {
			return validateResourceServerWrapper(data, fileStore, dbStore, resourceService)
		},
		IDExtractor: func(data interface{}) string {
			return data.(*providers.ResourceServer).ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileBasedStoreImpl)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load resource server resources: %w", err)
	}

	return nil
}

// parseAndValidateResourceServerWrapper combines parsing, processing, and validation for resource servers.
func parseAndValidateResourceServerWrapper(resourceService ResourceServiceInterface) func([]byte) (interface{}, error) {
	return func(data []byte) (interface{}, error) {
		// Parse YAML into providers.ResourceServer struct
		rs, err := parseToResourceServer(data)
		if err != nil {
			return nil, err
		}

		// Process and compute permissions in-place
		if err := ProcessResourceServer(rs); err != nil {
			return nil, fmt.Errorf("error processing resource server '%s': %w", rs.Name, err)
		}

		return rs, nil
	}
}

func parseToResourceServer(data []byte) (*providers.ResourceServer, error) {
	var rs providers.ResourceServer
	err := yaml.Unmarshal(data, &rs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource server YAML: %w", err)
	}

	if rs.ID == "" {
		return nil, fmt.Errorf("resource server ID cannot be empty")
	}
	if rs.Name == "" {
		return nil, fmt.Errorf("resource server name cannot be empty")
	}
	if rs.Type != "" && !rs.Type.IsValid() {
		return nil, fmt.Errorf("invalid type %q for resource server '%s'", rs.Type, rs.Name)
	}
	if rs.Type == "" {
		rs.Type = providers.ResourceServerTypeCustom
	}

	// Apply the action kind discriminator rules (mirrors the REST path). The kind is optional for all
	// resource server types; MCP actions default to "tool" when omitted, and any provided kind must be
	// one of the supported values (tool|resource).
	for i := range rs.Resources {
		for j := range rs.Resources[i].Actions {
			action := &rs.Resources[i].Actions[j]
			if rs.Type == providers.ResourceServerTypeMCP && action.Kind == "" {
				action.Kind = providers.ActionKindTool
			}
			if action.Kind != "" && !action.Kind.IsValid() {
				return nil, fmt.Errorf(
					"action %q in resource server '%s' has invalid kind %q (allowed: tool|resource)",
					action.Handle, rs.Name, action.Kind,
				)
			}
		}
	}

	return &rs, nil
}

// ProcessResourceServer processes the resource server and computes permissions in-place.
func ProcessResourceServer(rs *providers.ResourceServer) error {
	if rs.AuthorizationEngine.Type == "" {
		rs.AuthorizationEngine.Type = providers.AuthorizationEngineTypeRBAC
	}
	switch rs.AuthorizationEngine.Type {
	case providers.AuthorizationEngineTypeRBAC:
		rs.AuthorizationEngine.Properties = providers.AuthorizationEngineProperties{}
	case providers.AuthorizationEngineTypeAuthZENPDP:
		if err := validateAuthZENPDPConnectionID(&rs.AuthorizationEngine); err != nil {
			return fmt.Errorf("resource server %q: %w", rs.ID, err)
		}
	default:
		return fmt.Errorf("unsupported authorization engine type %q", rs.AuthorizationEngine.Type)
	}
	delimiter := rs.Delimiter
	if delimiter == "" {
		delimiter = ":" // Default delimiter
	}
	rs.Delimiter = delimiter

	// Resolve the parent chain of every resource and compute permissions.
	if err := resolveResourcePermissions(rs); err != nil {
		return err
	}

	// For MCP resource servers, a resource (group) and an action (tool/resource) in the same parent
	// context can derive an identical permission string under exact-string RBAC, silently collapsing
	// two distinct primitives. Mirror the REST cross-entity check (Rule 6) on the declarative path by
	// failing on the first duplicate derived permission across all resources and their nested actions.
	if rs.Type == providers.ResourceServerTypeMCP {
		if err := checkDuplicateMCPPermissions(rs); err != nil {
			return err
		}
	}

	return nil
}

// checkDuplicateMCPPermissions detects duplicate derived permission strings across all resources
// (groups) and their nested actions (tools/resources) for an MCP resource server. It returns an
// error naming the colliding permission and handles on the first duplicate found.
func checkDuplicateMCPPermissions(rs *providers.ResourceServer) error {
	seen := make(map[string]string)
	for i := range rs.Resources {
		res := &rs.Resources[i]
		if existing, ok := seen[res.Permission]; ok {
			return fmt.Errorf(
				"duplicate permission '%s' derived for handles '%s' and '%s' in resource server '%s'",
				res.Permission, existing, res.Handle, rs.ID,
			)
		}
		seen[res.Permission] = res.Handle

		for j := range res.Actions {
			action := &res.Actions[j]
			if existing, ok := seen[action.Permission]; ok {
				return fmt.Errorf(
					"duplicate permission '%s' derived for handles '%s' and '%s' in resource server '%s'",
					action.Permission, existing, action.Handle, rs.ID,
				)
			}
			seen[action.Permission] = action.Handle
		}
	}

	return nil
}

// resolveResourcePermissions resolves every resource's parent reference and computes the
// permission string of each resource and action in-place.
//
// A resource is identified by its path: the delimiter-joined chain of handles from the root, which
// is also its permission string. Handles are therefore required to be unique per parent rather than
// per resource server, matching the storage rule enforced by uq_resource_handle_with_parent and by
// the service layer when resources are created through the REST API.
//
// A `parent` value is first matched against the full path of an already resolved resource
// (`app:project`), and otherwise against a handle that is unique within the resource server
// (`project`). The full path is the unambiguous form and is the only way to reference a parent whose
// handle repeats under different parents.
func resolveResourcePermissions(rs *providers.ResourceServer) error {
	byPath := make(map[string]*providers.Resource, len(rs.Resources))
	pending := make([]*providers.Resource, 0, len(rs.Resources))
	for i := range rs.Resources {
		pending = append(pending, &rs.Resources[i])
	}

	// Resolve one level of the hierarchy per pass. Resolutions are committed only after the pass
	// completes, so a resource is always resolved against the paths of strictly shallower resources
	// and the outcome does not depend on the order resources are declared in.
	for len(pending) > 0 {
		type resolution struct {
			res        *providers.Resource
			path       string
			parentPath string
		}

		resolved := make([]resolution, 0, len(pending))
		unresolved := make([]*providers.Resource, 0, len(pending))

		for _, res := range pending {
			parentPath, found, err := resolveParentPath(rs, res, byPath)
			if err != nil {
				return err
			}
			if !found {
				unresolved = append(unresolved, res)
				continue
			}
			path := res.Handle
			if res.ParentHandle != "" {
				path = parentPath + rs.Delimiter + res.Handle
			}
			resolved = append(resolved, resolution{res: res, path: path, parentPath: parentPath})
		}

		if len(resolved) == 0 {
			return unresolvedParentError(rs, unresolved)
		}

		for _, r := range resolved {
			if existing, ok := byPath[r.path]; ok {
				return duplicateHandleError(rs, existing, r.res, r.parentPath)
			}
			byPath[r.path] = r.res
			r.res.Permission = r.path
			// Normalise the reference to the parent's full path so that consumers of the processed
			// resource server can key resources by path without repeating the resolution.
			r.res.ParentHandle = r.parentPath
		}

		pending = unresolved
	}

	for i := range rs.Resources {
		res := &rs.Resources[i]
		for j := range res.Actions {
			res.Actions[j].Permission = res.Permission + rs.Delimiter + res.Actions[j].Handle
		}
	}

	return nil
}

// resolveParentPath resolves a resource's `parent` reference to the path of its parent. It reports
// found as false when the reference names a resource that has not been resolved yet, which means
// the parent sits deeper in the hierarchy and the resource has to be retried on a later pass.
func resolveParentPath(
	rs *providers.ResourceServer,
	res *providers.Resource,
	byPath map[string]*providers.Resource,
) (string, bool, error) {
	ref := res.ParentHandle
	if ref == "" {
		return "", true, nil
	}

	// An exact path match is the unambiguous form and takes precedence.
	if _, ok := byPath[ref]; ok {
		return ref, true, nil
	}

	// Otherwise the reference is a bare handle, which resolves only while it is unique.
	var match string
	matches := 0
	for path, candidate := range byPath {
		if candidate.Handle == ref {
			matches++
			match = path
		}
	}
	switch {
	case matches == 1:
		return match, true, nil
	case matches > 1:
		return "", false, fmt.Errorf(
			"ambiguous parent reference '%s' for resource '%s' in resource server '%s': "+
				"%d resources share this handle, use the full parent path instead",
			ref, res.Handle, rs.ID, matches,
		)
	default:
		return "", false, nil
	}
}

// unresolvedParentError explains why a set of resources could not be attached to a parent. A
// reference whose last segment matches no declared handle names a parent that does not exist;
// anything else is a cycle, since the referenced resources can only be waiting on each other.
func unresolvedParentError(rs *providers.ResourceServer, unresolved []*providers.Resource) error {
	declared := make(map[string]struct{}, len(rs.Resources))
	for i := range rs.Resources {
		declared[rs.Resources[i].Handle] = struct{}{}
	}

	for _, res := range unresolved {
		segments := strings.Split(res.ParentHandle, rs.Delimiter)
		if _, ok := declared[segments[len(segments)-1]]; !ok {
			return fmt.Errorf(
				"parent resource handle '%s' not found for resource '%s': cannot resolve permission chain",
				res.ParentHandle, res.Handle,
			)
		}
	}

	return fmt.Errorf(
		"circular parent reference detected at handle '%s' for resource '%s'",
		unresolved[0].ParentHandle, unresolved[0].Handle,
	)
}

// duplicateHandleError reports two resources that share a handle under the same parent, which the
// storage layer would reject and which would derive the same permission string.
func duplicateHandleError(
	rs *providers.ResourceServer,
	existing, conflicting *providers.Resource,
	parentPath string,
) error {
	scope := "at the root level"
	if parentPath != "" {
		scope = fmt.Sprintf("under parent '%s'", parentPath)
	}

	return fmt.Errorf(
		"duplicate resource handle '%s' found %s: conflicting resources are '%s' and '%s' "+
			"in resource server '%s'",
		conflicting.Handle, scope, existing.Name, conflicting.Name, rs.ID,
	)
}

func validateResourceServerWrapper(
	data interface{},
	fileStore resourceStoreInterface,
	dbStore resourceStoreInterface,
	service ResourceServiceInterface,
) error {
	rs, ok := data.(*providers.ResourceServer)
	if !ok {
		return fmt.Errorf("invalid type: expected *ResourceServer")
	}

	if rs.Name == "" {
		return fmt.Errorf("resource server name cannot be empty")
	}

	if rs.Identifier == "" {
		return fmt.Errorf("resource server identifier cannot be empty")
	}

	if service != nil {
		if svcErr := service.ResolveResourceServerOUHandle(context.Background(), rs); svcErr != nil {
			return fmt.Errorf("organization unit with handle %q not found for resource server '%s'",
				rs.OUHandle, rs.Name)
		}
	}

	if rs.OUID == "" {
		return fmt.Errorf("ou_id or ou_handle is required for resource server '%s'", rs.Name)
	}

	// Check for duplicate ID in the file store
	_, err := fileStore.GetResourceServer(context.Background(), rs.ID)
	if err == nil {
		return fmt.Errorf("duplicate resource server ID '%s': "+
			"a resource server with this ID already exists in declarative resources", rs.ID)
	}
	// Propagate any error other than not-found
	if !errors.Is(err, errResourceServerNotFound) {
		return fmt.Errorf("failed to check for duplicate resource server in declarative store: %w", err)
	}

	// COMPOSITE MODE: Check for duplicate ID in the database store
	if dbStore != nil {
		_, err := dbStore.GetResourceServer(context.Background(), rs.ID)
		if err == nil {
			return fmt.Errorf("duplicate resource server ID '%s': "+
				"a resource server with this ID already exists in the database store", rs.ID)
		}
		// Propagate any error other than not-found
		if !errors.Is(err, errResourceServerNotFound) {
			return fmt.Errorf("failed to check for duplicate resource server in database store: %w", err)
		}
	}

	return nil
}
