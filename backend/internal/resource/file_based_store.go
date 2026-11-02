// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"errors"
	"fmt"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

var (
	errImmutableStore = errors.New("operation not supported in file-based store")
)

type fileBasedResourceStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newFileBasedResourceStore creates a new file-based resource store.
func newFileBasedResourceStore() (resourceStoreInterface, providers.Transactioner, error) {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeResourceServer)
	store := &fileBasedResourceStore{
		GenericFileBasedStore: genericStore,
	}

	return store, transaction.NewNoOpTransactioner(), nil
}

// Create implements declarativeresource.Storer interface for resource loader
func (f *fileBasedResourceStore) Create(id string, data interface{}) error {
	rs, ok := data.(*providers.ResourceServer)
	if !ok {
		return fmt.Errorf("invalid data type for resource server: expected *ResourceServer, got %T", data)
	}

	return f.GenericFileBasedStore.Create(id, rs)
}

// Resource Server operations

func (f *fileBasedResourceStore) CreateResourceServer(
	ctx context.Context,
	id string,
	rs providers.ResourceServer,
) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) GetResourceServer(ctx context.Context, id string) (providers.ResourceServer, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		return providers.ResourceServer{}, errResourceServerNotFound
	}

	rs, ok := data.(*providers.ResourceServer)
	if !ok {
		declarativeresource.LogTypeAssertionError("resource_server", id)
		return providers.ResourceServer{}, errors.New("data corrupted")
	}

	return *rs, nil
}

func (f *fileBasedResourceStore) GetResourceServerList(
	ctx context.Context, limit, offset int) ([]providers.ResourceServer, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	servers := make([]providers.ResourceServer, 0, len(list))
	for _, item := range list {
		if rs, ok := item.Data.(*providers.ResourceServer); ok {
			servers = append(servers, *rs)
		}
	}

	// Apply pagination with bounds checking
	start := offset
	if start < 0 {
		start = 0
	}
	if limit < 0 {
		limit = 0
	}
	end := start + limit
	if start >= len(servers) {
		return servers[:0], nil
	}
	if end > len(servers) {
		end = len(servers)
	}

	return servers[start:end], nil
}

func (f *fileBasedResourceStore) GetResourceServerListCount(ctx context.Context) (int, error) {
	return f.GenericFileBasedStore.Count()
}

func (f *fileBasedResourceStore) UpdateResourceServer(
	ctx context.Context,
	id string,
	rs providers.ResourceServer,
) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) DeleteResourceServer(ctx context.Context, id string) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) CheckResourceServerNameExists(ctx context.Context, name string) (bool, error) {
	_, err := f.GenericFileBasedStore.GetByField(name, func(d interface{}) string {
		return d.(*providers.ResourceServer).Name
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (f *fileBasedResourceStore) CheckResourceServerIdentifierExists(
	ctx context.Context, identifier string) (bool, error) {
	_, err := f.GenericFileBasedStore.GetByField(identifier, func(d interface{}) string {
		return d.(*providers.ResourceServer).Identifier
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (f *fileBasedResourceStore) GetResourceServerByIdentifier(
	ctx context.Context, identifier string) (providers.ResourceServer, error) {
	data, err := f.GenericFileBasedStore.GetByField(identifier, func(d interface{}) string {
		return d.(*providers.ResourceServer).Identifier
	})
	if err != nil {
		return providers.ResourceServer{}, errResourceServerNotFound
	}

	rs, ok := data.(*providers.ResourceServer)
	if !ok {
		return providers.ResourceServer{}, errors.New("data corrupted")
	}

	return *rs, nil
}

func (f *fileBasedResourceStore) CheckResourceServerHasDependencies(
	ctx context.Context, resServerID string,
) (bool, error) {
	// Fetch resource server from store
	data, err := f.GenericFileBasedStore.Get(resServerID)
	if err != nil {
		return false, nil
	}

	rs, ok := data.(*providers.ResourceServer)
	if !ok {
		return false, errors.New("data corrupted")
	}

	return len(rs.Resources) > 0 || len(rs.Actions) > 0, nil
}

func (f *fileBasedResourceStore) IsResourceServerDeclarative(id string) bool {
	// Check if the resource server actually exists in the file store
	_, err := f.GenericFileBasedStore.Get(id)
	return err == nil
}

// Resource operations

// declarativeResourceID builds the ID of a declarative resource from its path (its permission).
func declarativeResourceID(rs *providers.ResourceServer, path string) string {
	return rs.ID + rs.Delimiter + path
}

// declarativeActionID builds the ID of a declarative action from its permission. resourcePath is empty
// for an action of the resource server itself.
func declarativeActionID(rs *providers.ResourceServer, resourcePath string, actionHandle string) string {
	if resourcePath == "" {
		return declarativeResourceID(rs, actionHandle)
	}
	return declarativeResourceID(rs, resourcePath+rs.Delimiter+actionHandle)
}

// declarativeAction gives an action of a declarative resource server the ID a read of it returns.
// resourcePath is empty for an action of the resource server itself.
func declarativeAction(rs *providers.ResourceServer, resourcePath string, action providers.Action) providers.Action {
	return providers.Action{
		ID:          declarativeActionID(rs, resourcePath, action.Handle),
		Name:        action.Name,
		Handle:      action.Handle,
		Description: action.Description,
		Permission:  action.Permission,
		Kind:        action.Kind,
	}
}

// declarativeActionsAt returns the actions a declarative resource server holds at one level, and the
// path of the resource that holds them. A nil resID selects the actions of the resource server itself,
// mirroring the RESOURCE_ID IS NULL semantics of the database store. It reports false when no
// resource has the given ID.
func declarativeActionsAt(rs *providers.ResourceServer, resID *string) (string, []providers.Action, bool) {
	if resID == nil {
		return "", rs.Actions, true
	}
	for _, res := range rs.Resources {
		if declarativeResourceID(rs, res.Permission) == *resID {
			return res.Permission, res.Actions, true
		}
	}
	return "", nil, false
}

// declarativeParentID returns the ID of a declarative resource's parent, or nil at the top level.
func declarativeParentID(rs *providers.ResourceServer, res providers.Resource) *string {
	if res.ParentHandle == "" {
		return nil
	}
	id := declarativeResourceID(rs, res.ParentHandle)
	return &id
}

func (f *fileBasedResourceStore) CreateResource(
	ctx context.Context, uuid string, resServerID string, parentID *string, res providers.Resource,
) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) GetResource(
	ctx context.Context,
	id string,
	resServerID string,
) (providers.Resource, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return providers.Resource{}, err
	}

	for _, item := range list {
		if rs, ok := item.Data.(*providers.ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			for _, res := range rs.Resources {
				if declarativeResourceID(rs, res.Permission) == id {
					resource := providers.Resource{
						ID:           declarativeResourceID(rs, res.Permission),
						Name:         res.Name,
						Handle:       res.Handle,
						Description:  res.Description,
						Parent:       declarativeParentID(rs, res),
						ParentHandle: res.ParentHandle,
						Permission:   res.Permission,
					}
					return resource, nil
				}
			}
		}
	}

	return providers.Resource{}, errResourceNotFound
}

func (f *fileBasedResourceStore) GetResourceList(
	ctx context.Context, resServerID string, limit, offset int) ([]providers.Resource, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	resources := []providers.Resource{}

	// Iterate through all resource servers
	for _, item := range list {
		if rs, ok := item.Data.(*providers.ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			// Add all resources from this server (no parent filter)
			for _, res := range rs.Resources {
				if res.Parent == nil {
					resources = append(resources, providers.Resource{
						ID:           declarativeResourceID(rs, res.Permission),
						Name:         res.Name,
						Handle:       res.Handle,
						Description:  res.Description,
						Parent:       declarativeParentID(rs, res),
						ParentHandle: res.ParentHandle,
						Permission:   res.Permission,
					})
				}
			}
		}
	}

	// Apply pagination with bounds checking
	start := offset
	if start < 0 {
		start = 0
	}
	if limit < 0 {
		limit = 0
	}
	end := start + limit
	if start >= len(resources) {
		return []providers.Resource{}, nil
	}
	if end > len(resources) {
		end = len(resources)
	}

	return resources[start:end], nil
}

func (f *fileBasedResourceStore) GetResourceListByParent(
	ctx context.Context, resServerID string, parentID *string, limit, offset int,
) ([]providers.Resource, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	resources := []providers.Resource{}

	// If parentID is specified, find the parent resource UUID first
	var parentResUUID string
	if parentID != nil {
		parentResUUID = *parentID
	}

	// Iterate through all resource servers
	for _, item := range list {
		if rs, ok := item.Data.(*providers.ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			// Add resources that match the parent
			for _, res := range rs.Resources {
				matched := false
				if parentID == nil {
					// Root level: no UUID parent and no handle parent
					matched = res.Parent == nil && res.ParentHandle == ""
				} else {
					if res.Parent != nil {
						// DB resource: match by UUID
						matched = *res.Parent == parentResUUID
					} else if res.ParentHandle != "" {
						// Declarative resource: match the ID of the parent path
						matched = declarativeResourceID(rs, res.ParentHandle) == parentResUUID
					}
				}
				if matched {
					resources = append(resources, providers.Resource{
						ID:           declarativeResourceID(rs, res.Permission),
						Name:         res.Name,
						Handle:       res.Handle,
						Description:  res.Description,
						Parent:       declarativeParentID(rs, res),
						ParentHandle: res.ParentHandle,
						Permission:   res.Permission,
					})
				}
			}
		}
	}

	// Apply pagination with bounds checking
	start := offset
	if start < 0 {
		start = 0
	}
	if limit < 0 {
		limit = 0
	}
	end := start + limit
	if start >= len(resources) {
		return []providers.Resource{}, nil
	}
	if end > len(resources) {
		end = len(resources)
	}

	return resources[start:end], nil
}

func (f *fileBasedResourceStore) GetResourceListCount(
	ctx context.Context, resServerID string) (int, error) {
	resources, err := f.GetResourceList(ctx, resServerID, 10000, 0)
	if err != nil {
		return 0, err
	}
	return len(resources), nil
}

func (f *fileBasedResourceStore) GetResourceListCountByParent(
	ctx context.Context, resServerID string, parentID *string) (int, error) {
	resources, err := f.GetResourceListByParent(ctx, resServerID, parentID, 10000, 0)
	if err != nil {
		return 0, err
	}
	return len(resources), nil
}

func (f *fileBasedResourceStore) UpdateResource(
	ctx context.Context, id string, resServerID string, res providers.Resource) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) UpdateResourcePermission(
	ctx context.Context, id string, resServerID string, permission string) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) DeleteResource(ctx context.Context, id string, resServerID string) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) CheckResourceHandleExists(
	ctx context.Context, resServerID string, handle string, parentID *string,
) (bool, error) {
	// Without server context, cannot check accurately
	return false, nil
}

func (f *fileBasedResourceStore) CheckResourceHasDependencies(ctx context.Context, resID string) (bool, error) {
	return false, nil
}

func (f *fileBasedResourceStore) CheckCircularDependency(
	ctx context.Context, resourceID, newParentID string) (bool, error) {
	// File-based resources are validated during load time
	return false, nil
}

// Action operations

func (f *fileBasedResourceStore) CreateAction(
	ctx context.Context, uuid string, resServerID string, resID *string, action providers.Action) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) GetAction(
	ctx context.Context, id string, resServerID string, resID *string) (providers.Action, error) {
	// Search only the requested resource server and resource
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return providers.Action{}, err
	}

	for _, item := range list {
		if rs, ok := item.Data.(*providers.ResourceServer); ok {
			if rs.ID != resServerID {
				continue
			}
			resourcePath, actions, found := declarativeActionsAt(rs, resID)
			if !found {
				continue
			}
			for _, action := range actions {
				if declarativeActionID(rs, resourcePath, action.Handle) == id {
					return declarativeAction(rs, resourcePath, action), nil
				}
			}
		}
	}

	return providers.Action{}, errActionNotFound
}

func (f *fileBasedResourceStore) GetActionList(
	ctx context.Context, resServerID string, resID *string, kind providers.ActionKind, limit, offset int,
) ([]providers.Action, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	actions := []providers.Action{}

	// Iterate through all resource servers
	for _, item := range list {
		if rs, ok := item.Data.(*providers.ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			resourcePath, levelActions, found := declarativeActionsAt(rs, resID)
			if !found {
				continue
			}
			for _, action := range levelActions {
				if kind != "" && action.Kind != kind {
					continue
				}
				actions = append(actions, declarativeAction(rs, resourcePath, action))
			}
		}
	}

	// Apply pagination with bounds checking
	start := offset
	if start < 0 {
		start = 0
	}
	if limit < 0 {
		limit = 0
	}
	end := start + limit
	if start >= len(actions) {
		return []providers.Action{}, nil
	}
	if end > len(actions) {
		end = len(actions)
	}

	return actions[start:end], nil
}

func (f *fileBasedResourceStore) GetActionListCount(
	ctx context.Context, resServerID string, resID *string, kind providers.ActionKind) (int, error) {
	actions, err := f.GetActionList(ctx, resServerID, resID, kind, 1000, 0)
	if err != nil {
		return 0, err
	}
	return len(actions), nil
}

func (f *fileBasedResourceStore) UpdateAction(
	ctx context.Context, id string, resServerID string, resID *string, action providers.Action) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) UpdateActionPermission(
	ctx context.Context, id string, resServerID string, resID *string, permission string) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) DeleteAction(
	ctx context.Context, id string, resServerID string, resID *string) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) IsActionExist(
	ctx context.Context, id string, resServerID string, resID *string) (bool, error) {
	_, err := f.GetAction(ctx, id, resServerID, resID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errActionNotFound) {
		return false, nil
	}
	return false, err
}

func (f *fileBasedResourceStore) CheckActionHandleExists(
	ctx context.Context, resServerID string, resID *string, handle string,
) (bool, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return false, err
	}

	// Iterate through all resource servers
	for _, item := range list {
		if rs, ok := item.Data.(*providers.ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			_, actions, found := declarativeActionsAt(rs, resID)
			if !found {
				continue
			}
			for _, action := range actions {
				if action.Handle == handle {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (f *fileBasedResourceStore) ValidatePermissions(
	ctx context.Context, resServerID string, permissions []string) ([]string, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	validPermissions := make(map[string]struct{})

	// Collect all valid permissions from the resource servers
	for _, item := range list {
		if rs, ok := item.Data.(*providers.ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			for _, action := range rs.Actions {
				validPermissions[action.Permission] = struct{}{}
			}
			for _, res := range rs.Resources {
				validPermissions[res.Permission] = struct{}{}
				for _, action := range res.Actions {
					validPermissions[action.Permission] = struct{}{}
				}
			}
		}
	}

	// Collect invalid permissions (those not in validPermissions)
	invalidList := make([]string, 0)
	for _, perm := range permissions {
		if _, found := validPermissions[perm]; !found {
			invalidList = append(invalidList, perm)
		}
	}
	return invalidList, nil
}
