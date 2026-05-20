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
	"sort"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

var (
	errImmutableStore = errors.New("operation not supported in file-based store")
)

type fileBasedResourceStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newFileBasedResourceStore creates a new file-based resource store.
func newFileBasedResourceStore() (resourceStoreInterface, transaction.Transactioner, error) {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeResourceServer)
	store := &fileBasedResourceStore{
		GenericFileBasedStore: genericStore,
	}

	return store, transaction.NewNoOpTransactioner(), nil
}

// Create implements declarativeresource.Storer interface for resource loader
func (f *fileBasedResourceStore) Create(id string, data interface{}) error {
	rs, ok := data.(*ResourceServer)
	if !ok {
		return fmt.Errorf("invalid data type for resource server: expected *ResourceServer, got %T", data)
	}

	return f.GenericFileBasedStore.Create(id, rs)
}

// Resource Server operations

func (f *fileBasedResourceStore) CreateResourceServer(ctx context.Context, id string, rs ResourceServer) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) GetResourceServer(ctx context.Context, id string) (ResourceServer, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		return ResourceServer{}, errResourceServerNotFound
	}

	rs, ok := data.(*ResourceServer)
	if !ok {
		declarativeresource.LogTypeAssertionError("resource_server", id)
		return ResourceServer{}, errors.New("data corrupted")
	}

	return *rs, nil
}

func (f *fileBasedResourceStore) GetResourceServerList(
	ctx context.Context, limit, offset int) ([]ResourceServer, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	servers := make([]ResourceServer, 0, len(list))
	for _, item := range list {
		if rs, ok := item.Data.(*ResourceServer); ok {
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

func (f *fileBasedResourceStore) UpdateResourceServer(ctx context.Context, id string, rs ResourceServer) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) DeleteResourceServer(ctx context.Context, id string) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) CheckResourceServerNameExists(ctx context.Context, name string) (bool, error) {
	_, err := f.GenericFileBasedStore.GetByField(name, func(d interface{}) string {
		return d.(*ResourceServer).Name
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (f *fileBasedResourceStore) CheckResourceServerHandleExists(
	ctx context.Context, handle string) (bool, error) {
	_, err := f.GenericFileBasedStore.GetByField(handle, func(d interface{}) string {
		return d.(*ResourceServer).Handle
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (f *fileBasedResourceStore) CheckResourceServerIdentifierExists(
	ctx context.Context, identifier string) (bool, error) {
	_, err := f.GenericFileBasedStore.GetByField(identifier, func(d interface{}) string {
		return d.(*ResourceServer).Identifier
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (f *fileBasedResourceStore) GetResourceServerByIdentifier(
	ctx context.Context, identifier string) (ResourceServer, error) {
	data, err := f.GenericFileBasedStore.GetByField(identifier, func(d interface{}) string {
		return d.(*ResourceServer).Identifier
	})
	if err != nil {
		return ResourceServer{}, errResourceServerNotFound
	}

	rs, ok := data.(*ResourceServer)
	if !ok {
		return ResourceServer{}, errors.New("data corrupted")
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

	rs, ok := data.(*ResourceServer)
	if !ok {
		return false, errors.New("data corrupted")
	}

	// Check if it has any resources
	return len(rs.Resources) > 0, nil
}

func (f *fileBasedResourceStore) IsResourceServerDeclarative(id string) bool {
	// Check if the resource server actually exists in the file store
	_, err := f.GenericFileBasedStore.Get(id)
	return err == nil
}

// Resource operations

func (f *fileBasedResourceStore) CreateResource(
	ctx context.Context, uuid string, resServerID string, parentID *string, res Resource,
) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) GetResource(ctx context.Context, id string, resServerID string) (Resource, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return Resource{}, err
	}

	for _, item := range list {
		if rs, ok := item.Data.(*ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			for _, res := range rs.Resources {
				if res.Handle == id || fmt.Sprintf("%s_%s", rs.ID, res.Handle) == id {
					resource := Resource{
						ID:           fmt.Sprintf("%s_%s", rs.ID, res.Handle),
						Name:         res.Name,
						Handle:       res.Handle,
						Description:  res.Description,
						Parent:       res.Parent,
						ParentHandle: res.ParentHandle,
						Permission:   res.Permission,
					}
					return resource, nil
				}
			}
		}
	}

	return Resource{}, errResourceNotFound
}

func (f *fileBasedResourceStore) GetResourceList(
	ctx context.Context, resServerID string, limit, offset int) ([]Resource, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	resources := []Resource{}

	// Iterate through all resource servers
	for _, item := range list {
		if rs, ok := item.Data.(*ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			// Add all resources from this server (no parent filter)
			for _, res := range rs.Resources {
				if res.Parent == nil {
					resources = append(resources, Resource{
						ID:           fmt.Sprintf("%s_%s", rs.ID, res.Handle),
						Name:         res.Name,
						Handle:       res.Handle,
						Description:  res.Description,
						Parent:       res.Parent,
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
		return []Resource{}, nil
	}
	if end > len(resources) {
		end = len(resources)
	}

	return resources[start:end], nil
}

func (f *fileBasedResourceStore) GetResourceListByParent(
	ctx context.Context, resServerID string, parentID *string, limit, offset int,
) ([]Resource, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	resources := []Resource{}

	// If parentID is specified, find the parent resource UUID first
	var parentResUUID string
	if parentID != nil {
		parentResUUID = *parentID
	}

	// Iterate through all resource servers
	for _, item := range list {
		if rs, ok := item.Data.(*ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			// Add resources that match the parent
			for _, res := range rs.Resources {
				// Check if this resource matches the parent (by handle or UUID)
				if parentID == nil && res.Parent == nil {
					// Root level resources
					resources = append(resources, Resource{
						ID:           fmt.Sprintf("%s_%s", rs.ID, res.Handle),
						Name:         res.Name,
						Handle:       res.Handle,
						Description:  res.Description,
						Parent:       res.Parent,
						ParentHandle: res.ParentHandle,
						Permission:   res.Permission,
					})
				} else if parentID != nil && res.Parent != nil {
					// Check if parent handle matches the parent resource UUID
					if *res.Parent == parentResUUID {
						resources = append(resources, Resource{
							ID:           fmt.Sprintf("%s_%s", rs.ID, res.Handle),
							Name:         res.Name,
							Handle:       res.Handle,
							Description:  res.Description,
							Parent:       res.Parent,
							ParentHandle: res.ParentHandle,
							Permission:   res.Permission,
						})
					}
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
		return []Resource{}, nil
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
	ctx context.Context, id string, resServerID string, res Resource) error {
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
	ctx context.Context, uuid string, resServerID string, resID *string, action Action) error {
	return errImmutableStore
}

func (f *fileBasedResourceStore) GetAction(
	ctx context.Context, id string, resServerID string, resID *string) (Action, error) {
	// Search through all resource servers and their resources
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return Action{}, err
	}

	for _, item := range list {
		if rs, ok := item.Data.(*ResourceServer); ok {
			for _, res := range rs.Resources {
				for _, action := range res.Actions {
					actionID := fmt.Sprintf("%s_%s_%s", rs.ID, res.Handle, action.Handle)
					if actionID == id || action.Handle == id {
						return Action{
							ID:          actionID,
							Name:        action.Name,
							Handle:      action.Handle,
							Description: action.Description,
							Permission:  action.Permission,
						}, nil
					}
				}
			}
		}
	}

	return Action{}, errActionNotFound
}

func (f *fileBasedResourceStore) GetActionList(
	ctx context.Context, resServerID string, resID *string, limit, offset int) ([]Action, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	actions := []Action{}

	// Iterate through all resource servers
	for _, item := range list {
		if rs, ok := item.Data.(*ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			// Iterate through resources in this server
			for _, res := range rs.Resources {
				resUUID := fmt.Sprintf("%s_%s", rs.ID, res.Handle)

				// If resID is specified, only get actions from that resource
				if resID != nil && resUUID != *resID {
					continue
				}

				// Add all actions from this resource
				for _, action := range res.Actions {
					actionID := fmt.Sprintf("%s_%s_%s", rs.ID, res.Handle, action.Handle)
					actions = append(actions, Action{
						ID:          actionID,
						Name:        action.Name,
						Handle:      action.Handle,
						Description: action.Description,
						Permission:  action.Permission,
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
	if start >= len(actions) {
		return []Action{}, nil
	}
	if end > len(actions) {
		end = len(actions)
	}

	return actions[start:end], nil
}

func (f *fileBasedResourceStore) GetActionListCount(
	ctx context.Context, resServerID string, resID *string) (int, error) {
	actions, err := f.GetActionList(ctx, resServerID, resID, 1000, 0)
	if err != nil {
		return 0, err
	}
	return len(actions), nil
}

func (f *fileBasedResourceStore) UpdateAction(
	ctx context.Context, id string, resServerID string, resID *string, action Action) error {
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
		if rs, ok := item.Data.(*ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
			}

			// Iterate through resources in this server
			for _, res := range rs.Resources {
				resUUID := fmt.Sprintf("%s_%s", rs.ID, res.Handle)

				// If resID is specified, only check actions from that resource
				if resID != nil && resUUID != *resID {
					continue
				}

				// Check all actions from this resource
				for _, action := range res.Actions {
					if action.Handle == handle {
						return true, nil
					}
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
		if rs, ok := item.Data.(*ResourceServer); ok {
			// Skip if this is not the resource server we're looking for
			if rs.ID != resServerID {
				continue
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

func (f *fileBasedResourceStore) FindResourceServersByPermissions(
	ctx context.Context, permissions []string,
) ([]ResourceServer, error) {
	if len(permissions) == 0 {
		return []ResourceServer{}, nil
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	permSet := make(map[string]struct{}, len(permissions))
	for _, p := range permissions {
		permSet[p] = struct{}{}
	}

	matched := make([]ResourceServer, 0)
	for _, item := range list {
		rs, ok := item.Data.(*ResourceServer)
		if !ok || rs.Identifier == "" {
			continue
		}
		if containsAnyPermission(rs, permSet) {
			matched = append(matched, *rs)
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Identifier < matched[j].Identifier
	})
	return matched, nil
}

func containsAnyPermission(rs *ResourceServer, permSet map[string]struct{}) bool {
	for _, res := range rs.Resources {
		if _, ok := permSet[res.Permission]; ok {
			return true
		}
		for _, action := range res.Actions {
			if _, ok := permSet[action.Permission]; ok {
				return true
			}
		}
	}
	return false
}
