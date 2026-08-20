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

// Package dataplanetoken issues and holds the credential a data plane presents when it dials the
// control plane's channel.
//
// A token is minted when an environment is registered and shown once. It is held encrypted, so a
// database dump does not yield a set of working data-plane credentials, and it is never readable
// afterwards: the only way to recover from a lost one is to issue another.
package dataplanetoken

import (
	"context"
	"fmt"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

var (
	// queryPutToken records or replaces the token issued to one data plane.
	queryPutToken = dbmodel.DBQuery{
		ID: "DPTQ-TOKEN-001",
		Query: `INSERT INTO "DATA_PLANE_TOKEN" (DATA_PLANE_ID, DEPLOYMENT_ID, TOKEN) ` +
			`VALUES ($1, $2, $3) ON CONFLICT (DATA_PLANE_ID) DO UPDATE ` +
			`SET TOKEN = EXCLUDED.TOKEN, DEPLOYMENT_ID = EXCLUDED.DEPLOYMENT_ID`,
	}

	// queryGetToken reads one data plane's token. It is deliberately not scoped by deployment: the
	// handshake is authenticated before any tenant context exists.
	queryGetToken = dbmodel.DBQuery{
		ID:    "DPTQ-TOKEN-002",
		Query: `SELECT TOKEN FROM "DATA_PLANE_TOKEN" WHERE DATA_PLANE_ID = $1`,
	}

	// queryDeleteToken removes a data plane's token.
	queryDeleteToken = dbmodel.DBQuery{
		ID:    "DPTQ-TOKEN-003",
		Query: `DELETE FROM "DATA_PLANE_TOKEN" WHERE DATA_PLANE_ID = $1`,
	}
)

// store persists the ciphertext of each data plane's token.
type store struct {
	dbProvider provider.DBProviderInterface
}

func newStore() *store {
	return &store{dbProvider: provider.GetDBProvider()}
}

// Put records the encrypted token for a data plane, replacing any previous one.
func (s *store) Put(ctx context.Context, dataPlaneID, deploymentID, ciphertext string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryPutToken, dataPlaneID, deploymentID, ciphertext); err != nil {
		return fmt.Errorf("failed to record the data plane token: %w", err)
	}
	return nil
}

// Get reads a data plane's encrypted token. The boolean reports whether one has been issued.
func (s *store) Get(ctx context.Context, dataPlaneID string) (string, bool, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return "", false, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryGetToken, dataPlaneID)
	if err != nil {
		return "", false, fmt.Errorf("failed to read the data plane token: %w", err)
	}
	if len(results) == 0 {
		return "", false, nil
	}
	ciphertext, _ := results[0]["token"].(string)
	if ciphertext == "" {
		if raw, ok := results[0]["token"].([]byte); ok {
			ciphertext = string(raw)
		}
	}
	return ciphertext, ciphertext != "", nil
}

// Delete removes a data plane's token, which stops it connecting.
func (s *store) Delete(ctx context.Context, dataPlaneID string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteToken, dataPlaneID); err != nil {
		return fmt.Errorf("failed to delete the data plane token: %w", err)
	}
	return nil
}
