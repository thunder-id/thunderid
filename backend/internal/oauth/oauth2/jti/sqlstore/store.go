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

// Package sqlstore provides the relational-database-backed JTI replay cache.
// It is isolated from the jti package so that consumers needing only the
// interface (or the Redis implementation) do not transitively link the SQL
// database drivers.
package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// jtiStore is the database-backed JTI replay cache.
type jtiStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// NewStore returns a jti.JTIStoreInterface backed by the runtime database.
func NewStore(deploymentID string) jti.JTIStoreInterface {
	return &jtiStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: deploymentID,
	}
}

// RecordJTI inserts (namespace, jti) scoped to the deployment; returns false on replay.
func (s *jtiStore) RecordJTI(
	ctx context.Context, namespace, jti string, expiry time.Time,
) (bool, error) {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(
		ctx, queryInsertJTI, namespace, jti, expiry.UTC(), s.deploymentID,
	)
	if err != nil {
		return false, fmt.Errorf("failed to insert jti: %w", err)
	}
	return rowsAffected > 0, nil
}
