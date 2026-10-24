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

	// resourceServerPartResources and resourceServerPartActions name the collections a resource
	// server's API serves under /resource-servers/{id}.
	resourceServerPartResources = "resources"
	resourceServerPartActions   = "actions"
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

	// First pass: build ID-to-path map so parent paths can be resolved regardless of order
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
// The identifier is the audience each deployment serves under, so it is parameterized and every
// deployment supplies its own.
func (e *resourceServerExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		Variables: []string{
			"Identifier",
		},
	}
}

// ViewResource shows an exported resource server as a read of it returns it.
func (e *resourceServerExporter) ViewResource(_ context.Context, document *yaml.Node) (interface{}, error) {
	return declarativeresource.DecodeView(document, func(rs *providers.ResourceServer) (interface{}, error) {
		return toResourceServerResponse(rs), nil
	})
}

// ViewResourceParts shows the collections a resource server's API serves under it: "resources" (the
// top-level resources) and "actions", and for each resource "resources/{id}", its children at
// "resources/{id}/resources" and its actions at "resources/{id}/actions". A list part shows every item
// on one page. An export carries no database ids, so a resource and an action are given the ids a
// declarative resource server serves them by, built from the server id and their handles. An export
// carries no actions at the resource server level, so "actions" is an empty list.
func (e *resourceServerExporter) ViewResourceParts(_ context.Context, document *yaml.Node) (
	map[string]interface{}, error) {
	return declarativeresource.DecodeView(document, func(rs *providers.ResourceServer) (
		map[string]interface{}, error) {
		resources, err := viewResources(rs)
		if err != nil {
			return nil, err
		}
		parts := map[string]interface{}{
			resourceServerPartActions: toActionListResponse(&ActionList{StartIndex: 1, Actions: []providers.Action{}}),
		}
		topLevel := make([]providers.Resource, 0, len(resources))
		for i := range resources {
			res := &resources[i]
			if res.Parent == nil {
				topLevel = append(topLevel, *res)
			}
			// A resource's children, which the live API lists with ?parentId=.
			children := make([]providers.Resource, 0, len(resources))
			for _, child := range resources {
				if child.Parent != nil && *child.Parent == res.ID {
					children = append(children, child)
				}
			}
			base := resourceServerPartResources + "/" + res.ID
			parts[base] = toResourceResponse(res)
			parts[base+"/"+resourceServerPartResources] = toResourceListResponse(&ResourceList{
				TotalResults: len(children), StartIndex: 1, Count: len(children), Resources: children,
			})
			parts[base+"/"+resourceServerPartActions] = toActionListResponse(&ActionList{
				TotalResults: len(res.Actions), StartIndex: 1, Count: len(res.Actions), Actions: res.Actions,
			})
		}
		parts[resourceServerPartResources] = toResourceListResponse(&ResourceList{
			TotalResults: len(topLevel), StartIndex: 1, Count: len(topLevel), Resources: topLevel,
		})
		return parts, nil
	})
}

// viewResources gives the resources of an exported resource server, and their actions, the ids,
// parents and permissions a read of them returns.
func viewResources(rs *providers.ResourceServer) ([]providers.Resource, error) {
	view := *rs
	view.Resources = make([]providers.Resource, len(rs.Resources))
	for i := range rs.Resources {
		view.Resources[i] = rs.Resources[i]
		view.Resources[i].Actions = append([]providers.Action(nil), rs.Resources[i].Actions...)
	}
	if err := resolveResourcePermissions(&view); err != nil {
		return nil, err
	}

	for i := range view.Resources {
		res := &view.Resources[i]
		res.ID = declarativeResourceID(&view, res.Permission)
		if res.ParentHandle != "" {
			parentID := declarativeResourceID(&view, res.ParentHandle)
			res.Parent = &parentID
		}
		for j := range res.Actions {
			res.Actions[j].ID = declarativeActionID(&view, res.Permission, res.Actions[j].Handle)
		}
	}
	return view.Resources, nil
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

// resolveResourcePermissions validates handles, resolves parents and computes permissions in-place.
// Each parent reference is rewritten to the parent's full path.
func resolveResourcePermissions(rs *providers.ResourceServer) error {
	if rs.Delimiter == "" {
		rs.Delimiter = getDefaultDelimiter()
	}
	if svcErr := validateDelimiter(rs.Delimiter); svcErr != nil {
		return fmt.Errorf("invalid delimiter '%s' in resource server '%s': %s",
			rs.Delimiter, rs.ID, svcErr.ErrorDescription.DefaultValue)
	}

	// A handle must not contain the delimiter, so a path splits only one way.
	for i := range rs.Resources {
		res := &rs.Resources[i]
		if svcErr := validateHandle(res.Handle, rs.Delimiter); svcErr != nil {
			return fmt.Errorf("invalid handle '%s' for resource '%s' in resource server '%s': %s",
				res.Handle, res.Name, rs.ID, svcErr.ErrorDescription.DefaultValue)
		}
		for j := range res.Actions {
			if svcErr := validateHandle(res.Actions[j].Handle, rs.Delimiter); svcErr != nil {
				return fmt.Errorf("invalid handle '%s' for action '%s' in resource server '%s': %s",
					res.Actions[j].Handle, res.Actions[j].Name, rs.ID, svcErr.ErrorDescription.DefaultValue)
			}
		}
	}

	paths, parents, err := resolveResourcePaths(rs)
	if err != nil {
		return err
	}

	for i := range rs.Resources {
		res := &rs.Resources[i]
		res.Permission = paths[i]
		if parents[i] >= 0 {
			res.ParentHandle = paths[parents[i]]
		}
		for j := range res.Actions {
			res.Actions[j].Permission = paths[i] + rs.Delimiter + res.Actions[j].Handle
		}
	}

	return nil
}

// resolveResourcePaths returns each resource's path (its permission) and parent index (-1 at the top
// level). A parent is referenced by a unique bare handle or by its full path.
func resolveResourcePaths(rs *providers.ResourceServer) ([]string, []int, error) {
	resources := rs.Resources
	byHandle := make(map[string][]int, len(resources))
	for i := range resources {
		byHandle[resources[i].Handle] = append(byHandle[resources[i].Handle], i)
	}

	// Map bare handles to their parent. Paths resolve later. An ambiguous bare handle is reported
	// last, so a duplicate handle is reported first.
	parents := make([]int, len(resources))
	for i := range resources {
		parents[i] = -1
		ref := resources[i].ParentHandle
		if ref == "" || strings.Contains(ref, rs.Delimiter) {
			continue
		}
		switch candidates := byHandle[ref]; len(candidates) {
		case 0:
			return nil, nil, fmt.Errorf(
				"parent resource handle '%s' not found for resource '%s' in resource server '%s'",
				ref, resources[i].Handle, rs.ID,
			)
		case 1:
			parents[i] = candidates[0]
		default:
			parents[i] = ambiguousParent
		}
	}

	// Resolve each resource after its parent, so declaration order does not matter.
	paths := make([]string, len(resources))
	resolved := make([]bool, len(resources))
	byPath := make(map[string]int, len(resources))
	for remaining := len(resources); remaining > 0; {
		progressed := false
		for i := range resources {
			if resolved[i] {
				continue
			}
			res := &resources[i]
			parentPath := ""
			switch {
			case res.ParentHandle == "":
			case parents[i] == ambiguousParent:
				continue
			case parents[i] >= 0:
				if !resolved[parents[i]] {
					continue
				}
				parentPath = paths[parents[i]]
			default:
				parent, ok := byPath[res.ParentHandle]
				if !ok {
					continue
				}
				parents[i] = parent
				parentPath = res.ParentHandle
			}

			path := res.Handle
			if parentPath != "" {
				path = parentPath + rs.Delimiter + res.Handle
			}
			if existing, ok := byPath[path]; ok {
				scope := "at the top level"
				if parentPath != "" {
					scope = fmt.Sprintf("under parent '%s'", parentPath)
				}
				return nil, nil, fmt.Errorf(
					"duplicate resource handle '%s' found %s: conflicting resources are '%s' and '%s' "+
						"in resource server '%s'",
					res.Handle, scope, resources[existing].Name, res.Name, rs.ID,
				)
			}
			byPath[path] = i
			paths[i] = path
			resolved[i] = true
			remaining--
			progressed = true
		}
		if !progressed {
			return nil, nil, unresolvedParentError(rs, parents, resolved, byHandle)
		}
	}

	return paths, parents, nil
}

// ambiguousParent marks a resource whose bare parent handle is shared by several resources.
const ambiguousParent = -2

// unresolvedParentError reports a cycle, an ambiguous bare handle, or a missing parent path.
func unresolvedParentError(
	rs *providers.ResourceServer, parents []int, resolved []bool, byHandle map[string][]int,
) error {
	resources := rs.Resources
	ambiguous, missing := -1, -1
	for i := range resources {
		if resolved[i] {
			continue
		}
		if parents[i] == ambiguousParent && ambiguous < 0 {
			ambiguous = i
		}
		if parents[i] == -1 && missing < 0 {
			missing = i
		}
		seen := make(map[int]bool)
		for cur := i; parents[cur] >= 0; cur = parents[cur] {
			if seen[cur] {
				return fmt.Errorf(
					"circular parent reference detected at handle '%s' for resource '%s' in resource server '%s'",
					resources[cur].Handle, resources[i].Handle, rs.ID,
				)
			}
			seen[cur] = true
		}
	}

	if ambiguous >= 0 {
		ref := resources[ambiguous].ParentHandle
		return fmt.Errorf(
			"parent resource handle '%s' is ambiguous for resource '%s' in resource server '%s': "+
				"%d resources share this handle, use the full parent path instead",
			ref, resources[ambiguous].Handle, rs.ID, len(byHandle[ref]),
		)
	}

	return fmt.Errorf(
		"parent resource path '%s' not found for resource '%s' in resource server '%s'",
		resources[missing].ParentHandle, resources[missing].Handle, rs.ID,
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
