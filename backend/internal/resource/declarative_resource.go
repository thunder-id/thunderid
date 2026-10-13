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

	// First pass: build ID-to-resource map so parent references can be resolved regardless of order
	idToResourceMap := make(map[string]providers.Resource)
	handleCount := make(map[string]int)
	for _, res := range allResources {
		idToResourceMap[res.ID] = res
		handleCount[res.Handle]++
	}

	// Second pass: build declarative resources with resolved parent handles and actions
	for _, res := range allResources {
		resource := providers.Resource{
			Name:        res.Name,
			Handle:      res.Handle,
			Description: res.Description,
			Actions:     []providers.Action{},
		}

		if res.Parent != nil && *res.Parent != "" {
			if parent, ok := idToResourceMap[*res.Parent]; ok {
				// A handle shared by several resources is ambiguous, so refer to the parent by its full path.
				resource.ParentHandle = parent.Handle
				if handleCount[parent.Handle] > 1 {
					resource.ParentHandle = parent.Permission
				}
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

		if err := normalizeParentHandlesForFileStore(rs); err != nil {
			return nil, fmt.Errorf("error processing resource server '%s': %w", rs.Name, err)
		}

		return rs, nil
	}
}

// normalizeParentHandlesForFileStore rewrites path-based parent references to bare parent handles.
// The file-based store identifies resources by handle, so handles must be unique across the resource server.
func normalizeParentHandlesForFileStore(rs *providers.ResourceServer) error {
	handleByPath := make(map[string]string, len(rs.Resources))
	resourceByHandle := make(map[string]*providers.Resource, len(rs.Resources))
	for i := range rs.Resources {
		res := &rs.Resources[i]
		if existing, ok := resourceByHandle[res.Handle]; ok {
			return fmt.Errorf(
				"duplicate resource handle '%s' found: conflicting resources are '%s' and '%s' in resource "+
					"server '%s'; file-based resource servers require resource handles to be unique across "+
					"the resource server",
				res.Handle, existing.Name, res.Name, rs.ID,
			)
		}
		resourceByHandle[res.Handle] = res
		handleByPath[res.Permission] = res.Handle
	}

	for i := range rs.Resources {
		res := &rs.Resources[i]
		if res.ParentHandle != "" {
			res.ParentHandle = handleByPath[strings.TrimSuffix(res.Permission, rs.Delimiter+res.Handle)]
		}
	}

	return nil
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

	paths, err := resolveResourcePaths(rs.Resources, delimiter)
	if err != nil {
		return err
	}

	// Resource handles must be unique under the same parent, so each resolved path must be unique.
	pathIndex := make(map[string]int, len(paths))
	for i, path := range paths {
		if existing, ok := pathIndex[path]; ok {
			return fmt.Errorf(
				"duplicate resource handle '%s' found under the same parent: conflicting resources are '%s' "+
					"and '%s' at path '%s' in resource server '%s'",
				rs.Resources[i].Handle,
				rs.Resources[existing].Name,
				rs.Resources[i].Name,
				path,
				rs.ID,
			)
		}
		pathIndex[path] = i
	}

	// Parents referenced by path are not resolved during path computation, so verify they exist.
	for i := range rs.Resources {
		parentRef := rs.Resources[i].ParentHandle
		if !strings.Contains(parentRef, delimiter) {
			continue
		}
		if _, ok := pathIndex[parentRef]; !ok {
			return fmt.Errorf(
				"parent resource path '%s' not found for resource '%s': cannot resolve permission chain",
				parentRef, rs.Resources[i].Handle,
			)
		}
	}

	// Process resources and compute permissions
	for i := range rs.Resources {
		processResource(&rs.Resources[i], paths[i], delimiter)
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

// processResource sets the permission of a resource and its actions from the resource path.
func processResource(res *providers.Resource, path string, delimiter string) {
	res.Permission = path

	for i := range res.Actions {
		actionPermission := path + delimiter + res.Actions[i].Handle
		res.Actions[i].Permission = actionPermission
	}
}

// resolveResourcePaths computes the delimiter-joined handle path of each resource from its parent
// reference. A parent reference containing the delimiter is the full path of the parent from the root.
// A bare handle refers to the root resource with that handle, or else to the only resource with that
// handle, and is rejected as ambiguous when several non-root resources share it.
func resolveResourcePaths(resources []providers.Resource, delimiter string) ([]string, error) {
	handleIndex := make(map[string][]int)
	for i := range resources {
		handleIndex[resources[i].Handle] = append(handleIndex[resources[i].Handle], i)
	}

	paths := make([]string, len(resources))
	resolved := make([]bool, len(resources))
	visiting := make([]bool, len(resources))

	var resolve func(i int) (string, error)
	resolve = func(i int) (string, error) {
		if resolved[i] {
			return paths[i], nil
		}
		res := &resources[i]

		var path string
		switch {
		case res.ParentHandle == "":
			path = res.Handle
		case strings.Contains(res.ParentHandle, delimiter):
			path = res.ParentHandle + delimiter + res.Handle
		default:
			if visiting[i] {
				return "", fmt.Errorf("circular parent reference detected for resource '%s'", res.Handle)
			}
			parent, err := findParentByHandle(resources, handleIndex[res.ParentHandle], res)
			if err != nil {
				return "", err
			}
			visiting[i] = true
			parentPath, err := resolve(parent)
			visiting[i] = false
			if err != nil {
				return "", err
			}
			path = parentPath + delimiter + res.Handle
		}

		paths[i] = path
		resolved[i] = true
		return path, nil
	}

	for i := range resources {
		if _, err := resolve(i); err != nil {
			return nil, err
		}
	}

	return paths, nil
}

// findParentByHandle selects the parent of a resource among the candidates sharing its bare parent handle.
func findParentByHandle(resources []providers.Resource, candidates []int, res *providers.Resource) (int, error) {
	for _, c := range candidates {
		if resources[c].ParentHandle == "" {
			return c, nil
		}
	}

	switch len(candidates) {
	case 0:
		return 0, fmt.Errorf(
			"parent resource handle '%s' not found for resource '%s': cannot resolve permission chain",
			res.ParentHandle, res.Handle,
		)
	case 1:
		return candidates[0], nil
	default:
		return 0, fmt.Errorf(
			"parent resource handle '%s' is ambiguous for resource '%s': multiple resources share this handle, "+
				"use the full parent path instead",
			res.ParentHandle, res.Handle,
		)
	}
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
