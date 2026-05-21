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

package par

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

func TestInitializeWithInjectedStore(t *testing.T) {
	host := newTestMemoryRuntimeStore()
	store := NewStoreFromRuntime(host)

	internal, err := InternalFromPublic(thunderidengine.PushedAuthorizationRequest{
		ClientID: "client-a",
		OAuthParameters: mustMarshalOAuthParams(t, oauth2model.OAuthParameters{
			ClientID: "client-a",
			State:    "xyz",
		}),
	})
	require.NoError(t, err)

	key, err := store.Store(context.Background(), internal, 120)
	require.NoError(t, err)
	require.NotEmpty(t, key)

	got, found, err := store.Consume(context.Background(), key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "client-a", got.ClientID)
}

type testMemoryRuntimeStore struct {
	mu  sync.Mutex
	par map[string]thunderidengine.PushedAuthorizationRequest
}

func newTestMemoryRuntimeStore() *testMemoryRuntimeStore {
	return &testMemoryRuntimeStore{par: make(map[string]thunderidengine.PushedAuthorizationRequest)}
}

func (m *testMemoryRuntimeStore) Store(
	_ context.Context, parRequest thunderidengine.PushedAuthorizationRequest, _ int64,
) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "k-" + parRequest.ClientID
	m.par[key] = parRequest
	return requestURIPrefix + key, nil
}

func (m *testMemoryRuntimeStore) Consume(
	_ context.Context, requestURI string,
) (thunderidengine.PushedAuthorizationRequest, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := requestURI[len(requestURIPrefix):]
	req, ok := m.par[key]
	if !ok {
		return thunderidengine.PushedAuthorizationRequest{}, false, nil
	}
	delete(m.par, key)
	return req, true, nil
}

func (m *testMemoryRuntimeStore) AddRequest(context.Context, thunderidengine.AuthRequestContext) (string, error) {
	return "", nil
}

func (m *testMemoryRuntimeStore) GetRequest(context.Context, string) (bool, thunderidengine.AuthRequestContext, error) {
	return false, thunderidengine.AuthRequestContext{}, nil
}

func (m *testMemoryRuntimeStore) ClearRequest(context.Context, string) error { return nil }

func (m *testMemoryRuntimeStore) InsertAuthorizationCode(context.Context, thunderidengine.AuthorizationCode) error {
	return nil
}

func (m *testMemoryRuntimeStore) ConsumeAuthorizationCode(context.Context, string) (bool, error) {
	return false, nil
}

func (m *testMemoryRuntimeStore) GetAuthorizationCode(
	context.Context, string,
) (*thunderidengine.AuthorizationCode, error) {
	return nil, nil
}

func (m *testMemoryRuntimeStore) StoreFlowContext(context.Context, thunderidengine.FlowContext, int64) error {
	return nil
}

func (m *testMemoryRuntimeStore) GetFlowContext(context.Context, string) (*thunderidengine.FlowContext, error) {
	return nil, nil
}

func (m *testMemoryRuntimeStore) UpdateFlowContext(context.Context, thunderidengine.FlowContext) error {
	return nil
}

func (m *testMemoryRuntimeStore) DeleteFlowContext(context.Context, string) error { return nil }

func mustMarshalOAuthParams(t *testing.T, params oauth2model.OAuthParameters) thunderidengine.OAuthParameters {
	t.Helper()
	raw, err := json.Marshal(params)
	require.NoError(t, err)
	return thunderidengine.OAuthParameters(raw)
}
