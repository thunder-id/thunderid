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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package secretresolver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeProvider stands in for the secret provider service and counts requests, so the caching and
// throttling behavior can be asserted.
type fakeProvider struct {
	mu       sync.Mutex
	secrets  map[string]string
	listHits int
	getHits  int
	token    string
}

func (f *fakeProvider) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /secrets", func(w http.ResponseWriter, r *http.Request) {
		if !f.authorized(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		f.listHits++
		secrets := f.secrets
		f.mu.Unlock()
		typed := map[string]interface{}{}
		for k, v := range secrets {
			typed[k] = map[string]interface{}{"name": k, "kind": "value", "value": v}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"secrets": typed})
	})
	mux.HandleFunc("GET /secrets/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !f.authorized(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		f.getHits++
		value, ok := f.secrets[r.PathValue("name")]
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name": r.PathValue("name"), "kind": "value", "value": value,
		})
	})
	return mux
}

func (f *fakeProvider) authorized(r *http.Request) bool {
	if f.token == "" {
		return true
	}
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") == f.token
}

func newTestResolver(t *testing.T, fake *fakeProvider) *Resolver {
	t.Helper()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, Token: fake.token})
}

func TestIsReference(t *testing.T) {
	if !IsReference("kv:MY_SECRET") || ReferenceName("kv:MY_SECRET") != "MY_SECRET" {
		t.Fatal("a kv reference should be recognized and its name extracted")
	}
	// Values that must not be mistaken for references.
	for _, value := range []string{"plain-secret", "${MY_SECRET}", "{{.MY_SECRET}}", ""} {
		if IsReference(value) {
			t.Fatalf("%q must not be treated as a reference", value)
		}
	}
}

func TestResolvePassesNonReferencesThrough(t *testing.T) {
	r := newTestResolver(t, &fakeProvider{secrets: map[string]string{}})

	got, err := r.Resolve(context.Background(), "a-literal-secret")
	if err != nil || got != "a-literal-secret" {
		t.Fatalf("expected the value unchanged, got %q err=%v", got, err)
	}
}

func TestLoadAllPopulatesTheCacheAndResolvesWithoutFurtherCalls(t *testing.T) {
	fake := &fakeProvider{secrets: map[string]string{"MY_SECRET": "shhh"}, token: "tok"}
	r := newTestResolver(t, fake)

	if err := r.LoadAll(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	if r.Count() != 1 {
		t.Fatalf("expected one cached secret, got %d", r.Count())
	}

	for i := 0; i < 3; i++ {
		got, err := r.Resolve(context.Background(), "kv:MY_SECRET")
		if err != nil || got != "shhh" {
			t.Fatalf("resolve: %q err=%v", got, err)
		}
	}
	// Resolution must be served from memory, so no single-name fetch should have happened.
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.getHits != 0 {
		t.Fatalf("expected no single-secret fetches, got %d", fake.getHits)
	}
}

func TestResolveRefreshesOnCacheMiss(t *testing.T) {
	fake := &fakeProvider{secrets: map[string]string{}}
	r := newTestResolver(t, fake)
	if err := r.LoadAll(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}

	// The secret appears after startup, which is exactly the case the refresh exists for.
	fake.mu.Lock()
	fake.secrets["LATE_SECRET"] = "arrived"
	fake.mu.Unlock()

	got, err := r.Resolve(context.Background(), "kv:LATE_SECRET")
	if err != nil || got != "arrived" {
		t.Fatalf("expected the late secret to be fetched, got %q err=%v", got, err)
	}
	// It is cached afterwards, so a second resolve costs nothing.
	before := fake.getHits
	if _, err := r.Resolve(context.Background(), "kv:LATE_SECRET"); err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if fake.getHits != before {
		t.Fatalf("the second resolve should be served from cache")
	}
}

func TestResolveThrottlesRepeatedMisses(t *testing.T) {
	fake := &fakeProvider{secrets: map[string]string{}}
	r := newTestResolver(t, fake)

	for i := 0; i < 5; i++ {
		_, err := r.Resolve(context.Background(), "kv:ABSENT")
		if !errors.Is(err, ErrSecretNotFound) {
			t.Fatalf("expected ErrSecretNotFound, got %v", err)
		}
	}
	// Only the first attempt should reach the provider; a known miss must not be retried per request.
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.getHits != 1 {
		t.Fatalf("expected a single provider call for a repeated miss, got %d", fake.getHits)
	}
}

func TestResolveFailsClearlyWhenNotConfigured(t *testing.T) {
	r := New(Config{})
	if r.Enabled() {
		t.Fatal("an empty base URL must leave the resolver disabled")
	}

	// A non-reference still passes through, so an unconfigured deployment keeps working.
	if got, err := r.Resolve(context.Background(), "plain"); err != nil || got != "plain" {
		t.Fatalf("expected pass-through, got %q err=%v", got, err)
	}
	// A reference cannot be resolved, and the error says why rather than yielding an empty secret.
	if _, err := r.Resolve(context.Background(), "kv:MY_SECRET"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestLoadAllIsANoOpWhenDisabled(t *testing.T) {
	if err := New(Config{}).LoadAll(context.Background()); err != nil {
		t.Fatalf("expected no error when disabled, got %v", err)
	}
}

func TestTokenIsSent(t *testing.T) {
	fake := &fakeProvider{secrets: map[string]string{"MY_SECRET": "shhh"}, token: "expected-token"}
	r := newTestResolver(t, fake)

	if err := r.LoadAll(context.Background()); err != nil {
		t.Fatalf("load with the right token should succeed: %v", err)
	}

	// A wrong token must surface as an error rather than an empty cache.
	wrong := New(Config{BaseURL: r.cfg.BaseURL, Token: "nope"})
	if err := wrong.LoadAll(context.Background()); err == nil {
		t.Fatal("expected an error for an unauthorized load")
	}
}

func TestResolveRendersAHashAsADeclarativeCredential(t *testing.T) {
	// A hash cannot be substituted as itself: the configuration expects a credential. It is rendered as
	// the declarative credential form so the importer stores it rather than hashing it again.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"secrets": map[string]interface{}{
			"ADMIN_PASSWORD": map[string]interface{}{
				"kind":       "hash",
				"value":      "the-hash",
				"algorithm":  "PBKDF2",
				"parameters": map[string]interface{}{"salt": "the-salt", "iterations": 1000, "keySize": 32},
			},
		}})
	}))
	defer srv.Close()

	r := New(Config{BaseURL: srv.URL})
	if err := r.LoadAll(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}

	got, err := r.Resolve(context.Background(), "kv:ADMIN_PASSWORD")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// The substitution must be JSON, which is also valid YAML, so it parses as a credential list.
	var creds []map[string]interface{}
	if err := json.Unmarshal([]byte(got), &creds); err != nil {
		t.Fatalf("substitution is not valid JSON: %q", got)
	}
	if len(creds) != 1 || creds[0]["value"] != "the-hash" || creds[0]["storageType"] != "hash" {
		t.Fatalf("unexpected credential: %s", got)
	}
	if creds[0]["storageAlgo"] != "PBKDF2" {
		t.Fatalf("the algorithm must be carried through: %s", got)
	}
}

func TestResolveSubstitutesAReadableValueAsItself(t *testing.T) {
	fake := &fakeProvider{secrets: map[string]string{"VONAGE_API_SECRET": "the-api-secret"}}
	r := newTestResolver(t, fake)
	if err := r.LoadAll(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}

	// A credential replayed to a third party has to arrive verbatim.
	got, err := r.Resolve(context.Background(), "kv:VONAGE_API_SECRET")
	if err != nil || got != "the-api-secret" {
		t.Fatalf("expected the value verbatim, got %q err=%v", got, err)
	}
}

// A credential regenerated on the control plane is written into this deployment's store while the
// server is running. Resolution has to see the new value: serving the old one means every login
// against that credential fails until the process restarts.
func TestARegeneratedSecretResolvesWithoutARestart(t *testing.T) {
	stored := map[string]providerSecret{
		"APP_CLIENT_SECRET": {Kind: "value", Value: "the-old-value"},
	}
	r := New(Config{Local: func(_ context.Context, name string) (LocalSecret, bool, error) {
		s, ok := stored[name]
		if !ok {
			return LocalSecret{}, false, nil
		}
		return LocalSecret{Kind: s.Kind, Value: s.Value, Algorithm: s.Algorithm}, true, nil
	}})
	ctx := context.Background()

	first, err := r.Resolve(ctx, "kv:APP_CLIENT_SECRET")
	if err != nil || first != "the-old-value" {
		t.Fatalf("expected the stored value, got %q %v", first, err)
	}

	// The control plane pushes a regenerated credential into the store.
	stored["APP_CLIENT_SECRET"] = providerSecret{Kind: "value", Value: "the-new-value"}

	got, err := r.Resolve(ctx, "kv:APP_CLIENT_SECRET")
	if err != nil {
		t.Fatalf("resolve after regeneration: %v", err)
	}
	if got != "the-new-value" {
		t.Fatalf("expected the regenerated value, got %q", got)
	}
}

// The same for a hash: a regenerated password or client secret is verified against the hash held
// here, so a stale one rejects every attempt.
func TestARegeneratedHashResolvesWithoutARestart(t *testing.T) {
	stored := map[string]LocalSecret{
		"APP_CLIENT_SECRET": {Kind: "hash", Value: "the-old-hash", Algorithm: "PBKDF2", Salt: "old-salt"},
	}
	r := New(Config{Local: func(_ context.Context, name string) (LocalSecret, bool, error) {
		s, ok := stored[name]
		return s, ok, nil
	}})
	ctx := context.Background()

	if _, _, err := r.ResolveHash(ctx, "kv:APP_CLIENT_SECRET"); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	stored["APP_CLIENT_SECRET"] = LocalSecret{
		Kind: "hash", Value: "the-new-hash", Algorithm: "PBKDF2", Salt: "new-salt",
	}

	hash, found, err := r.ResolveHash(ctx, "kv:APP_CLIENT_SECRET")

	if err != nil || !found {
		t.Fatalf("expected the hash, got %v %v", found, err)
	}
	if hash.Value != "the-new-hash" || hash.Salt != "new-salt" {
		t.Fatalf("expected the regenerated hash, got %+v", hash)
	}
}
