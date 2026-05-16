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

// Package jti provides a shared replay cache for JWT jti values. Consumers (DPoP,
// client_assertion, token-exchange subject tokens, etc.) record a (namespace,
// contextKey, jti) tuple and learn from the return value whether the proof/assertion
// has been seen before within its acceptance window.
package jti

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// JTIStoreInterface is the JTI replay cache. RecordJTI returns (true, nil) on
// fresh insert, (false, nil) on replay, and an error on backend failure.
//
// namespace identifies the consumer (e.g. "dpop") so multiple consumers can share
// the same backend without collision. Uniqueness of jti is enforced within a
// namespace per deployment.
type JTIStoreInterface interface {
	RecordJTI(ctx context.Context, namespace, jti string, expiry time.Time) (bool, error)
}

// jtiStore is the database-backed JTI replay cache.
type jtiStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newDBStore returns a JTIStoreInterface backed by the runtime database.
func newDBStore(deploymentID string) JTIStoreInterface {
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
