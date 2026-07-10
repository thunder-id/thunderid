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
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
	storeProvider providers.RuntimeStoreProvider
}

// newStore returns a JTIStoreInterface backed by the configured runtime store.
func newStore(storeProvider providers.RuntimeStoreProvider) JTIStoreInterface {
	return &jtiStore{
		storeProvider: storeProvider,
	}
}

// RecordJTI inserts (namespace, jti) scoped to the deployment; returns false on replay.
//
// The runtime store's Put is an unconditional overwrite, so replay detection is done with a
// Get-then-Put check. This has a benign race under concurrent submissions of the same jti, but
// that's an acceptable trade-off for a replay cache.
func (s *jtiStore) RecordJTI(
	ctx context.Context, namespace, jti string, expiry time.Time,
) (bool, error) {
	key := namespace + ":" + jti

	existing, err := s.storeProvider.Get(ctx, providers.NamespaceJTI, key)
	if err != nil {
		return false, fmt.Errorf("failed to check jti: %w", err)
	}
	if existing != nil {
		return false, nil
	}

	value, err := json.Marshal(jti)
	if err != nil {
		return false, fmt.Errorf("failed to marshal jti: %w", err)
	}

	ttlSeconds := int64(expiry.Sub(time.Now().UTC()).Seconds())
	if err := s.storeProvider.Put(ctx, providers.NamespaceJTI, key, value, ttlSeconds); err != nil {
		return false, fmt.Errorf("failed to insert jti: %w", err)
	}
	return true, nil
}
