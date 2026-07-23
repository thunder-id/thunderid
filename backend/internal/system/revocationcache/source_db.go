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
	columnNameJTI            = "jti"
	columnNameCriterionValue = "criterion_value"
	columnNameExpiryTime     = "expiry_time"
	// criterionTypeTokenFamily mirrors the revocation package's token_family criterion type. It is
	// duplicated here (not imported) so this read-only RS package stays decoupled from the write path.
	criterionTypeTokenFamily = "token_family"
)

// dbSource reads the deny-list snapshot from the runtime persistent database. It is the only source today; it
// issues a single read-only bulk query per sync and never writes.
type dbSource struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newDBSource creates a dbSource bound to the runtime persistent database and this deployment's identifier.
func newDBSource() syncSource {
	return &dbSource{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// Snapshot returns all non-expired deny-list entries for this deployment: the revoked single-token
// jtis and the revoked token-family ids.
func (s *dbSource) Snapshot(ctx context.Context) (revokedSnapshot, error) {
	dbClient, err := s.dbProvider.GetRuntimePersistentDBClient()
	if err != nil {
		return revokedSnapshot{}, fmt.Errorf("failed to get runtime persistent database client: %w", err)
	}

	now := time.Now().UTC()

	tokenRows, err := dbClient.QueryContext(ctx, querySnapshotRevokedTokens, now, s.deploymentID)
	if err != nil {
		return revokedSnapshot{}, fmt.Errorf("error reading revoked token snapshot: %w", err)
	}
	tokens, err := parseEntries(tokenRows, columnNameJTI)
	if err != nil {
		return revokedSnapshot{}, err
	}

	tokenFamilyRows, err := dbClient.QueryContext(ctx, querySnapshotRevokedTokenFamilies,
		criterionTypeTokenFamily, now, s.deploymentID)
	if err != nil {
		return revokedSnapshot{}, fmt.Errorf("error reading revoked token family snapshot: %w", err)
	}
	families, err := parseEntries(tokenFamilyRows, columnNameCriterionValue)
	if err != nil {
		return revokedSnapshot{}, err
	}

	return revokedSnapshot{Tokens: tokens, Families: families}, nil
}

// parseEntries maps deny-list rows into revoked entries, reading the lookup value from valueColumn and
// the expiry from the standard expiry-time column.
func parseEntries(rows []map[string]interface{}, valueColumn string) ([]revokedEntry, error) {
	entries := make([]revokedEntry, 0, len(rows))
	for _, row := range rows {
		value, ok := row[valueColumn].(string)
		if !ok || value == "" {
			return nil, fmt.Errorf("invalid or missing %s in revocation snapshot", valueColumn)
		}
		expiryTime, err := utils.ParseDBTimeField(row[columnNameExpiryTime], columnNameExpiryTime)
		if err != nil {
			return nil, fmt.Errorf("error parsing revocation snapshot: %w", err)
		}
		entries = append(entries, revokedEntry{Value: value, ExpiryTime: expiryTime})
	}
	return entries, nil
}
