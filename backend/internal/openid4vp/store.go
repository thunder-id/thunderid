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

package openid4vp

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// openID4VPStoreInterface persists short-lived OpenID4VP request state keyed by State. The
// production implementation (openID4VPStore) is runtime-database-backed so state
// survives restarts and is visible across replicas.
type openID4VPStoreInterface interface {
	SaveRequestState(ctx context.Context, st *RequestState) error
	GetRequestState(ctx context.Context, state string) (*RequestState, bool)
	DeleteRequestState(ctx context.Context, state string) error
}

// openID4VPStore persists OpenID4VP request state in the runtime database so it
// survives restarts and is visible to every replica (the replica that receives
// the wallet response may differ from the one that initiated the request). The
// ephemeral response-decryption key is encrypted at rest with the server's
// configured symmetric key.
type openID4VPStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
	crypto       kmprovider.ConfigCryptoProvider
}

// newOpenID4VPStore constructs a database-backed request state store using the given crypto provider.
func newOpenID4VPStore(crypto kmprovider.ConfigCryptoProvider) openID4VPStoreInterface {
	return &openID4VPStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
		crypto:       crypto,
	}
}

// SaveRequestState upserts the request state into the database, encrypting the ephemeral key.
func (s *openID4VPStore) SaveRequestState(ctx context.Context, st *RequestState) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get runtime database client: %w", err)
	}

	var encKey []byte
	if st.EphemeralKey != nil {
		pkcs8, err := x509.MarshalPKCS8PrivateKey(st.EphemeralKey)
		if err != nil {
			return fmt.Errorf("failed to marshal ephemeral key: %w", err)
		}
		encKey, err = s.crypto.Encrypt(ctx, pkcs8)
		if err != nil {
			return fmt.Errorf("failed to encrypt ephemeral key: %w", err)
		}
	}

	var resultJSON []byte
	if st.Result != nil {
		resultJSON, err = json.Marshal(st.Result)
		if err != nil {
			return fmt.Errorf("failed to marshal verification result: %w", err)
		}
	}

	_, err = dbClient.ExecuteContext(ctx, queryUpsertRequestState,
		st.State, s.deploymentID, st.DefinitionID, st.Nonce, encKey, st.ClientID, st.RPID,
		st.RequestURI, string(st.Status), resultJSON, st.FailureReason, st.ExpiresAt.UTC())
	if err != nil {
		return fmt.Errorf("failed to upsert request state: %w", err)
	}
	return nil
}

// GetRequestState retrieves and reconstructs the request state for the given state from the database.
func (s *openID4VPStore) GetRequestState(ctx context.Context, state string) (*RequestState, bool) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VPStateStore"))

	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		logger.Error(ctx, "Failed to get runtime database client", log.Error(err))
		return nil, false
	}
	results, err := dbClient.QueryContext(ctx, queryGetRequestState, state, s.deploymentID)
	if err != nil {
		logger.Error(ctx, "Failed to query request state", log.Error(err))
		return nil, false
	}
	if len(results) == 0 {
		return nil, false
	}
	rs, err := s.buildRequestStateFromRow(ctx, results[0])
	if err != nil {
		logger.Error(ctx, "Failed to build request state from row", log.Error(err))
		return nil, false
	}
	return rs, true
}

// DeleteRequestState deletes the request state for the given state from the database.
func (s *openID4VPStore) DeleteRequestState(ctx context.Context, state string) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get runtime database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteRequestState, state, s.deploymentID); err != nil {
		return fmt.Errorf("failed to delete request state: %w", err)
	}
	return nil
}

// buildRequestStateFromRow reconstructs a RequestState from a result row,
// decrypting the ephemeral key and decoding the stored result.
func (s *openID4VPStore) buildRequestStateFromRow(
	ctx context.Context, row map[string]interface{},
) (*RequestState, error) {
	rs := &RequestState{
		State:         columnString(row["state"]),
		DefinitionID:  columnString(row["definition_id"]),
		Nonce:         columnString(row["nonce"]),
		ClientID:      columnString(row["client_id"]),
		RPID:          columnString(row["rp_id"]),
		RequestURI:    columnString(row["request_uri"]),
		Status:        Status(columnString(row["status"])),
		FailureReason: columnString(row["failure_reason"]),
	}

	expiry, err := parseStateTime(row["expiry_time"])
	if err != nil {
		return nil, err
	}
	rs.ExpiresAt = expiry

	if encKey := columnBytes(row["ephemeral_key"]); len(encKey) > 0 {
		pkcs8, err := s.crypto.Decrypt(ctx, encKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt ephemeral key: %w", err)
		}
		parsed, err := x509.ParsePKCS8PrivateKey(pkcs8)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ephemeral key: %w", err)
		}
		ecKey, ok := parsed.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("%w: ephemeral key is not an EC private key", ErrPolicy)
		}
		rs.EphemeralKey = ecKey
	}

	if resultBytes := columnBytes(row["result"]); len(resultBytes) > 0 {
		var vp VerifiedPresentation
		if err := json.Unmarshal(resultBytes, &vp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal verification result: %w", err)
		}
		rs.Result = &vp
	}
	return rs, nil
}

// columnString coerces a result-row value to a string, tolerating string/[]byte.
func columnString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

// columnBytes coerces a result-row value to bytes, tolerating []byte/string.
func columnBytes(v interface{}) []byte {
	switch t := v.(type) {
	case []byte:
		return t
	case string:
		return []byte(t)
	default:
		return nil
	}
}

// parseStateTime parses an EXPIRY_TIME column across Postgres (time.Time) and
// SQLite (datetime string) drivers.
func parseStateTime(field interface{}) (time.Time, error) {
	const layout = "2006-01-02 15:04:05.999999999"
	switch v := field.(type) {
	case time.Time:
		return v, nil
	case []byte:
		return parseStateTime(string(v))
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
