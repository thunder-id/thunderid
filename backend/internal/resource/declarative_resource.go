/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
		ID:          server.ID,
		Name:        server.Name,
		Description: server.Description,
		Identifier:  server.Identifier,
		Type:        server.Type,
		OUID:        server.OUID,
		Delimiter:   server.Delimiter,
		Resources:   []providers.Resource{},
	}

	allResources, err := e.service.GetAllResourceList(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// First pass: build ID-to-Handle map so parent handles can be resolved regardless of order
	idToHandleMap := make(map[string]string)
	for _, res := range allResources {
		idToHandleMap[res.ID] = res.Handle
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
			if parentHandle, ok := idToHandleMap[*res.Parent]; ok {
				resource.ParentHandle = parentHandle
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
	delimiter := rs.Delimiter
	if delimiter == "" {
		delimiter = ":" // Default delimiter
	}
	rs.Delimiter = delimiter

	// Build a map of handle to resource for parent resolution and detect duplicates
	resourceHandleMap := make(map[string]*providers.Resource)
	for i := range rs.Resources {
		handle := rs.Resources[i].Handle
		if existing, ok := resourceHandleMap[handle]; ok {
			// Duplicate handle detected
			return fmt.Errorf(
				"duplicate resource handle '%s' found: conflicting resources are '%s' and '%s' in resource server '%s'",
				handle,
				existing.Name,
				rs.Resources[i].Name,
				rs.ID,
			)
		}
		resourceHandleMap[handle] = &rs.Resources[i]
	}

	// Process resources and compute permissions
	for i := range rs.Resources {
		if err := processResource(&rs.Resources[i], resourceHandleMap, delimiter); err != nil {
			return err
		}
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

// processResource processes a resource and its actions, computing permissions in-place.
func processResource(
	res *providers.Resource,
	resourceHandleMap map[string]*providers.Resource,
	delimiter string,
) error {
	permission, err := buildPermissionString(res, resourceHandleMap, delimiter)
	if err != nil {
		return err
	}
	res.Permission = permission

	for i := range res.Actions {
		actionPermission := permission + delimiter + res.Actions[i].Handle
		res.Actions[i].Permission = actionPermission
	}

	return nil
}

// buildPermissionString constructs the permission string by traversing parent chain.
func buildPermissionString(
	res *providers.Resource,
	resourceHandleMap map[string]*providers.Resource,
	delimiter string,
) (string, error) {
	var parts []string

	parentChain := []string{}
	visited := make(map[string]bool)
	current := res
	for current != nil {
		if current.Handle != "" {
			if visited[current.Handle] {
				return "", fmt.Errorf(
					"circular parent reference detected at handle '%s' for resource '%s'",
					current.Handle, res.Handle,
				)
			}
			visited[current.Handle] = true
			parentChain = append([]string{current.Handle}, parentChain...)
		}

		if current.ParentHandle == "" {
			break
		}

		parent, exists := resourceHandleMap[current.ParentHandle]
		if !exists {
			return "", fmt.Errorf(
				"parent resource handle '%s' not found for resource '%s': cannot resolve permission chain",
				current.ParentHandle,
				res.Handle,
			)
		}
		current = parent
	}

	if len(parentChain) > 1 {
		parts = append(parts, parentChain[:len(parentChain)-1]...)
	}

	parts = append(parts, res.Handle)

	return strings.Join(parts, delimiter), nil
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
