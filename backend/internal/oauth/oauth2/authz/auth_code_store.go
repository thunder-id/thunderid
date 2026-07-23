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

// AuthorizationCodeStoreInterface defines the interface for managing authorization codes.
type AuthorizationCodeStoreInterface interface {
	InsertAuthorizationCode(ctx context.Context, authzCode AuthorizationCode) error
	ConsumeAuthorizationCode(ctx context.Context, authCode string) (bool, error)
	GetAuthorizationCode(ctx context.Context, authCode string) (*AuthorizationCode, error)
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
