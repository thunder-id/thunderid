/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// resourceStoreInterface defines the interface for resource store operations.
type resourceStoreInterface interface {
	// Resource Server operations
	CreateResourceServer(ctx context.Context, id string, rs ResourceServer) error
	GetResourceServer(ctx context.Context, id string) (ResourceServer, error)
	GetResourceServerList(ctx context.Context, limit, offset int) ([]ResourceServer, error)
	GetResourceServerListCount(ctx context.Context) (int, error)
	UpdateResourceServer(ctx context.Context, id string, rs ResourceServer) error
	DeleteResourceServer(ctx context.Context, id string) error
	CheckResourceServerNameExists(ctx context.Context, name string) (bool, error)
	CheckResourceServerHandleExists(ctx context.Context, handle string) (bool, error)
	CheckResourceServerIdentifierExists(ctx context.Context, identifier string) (bool, error)
	GetResourceServerByIdentifier(ctx context.Context, identifier string) (ResourceServer, error)
	CheckResourceServerHasDependencies(ctx context.Context, resServerID string) (bool, error)
	IsResourceServerDeclarative(id string) bool

	// Resource operations
	CreateResource(ctx context.Context, uuid string, resServerID string, parentID *string, res Resource) error
	GetResource(ctx context.Context, id string, resServerID string) (Resource, error)
	GetResourceList(ctx context.Context, resServerID string, limit, offset int) ([]Resource, error)
	GetResourceListByParent(
		ctx context.Context, resServerID string, parentID *string, limit, offset int,
	) ([]Resource, error)
	GetResourceListCount(ctx context.Context, resServerID string) (int, error)
	GetResourceListCountByParent(ctx context.Context, resServerID string, parentID *string) (int, error)
	UpdateResource(ctx context.Context, id string, resServerID string, res Resource) error
	UpdateResourcePermission(ctx context.Context, id string, resServerID string, permission string) error
	DeleteResource(ctx context.Context, id string, resServerID string) error
	CheckResourceHandleExists(
		ctx context.Context, resServerID string, handle string, parentID *string,
	) (bool, error)
	CheckResourceHasDependencies(ctx context.Context, resID string) (bool, error)
	CheckCircularDependency(ctx context.Context, resourceID, newParentID string) (bool, error)

	// Action operations
	CreateAction(ctx context.Context, uuid string, resServerID string, resID *string, action Action) error
	GetAction(ctx context.Context, id string, resServerID string, resID *string) (Action, error)
	GetActionList(ctx context.Context, resServerID string, resID *string, limit, offset int) ([]Action, error)
	GetActionListCount(ctx context.Context, resServerID string, resID *string) (int, error)
	UpdateAction(ctx context.Context, id string, resServerID string, resID *string, action Action) error
	UpdateActionPermission(ctx context.Context, id string, resServerID string, resID *string, permission string) error
	DeleteAction(ctx context.Context, id string, resServerID string, resID *string) error
	IsActionExist(ctx context.Context, id string, resServerID string, resID *string) (bool, error)
	CheckActionHandleExists(
		ctx context.Context, resServerID string, resID *string, handle string,
	) (bool, error)
	ValidatePermissions(ctx context.Context, resServerID string, permissions []string) ([]string, error)
	FindResourceServersByPermissions(ctx context.Context, permissions []string) ([]ResourceServer, error)
}

// resourceStore is the default implementation of resourceStoreInterface.
type resourceStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// resourceServerProperties represents the JSON structure of PROPERTIES column.
type resourceServerProperties struct {
	Delimiter string `json:"delimiter"`
}

// newResourceStore creates a new instance of resourceStore.
func newResourceStore() (resourceStoreInterface, transaction.Transactioner, error) {
	dbProvider := provider.GetDBProvider()
	transactioner, err := dbProvider.GetConfigDBTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &resourceStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}, transactioner, nil
}

// CreateResourceServer creates a new resource server in the database.
func (s *resourceStore) CreateResourceServer(ctx context.Context, id string, rs ResourceServer) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(
			ctx,
			queryCreateResourceServer,
			id,
			rs.OUID,
			rs.Name,
			rs.Description,
			resolveNullableString(rs.Handle),
			resolveNullableString(rs.Identifier),
			buildPropertiesJSON(rs),
			s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to create resource server: %w", err)
		}

		return nil
	})
}

// GetResourceServer retrieves a resource server by UUID.
func (s *resourceStore) GetResourceServer(ctx context.Context, id string) (ResourceServer, error) {
	var rs ResourceServer
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryGetResourceServerByID, id, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get resource server: %w", err)
		}

		if len(results) == 0 {
			return errResourceServerNotFound
		}

		rs, err = buildResourceServerFromResultRow(results[0])
		return err
	})
	return rs, err
}

// GetResourceServerList retrieves a list of resource servers with pagination.
func (s *resourceStore) GetResourceServerList(ctx context.Context, limit, offset int) ([]ResourceServer, error) {
	var resourceServers []ResourceServer
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryGetResourceServerList, limit, offset, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get resource server list: %w", err)
		}

		resourceServers = make([]ResourceServer, 0, len(results))
		for _, row := range results {
			rs, err := buildResourceServerFromResultRow(row)
			if err != nil {
				return fmt.Errorf("failed to build resource server: %w", err)
			}
			resourceServers = append(resourceServers, rs)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return resourceServers, nil
}

// GetResourceServerListCount retrieves the total count of resource servers.
func (s *resourceStore) GetResourceServerListCount(ctx context.Context) (int, error) {
	var count int
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryGetResourceServerListCount, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get resource server count: %w", err)
		}

		count, err = parseCountResult(results)
		return err
	})
	return count, err
}

// UpdateResourceServer updates a resource server.
func (s *resourceStore) UpdateResourceServer(ctx context.Context, id string, rs ResourceServer) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(
			ctx,
			queryUpdateResourceServer,
			rs.OUID,
			rs.Name,
			rs.Description,
			resolveNullableString(rs.Handle),
			resolveNullableString(rs.Identifier),
			buildPropertiesJSON(rs),
			id,
			s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to update resource server: %w", err)
		}

		return nil
	})
}

// DeleteResourceServer deletes a resource server.
func (s *resourceStore) DeleteResourceServer(ctx context.Context, id string) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, queryDeleteResourceServer, id, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to delete resource server: %w", err)
		}

		return nil
	})
}

// CheckResourceServerNameExists checks if a resource server name exists.
func (s *resourceStore) CheckResourceServerNameExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryCheckResourceServerNameExists, name, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to check resource server name: %w", err)
		}

		exists, err = parseBoolFromCount(results)
		return err
	})
	return exists, err
}

// CheckResourceServerHandleExists checks if a resource server handle exists.
func (s *resourceStore) CheckResourceServerHandleExists(ctx context.Context, handle string) (bool, error) {
	var exists bool
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryCheckResourceServerHandleExists, handle, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to check resource server handle: %w", err)
		}

		exists, err = parseBoolFromCount(results)
		return err
	})
	return exists, err
}

// CheckResourceServerIdentifierExists checks if a resource server identifier exists.
func (s *resourceStore) CheckResourceServerIdentifierExists(ctx context.Context, identifier string) (bool, error) {
	var exists bool
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryCheckResourceServerIdentifierExists, identifier, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to check resource server identifier: %w", err)
		}

		exists, err = parseBoolFromCount(results)
		return err
	})
	return exists, err
}

// GetResourceServerByIdentifier retrieves a resource server by its identifier.
func (s *resourceStore) GetResourceServerByIdentifier(ctx context.Context, identifier string) (ResourceServer, error) {
	var rs ResourceServer
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryGetResourceServerByIdentifier, identifier, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get resource server by identifier: %w", err)
		}

		if len(results) == 0 {
			return errResourceServerNotFound
		}

		rs, err = buildResourceServerFromResultRow(results[0])
		return err
	})
	return rs, err
}

// CheckResourceServerHasDependencies checks if resource server has dependencies.
func (s *resourceStore) CheckResourceServerHasDependencies(ctx context.Context, resServerID string) (bool, error) {
	var hasDeps bool
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(
			ctx, queryCheckResourceServerHasDependencies, resServerID, s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to check dependencies: %w", err)
		}

		hasDeps, err = parseBoolFromCount(results)
		return err
	})
	return hasDeps, err
}

// IsResourceServerDeclarative checks if a resource server is declarative (immutable).
// For database store, all resource servers are mutable, so this always returns false.
func (s *resourceStore) IsResourceServerDeclarative(id string) bool {
	return false
}

// Resource Store Methods

// CreateResource creates a new resource.
func (s *resourceStore) CreateResource(
	ctx context.Context,
	uuid string,
	resServerID string,
	parentID *string,
	res Resource,
) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(
			ctx,
			queryCreateResource,
			uuid,            // $1: RESOURCE_ID (UUID)
			resServerID,     // $2: RESOURCE_SERVER_ID (UUID FK)
			res.Name,        // $3: NAME
			res.Handle,      // $4: HANDLE
			res.Description, // $5: DESCRIPTION
			res.Permission,  // $6: PERMISSION
			"{}",            // $7: PROPERTIES (empty JSON).
			parentID,        // $8: PARENT_RESOURCE_ID (UUID FK or NULL)
			s.deploymentID,  // $9: DEPLOYMENT_ID
		)
		if err != nil {
			return fmt.Errorf("failed to create resource: %w", err)
		}

		return nil
	})
}

// GetResource retrieves a resource by UUID.
func (s *resourceStore) GetResource(ctx context.Context, id string, resServerID string) (Resource, error) {
	var res Resource
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryGetResourceByID, id, resServerID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get resource: %w", err)
		}

		if len(results) == 0 {
			return errResourceNotFound
		}

		res, err = buildResourceFromResultRow(results[0])
		return err
	})
	return res, err
}

// GetResourceList retrieves all resources for a resource server.
func (s *resourceStore) GetResourceList(
	ctx context.Context, resServerID string, limit, offset int,
) ([]Resource, error) {
	var resources []Resource
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(
			ctx, queryGetResourceList, resServerID, limit, offset, s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to get resource list: %w", err)
		}

		resources = make([]Resource, 0, len(results))
		for _, row := range results {
			res, err := buildResourceFromResultRow(row)
			if err != nil {
				return fmt.Errorf("failed to build resource: %w", err)
			}
			resources = append(resources, res)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return resources, nil
}

// GetResourceListByParent retrieves resources filtered by parent.
func (s *resourceStore) GetResourceListByParent(
	ctx context.Context,
	resServerID string, parentID *string, limit, offset int,
) ([]Resource, error) {
	var resources []Resource
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		var results []map[string]interface{}
		var err error
		// Treat nil parent ID as top-level resources
		if parentID == nil {
			results, err = dbClient.QueryContext(
				ctx,
				queryGetResourceListByNullParent, resServerID, limit, offset, s.deploymentID,
			)
		} else {
			results, err = dbClient.QueryContext(
				ctx,
				queryGetResourceListByParent, resServerID, *parentID, limit, offset, s.deploymentID,
			)
		}

		if err != nil {
			return fmt.Errorf("failed to get resource list by parent: %w", err)
		}

		resources = make([]Resource, 0, len(results))
		for _, row := range results {
			res, err := buildResourceFromResultRow(row)
			if err != nil {
				return fmt.Errorf("failed to build resource: %w", err)
			}
			resources = append(resources, res)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return resources, nil
}

// GetResourceListCount retrieves the count of all resources.
func (s *resourceStore) GetResourceListCount(ctx context.Context, resServerID string) (int, error) {
	var count int
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryGetResourceListCount, resServerID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get resource count: %w", err)
		}

		count, err = parseCountResult(results)
		return err
	})
	return count, err
}

// GetResourceListCountByParent retrieves count of resources by parent.
func (s *resourceStore) GetResourceListCountByParent(
	ctx context.Context, resServerID string, parentID *string,
) (int, error) {
	var count int
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		var results []map[string]interface{}
		var err error
		// Treat nil parent ID as top-level resources
		if parentID == nil {
			results, err = dbClient.QueryContext(
				ctx, queryGetResourceListCountByNullParent, resServerID, s.deploymentID,
			)
		} else {
			results, err = dbClient.QueryContext(
				ctx,
				queryGetResourceListCountByParent, resServerID, *parentID, s.deploymentID)
		}

		if err != nil {
			return fmt.Errorf("failed to get resource count by parent: %w", err)
		}

		count, err = parseCountResult(results)
		return err
	})
	return count, err
}

// UpdateResource updates a resource.
func (s *resourceStore) UpdateResource(ctx context.Context, id string, resServerID string, res Resource) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(
			ctx,
			queryUpdateResource,
			res.Name,        // $1: NAME
			res.Description, // $2: DESCRIPTION
			"{}",            // $3: PROPERTIES (empty JSON).
			id,              // $4: RESOURCE_ID
			resServerID,     // $5: RESOURCE_SERVER_ID (UUID FK)
			s.deploymentID,  // $6: DEPLOYMENT_ID
		)
		if err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}

		return nil
	})
}

// UpdateResourcePermission updates only the permission field of a resource.
func (s *resourceStore) UpdateResourcePermission(
	ctx context.Context, id string, resServerID string, permission string,
) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(
			ctx,
			queryUpdateResourcePermission,
			permission,
			id,
			resServerID,
			s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to update resource permission: %w", err)
		}
		return nil
	})
}

// DeleteResource deletes a resource.
func (s *resourceStore) DeleteResource(ctx context.Context, id string, resServerID string) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(ctx, queryDeleteResource, id, resServerID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to delete resource: %w", err)
		}

		return nil
	})
}

// CheckResourceHandleExists checks if resource handle exists under parent.
func (s *resourceStore) CheckResourceHandleExists(
	ctx context.Context,
	resServerID string, handle string, parentID *string,
) (bool, error) {
	var exists bool
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		var results []map[string]interface{}
		var err error
		if parentID == nil {
			results, err = dbClient.QueryContext(
				ctx,
				queryCheckResourceHandleExistsUnderNullParent, resServerID, handle, s.deploymentID,
			)
		} else {
			results, err = dbClient.QueryContext(
				ctx,
				queryCheckResourceHandleExistsUnderParent, resServerID, handle, *parentID,
				s.deploymentID,
			)
		}

		if err != nil {
			return fmt.Errorf("failed to check resource handle: %w", err)
		}

		exists, err = parseBoolFromCount(results)
		return err
	})
	return exists, err
}

// CheckResourceHasDependencies checks if resource has dependencies.
func (s *resourceStore) CheckResourceHasDependencies(ctx context.Context, resID string) (bool, error) {
	var hasDeps bool
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(ctx, queryCheckResourceHasDependencies, resID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to check dependencies: %w", err)
		}

		hasDeps, err = parseBoolFromCount(results)
		return err
	})
	return hasDeps, err
}

// CheckCircularDependency checks if setting a parent would create circular dependency.
func (s *resourceStore) CheckCircularDependency(ctx context.Context, resourceID, newParentID string) (bool, error) {
	var hasCircular bool
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(
			ctx, queryCheckCircularDependency, newParentID, resourceID, s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to check circular dependency: %w", err)
		}

		hasCircular, err = parseBoolFromCount(results)
		return err
	})
	return hasCircular, err
}

// Action Store Methods

// CreateAction creates a new action.
func (s *resourceStore) CreateAction(
	ctx context.Context,
	uuid string,
	resServerID string,
	resID *string,
	action Action,
) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(
			ctx,
			queryCreateAction,
			uuid,               // $1: ACTION_ID (UUID)
			resServerID,        // $2: RESOURCE_SERVER_ID (UUID FK)
			resID,              // $3: RESOURCE_ID (UUID FK or NULL)
			action.Name,        // $4: NAME
			action.Handle,      // $5: HANDLE
			action.Description, // $6: DESCRIPTION
			action.Permission,  // $7: PERMISSION
			"{}",               // $8: PROPERTIES (empty JSON).
			s.deploymentID,     // $9: DEPLOYMENT_ID
		)
		if err != nil {
			return fmt.Errorf("failed to create action: %w", err)
		}

		return nil
	})
}

// GetAction retrieves an action by UUID.
// If resID is nil, retrieves action at resource server level.
// If resID is provided, retrieves action at resource level.
func (s *resourceStore) GetAction(
	ctx context.Context, id string, resServerID string, resID *string,
) (Action, error) {
	var action Action
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		// Single unified query handles both resource server and resource level via nullable parameter
		results, err := dbClient.QueryContext(
			ctx, queryGetActionByID, id, resServerID, resID, s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to get action: %w", err)
		}

		if len(results) == 0 {
			return errActionNotFound
		}

		action, err = buildActionFromResultRow(results[0])
		return err
	})
	return action, err
}

// GetActionList retrieves actions with pagination.
func (s *resourceStore) GetActionList(
	ctx context.Context,
	resServerID string, resID *string, limit, offset int,
) ([]Action, error) {
	var actions []Action
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(
			ctx, queryGetActionList, resServerID, resID, limit, offset,
			s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to get action list: %w", err)
		}

		actions = make([]Action, 0, len(results))
		for _, row := range results {
			action, err := buildActionFromResultRow(row)
			if err != nil {
				return fmt.Errorf("failed to build action: %w", err)
			}
			actions = append(actions, action)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return actions, nil
}

// GetActionListCount retrieves count of actions.
func (s *resourceStore) GetActionListCount(
	ctx context.Context, resServerID string, resID *string,
) (int, error) {
	var count int
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(
			ctx, queryGetActionListCount, resServerID, resID, s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to get action count: %w", err)
		}

		count, err = parseCountResult(results)
		return err
	})
	return count, err
}

// UpdateAction updates an action.
func (s *resourceStore) UpdateAction(
	ctx context.Context, id string, resServerID string, resID *string, action Action,
) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		// Single unified query handles both levels via nullable parameter
		_, err := dbClient.ExecuteContext(
			ctx,
			queryUpdateAction,
			action.Name,        // $1: NAME
			action.Description, // $2: DESCRIPTION
			"{}",               // $3: PROPERTIES (empty JSON).
			id,                 // $4: ACTION_ID
			resServerID,        // $5: RESOURCE_SERVER_ID (UUID FK)
			resID,              // $6: RESOURCE_ID (UUID FK or NULL)
			s.deploymentID,     // $7: DEPLOYMENT_ID
		)
		if err != nil {
			return fmt.Errorf("failed to update action: %w", err)
		}

		return nil
	})
}

// UpdateActionPermission updates only the permission field of an action.
func (s *resourceStore) UpdateActionPermission(
	ctx context.Context, id string, resServerID string, resID *string, permission string,
) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(
			ctx,
			queryUpdateActionPermission,
			permission,     // $1: PERMISSION
			id,             // $2: ACTION_ID
			resServerID,    // $3: RESOURCE_SERVER_ID
			resID,          // $4: RESOURCE_ID (nullable)
			s.deploymentID, // $5: DEPLOYMENT_ID
		)
		if err != nil {
			return fmt.Errorf("failed to update action permission: %w", err)
		}
		return nil
	})
}

// DeleteAction deletes an action.
func (s *resourceStore) DeleteAction(
	ctx context.Context, id string, resServerID string, resID *string,
) error {
	return s.withDBClient(func(dbClient provider.DBClientInterface) error {
		_, err := dbClient.ExecuteContext(
			ctx,
			queryDeleteAction,
			id,             // $1: ACTION_ID
			resServerID,    // $2: RESOURCE_SERVER_ID (UUID FK)
			resID,          // $3: RESOURCE_ID (UUID FK or NULL)
			s.deploymentID, // $4: DEPLOYMENT_ID
		)
		if err != nil {
			return fmt.Errorf("failed to delete action: %w", err)
		}

		return nil
	})
}

// IsActionExist checks if an action exists.
func (s *resourceStore) IsActionExist(
	ctx context.Context, id string, resServerID string, resID *string,
) (bool, error) {
	var exists bool
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(
			ctx, queryCheckActionExists, id, resServerID, resID, s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to check action existence: %w", err)
		}

		exists, err = parseBoolFromCount(results)
		return err
	})
	return exists, err
}

// CheckActionHandleExists checks if action handle exists.
func (s *resourceStore) CheckActionHandleExists(
	ctx context.Context,
	resServerID string, resID *string, handle string,
) (bool, error) {
	var exists bool
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		results, err := dbClient.QueryContext(
			ctx,
			queryCheckActionHandleExists, resServerID, resID, handle, s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to check action handle: %w", err)
		}

		exists, err = parseBoolFromCount(results)
		return err
	})
	return exists, err
}

// ValidatePermissions validates that permissions exist for a given resource server.
// Returns array of invalid permissions (empty if all are valid).
func (s *resourceStore) ValidatePermissions(
	ctx context.Context, resServerID string, permissions []string,
) ([]string, error) {
	// Early return for empty input
	if len(permissions) == 0 {
		return []string{}, nil
	}

	var invalidPermissions []string

	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		// Convert permissions to JSON array for json_each()
		permissionsJSON, jsonErr := json.Marshal(permissions)
		if jsonErr != nil {
			return fmt.Errorf("failed to marshal permissions to JSON: %w", jsonErr)
		}

		// Query directly returns invalid permissions
		results, err := dbClient.QueryContext(
			ctx,
			queryValidatePermissions,
			resServerID,
			s.deploymentID,
			string(permissionsJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to validate permissions: %w", err)
		}

		// Simply collect the invalid permissions returned by the query
		for _, row := range results {
			perm, ok := row["permission"].(string)
			if !ok {
				return fmt.Errorf("permission field is missing or invalid in query result")
			}
			invalidPermissions = append(invalidPermissions, perm)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return invalidPermissions, nil
}

// FindResourceServersByPermissions returns distinct resource servers that define at least one of
// the supplied permissions.
func (s *resourceStore) FindResourceServersByPermissions(
	ctx context.Context, permissions []string,
) ([]ResourceServer, error) {
	if len(permissions) == 0 {
		return []ResourceServer{}, nil
	}

	var resourceServers []ResourceServer
	err := s.withDBClient(func(dbClient provider.DBClientInterface) error {
		permissionsJSON, jsonErr := json.Marshal(permissions)
		if jsonErr != nil {
			return fmt.Errorf("failed to marshal permissions to JSON: %w", jsonErr)
		}

		results, err := dbClient.QueryContext(
			ctx,
			queryFindResourceServersByPermissions,
			s.deploymentID,
			string(permissionsJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to find resource servers by permissions: %w", err)
		}

		resourceServers = make([]ResourceServer, 0, len(results))
		for _, row := range results {
			rs, buildErr := buildResourceServerFromResultRow(row)
			if buildErr != nil {
				return fmt.Errorf("failed to build resource server: %w", buildErr)
			}
			resourceServers = append(resourceServers, rs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resourceServers, nil
}

// Helper methods

// getConfigDBClient retrieves the config database client.
func (s *resourceStore) getConfigDBClient() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get config DB client: %w", err)
	}
	return dbClient, nil
}

// withDBClient executes a function with a DB client, handling client retrieval errors.
func (s *resourceStore) withDBClient(fn func(provider.DBClientInterface) error) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}
	return fn(dbClient)
}

// resolveNullableString converts empty string to nil for database storage.
func resolveNullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

// parseCountResult parses a count result from database query.
func parseCountResult(results []map[string]interface{}) (int, error) {
	if len(results) == 0 {
		return 0, fmt.Errorf("no count result returned")
	}

	countVal, ok := results[0]["total"]
	if !ok {
		countVal, ok = results[0]["count"]
		if !ok {
			return 0, fmt.Errorf("count field not found in result")
		}
	}

	switch v := countVal.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected count type: %T", countVal)
	}
}

// parseBoolFromCount parses a boolean from a count result.
func parseBoolFromCount(results []map[string]interface{}) (bool, error) {
	count, err := parseCountResult(results)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// resolveProperties extracts and sets the properties from the PROPERTIES column.
func resolveProperties(row map[string]interface{}, rs *ResourceServer) {
	if propsVal, ok := row["properties"]; ok && propsVal != nil {
		var props resourceServerProperties
		var propsBytes []byte

		switch v := propsVal.(type) {
		case string:
			propsBytes = []byte(v)
		case []byte:
			propsBytes = v
		}

		if len(propsBytes) > 0 {
			if err := json.Unmarshal(propsBytes, &props); err == nil {
				rs.Delimiter = props.Delimiter
			}
		}
	}
}

// buildPropertiesJSON builds the PROPERTIES JSON for a ResourceServer.
func buildPropertiesJSON(rs ResourceServer) interface{} {
	properties := resourceServerProperties{Delimiter: rs.Delimiter}
	if propsJSON, err := json.Marshal(properties); err == nil {
		return propsJSON
	}
	return json.RawMessage("{}")
}

// buildResourceServerFromResultRow builds a ResourceServer from a database result row.
func buildResourceServerFromResultRow(row map[string]interface{}) (ResourceServer, error) {
	rs := ResourceServer{}

	if id, ok := row["id"].(string); ok {
		rs.ID = id
	} else {
		return rs, fmt.Errorf("id field is missing or invalid")
	}

	if ouID, ok := row["ou_id"].(string); ok {
		rs.OUID = ouID
	} else {
		return rs, fmt.Errorf("ou_id field is missing or invalid")
	}

	if name, ok := row["name"].(string); ok {
		rs.Name = name
	} else {
		return rs, fmt.Errorf("name field is missing or invalid")
	}

	if desc, ok := row["description"].(string); ok {
		rs.Description = desc
	}

	if handle, ok := row["handle"].(string); ok {
		rs.Handle = handle
	}

	if identifier, ok := row["identifier"].(string); ok {
		rs.Identifier = identifier
	}

	resolveProperties(row, &rs)

	return rs, nil
}

// buildResourceFromResultRow builds a Resource from a database result row.
func buildResourceFromResultRow(row map[string]interface{}) (Resource, error) {
	res := Resource{}

	if id, ok := row["id"].(string); ok {
		res.ID = id
	} else {
		return res, fmt.Errorf("id field is missing or invalid")
	}

	if name, ok := row["name"].(string); ok {
		res.Name = name
	} else {
		return res, fmt.Errorf("name field is missing or invalid")
	}

	if handle, ok := row["handle"].(string); ok {
		res.Handle = handle
	} else {
		return res, fmt.Errorf("handle field is missing or invalid")
	}

	if desc, ok := row["description"].(string); ok {
		res.Description = desc
	}

	if permission, ok := row["permission"].(string); ok {
		res.Permission = permission
	}

	// PROPERTIES column exists in DB but not mapped to model (store as empty JSON)

	if parentID, ok := row["parent_resource_id"].(string); ok && parentID != "" {
		res.Parent = &parentID
	}

	return res, nil
}

// buildActionFromResultRow builds an Action from a database result row.
func buildActionFromResultRow(row map[string]interface{}) (Action, error) {
	action := Action{}

	if id, ok := row["id"].(string); ok {
		action.ID = id
	} else {
		return action, fmt.Errorf("id field is missing or invalid")
	}

	if name, ok := row["name"].(string); ok {
		action.Name = name
	} else {
		return action, fmt.Errorf("name field is missing or invalid")
	}

	if handle, ok := row["handle"].(string); ok {
		action.Handle = handle
	} else {
		return action, fmt.Errorf("handle field is missing or invalid")
	}

	if desc, ok := row["description"].(string); ok {
		action.Description = desc
	}

	if permission, ok := row["permission"].(string); ok {
		action.Permission = permission
	}

	// PROPERTIES column exists in DB but not mapped to model (store as empty JSON)

	return action, nil
}
