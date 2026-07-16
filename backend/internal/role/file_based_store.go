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

package role

import (
	"context"
	"errors"
	"strings"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

type fileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newFileBasedStore creates a new file-based store for roles.
func newFileBasedStore() (roleStoreInterface, transaction.Transactioner) {
	return &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeRole),
	}, transaction.NewNoOpTransactioner()
}

// Create implements declarativeresource.Storer interface for resource loader.
func (f *fileBasedStore) Create(id string, data interface{}) error {
	role, ok := data.(*RoleWithPermissionsAndAssignments)
	if !ok {
		return ErrRoleDataCorrupted
	}
	if role.ID == "" {
		role.ID = id
	}
	return f.GenericFileBasedStore.Create(id, role)
}

// GetRoleListCount returns the total count of roles in the file-based store.
func (f *fileBasedStore) GetRoleListCount(ctx context.Context) (int, error) {
	return f.GenericFileBasedStore.Count()
}

// GetRoleList returns the list of roles from the file-based store.
func (f *fileBasedStore) GetRoleList(ctx context.Context, limit, offset int) ([]Role, error) {
	if limit <= 0 {
		return []Role{}, nil
	}
	if offset < 0 {
		offset = 0
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	roles := make([]Role, 0, len(list))
	for _, item := range list {
		roleData, err := roleFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			// Log warning for malformed declarative entry
			log.GetLogger().Warn(ctx, "Skipping malformed role in GetRoleList",
				log.String("roleID", item.ID.ID),
				log.Error(err))
			continue
		}
		roles = append(roles, Role{
			ID:          roleData.ID,
			Name:        roleData.Name,
			Description: roleData.Description,
			OUID:        roleData.OUID,
		})
	}

	start := offset
	if start >= len(roles) {
		return []Role{}, nil
	}
	end := start + limit
	if end > len(roles) {
		end = len(roles)
	}

	return roles[start:end], nil
}

// GetRoleListCountByOUID returns the count of roles belonging to the given organization unit
// in the file-based store.
func (f *fileBasedStore) GetRoleListCountByOUID(ctx context.Context, ouID string) (int, error) {
	roles, err := f.rolesByOUID(ctx, ouID)
	if err != nil {
		return 0, err
	}
	return len(roles), nil
}

// GetRoleListByOUID returns the list of roles belonging to the given organization unit from the
// file-based store, with pagination.
func (f *fileBasedStore) GetRoleListByOUID(ctx context.Context, ouID string, limit, offset int) ([]Role, error) {
	if limit <= 0 {
		return []Role{}, nil
	}
	if offset < 0 {
		offset = 0
	}

	roles, err := f.rolesByOUID(ctx, ouID)
	if err != nil {
		return nil, err
	}

	start := offset
	if start >= len(roles) {
		return []Role{}, nil
	}
	end := start + limit
	if end > len(roles) {
		end = len(roles)
	}

	return roles[start:end], nil
}

// rolesByOUID returns all roles belonging to the given organization unit from the file-based store.
func (f *fileBasedStore) rolesByOUID(ctx context.Context, ouID string) ([]Role, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	roles := make([]Role, 0, len(list))
	for _, item := range list {
		roleData, err := roleFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			// Log warning for malformed declarative entry
			log.GetLogger().Warn(ctx, "Skipping malformed role in rolesByOUID",
				log.String("roleID", item.ID.ID),
				log.Error(err))
			continue
		}
		if roleData.OUID != ouID {
			continue
		}
		roles = append(roles, Role{
			ID:          roleData.ID,
			Name:        roleData.Name,
			Description: roleData.Description,
			OUID:        roleData.OUID,
		})
	}

	return roles, nil
}

// CreateRole is not supported in file-based store.
func (f *fileBasedStore) CreateRole(ctx context.Context, id string, role RoleCreationDetail) error {
	return errors.New("CreateRole is not supported in file-based store")
}

// GetRole returns a role from the file-based store.
func (f *fileBasedStore) GetRole(ctx context.Context, id string) (RoleWithPermissions, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		// Distinguish "not found" from other storage errors
		if isEntityNotFoundError(err) {
			return RoleWithPermissions{}, ErrRoleNotFound
		}
		// Propagate other storage errors
		return RoleWithPermissions{}, err
	}

	roleData, err := roleFromDeclarativeData(id, data)
	if err != nil {
		// Propagate parsing errors
		return RoleWithPermissions{}, err
	}

	return RoleWithPermissions{
		ID:          roleData.ID,
		Name:        roleData.Name,
		Description: roleData.Description,
		OUID:        roleData.OUID,
		Permissions: roleData.Permissions,
	}, nil
}

// IsRoleExist checks if a role exists in the file-based store.
func (f *fileBasedStore) IsRoleExist(ctx context.Context, id string) (bool, error) {
	_, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		// Distinguish "not found" from other storage errors
		if isEntityNotFoundError(err) {
			return false, nil
		}
		// Propagate other storage errors
		return false, err
	}
	return true, nil
}

// GetRoleAssignments returns role assignments from the file-based store.
func (f *fileBasedStore) GetRoleAssignments(
	ctx context.Context,
	id string,
	limit, offset int,
) ([]RoleAssignment, error) {
	return f.GetRoleAssignmentsByType(ctx, id, limit, offset, "")
}

// GetRoleAssignmentsByType returns assignments for a role filtered by assignee type.
func (f *fileBasedStore) GetRoleAssignmentsByType(
	ctx context.Context,
	id string,
	limit, offset int,
	assigneeType string,
) ([]RoleAssignment, error) {
	if limit <= 0 {
		return []RoleAssignment{}, nil
	}
	if offset < 0 {
		offset = 0
	}

	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		// Distinguish "not found" from other storage errors
		if isEntityNotFoundError(err) {
			return []RoleAssignment{}, nil
		}
		// Propagate other storage errors
		return nil, err
	}

	roleData, err := roleFromDeclarativeData(id, data)
	if err != nil {
		// Propagate parsing errors
		return nil, err
	}

	assignments := filterAssignmentsByType(roleData.Assignments, assigneeType)
	start := offset
	if start >= len(assignments) {
		return []RoleAssignment{}, nil
	}
	end := start + limit
	if end > len(assignments) {
		end = len(assignments)
	}

	return assignments[start:end], nil
}

// GetRoleAssignmentsCount returns the assignment count for a role in the file-based store.
func (f *fileBasedStore) GetRoleAssignmentsCount(ctx context.Context, id string) (int, error) {
	return f.GetRoleAssignmentsCountByType(ctx, id, "")
}

// GetRoleAssignmentsCountByType returns the assignment count for a role filtered by type.
func (f *fileBasedStore) GetRoleAssignmentsCountByType(
	ctx context.Context, id string, assigneeType string,
) (int, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		// Distinguish "not found" from other storage errors
		if isEntityNotFoundError(err) {
			return 0, nil
		}
		// Propagate other storage errors
		return 0, err
	}

	roleData, err := roleFromDeclarativeData(id, data)
	if err != nil {
		// Propagate parsing errors
		return 0, err
	}

	return len(filterAssignmentsByType(roleData.Assignments, assigneeType)), nil
}

// filterAssignmentsByType filters assignments by assignee type. If assigneeType is empty, all assignments are returned.
func filterAssignmentsByType(assignments []RoleAssignment, assigneeType string) []RoleAssignment {
	if assigneeType == "" {
		return assignments
	}
	filtered := make([]RoleAssignment, 0)
	for _, a := range assignments {
		if string(a.Type) == assigneeType {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// UpdateRole is not supported in file-based store.
func (f *fileBasedStore) UpdateRole(ctx context.Context, id string, role RoleUpdateDetail) error {
	return errors.New("UpdateRole is not supported in file-based store")
}

// DeleteRole is not supported in file-based store.
func (f *fileBasedStore) DeleteRole(ctx context.Context, id string) error {
	return errors.New("DeleteRole is not supported in file-based store")
}

// DeleteAssignmentsByRoleID is not supported in file-based store.
func (f *fileBasedStore) DeleteAssignmentsByRoleID(ctx context.Context, id string) error {
	return errors.New("DeleteAssignmentsByRoleID is not supported in file-based store")
}

// DeleteAssignmentsByAssignee is a no-op for the file-based store: declarative roles hold no
// mutable runtime assignments to cascade-delete, so there is nothing to remove.
func (f *fileBasedStore) DeleteAssignmentsByAssignee(
	_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}

// AddAssignments is not supported in file-based store.
func (f *fileBasedStore) AddAssignments(ctx context.Context, id string, assignments []RoleAssignment) error {
	return errors.New("AddAssignments is not supported in file-based store")
}

// RemoveAssignments is not supported in file-based store.
func (f *fileBasedStore) RemoveAssignments(ctx context.Context, id string, assignments []RoleAssignment) error {
	return errors.New("RemoveAssignments is not supported in file-based store")
}

// CheckRoleNameExists checks if a role with the given name exists in the file-based store.
func (f *fileBasedStore) CheckRoleNameExists(ctx context.Context, ouID, name string) (bool, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return false, err
	}

	for _, item := range list {
		roleData, err := roleFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			// Log warning for malformed declarative entry
			log.GetLogger().Warn(ctx, "Skipping malformed role in CheckRoleNameExists",
				log.String("roleID", item.ID.ID),
				log.Error(err))
			continue
		}
		if roleData.OUID == ouID && roleData.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// CheckRoleNameExistsExcludingID checks for a role name conflict excluding a specific role ID.
func (f *fileBasedStore) CheckRoleNameExistsExcludingID(
	ctx context.Context,
	ouID, name, excludeRoleID string,
) (bool, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return false, err
	}

	for _, item := range list {
		roleData, err := roleFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			// Log warning for malformed declarative entry
			log.GetLogger().Warn(ctx, "Skipping malformed role in CheckRoleNameExistsExcludingID",
				log.String("roleID", item.ID.ID),
				log.Error(err))
			continue
		}
		if roleData.ID == excludeRoleID {
			continue
		}
		if roleData.OUID == ouID && roleData.Name == name {
			return true, nil
		}
	}

	return false, nil
}

// GetAuthorizedPermissionsByResourceServer returns permissions from roles assigned to the entity or groups in
// the file store, scoped to a resource server when provided.
func (f *fileBasedStore) GetAuthorizedPermissionsByResourceServer(
	ctx context.Context,
	entityID string,
	groupIDs []string,
	resourceServerID string,
	requestPermissions []string,
) ([]string, error) {
	if len(requestPermissions) == 0 {
		return []string{}, nil
	}
	if entityID == "" && len(groupIDs) == 0 {
		return []string{}, nil
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	requestedSet := make(map[string]bool, len(requestPermissions))
	for _, perm := range requestPermissions {
		requestedSet[perm] = true
	}

	groupSet := make(map[string]bool, len(groupIDs))
	for _, groupID := range groupIDs {
		groupSet[groupID] = true
	}

	permitted := make(map[string]bool)
	for _, item := range list {
		roleData, err := roleFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			// Log warning for malformed declarative entry
			log.GetLogger().Warn(ctx, "Skipping malformed role in GetAuthorizedPermissions",
				log.String("roleID", item.ID.ID),
				log.Error(err))
			continue
		}
		if !matchesAssignee(roleData.Assignments, entityID, groupSet) {
			continue
		}
		for _, resourcePerms := range roleData.Permissions {
			if resourceServerID != "" && resourcePerms.ResourceServerID != resourceServerID {
				continue
			}
			for _, perm := range resourcePerms.Permissions {
				if requestedSet[perm] {
					permitted[perm] = true
				}
			}
		}
	}

	result := make([]string, 0, len(permitted))
	for _, perm := range requestPermissions {
		if permitted[perm] {
			result = append(result, perm)
		}
	}
	return result, nil
}

// GetUserRoles retrieves the names of roles assigned to an entity directly and/or through group membership.
func (f *fileBasedStore) GetUserRoles(
	ctx context.Context, entityID string, groupIDs []string,
) ([]string, error) {
	if entityID == "" && len(groupIDs) == 0 {
		return []string{}, nil
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	groupSet := make(map[string]bool, len(groupIDs))
	for _, groupID := range groupIDs {
		groupSet[groupID] = true
	}

	roleNames := make([]string, 0)
	for _, item := range list {
		roleData, err := roleFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			log.GetLogger().Warn(ctx, "Skipping malformed role in GetUserRoles",
				log.String("roleID", item.ID.ID),
				log.Error(err))
			continue
		}
		if !matchesAssignee(roleData.Assignments, entityID, groupSet) {
			continue
		}
		roleNames = append(roleNames, roleData.Name)
	}

	return roleNames, nil
}

// GetEntityRoleIDs is a no-op for the file-based store. Role assignments are persisted in the
// database store and queried from there by the composite store; the file store has no
// independent record of API-added assignments. Returning an empty slice keeps the composite
// merge correct (callers union the result with the DB store's output).
func (f *fileBasedStore) GetEntityRoleIDs(
	ctx context.Context, entityID string, groupIDs []string,
) ([]string, error) {
	return []string{}, nil
}

// IsRoleDeclarative returns true for roles in the file-based store because they are declarative.
func (f *fileBasedStore) IsRoleDeclarative(ctx context.Context, roleID string) (bool, error) {
	exists, err := f.IsRoleExist(ctx, roleID)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// roleFromDeclarativeData converts raw data from the declarative store into a RoleWithPermissionsAndAssignments struct.
func roleFromDeclarativeData(id string, data interface{}) (RoleWithPermissionsAndAssignments, error) {
	role, ok := data.(*RoleWithPermissionsAndAssignments)
	if !ok || role == nil {
		declarativeresource.LogTypeAssertionError("role", id)
		return RoleWithPermissionsAndAssignments{}, ErrRoleDataCorrupted
	}

	return *role, nil
}

// isEntityNotFoundError checks if an error indicates an entity was not found.
// This function checks for error messages from both GenericFileBasedStore
// and the underlying entity store.
func isEntityNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return errMsg == "entity not found" || strings.Contains(errMsg, "not found")
}

// matchesAssignee returns true when the entity or any of the entity's groups is assigned.
func matchesAssignee(assignments []RoleAssignment, entityID string, groupSet map[string]bool) bool {
	for _, assignment := range assignments {
		if assignment.Type == assigneeTypeEntity && assignment.ID == entityID {
			return true
		}
		if assignment.Type == AssigneeTypeGroup && groupSet[assignment.ID] {
			return true
		}
	}
	return false
}
