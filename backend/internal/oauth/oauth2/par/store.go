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

package par

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// requestURIRandomBytes is the number of random bytes for the request URI (32 bytes = 256 bits).
const requestURIRandomBytes = 32

// parStoreInterface defines the interface for PAR request storage.
// Implementations operate on opaque random keys; the request_uri URN prefix is
// added and stripped by the service layer.
type parStoreInterface interface {
	Store(ctx context.Context, request pushedAuthorizationRequest, expirySeconds int64) (string, error)
	Consume(ctx context.Context, randomKey string) (pushedAuthorizationRequest, bool, error)
}

// parRequestStore is the relational-DB-backed implementation of parStoreInterface.
type parRequestStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newPARRequestStore creates a new DB-backed PAR request store.
func newPARRequestStore(deploymentID string) parStoreInterface {
	return &parRequestStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: deploymentID,
	}
}

// Store persists a pushed authorization request and returns the generated random key.
func (s *parRequestStore) Store(
	ctx context.Context, request pushedAuthorizationRequest, expirySeconds int64,
) (string, error) {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return "", fmt.Errorf("failed to get database client: %w", err)
	}

	randomKey, err := generateRandomKey()
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
) (pushedAuthorizationRequest, bool, error) {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return pushedAuthorizationRequest{}, false, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetPARRequest, randomKey, time.Now().UTC(), s.deploymentID)
	if err != nil {
		return pushedAuthorizationRequest{}, false, fmt.Errorf("failed to query PAR request: %w", err)
	}
	if len(results) == 0 {
		return pushedAuthorizationRequest{}, false, nil
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryDeletePARRequest, randomKey, s.deploymentID)
	if err != nil {
		return pushedAuthorizationRequest{}, false, fmt.Errorf("failed to delete PAR request: %w", err)
	}
	// Another consumer raced us to the delete; treat as already consumed.
	if rowsAffected == 0 {
		return pushedAuthorizationRequest{}, false, nil
	}

	request, err := buildPARRequestFromRow(results[0])
	if err != nil {
		return pushedAuthorizationRequest{}, false, err
	}
	return request, true, nil
}

// buildPARRequestFromRow reconstructs a pushedAuthorizationRequest from a database row.
func buildPARRequestFromRow(row map[string]any) (pushedAuthorizationRequest, error) {
	var dataJSON []byte
	if val, ok := row[dbColumnRequestParams].(string); ok && val != "" {
		dataJSON = []byte(val)
	} else if val, ok := row[dbColumnRequestParams].([]byte); ok && len(val) > 0 {
		dataJSON = val
	} else {
		return pushedAuthorizationRequest{}, errors.New("request_params is missing or of unexpected type")
	}

	var request pushedAuthorizationRequest
	if err := json.Unmarshal(dataJSON, &request); err != nil {
		return pushedAuthorizationRequest{}, fmt.Errorf("failed to unmarshal PAR request: %w", err)
	}
	return request, nil
}

// generateRandomKey generates a cryptographically random key for the request URI.
func generateRandomKey() (string, error) {
	b := make([]byte, requestURIRandomBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
