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

package channel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/secretstore"
)

// fakeStore is an in-memory SecretStore.
type fakeStore struct {
	secrets map[string]secretstore.Secret
	putErr  error
}

func newFakeStore() *fakeStore {
	return &fakeStore{secrets: map[string]secretstore.Secret{}}
}

func (f *fakeStore) Put(_ context.Context, secret secretstore.Secret) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.secrets[secret.Name] = secret
	return nil
}

func (f *fakeStore) Get(_ context.Context, name string) (secretstore.Secret, bool) {
	s, ok := f.secrets[name]
	return s, ok
}

func (f *fakeStore) All(context.Context) map[string]secretstore.Secret { return f.secrets }

func (f *fakeStore) Names(context.Context) []string {
	out := make([]string, 0, len(f.secrets))
	for name := range f.secrets {
		out = append(out, name)
	}
	return out
}

// call invokes a registered handler the way the read loop would.
func call(t *testing.T, router *Router, method string, params any) (json.RawMessage, *Error) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	resp := router.Dispatch(context.Background(), Request{
		JSONRPC: Version, ID: "1", Method: method, Params: raw,
	})
	return resp.Result, resp.Error
}

func TestSecretPutStoresAndDoesNotEchoTheValue(t *testing.T) {
	store := newFakeStore()
	router := NewRouter()
	RegisterSecretMethods(router, store)

	secret := secretstore.Secret{
		Name: "CONNECTION_A_CLIENT_SECRET", Kind: secretstore.KindValue, Value: "the-secret",
	}
	raw, rpcErr := call(t, router, MethodSecretPut, secret)
	if rpcErr != nil {
		t.Fatalf("put: %+v", rpcErr)
	}
	if store.secrets["CONNECTION_A_CLIENT_SECRET"].Value != "the-secret" {
		t.Fatalf("the secret should have been stored, got %+v", store.secrets)
	}
	// A reply travels back over the same socket, so it must not carry the value.
	if bytesContain(raw, "the-secret") {
		t.Fatalf("the stored value must not be echoed back, got %s", raw)
	}
}

func TestSecretNamesReportsKindsWithoutValues(t *testing.T) {
	store := newFakeStore()
	store.secrets["USER_ADMIN_PASSWORD"] = secretstore.Secret{
		Name: "USER_ADMIN_PASSWORD", Kind: secretstore.KindHash, Value: "the-hash",
	}
	router := NewRouter()
	RegisterSecretMethods(router, store)

	raw, rpcErr := call(t, router, MethodSecretNames, nil)
	if rpcErr != nil {
		t.Fatalf("names: %+v", rpcErr)
	}
	var result secretNamesResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Names) != 1 || result.Names[0] != "USER_ADMIN_PASSWORD" {
		t.Fatalf("unexpected names: %v", result.Names)
	}
	if result.Kinds["USER_ADMIN_PASSWORD"] != secretstore.KindHash {
		t.Fatalf("unexpected kinds: %v", result.Kinds)
	}
	// Knowing a credential is present must not require shipping it.
	if bytesContain(raw, "the-hash") {
		t.Fatalf("a listing must carry no values, got %s", raw)
	}
}

func TestSecretGetReportsAbsenceRatherThanFailing(t *testing.T) {
	router := NewRouter()
	RegisterSecretMethods(router, newFakeStore())

	raw, rpcErr := call(t, router, MethodSecretGet, secretGetParams{Name: "NOT_HELD"})
	if rpcErr != nil {
		t.Fatalf("a secret that is not held is an answer, not an error: %+v", rpcErr)
	}
	var result secretGetResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Found {
		t.Fatal("expected found to be false")
	}
}

// A Data Plane that serves no store of its own must say so, rather than answering as if it holds
// nothing: the two are different, and only the first is a misconfiguration.
func TestNoStoreRegistersNoSecretMethods(t *testing.T) {
	router := NewRouter()
	RegisterSecretMethods(router, nil)

	_, rpcErr := call(t, router, MethodSecretNames, nil)
	if rpcErr == nil || rpcErr.Code != CodeMethodNotFound {
		t.Fatalf("expected method-not-found, got %+v", rpcErr)
	}
}

func TestSecretPutReportsAStoreRejection(t *testing.T) {
	store := newFakeStore()
	store.putErr = errors.New("a hash needs an algorithm")
	router := NewRouter()
	RegisterSecretMethods(router, store)

	_, rpcErr := call(t, router, MethodSecretPut,
		secretstore.Secret{Name: "X", Kind: secretstore.KindHash})
	if rpcErr == nil || rpcErr.Code != CodeInvalidParams {
		t.Fatalf("expected invalid-params, got %+v", rpcErr)
	}
}

func bytesContain(raw json.RawMessage, want string) bool {
	return strings.Contains(string(raw), want)
}
