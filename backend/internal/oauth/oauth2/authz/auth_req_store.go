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

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// authRequestContext holds OAuth authorization request information.
type authRequestContext struct {
	OAuthParameters model.OAuthParameters
}

// authorizationRequestStoreInterface defines the interface for authorization request storage.
type authorizationRequestStoreInterface interface {
	AddRequest(ctx context.Context, value authRequestContext) (string, error)
	GetRequest(ctx context.Context, key string) (bool, authRequestContext, error)
	ClearRequest(ctx context.Context, key string) error
}

// authorizationRequestStore provides the authorization request store functionality using database.
type authorizationRequestStore struct {
	storeProvider  providers.RuntimeStoreProvider
	validityPeriod time.Duration
}

// newAuthorizationRequestStore creates a new instance of authorizationRequestStore with injected dependencies.
func newAuthorizationRequestStore(storeProvider providers.RuntimeStoreProvider) authorizationRequestStoreInterface {
	return &authorizationRequestStore{
		storeProvider:  storeProvider,
		validityPeriod: 10 * time.Minute,
	}
}

// AddRequest adds an authorization request context entry to the store.
func (authzRS *authorizationRequestStore) AddRequest(ctx context.Context, value authRequestContext) (string, error) {
	key, err := utils.GenerateUUIDv7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}

	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request context: %w", err)
	}

	ttlSeconds := int64(authzRS.validityPeriod.Seconds())
	err = authzRS.storeProvider.Put(ctx, providers.NamespaceAuthzReq, key, data, ttlSeconds)
	if err != nil {
		return "", fmt.Errorf("failed to insert authorization request: %w", err)
	}

	return key, nil
}

// GetRequest retrieves an authorization request context entry from the store.
func (authzRS *authorizationRequestStore) GetRequest(
	ctx context.Context, key string) (bool, authRequestContext, error) {
	if key == "" {
		return false, authRequestContext{}, nil
	}

	data, err := authzRS.storeProvider.Get(ctx, providers.NamespaceAuthzReq, key)
	if err != nil {
		return false, authRequestContext{}, fmt.Errorf("failed to get authorization request: %w", err)
	}
	if data == nil {
		return false, authRequestContext{}, nil
	}
	var value authRequestContext
	if err = json.Unmarshal(data, &value); err != nil {
		return false, authRequestContext{}, fmt.Errorf("failed to unmarshal authorization request: %w", err)
	}

	return true, value, nil
}

// ClearRequest removes a specific authorization request context entry from the store.
func (authzRS *authorizationRequestStore) ClearRequest(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}

	err := authzRS.storeProvider.Delete(ctx, providers.NamespaceAuthzReq, key)
	if err != nil {
		return fmt.Errorf("failed to delete authorization request: %w", err)
	}

	return nil
}
