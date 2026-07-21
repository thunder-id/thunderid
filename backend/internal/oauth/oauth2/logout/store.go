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

package logout

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	jsonKeyLogoutAppID       = "app_id"
	jsonKeyLogoutRedirectURI = "post_logout_redirect_uri"
	jsonKeyLogoutState       = "state"
)

// logoutRequestContext is the validated RP-initiated logout target held server-side between the
// end_session_endpoint request and the sign-out flow's completion callback. Keeping it here (rather
// than in the flow) leaves the flow engine protocol-agnostic and gives OAuth a hook to run
// protocol-level actions on completion.
type logoutRequestContext struct {
	AppID                 string
	PostLogoutRedirectURI string
	State                 string
}

// logoutRequestStoreInterface stores and retrieves logout request contexts.
type logoutRequestStoreInterface interface {
	// AddRequest persists a logout request context and returns its generated id.
	AddRequest(ctx context.Context, value logoutRequestContext) (string, error)
	// GetRequest returns the context for an id, reporting whether a live (unexpired) entry was found.
	GetRequest(ctx context.Context, key string) (bool, logoutRequestContext, error)
	// ClearRequest removes the entry for an id so it cannot be replayed.
	ClearRequest(ctx context.Context, key string) error
}

// logoutRequestStore persists logout request contexts in the runtime store, so the backend follows the
// configured runtime transient datasource (relational database, Redis, or in-memory) rather than being tied to one.
type logoutRequestStore struct {
	runtimeStore   providers.RuntimeStoreProvider
	validityPeriod time.Duration
}

func newLogoutRequestStore(runtimeStore providers.RuntimeStoreProvider) logoutRequestStoreInterface {
	return &logoutRequestStore{
		runtimeStore:   runtimeStore,
		validityPeriod: 10 * time.Minute,
	}
}

func (s *logoutRequestStore) AddRequest(ctx context.Context, value logoutRequestContext) (string, error) {
	key, err := utils.GenerateUUIDv7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	jsonDataBytes, err := json.Marshal(map[string]interface{}{
		jsonKeyLogoutAppID:       value.AppID,
		jsonKeyLogoutRedirectURI: value.PostLogoutRedirectURI,
		jsonKeyLogoutState:       value.State,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal logout request context to JSON: %w", err)
	}
	ttlSeconds := int64(s.validityPeriod.Seconds())
	if err := s.runtimeStore.Put(ctx, providers.NamespaceLogoutReq, key, jsonDataBytes, ttlSeconds); err != nil {
		return "", fmt.Errorf("failed to store logout request: %w", err)
	}
	return key, nil
}

func (s *logoutRequestStore) GetRequest(ctx context.Context, key string) (bool, logoutRequestContext, error) {
	if key == "" {
		return false, logoutRequestContext{}, nil
	}
	data, err := s.runtimeStore.Get(ctx, providers.NamespaceLogoutReq, key)
	if err != nil {
		return false, logoutRequestContext{}, fmt.Errorf("failed to get logout request: %w", err)
	}
	if data == nil {
		return false, logoutRequestContext{}, nil
	}
	value, err := unmarshalLogoutRequestContext(data)
	if err != nil {
		return false, logoutRequestContext{}, err
	}
	return true, value, nil
}

func (s *logoutRequestStore) ClearRequest(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	if err := s.runtimeStore.Delete(ctx, providers.NamespaceLogoutReq, key); err != nil {
		return fmt.Errorf("failed to delete logout request: %w", err)
	}
	return nil
}

func unmarshalLogoutRequestContext(dataBytes []byte) (logoutRequestContext, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return logoutRequestContext{}, fmt.Errorf("failed to unmarshal logout request JSON: %w", err)
	}

	value := logoutRequestContext{}
	if s, ok := data[jsonKeyLogoutAppID].(string); ok {
		value.AppID = s
	}
	if s, ok := data[jsonKeyLogoutRedirectURI].(string); ok {
		value.PostLogoutRedirectURI = s
	}
	if s, ok := data[jsonKeyLogoutState].(string); ok {
		value.State = s
	}
	return value, nil
}
