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

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// compositeResourceStore implements a composite store that combines file-based (immutable) and
// database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative resource servers (from YAML files) cannot be modified or deleted
type compositeResourceStore struct {
	fileStore resourceStoreInterface
	dbStore   resourceStoreInterface
}

// newCompositeResourceStore creates a new composite store with both file-based and database stores.
func newCompositeResourceStore(fileStore, dbStore resourceStoreInterface) *compositeResourceStore {
	return &compositeResourceStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// Resource Server operations

// CreateResourceServer creates a resource server in the database store.
func (c *compositeResourceStore) CreateResourceServer(
	ctx context.Context,
	id string,
	rs providers.ResourceServer,
) error {
	return c.dbStore.CreateResourceServer(ctx, id, rs)
}

// GetResourceServer retrieves a resource server from the composite store.
func (c *compositeResourceStore) GetResourceServer(ctx context.Context, id string) (providers.ResourceServer, error) {
	server, err := declarativeresource.CompositeGetHelper(
		func() (providers.ResourceServer, error) {
			s, err := c.dbStore.GetResourceServer(ctx, id)
			if err == nil {
				s.IsReadOnly = false
			}
			return s, err
		},
		func() (providers.ResourceServer, error) {
			s, err := c.fileStore.GetResourceServer(ctx, id)
			if err == nil {
				s.IsReadOnly = true
			}
			return s, err
		},
		errResourceServerNotFound,
	)
	return server, err
}

// GetResourceServerList returns a paginated, deduplicated list of resource servers from both stores.
func (c *compositeResourceStore) GetResourceServerList(
	ctx context.Context, limit, offset int) ([]providers.ResourceServer, error) {
	dbCount, err := c.dbStore.GetResourceServerListCount(ctx)
	if err != nil {
		return nil, err
	}

	fileCount, err := c.fileStore.GetResourceServerListCount(ctx)
	if err != nil {
		return nil, err
	}

	if dbCount == 0 && fileCount == 0 {
		return []providers.ResourceServer{}, nil
	}

	// Fetch all from both stores, then deduplicate before applying the cap.
	// This avoids spurious limit errors when IDs overlap across the two stores.
	dbServers, err := c.dbStore.GetResourceServerList(ctx, dbCount, 0)
	if err != nil {
		return nil, err
	}

	fileServers, err := c.fileStore.GetResourceServerList(ctx, fileCount, 0)
	if err != nil {
		return nil, err
	}

	resourceServers := mergeAndDeduplicateResourceServers(dbServers, fileServers)
	if len(resourceServers) > serverconst.MaxCompositeStoreRecords {
		return nil, errResultLimitExceededInCompositeMode
	}

	// Apply pagination on the deduplicated slice.
	start := offset
	if start > len(resourceServers) {
		return []providers.ResourceServer{}, nil
	}
	end := start + limit
	if end > len(resourceServers) {
		end = len(resourceServers)
	}
	return resourceServers[start:end], nil
}

// GetResourceServerListCount returns the deduplicated resource server count across both stores.
func (c *compositeResourceStore) GetResourceServerListCount(ctx context.Context) (int, error) {
	dbCount, err := c.dbStore.GetResourceServerListCount(ctx)
	if err != nil {
		return 0, err
	}

	fileCount, err := c.fileStore.GetResourceServerListCount(ctx)
	if err != nil {
		return 0, err
	}

	if dbCount == 0 && fileCount == 0 {
		return 0, nil
	}

	// Fetch all from both stores, deduplicate, then apply the cap.
	// Checking the raw sum before dedup would cause spurious errors when IDs overlap.
	dbServers, err := c.dbStore.GetResourceServerList(ctx, dbCount, 0)
	if err != nil {
		return 0, err
	}

	fileServers, err := c.fileStore.GetResourceServerList(ctx, fileCount, 0)
	if err != nil {
		return 0, err
	}

	merged := mergeAndDeduplicateResourceServers(dbServers, fileServers)
	if len(merged) > serverconst.MaxCompositeStoreRecords {
		return 0, errResultLimitExceededInCompositeMode
	}
	return len(merged), nil
}

// UpdateResourceServer updates a resource server in the database store.
func (c *compositeResourceStore) UpdateResourceServer(
	ctx context.Context,
	id string,
	rs providers.ResourceServer,
) error {
	return c.dbStore.UpdateResourceServer(ctx, id, rs)
}

// DeleteResourceServer deletes a resource server from the database store.
func (c *compositeResourceStore) DeleteResourceServer(ctx context.Context, id string) error {
	return c.dbStore.DeleteResourceServer(ctx, id)
}

// CheckResourceServerNameExists checks whether a resource server name exists in either store.
func (c *compositeResourceStore) CheckResourceServerNameExists(ctx context.Context, name string) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.CheckResourceServerNameExists(ctx, name) },
		func() (bool, error) { return c.dbStore.CheckResourceServerNameExists(ctx, name) },
	)
}

// CheckResourceServerIdentifierExists checks whether a resource server identifier exists in either store.
func (c *compositeResourceStore) CheckResourceServerIdentifierExists(
	ctx context.Context, identifier string) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.CheckResourceServerIdentifierExists(ctx, identifier) },
		func() (bool, error) { return c.dbStore.CheckResourceServerIdentifierExists(ctx, identifier) },
	)
}

// GetResourceServerByIdentifier retrieves a resource server by identifier from the composite store.
func (c *compositeResourceStore) GetResourceServerByIdentifier(
	ctx context.Context, identifier string) (providers.ResourceServer, error) {
	server, err := declarativeresource.CompositeGetHelper(
		func() (providers.ResourceServer, error) {
			s, err := c.dbStore.GetResourceServerByIdentifier(ctx, identifier)
			if err == nil {
				s.IsReadOnly = false
			}
			return s, err
		},
		func() (providers.ResourceServer, error) {
			s, err := c.fileStore.GetResourceServerByIdentifier(ctx, identifier)
			if err == nil {
				s.IsReadOnly = true
			}
			return s, err
		},
		errResourceServerNotFound,
	)
	return server, err
}

// CheckResourceServerHasDependencies checks whether a resource server has dependencies in either store.
func (c *compositeResourceStore) CheckResourceServerHasDependencies(
	ctx context.Context, resServerID string) (bool, error) {
	// Check in DB store first
	hasDeps, err := c.dbStore.CheckResourceServerHasDependencies(ctx, resServerID)
	if err != nil {
		return false, err
	}
	if hasDeps {
		return true, nil
	}

	// Also check in file store
	return c.fileStore.CheckResourceServerHasDependencies(ctx, resServerID)
}

// IsResourceServerDeclarative checks whether a resource server is defined in the file store.
func (c *compositeResourceStore) IsResourceServerDeclarative(id string) bool {
	return declarativeresource.CompositeIsDeclarativeHelper(
		id,
		func(id string) (bool, error) {
			_, err := c.fileStore.GetResourceServer(context.Background(), id)
			return err == nil, nil
		},
	)
}

// Resource operations

// CreateResource creates a resource in the database store.
func (c *compositeResourceStore) CreateResource(
	ctx context.Context, uuid string, resServerID string, parentID *string, res providers.Resource) error {
	return c.dbStore.CreateResource(ctx, uuid, resServerID, parentID, res)
}

// GetResource retrieves a resource from the composite store.
func (c *compositeResourceStore) GetResource(
	ctx context.Context, id string, resServerID string) (providers.Resource, error) {
	resource, err := declarativeresource.CompositeGetHelper(
		func() (providers.Resource, error) { return c.dbStore.GetResource(ctx, id, resServerID) },
		func() (providers.Resource, error) { return c.fileStore.GetResource(ctx, id, resServerID) },
		errResourceNotFound,
	)
	return resource, err
}

// GetResourceList returns a paginated, deduplicated resource list from both stores.
func (c *compositeResourceStore) GetResourceList(
	ctx context.Context, resServerID string, limit, offset int) ([]providers.Resource, error) {
	merged, err := c.getMergedResources(ctx, resServerID)
	if err != nil {
		return nil, err
	}

	// Apply pagination
	start := offset
	end := offset + limit
	if start > len(merged) {
		return []providers.Resource{}, nil
	}
	if end > len(merged) {
		end = len(merged)
	}

	return merged[start:end], nil
}

// GetResourceListByParent returns a paginated, deduplicated resource list for a parent from both stores.
func (c *compositeResourceStore) GetResourceListByParent(
	ctx context.Context, resServerID string, parentID *string, limit, offset int,
) ([]providers.Resource, error) {
	merged, err := c.getMergedResourcesByParent(ctx, resServerID, parentID)
	if err != nil {
		return nil, err
	}

	// Apply pagination
	start := offset
	end := offset + limit
	if start > len(merged) {
		return []providers.Resource{}, nil
	}
	if end > len(merged) {
		end = len(merged)
	}

	return merged[start:end], nil
}

// GetResourceListCount returns the deduplicated resource count across both stores.
func (c *compositeResourceStore) GetResourceListCount(ctx context.Context, resServerID string) (int, error) {
	merged, err := c.getMergedResources(ctx, resServerID)
	if err != nil {
		return 0, err
	}
	return len(merged), nil
}

// GetResourceListCountByParent returns the deduplicated resource count for a parent across both stores.
func (c *compositeResourceStore) GetResourceListCountByParent(
	ctx context.Context, resServerID string, parentID *string) (int, error) {
	merged, err := c.getMergedResourcesByParent(ctx, resServerID, parentID)
	if err != nil {
		return 0, err
	}
	return len(merged), nil
}

// UpdateResource updates a resource in the database store.
func (c *compositeResourceStore) UpdateResource(
	ctx context.Context, id string, resServerID string, res providers.Resource) error {
	return c.dbStore.UpdateResource(ctx, id, resServerID, res)
}

// UpdateResourcePermission updates a resource permission in the database store.
func (c *compositeResourceStore) UpdateResourcePermission(
	ctx context.Context, id string, resServerID string, permission string) error {
	return c.dbStore.UpdateResourcePermission(ctx, id, resServerID, permission)
}

// DeleteResource deletes a resource from the database store.
func (c *compositeResourceStore) DeleteResource(
	ctx context.Context, id string, resServerID string) error {
	return c.dbStore.DeleteResource(ctx, id, resServerID)
}

// CheckResourceHandleExists checks whether a resource handle exists in either store.
func (c *compositeResourceStore) CheckResourceHandleExists(
	ctx context.Context, resServerID string, handle string, parentID *string,
) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) {
			return c.fileStore.CheckResourceHandleExists(ctx, resServerID, handle, parentID)
		},
		func() (bool, error) {
			return c.dbStore.CheckResourceHandleExists(ctx, resServerID, handle, parentID)
		},
	)
}

// CheckResourceHasDependencies checks whether a resource has dependencies in the database store.
func (c *compositeResourceStore) CheckResourceHasDependencies(ctx context.Context, resID string) (bool, error) {
	return c.dbStore.CheckResourceHasDependencies(ctx, resID)
}

// CheckCircularDependency checks whether assigning a new parent would create a circular dependency.
func (c *compositeResourceStore) CheckCircularDependency(
	ctx context.Context, resourceID, newParentID string) (bool, error) {
	return c.dbStore.CheckCircularDependency(ctx, resourceID, newParentID)
}

// Action operations

// CreateAction creates an action in the database store.
func (c *compositeResourceStore) CreateAction(
	ctx context.Context, uuid string, resServerID string, resID *string, action providers.Action) error {
	return c.dbStore.CreateAction(ctx, uuid, resServerID, resID, action)
}

// GetAction retrieves an action from the composite store.
func (c *compositeResourceStore) GetAction(
	ctx context.Context, id string, resServerID string, resID *string) (providers.Action, error) {
	action, err := declarativeresource.CompositeGetHelper(
		func() (providers.Action, error) { return c.dbStore.GetAction(ctx, id, resServerID, resID) },
		func() (providers.Action, error) { return c.fileStore.GetAction(ctx, id, resServerID, resID) },
		errActionNotFound,
	)
	return action, err
}

// GetActionList returns a paginated, deduplicated action list from both stores.
func (c *compositeResourceStore) GetActionList(
	ctx context.Context, resServerID string, resID *string, kind providers.ActionKind, limit, offset int,
) ([]providers.Action, error) {
	merged, err := c.getMergedActions(ctx, resServerID, resID, kind)
	if err != nil {
		return nil, err
	}

	// Apply pagination
	start := offset
	end := offset + limit
	if start > len(merged) {
		return []providers.Action{}, nil
	}
	if end > len(merged) {
		end = len(merged)
	}

	return merged[start:end], nil
}

// GetActionListCount returns the deduplicated action count across both stores, optionally filtered by kind.
func (c *compositeResourceStore) GetActionListCount(
	ctx context.Context, resServerID string, resID *string, kind providers.ActionKind) (int, error) {
	merged, err := c.getMergedActions(ctx, resServerID, resID, kind)
	if err != nil {
		return 0, err
	}
	return len(merged), nil
}

// getMergedResources returns the deduplicated resource list across both stores.
func (c *compositeResourceStore) getMergedResources(
	ctx context.Context,
	resServerID string,
) ([]providers.Resource, error) {
	dbCount, err := c.dbStore.GetResourceListCount(ctx, resServerID)
	if err != nil {
		return nil, err
	}

	fileCount, err := c.fileStore.GetResourceListCount(ctx, resServerID)
	if err != nil {
		return nil, err
	}

	resources, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return dbCount, nil },
		func() (int, error) { return fileCount, nil },
		func(count int) ([]providers.Resource, error) {
			return c.dbStore.GetResourceList(ctx, resServerID, count, 0)
		},
		func(count int) ([]providers.Resource, error) {
			return c.fileStore.GetResourceList(ctx, resServerID, count, 0)
		},
		mergeAndDeduplicateResources,
		dbCount+fileCount,
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}

	return resources, nil
}

// getMergedResourcesByParent returns the deduplicated resource list for a parent across both stores.
func (c *compositeResourceStore) getMergedResourcesByParent(
	ctx context.Context,
	resServerID string,
	parentID *string,
) ([]providers.Resource, error) {
	dbCount, err := c.dbStore.GetResourceListCountByParent(ctx, resServerID, parentID)
	if err != nil {
		return nil, err
	}

	fileCount, err := c.fileStore.GetResourceListCountByParent(ctx, resServerID, parentID)
	if err != nil {
		return nil, err
	}

	return mergeCompositeListWithLimit(
		dbCount,
		fileCount,
		func(count int) ([]providers.Resource, error) {
			return c.dbStore.GetResourceListByParent(ctx, resServerID, parentID, count, 0)
		},
		func(count int) ([]providers.Resource, error) {
			return c.fileStore.GetResourceListByParent(ctx, resServerID, parentID, count, 0)
		},
		mergeAndDeduplicateResources,
	)
}

// getMergedActions returns the deduplicated action list across both stores, optionally filtered by kind.
func (c *compositeResourceStore) getMergedActions(
	ctx context.Context,
	resServerID string,
	resID *string,
	kind providers.ActionKind,
) ([]providers.Action, error) {
	dbCount, err := c.dbStore.GetActionListCount(ctx, resServerID, resID, kind)
	if err != nil {
		return nil, err
	}

	fileCount, err := c.fileStore.GetActionListCount(ctx, resServerID, resID, kind)
	if err != nil {
		return nil, err
	}

	return mergeCompositeListWithLimit(
		dbCount,
		fileCount,
		func(count int) ([]providers.Action, error) {
			return c.dbStore.GetActionList(ctx, resServerID, resID, kind, count, 0)
		},
		func(count int) ([]providers.Action, error) {
			return c.fileStore.GetActionList(ctx, resServerID, resID, kind, count, 0)
		},
		mergeAndDeduplicateActions,
	)
}

// mergeCompositeListWithLimit merges composite store lists and returns an error when the result limit is exceeded.
func mergeCompositeListWithLimit[T any](
	dbCount int,
	fileCount int,
	dbListFn func(count int) ([]T, error),
	fileListFn func(count int) ([]T, error),
	mergeFn func(dbList []T, fileList []T) []T,
) ([]T, error) {
	merged, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return dbCount, nil },
		func() (int, error) { return fileCount, nil },
		dbListFn,
		fileListFn,
		mergeFn,
		dbCount+fileCount,
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}

	return merged, nil
}

// UpdateAction updates an action in the database store.
func (c *compositeResourceStore) UpdateAction(
	ctx context.Context, id string, resServerID string, resID *string, action providers.Action) error {
	return c.dbStore.UpdateAction(ctx, id, resServerID, resID, action)
}

// UpdateActionPermission updates an action permission in the database store.
func (c *compositeResourceStore) UpdateActionPermission(
	ctx context.Context, id string, resServerID string, resID *string, permission string) error {
	return c.dbStore.UpdateActionPermission(ctx, id, resServerID, resID, permission)
}

// DeleteAction deletes an action from the database store.
func (c *compositeResourceStore) DeleteAction(
	ctx context.Context, id string, resServerID string, resID *string) error {
	return c.dbStore.DeleteAction(ctx, id, resServerID, resID)
}

// IsActionExist checks whether an action exists in either store.
func (c *compositeResourceStore) IsActionExist(
	ctx context.Context, id string, resServerID string, resID *string) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.IsActionExist(ctx, id, resServerID, resID) },
		func() (bool, error) { return c.dbStore.IsActionExist(ctx, id, resServerID, resID) },
	)
}

// CheckActionHandleExists checks whether an action handle exists in either store.
func (c *compositeResourceStore) CheckActionHandleExists(
	ctx context.Context, resServerID string, resID *string, handle string,
) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) {
			return c.fileStore.CheckActionHandleExists(ctx, resServerID, resID, handle)
		},
		func() (bool, error) {
			return c.dbStore.CheckActionHandleExists(ctx, resServerID, resID, handle)
		},
	)
}

// ValidatePermissions returns permissions that are invalid in both stores.
func (c *compositeResourceStore) ValidatePermissions(
	ctx context.Context, resServerID string, permissions []string) ([]string, error) {
	// Call db store
	dbInvalid, err := c.dbStore.ValidatePermissions(ctx, resServerID, permissions)
	if err != nil {
		return nil, err
	}

	// Call file store (declarative store)
	fileInvalid, err := c.fileStore.ValidatePermissions(ctx, resServerID, permissions)
	if err != nil {
		return nil, err
	}

	// Create set of file invalid permissions for efficient lookup
	fileInvalidSet := make(map[string]struct{})
	for _, perm := range fileInvalid {
		fileInvalidSet[perm] = struct{}{}
	}

	// Return only permissions that are invalid in both stores (intersection)
	// A permission is valid if present in either store
	var result []string
	for _, perm := range dbInvalid {
		if _, ok := fileInvalidSet[perm]; ok {
			result = append(result, perm)
		}
	}

	return result, nil
}

// FindResourceServersByPermissions finds resource servers that contain any of the given permissions.
func (c *compositeResourceStore) FindResourceServersByPermissions(
	ctx context.Context, permissions []string,
) ([]providers.ResourceServer, error) {
	if len(permissions) == 0 {
		return []providers.ResourceServer{}, nil
	}

	dbServers, err := c.dbStore.FindResourceServersByPermissions(ctx, permissions)
	if err != nil {
		return nil, err
	}

	fileServers, err := c.fileStore.FindResourceServersByPermissions(ctx, permissions)
	if err != nil {
		return nil, err
	}

	return mergeAndDeduplicateResourceServers(dbServers, fileServers), nil
}

// mergeAndDeduplicateResourceServers merges resource servers with database entries taking precedence.
func mergeAndDeduplicateResourceServers(dbServers, fileServers []providers.ResourceServer) []providers.ResourceServer {
	seen := make(map[string]bool)
	result := make([]providers.ResourceServer, 0, len(dbServers)+len(fileServers))

	// Add DB servers first (they take precedence) - mark as mutable (isReadOnly=false)
	for i := range dbServers {
		if !seen[dbServers[i].ID] {
			seen[dbServers[i].ID] = true
			dbServers[i].IsReadOnly = false
			result = append(result, dbServers[i])
		}
	}

	// Add file servers if not already present - mark as immutable (isReadOnly=true)
	for i := range fileServers {
		if !seen[fileServers[i].ID] {
			seen[fileServers[i].ID] = true
			fileServers[i].IsReadOnly = true
			result = append(result, fileServers[i])
		}
	}

	return result
}

// mergeAndDeduplicateResources merges resources with database entries taking precedence.
func mergeAndDeduplicateResources(dbResources, fileResources []providers.Resource) []providers.Resource {
	seen := make(map[string]bool)
	result := make([]providers.Resource, 0, len(dbResources)+len(fileResources))

	// Add DB resources first
	for i := range dbResources {
		if !seen[dbResources[i].ID] {
			seen[dbResources[i].ID] = true
			result = append(result, dbResources[i])
		}
	}

	// Add file resources if not already present
	for i := range fileResources {
		if !seen[fileResources[i].ID] {
			seen[fileResources[i].ID] = true
			result = append(result, fileResources[i])
		}
	}

	return result
}

// mergeAndDeduplicateActions merges actions with database entries taking precedence.
func mergeAndDeduplicateActions(dbActions, fileActions []providers.Action) []providers.Action {
	seen := make(map[string]bool)
	result := make([]providers.Action, 0, len(dbActions)+len(fileActions))

	// Add DB actions first
	for i := range dbActions {
		if !seen[dbActions[i].ID] {
			seen[dbActions[i].ID] = true
			result = append(result, dbActions[i])
		}
	}

	// Add file actions if not already present
	for i := range fileActions {
		if !seen[fileActions[i].ID] {
			seen[fileActions[i].ID] = true
			result = append(result, fileActions[i])
		}
	}

	return result
}
