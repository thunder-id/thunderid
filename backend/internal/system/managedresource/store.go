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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package managedresource

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// storeInterface is the persistence the registry needs. It is an interface so a test can substitute
// one without a database.
type storeInterface interface {
	Mark(ctx context.Context, resourceType, resourceID string) error
	Unmark(ctx context.Context, resourceType, resourceID string) error
	IsManaged(ctx context.Context, resourceType, resourceID string) (bool, error)
	ManagedIDs(ctx context.Context, resourceType string) (map[string]bool, error)
}

// store is the config database backed registry.
type store struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

func newStore(deploymentID string) storeInterface {
	return &store{dbProvider: provider.GetDBProvider(), deploymentID: deploymentID}
}

func (s *store) Mark(ctx context.Context, resourceType, resourceID string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.QueryContext(ctx, queryMarkManagedResource,
		deployment.Resolve(ctx, s.deploymentID), resourceType, resourceID)
	if err != nil {
		return fmt.Errorf("failed to record the managed resource: %w", err)
	}
	return nil
}

func (s *store) Unmark(ctx context.Context, resourceType, resourceID string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.QueryContext(ctx, queryUnmarkManagedResource,
		deployment.Resolve(ctx, s.deploymentID), resourceType, resourceID)
	if err != nil {
		return fmt.Errorf("failed to drop the managed resource record: %w", err)
	}
	return nil
}

func (s *store) IsManaged(ctx context.Context, resourceType, resourceID string) (bool, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryIsManagedResource,
		deployment.Resolve(ctx, s.deploymentID), resourceType, resourceID)
	if err != nil {
		return false, fmt.Errorf("failed to look up the managed resource: %w", err)
	}
	if len(results) == 0 {
		return false, nil
	}
	count, ok := results[0]["total"].(int64)
	if !ok {
		return false, fmt.Errorf("failed to parse the managed resource count")
	}
	return count > 0, nil
}

func (s *store) ManagedIDs(ctx context.Context, resourceType string) (map[string]bool, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryListManagedResourceIDs,
		deployment.Resolve(ctx, s.deploymentID), resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to list the managed resources: %w", err)
	}
	ids := make(map[string]bool, len(results))
	for _, row := range results {
		if id, ok := row["resource_id"].(string); ok {
			ids[id] = true
		}
	}
	return ids, nil
}
