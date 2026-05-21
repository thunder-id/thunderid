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

package enginebridge

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	oauthauthz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type memoryRuntimeStore struct {
	mu sync.Mutex

	parByKey map[string]thunderidengine.PushedAuthorizationRequest
	codes    map[string]thunderidengine.AuthorizationCode
}

func newMemoryRuntimeStore() *memoryRuntimeStore {
	return &memoryRuntimeStore{
		parByKey: make(map[string]thunderidengine.PushedAuthorizationRequest),
		codes:    make(map[string]thunderidengine.AuthorizationCode),
	}
}

func (m *memoryRuntimeStore) Store(
	_ context.Context, parRequest thunderidengine.PushedAuthorizationRequest, _ int64,
) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "par-key-" + parRequest.ClientID
	m.parByKey[key] = parRequest
	return parRequestURIPrefix + key, nil
}

func (m *memoryRuntimeStore) Consume(
	_ context.Context, requestURI string,
) (thunderidengine.PushedAuthorizationRequest, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := stringsTrimPARPrefix(requestURI)
	req, ok := m.parByKey[key]
	if !ok {
		return thunderidengine.PushedAuthorizationRequest{}, false, nil
	}
	delete(m.parByKey, key)
	return req, true, nil
}

func (m *memoryRuntimeStore) AddRequest(context.Context, thunderidengine.AuthRequestContext) (string, error) {
	return "", nil
}

func (m *memoryRuntimeStore) GetRequest(context.Context, string) (bool, thunderidengine.AuthRequestContext, error) {
	return false, thunderidengine.AuthRequestContext{}, nil
}

func (m *memoryRuntimeStore) ClearRequest(context.Context, string) error { return nil }

func (m *memoryRuntimeStore) InsertAuthorizationCode(
	_ context.Context, code thunderidengine.AuthorizationCode,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[code.Code] = code
	return nil
}

func (m *memoryRuntimeStore) ConsumeAuthorizationCode(_ context.Context, authCodeString string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.codes[authCodeString]; !ok {
		return false, nil
	}
	delete(m.codes, authCodeString)
	return true, nil
}

func (m *memoryRuntimeStore) GetAuthorizationCode(
	_ context.Context, authCodeString string,
) (*thunderidengine.AuthorizationCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	code, ok := m.codes[authCodeString]
	if !ok {
		return nil, nil
	}
	return &code, nil
}

func (m *memoryRuntimeStore) StoreFlowContext(context.Context, thunderidengine.FlowContext, int64) error {
	return nil
}

func (m *memoryRuntimeStore) GetFlowContext(context.Context, string) (*thunderidengine.FlowContext, error) {
	return nil, nil
}

func (m *memoryRuntimeStore) UpdateFlowContext(context.Context, thunderidengine.FlowContext) error {
	return nil
}

func (m *memoryRuntimeStore) DeleteFlowContext(context.Context, string) error { return nil }

func stringsTrimPARPrefix(requestURI string) string {
	return stringsTrimPrefix(requestURI, parRequestURIPrefix)
}

func stringsTrimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return ""
}

func TestHostRuntimeStorePARRoundTrip(t *testing.T) {
	host := newMemoryRuntimeStore()
	parStore := par.NewStoreFromRuntime(host)

	params := oauth2model.OAuthParameters{
		ClientID:     "client-1",
		State:        "state-1",
		RedirectURI:  "https://app.example/cb",
		ResponseType: "code",
	}
	raw, err := json.Marshal(params)
	require.NoError(t, err)

	internal, err := par.InternalFromPublic(thunderidengine.PushedAuthorizationRequest{
		ClientID:        "client-1",
		OAuthParameters: thunderidengine.OAuthParameters(raw),
	})
	require.NoError(t, err)

	randomKey, err := parStore.Store(context.Background(), internal, 600)
	require.NoError(t, err)
	require.NotEmpty(t, randomKey)

	consumed, found, err := parStore.Consume(context.Background(), randomKey)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "client-1", consumed.ClientID)
	require.Equal(t, params.State, consumed.OAuthParameters.State)
}

func TestHostRuntimeStoreAuthorizationCodeRoundTrip(t *testing.T) {
	host := newMemoryRuntimeStore()
	codeStore := oauthauthz.NewCodeStoreFromRuntime(host)

	now := time.Now().UTC()
	internal := oauthauthz.AuthorizationCode{
		CodeID:           "id-1",
		Code:             "auth-code-1",
		ClientID:         "client-1",
		RedirectURI:      "https://app.example/cb",
		AuthorizedUserID: "user-1",
		TimeCreated:      now,
		ExpiryTime:       now.Add(time.Minute),
		Scopes:           "openid",
	}

	err := codeStore.InsertAuthorizationCode(context.Background(), internal)
	require.NoError(t, err)

	got, err := codeStore.GetAuthorizationCode(context.Background(), "auth-code-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, internal.Code, got.Code)
	require.Equal(t, internal.ClientID, got.ClientID)

	consumed, err := codeStore.ConsumeAuthorizationCode(context.Background(), "auth-code-1")
	require.NoError(t, err)
	require.True(t, consumed)

	got, err = codeStore.GetAuthorizationCode(context.Background(), "auth-code-1")
	require.NoError(t, err)
	require.Nil(t, got)
}
