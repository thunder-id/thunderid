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

package revocationcache

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	columnNameJTI        = "jti"
	columnNameExpiryTime = "expiry_time"
)

// dbSource reads the deny-list snapshot from the operation database. It is the only source today; it
// issues a single read-only bulk query per sync and never writes.
type dbSource struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newDBSource creates a dbSource bound to the operation database and this deployment's identifier.
func newDBSource() syncSource {
	return &dbSource{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// Snapshot returns all non-expired deny-list entries for this deployment.
func (s *dbSource) Snapshot(ctx context.Context) ([]revokedEntry, error) {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get operation database client: %w", err)
	}

	rows, err := dbClient.QueryContext(ctx, querySnapshotRevokedTokens, time.Now().UTC(), s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("error reading revoked token snapshot: %w", err)
	}

	entries := make([]revokedEntry, 0, len(rows))
	for _, row := range rows {
		jti, ok := row[columnNameJTI].(string)
		if !ok || jti == "" {
			return nil, fmt.Errorf("invalid or missing %s in revoked token snapshot", columnNameJTI)
		}
		expiryTime, err := utils.ParseDBTimeField(row[columnNameExpiryTime], columnNameExpiryTime)
		if err != nil {
			return nil, fmt.Errorf("error parsing revoked token snapshot: %w", err)
		}
		entries = append(entries, revokedEntry{JTI: jti, ExpiryTime: expiryTime})
	}

	return entries, nil
}
