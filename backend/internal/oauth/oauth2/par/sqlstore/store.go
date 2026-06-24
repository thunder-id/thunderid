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

// Package sqlstore provides the relational-database-backed PAR request store.
// It is isolated from the par package so that consumers needing only the
// interface (or the Redis implementation) do not transitively link the SQL
// database drivers.
package sqlstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// parRequestStore is the relational-DB-backed implementation of par.PARStoreInterface.
type parRequestStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// NewStore creates a new DB-backed PAR request store.
func NewStore(deploymentID string) par.PARStoreInterface {
	return &parRequestStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: deploymentID,
	}
}

// Store persists a pushed authorization request and returns the generated random key.
func (s *parRequestStore) Store(
	ctx context.Context, request par.PushedAuthorizationRequest, expirySeconds int64,
) (string, error) {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return "", fmt.Errorf("failed to get database client: %w", err)
	}

	randomKey, err := par.GenerateRandomKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate request URI: %w", err)
	}

	data, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal PAR request: %w", err)
	}

	expiryTime := time.Now().UTC().Add(time.Duration(expirySeconds) * time.Second)
	if _, err := dbClient.ExecuteContext(
		ctx, queryInsertPARRequest, randomKey, s.deploymentID, data, expiryTime,
	); err != nil {
		return "", fmt.Errorf("failed to insert PAR request: %w", err)
	}

	return randomKey, nil
}

// Consume atomically retrieves and deletes a pushed authorization request from the store.
// Returns the request, a boolean indicating if found, and any error.
func (s *parRequestStore) Consume(
	ctx context.Context, randomKey string,
) (par.PushedAuthorizationRequest, bool, error) {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return par.PushedAuthorizationRequest{}, false, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetPARRequest, randomKey, time.Now().UTC(), s.deploymentID)
	if err != nil {
		return par.PushedAuthorizationRequest{}, false, fmt.Errorf("failed to query PAR request: %w", err)
	}
	if len(results) == 0 {
		return par.PushedAuthorizationRequest{}, false, nil
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryDeletePARRequest, randomKey, s.deploymentID)
	if err != nil {
		return par.PushedAuthorizationRequest{}, false, fmt.Errorf("failed to delete PAR request: %w", err)
	}
	// Another consumer raced us to the delete; treat as already consumed.
	if rowsAffected == 0 {
		return par.PushedAuthorizationRequest{}, false, nil
	}

	request, err := buildPARRequestFromRow(results[0])
	if err != nil {
		return par.PushedAuthorizationRequest{}, false, err
	}
	return request, true, nil
}

// buildPARRequestFromRow reconstructs a par.PushedAuthorizationRequest from a database row.
func buildPARRequestFromRow(row map[string]any) (par.PushedAuthorizationRequest, error) {
	var dataJSON []byte
	if val, ok := row[dbColumnRequestParams].(string); ok && val != "" {
		dataJSON = []byte(val)
	} else if val, ok := row[dbColumnRequestParams].([]byte); ok && len(val) > 0 {
		dataJSON = val
	} else {
		return par.PushedAuthorizationRequest{}, errors.New("request_params is missing or of unexpected type")
	}

	var request par.PushedAuthorizationRequest
	if err := json.Unmarshal(dataJSON, &request); err != nil {
		return par.PushedAuthorizationRequest{}, fmt.Errorf("failed to unmarshal PAR request: %w", err)
	}
	return request, nil
}
