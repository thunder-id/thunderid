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

package revocation

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// RevokedTokenWriterInterface defines the deny-list write path for single-token revocation. It is
// the only way to populate the deny list, kept separate from the enforcement read path so that the
// AS hot path cannot revoke tokens, only check them.
type RevokedTokenWriterInterface interface {
	// InsertRevokedToken writes a JTI to the deny list. The write is idempotent.
	InsertRevokedToken(ctx context.Context, token RevokedToken) error
}

// revokedTokenWriter implements RevokedTokenWriterInterface against the operation database.
type revokedTokenWriter struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newRevokedTokenWriter creates a new revokedTokenWriter. It is intentionally unexported so the
// deny-list write path (InsertRevokedToken) cannot be reached from outside the package, bypassing
// the revocation service's validation and ownership checks.
func newRevokedTokenWriter() RevokedTokenWriterInterface {
	return &revokedTokenWriter{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// InsertRevokedToken writes a JTI to the deny list. A duplicate (deployment, jti) is a no-op.
// A UUID v7 surrogate primary key is generated when the token has no ID.
func (s *revokedTokenWriter) InsertRevokedToken(ctx context.Context, token RevokedToken) error {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return fmt.Errorf("failed to get operation database client: %w", err)
	}

	id := token.ID
	if id == "" {
		id, err = utils.GenerateUUIDv7()
		if err != nil {
			return fmt.Errorf("failed to generate revoked token id: %w", err)
		}
	}

	_, err = dbClient.ExecuteContext(ctx, queryInsertRevokedToken, id, token.JTI,
		string(token.RevocationReason), token.RevokedAt, token.ExpiryTime, s.deploymentID)
	if err != nil {
		return fmt.Errorf("error inserting revoked token: %w", err)
	}

	return nil
}
