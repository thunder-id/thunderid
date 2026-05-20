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

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
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
func (e *resourceServerExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
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
	interface{}, string, *serviceerror.ServiceError,
) {
	// Get the resource server
	server, err := e.service.GetResourceServer(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// Build ResourceServer with nested structure
	rs := &ResourceServer{
		ID:          server.ID,
		Name:        server.Name,
		Description: server.Description,
		Identifier:  server.Identifier,
		OUID:        server.OUID,
		Delimiter:   server.Delimiter,
		Resources:   []Resource{},
	}

	// Get all resources for this server
	var allResources []Resource
	resOffset := 0
	for {
		resources, err := e.service.GetResourceList(ctx, id, nil, serverconst.MaxPageSize, resOffset)
		if err != nil {
			return nil, "", err
		}
		if len(resources.Resources) == 0 {
			break
		}
		allResources = append(allResources, resources.Resources...)
		resOffset += len(resources.Resources)
		if resOffset >= resources.TotalResults {
			break
		}
	}

	// Build map for hierarchical structure keyed by resource ID
	resourceIDMap := make(map[string]*Resource)
	// Build separate map for ID to Handle lookups (for parent resolution)
	idToHandleMap := make(map[string]string)

	for _, res := range allResources {
		resource := Resource{
			Name:        res.Name,
			Handle:      res.Handle,
			Description: res.Description,
			Actions:     []Action{},
		}

		// Set parent handle from parent ID using the ID to Handle map
		if res.Parent != nil && *res.Parent != "" {
			if parentHandle, ok := idToHandleMap[*res.Parent]; ok {
				resource.ParentHandle = parentHandle
			}
		}

		// Get actions for this resource
		var allActions []Action
		actOffset := 0
		for {
			actions, err := e.service.GetActionList(ctx, id, &res.ID, serverconst.MaxPageSize, actOffset)
			if err != nil {
				return nil, "", err
			}
			if len(actions.Actions) == 0 {
				break
			}
			allActions = append(allActions, actions.Actions...)
			actOffset += len(actions.Actions)
			if actOffset >= actions.TotalResults {
				break
			}
		}

		for _, action := range allActions {
			resource.Actions = append(resource.Actions, Action{
				Name:        action.Name,
				Handle:      action.Handle,
				Description: action.Description,
			})
		}

		// Store in map keyed by resource ID (not handle) to avoid duplicates
		resourceIDMap[res.ID] = &resource
		// Also store ID to Handle mapping for parent lookup
		idToHandleMap[res.ID] = res.Handle
	}

	// Build the hierarchical structure - add root-level resources
	for _, res := range allResources {
		if res.ParentHandle == "" {
			if resource, ok := resourceIDMap[res.ID]; ok {
				rs.Resources = append(rs.Resources, *resource)
			}
		}
	}

	return rs, server.Name, nil
}

// ValidateResource validates a resource server resource.
func (e *resourceServerExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	server, ok := resource.(*ResourceServer)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeResourceServer, id)
	}

	if err := declarativeresource.ValidateResourceName(
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
			return validateResourceServerWrapper(data, fileStore, dbStore)
		},
		IDExtractor: func(data interface{}) string {
			return data.(*ResourceServer).ID
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
		// Parse YAML into ResourceServer struct
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

func parseToResourceServer(data []byte) (*ResourceServer, error) {
	var rs ResourceServer
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
	if rs.OUID == "" {
		return nil, fmt.Errorf("resource server organization unit ID cannot be empty")
	}

	return &rs, nil
}

// ProcessResourceServer processes the resource server and computes permissions in-place.
func ProcessResourceServer(rs *ResourceServer) error {
	delimiter := rs.Delimiter
	if delimiter == "" {
		delimiter = ":" // Default delimiter
	}
	rs.Delimiter = delimiter

	// Build a map of handle to resource for parent resolution and detect duplicates
	resourceHandleMap := make(map[string]*Resource)
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
		if err := processResource(&rs.Resources[i], resourceHandleMap, rs.Handle, delimiter); err != nil {
			return err
		}
	}

	return nil
}

// processResource processes a resource and its actions, computing permissions in-place.
func processResource(
	res *Resource,
	resourceHandleMap map[string]*Resource,
	rsHandle string,
	delimiter string,
) error {
	permission, err := buildPermissionString(res, resourceHandleMap, rsHandle, delimiter)
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
	res *Resource,
	resourceHandleMap map[string]*Resource,
	rsHandle string,
	delimiter string,
) (string, error) {
	var parts []string

	if rsHandle != "" {
		parts = append(parts, rsHandle)
	}

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
) error {
	rs, ok := data.(*ResourceServer)
	if !ok {
		return fmt.Errorf("invalid type: expected *ResourceServer")
	}

	if rs.Name == "" {
		return fmt.Errorf("resource server name cannot be empty")
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
