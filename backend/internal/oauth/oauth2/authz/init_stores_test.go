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

package authz

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

func TestInitOptionsInjectedCodeStore(t *testing.T) {
	host := newTestMemoryCodeStore()
	codeStore := NewCodeStoreFromRuntime(host)

	code := AuthorizationCode{
		CodeID:      "id",
		Code:        "secret-code",
		ClientID:    "client",
		TimeCreated: time.Now().UTC(),
		ExpiryTime:  time.Now().UTC().Add(time.Minute),
	}

	err := codeStore.InsertAuthorizationCode(context.Background(), code)
	require.NoError(t, err)

	got, err := codeStore.GetAuthorizationCode(context.Background(), "secret-code")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, code.Code, got.Code)
}

type testMemoryCodeStore struct {
	mu    sync.Mutex
	codes map[string]thunderidengine.AuthorizationCode
}

func newTestMemoryCodeStore() *testMemoryCodeStore {
	return &testMemoryCodeStore{codes: make(map[string]thunderidengine.AuthorizationCode)}
}

func (m *testMemoryCodeStore) Store(
	context.Context, thunderidengine.PushedAuthorizationRequest, int64,
) (string, error) {
	return "", nil
}

func (m *testMemoryCodeStore) Consume(
	context.Context, string,
) (thunderidengine.PushedAuthorizationRequest, bool, error) {
	return thunderidengine.PushedAuthorizationRequest{}, false, nil
}

func (m *testMemoryCodeStore) AddRequest(context.Context, thunderidengine.AuthRequestContext) (string, error) {
	return "", nil
}

func (m *testMemoryCodeStore) GetRequest(context.Context, string) (bool, thunderidengine.AuthRequestContext, error) {
	return false, thunderidengine.AuthRequestContext{}, nil
}

func (m *testMemoryCodeStore) ClearRequest(context.Context, string) error { return nil }

func (m *testMemoryCodeStore) InsertAuthorizationCode(
	_ context.Context, code thunderidengine.AuthorizationCode,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[code.Code] = code
	return nil
}

func (m *testMemoryCodeStore) ConsumeAuthorizationCode(_ context.Context, authCode string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.codes[authCode]; !ok {
		return false, nil
	}
	delete(m.codes, authCode)
	return true, nil
}

func (m *testMemoryCodeStore) GetAuthorizationCode(
	_ context.Context, authCode string,
) (*thunderidengine.AuthorizationCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	code, ok := m.codes[authCode]
	if !ok {
		return nil, nil
	}
	return &code, nil
}

func (m *testMemoryCodeStore) StoreFlowContext(context.Context, thunderidengine.FlowContext, int64) error {
	return nil
}

func (m *testMemoryCodeStore) GetFlowContext(context.Context, string) (*thunderidengine.FlowContext, error) {
	return nil, nil
}

func (m *testMemoryCodeStore) UpdateFlowContext(context.Context, thunderidengine.FlowContext) error {
	return nil
}

func (m *testMemoryCodeStore) DeleteFlowContext(context.Context, string) error { return nil }
