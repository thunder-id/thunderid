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

package openid4vci

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// openID4VCIStoreInterface persists the OpenID4VCI issuer's short-lived runtime
// state — c_nonces and issuer-initiated credential offers — keyed by nonce/id.
type openID4VCIStoreInterface interface {
	SaveNonce(ctx context.Context, rec *nonceRecord) error
	GetNonce(ctx context.Context, nonce string) (*nonceRecord, bool)
	DeleteNonce(ctx context.Context, nonce string) error
	SaveOffer(ctx context.Context, rec *offerRecord) error
	GetOffer(ctx context.Context, id string) (*offerRecord, bool)
}

// openID4VCIStore persists the issuer's runtime state (c_nonces and credential
// offers) in the runtime database so it is visible across replicas — the replica
// that issues a nonce/offer may differ from the one that consumes it.
type openID4VCIStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newOpenID4VCIStore creates a new openID4VCIStore backed by the runtime database provider.
func newOpenID4VCIStore() openID4VCIStoreInterface {
	return &openID4VCIStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// SaveNonce persists a nonce record to the runtime database.
func (s *openID4VCIStore) SaveNonce(ctx context.Context, rec *nonceRecord) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get runtime database client: %w", err)
	}
	if _, err = dbClient.ExecuteContext(ctx, queryInsertNonce,
		rec.Nonce, s.deploymentID, rec.ExpiresAt.UTC()); err != nil {
		return fmt.Errorf("failed to insert nonce: %w", err)
	}
	return nil
}

// GetNonce retrieves a stored nonce record, returning false if it is not found.
func (s *openID4VCIStore) GetNonce(ctx context.Context, nonce string) (*nonceRecord, bool) {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return nil, false
	}
	results, err := dbClient.QueryContext(ctx, queryGetNonce, nonce, s.deploymentID)
	if err != nil || len(results) == 0 {
		return nil, false
	}
	row := results[0]
	expiry, err := parseVCITime(row["expiry_time"])
	if err != nil {
		return nil, false
	}
	return &nonceRecord{
		Nonce:     vciColumnString(row["nonce"]),
		ExpiresAt: expiry,
	}, true
}

// DeleteNonce removes a nonce record from the runtime database.
func (s *openID4VCIStore) DeleteNonce(ctx context.Context, nonce string) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get runtime database client: %w", err)
	}
	if _, err = dbClient.ExecuteContext(ctx, queryDeleteNonce, nonce, s.deploymentID); err != nil {
		return fmt.Errorf("failed to delete nonce: %w", err)
	}
	return nil
}

// SaveOffer persists a credential offer record to the runtime database.
func (s *openID4VCIStore) SaveOffer(ctx context.Context, rec *offerRecord) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get runtime database client: %w", err)
	}
	offerJSON, err := json.Marshal(rec.Offer)
	if err != nil {
		return fmt.Errorf("failed to marshal credential offer: %w", err)
	}
	if _, err = dbClient.ExecuteContext(ctx, queryInsertOffer,
		rec.ID, s.deploymentID, string(offerJSON), rec.ExpiresAt.UTC()); err != nil {
		return fmt.Errorf("failed to insert credential offer: %w", err)
	}
	return nil
}

// GetOffer retrieves a stored credential offer by ID, returning false if it is not found.
func (s *openID4VCIStore) GetOffer(ctx context.Context, id string) (*offerRecord, bool) {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return nil, false
	}
	results, err := dbClient.QueryContext(ctx, queryGetOffer, id, s.deploymentID)
	if err != nil || len(results) == 0 {
		return nil, false
	}
	row := results[0]
	offerBytes := vciColumnBytes(row["offer"])
	var offerMap map[string]interface{}
	if err = json.Unmarshal(offerBytes, &offerMap); err != nil {
		return nil, false
	}
	expiry, err := parseVCITime(row["expiry_time"])
	if err != nil {
		return nil, false
	}
	return &offerRecord{
		ID:        vciColumnString(row["id"]),
		Offer:     offerMap,
		ExpiresAt: expiry,
	}, true
}

// vciColumnString coerces a result-row value to a string, tolerating string/[]byte.
func vciColumnString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

// vciColumnBytes coerces a result-row value to bytes, tolerating []byte/string.
func vciColumnBytes(v interface{}) []byte {
	switch t := v.(type) {
	case []byte:
		return t
	case string:
		return []byte(t)
	default:
		return nil
	}
}

// parseVCITime parses an EXPIRY_TIME column across Postgres (time.Time) and
// SQLite (datetime string) drivers.
func parseVCITime(field interface{}) (time.Time, error) {
	const layout = "2006-01-02 15:04:05.999999999"
	switch v := field.(type) {
	case time.Time:
		return v, nil
	case []byte:
		return parseVCITime(string(v))
	case string:
		trimmed := v
		if parts := strings.SplitN(v, " ", 3); len(parts) >= 2 {
			trimmed = parts[0] + " " + parts[1]
		}
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t, nil
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, nil
		}
		return time.Time{}, fmt.Errorf("error parsing expiry_time: %q", v)
	default:
		return time.Time{}, fmt.Errorf("unexpected type for expiry_time")
	}
}
