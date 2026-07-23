/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// consumedCodeReplayKeyPrefix namespaces the short-lived replay markers written when an authorization
// code is consumed. A marker records the code's token family id so a later replay of the (now removed)
// code can still revoke the grant. It shares NamespaceAuthzCode but cannot collide with a code key,
// which is a bare UUID.
const consumedCodeReplayKeyPrefix = "consumed:"

// AuthorizationCodeStoreInterface defines the interface for managing authorization codes.
type AuthorizationCodeStoreInterface interface {
	InsertAuthorizationCode(ctx context.Context, authzCode AuthorizationCode) error
	ConsumeAuthorizationCode(ctx context.Context, authCode string) (bool, error)
	GetAuthorizationCode(ctx context.Context, authCode string) (*AuthorizationCode, error)
	// MarkConsumedTokenFamily records tokenFamilyID under a replay-lookup key for a just-consumed code,
	// bounded by ttl, so a later replay of the removed code can recover the tfid. An empty
	// tokenFamilyID is a no-op.
	MarkConsumedTokenFamily(ctx context.Context, authCode, tokenFamilyID string, ttl time.Duration) error
	// ConsumedTokenFamily returns the token family id recorded for a consumed authorization code, and
	// whether such a marker exists.
	ConsumedTokenFamily(ctx context.Context, authCode string) (string, bool, error)
}

// authorizationCodeStore implements the AuthorizationCodeStoreInterface for managing authorization codes.
type authorizationCodeStore struct {
	storeProvider providers.RuntimeStoreProvider
}

// newAuthorizationCodeStore creates a new instance of authorizationCodeStore with injected dependencies.
func newAuthorizationCodeStore(storeProvider providers.RuntimeStoreProvider) AuthorizationCodeStoreInterface {
	return &authorizationCodeStore{
		storeProvider: storeProvider,
	}
}

// InsertAuthorizationCode inserts a new authorization code into the runtime store.
func (acs *authorizationCodeStore) InsertAuthorizationCode(
	ctx context.Context, authzCode AuthorizationCode) error {
	data, err := json.Marshal(authzCode)
	if err != nil {
		return fmt.Errorf("failed to marshal authzCode request: %w", err)
	}

	ttl := time.Until(authzCode.ExpiryTime)
	if ttl < time.Second {
		return fmt.Errorf("authorization code already expired")
	}

	err = acs.storeProvider.Put(ctx, providers.NamespaceAuthzCode, authzCode.Code, data, int64(ttl.Seconds()))
	if err != nil {
		return fmt.Errorf("error inserting authorization code: %w", err)
	}

	return nil
}

// ConsumeAuthorizationCode atomically reads and removes authorization code
// Returns true if the code was successfully consumed, false if the code was already consumed,
// and false if a runtime store error occurs.
func (acs *authorizationCodeStore) ConsumeAuthorizationCode(ctx context.Context, authCode string) (bool, error) {
	data, err := acs.storeProvider.Take(ctx, providers.NamespaceAuthzCode, authCode)
	if err != nil {
		return false, fmt.Errorf("error consuming authorization code: %w", err)
	}
	return data != nil, nil
}

// MarkConsumedTokenFamily records the token family id of a just-consumed authorization code so a later
// replay of the (now removed) code can recover it and revoke the grant. An empty token family id is a
// no-op. ttl bounds the marker; a non-positive ttl is floored to one second.
func (acs *authorizationCodeStore) MarkConsumedTokenFamily(
	ctx context.Context, authCode, tokenFamilyID string, ttl time.Duration) error {
	if tokenFamilyID == "" {
		return nil
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	err := acs.storeProvider.Put(ctx, providers.NamespaceAuthzCode, consumedCodeReplayKeyPrefix+authCode,
		[]byte(tokenFamilyID), int64(ttl.Seconds()))
	if err != nil {
		return fmt.Errorf("error recording consumed authorization code marker: %w", err)
	}
	return nil
}

// ConsumedTokenFamily returns the token family id recorded for a consumed authorization code, and
// whether such a marker exists.
func (acs *authorizationCodeStore) ConsumedTokenFamily(
	ctx context.Context, authCode string) (string, bool, error) {
	data, err := acs.storeProvider.Get(ctx, providers.NamespaceAuthzCode, consumedCodeReplayKeyPrefix+authCode)
	if err != nil {
		return "", false, fmt.Errorf("error reading consumed authorization code marker: %w", err)
	}
	if data == nil {
		return "", false, nil
	}
	return string(data), true, nil
}

// GetAuthorizationCode retrieves an authorization code by code value.
func (acs *authorizationCodeStore) GetAuthorizationCode(
	ctx context.Context, authCode string,
) (*AuthorizationCode, error) {
	data, err := acs.storeProvider.Get(ctx, providers.NamespaceAuthzCode, authCode)
	if err != nil {
		return nil, fmt.Errorf("error while retrieving authorization code: %w", err)
	}
	if data == nil {
		return nil, errAuthorizationCodeNotFound
	}

	var authzCode AuthorizationCode
	if err := json.Unmarshal(data, &authzCode); err != nil {
		return nil, fmt.Errorf("failed to unmarshal authorization code: %w", err)
	}

	return &authzCode, nil
}
