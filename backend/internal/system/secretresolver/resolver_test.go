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
	"time"
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

// gets reports the counter under the lock, because the handler increments it on the server's
// goroutine while the test reads it on its own.
func (f *fakeProvider) gets() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getHits
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
	return r.Header.Get("Authorization") == "Bearer "+f.token
}

func newTestResolver(t *testing.T, fake *fakeProvider) *Resolver {
	t.Helper()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, Token: fake.token})
}

func TestIsReference(t *testing.T) {
	if !IsReference("secret:MY_SECRET") || ReferenceName("secret:MY_SECRET") != "MY_SECRET" {
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
		got, err := r.Resolve(context.Background(), "secret:MY_SECRET")
		if err != nil || got != "shhh" {
			t.Fatalf("resolve: %q err=%v", got, err)
		}
	}
	// Resolution must be served from memory, so no single-name fetch should have happened.
	if fake.gets() != 0 {
		t.Fatalf("expected no single-secret fetches, got %d", fake.gets())
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

	got, err := r.Resolve(context.Background(), "secret:LATE_SECRET")
	if err != nil || got != "arrived" {
		t.Fatalf("expected the late secret to be fetched, got %q err=%v", got, err)
	}
	// It is cached afterwards, so a second resolve costs nothing.
	before := fake.gets()
	if _, err := r.Resolve(context.Background(), "secret:LATE_SECRET"); err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if fake.gets() != before {
		t.Fatalf("the second resolve should be served from cache")
	}
}

func TestResolveThrottlesRepeatedMisses(t *testing.T) {
	fake := &fakeProvider{secrets: map[string]string{}}
	r := newTestResolver(t, fake)

	for i := 0; i < 5; i++ {
		_, err := r.Resolve(context.Background(), "secret:ABSENT")
		if !errors.Is(err, ErrSecretNotFound) {
			t.Fatalf("expected ErrSecretNotFound, got %v", err)
		}
	}
	// Only the first attempt should reach the provider; a known miss must not be retried per request.
	if fake.gets() != 1 {
		t.Fatalf("expected a single provider call for a repeated miss, got %d", fake.gets())
	}
}

// ResolveHash runs on every authentication against a credential held as a reference, so a name the
// provider does not hold must not mean an outbound request per attempt.
func TestResolveHashThrottlesRepeatedMisses(t *testing.T) {
	fake := &fakeProvider{secrets: map[string]string{}}
	r := newTestResolver(t, fake)

	for i := 0; i < 5; i++ {
		hash, found, err := r.ResolveHash(context.Background(), "secret:ABSENT")
		if err != nil {
			t.Fatalf("a missing secret is not an error, got %v", err)
		}
		if found {
			t.Fatalf("expected no hash for an absent secret, got %#v", hash)
		}
	}

	if fake.gets() != 1 {
		t.Fatalf("expected a single provider call for a repeated miss, got %d", fake.gets())
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
	if _, err := r.Resolve(context.Background(), "secret:MY_SECRET"); !errors.Is(err, ErrNotConfigured) {
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

	got, err := r.Resolve(context.Background(), "secret:ADMIN_PASSWORD")
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
	got, err := r.Resolve(context.Background(), "secret:VONAGE_API_SECRET")
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

	first, err := r.Resolve(ctx, "secret:APP_CLIENT_SECRET")
	if err != nil || first != "the-old-value" {
		t.Fatalf("expected the stored value, got %q %v", first, err)
	}

	// The control plane pushes a regenerated credential into the store.
	stored["APP_CLIENT_SECRET"] = providerSecret{Kind: "value", Value: "the-new-value"}

	got, err := r.Resolve(ctx, "secret:APP_CLIENT_SECRET")
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

	if _, _, err := r.ResolveHash(ctx, "secret:APP_CLIENT_SECRET"); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	stored["APP_CLIENT_SECRET"] = LocalSecret{
		Kind: "hash", Value: "the-new-hash", Algorithm: "PBKDF2", Salt: "new-salt",
	}

	hash, found, err := r.ResolveHash(ctx, "secret:APP_CLIENT_SECRET")

	if err != nil || !found {
		t.Fatalf("expected the hash, got %v %v", found, err)
	}
	if hash.Value != "the-new-hash" || hash.Salt != "new-salt" {
		t.Fatalf("expected the regenerated hash, got %+v", hash)
	}
}

// A secret name comes from stored configuration, so it is not necessarily well formed. Escaping it
// keeps a name carrying a slash or a traversal sequence from addressing something other than itself.
func TestSecretNameIsEscapedIntoOnePathSegment(t *testing.T) {
	cases := []struct{ name, want string }{
		{"MY_SECRET", "MY_SECRET"},
		{"a/b", "a%2Fb"},
		{"../admin", "..%2Fadmin"},
		{"a?b=c", "a%3Fb=c"},
	}
	for _, c := range cases {
		got, err := secretPathSegment(c.name)
		if err != nil {
			t.Errorf("secretPathSegment(%q) returned %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("secretPathSegment(%q) = %q, want %q", c.name, got, c.want)
		}
	}

	for _, name := range []string{"", "   ", ".", ".."} {
		if _, err := secretPathSegment(name); err == nil {
			t.Errorf("expected %q to be refused as a secret name", name)
		}
	}
}

// The provider token is a bearer credential, so it is sent over TLS, or to a provider on this host
// where the request never reaches a network. Anything else is refused rather than sent in the clear.
func TestTokenIsNotSentOverPlaintextToARemoteProvider(t *testing.T) {
	var authorized bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorized = r.Header.Get("Authorization") != ""
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// The test server answers on loopback under a name that is not one, so the request is refused.
	remote := strings.Replace(srv.URL, "127.0.0.1", "provider.invalid", 1)
	r := New(Config{BaseURL: remote, Token: "shhh", Timeout: time.Second})

	_, err := r.Resolve(context.Background(), "secret:MY_SECRET")
	if err == nil {
		t.Fatal("expected the plaintext request to be refused")
	}
	if !strings.Contains(err.Error(), "refusing to send") {
		t.Fatalf("expected the refusal to say why, got %v", err)
	}
	if authorized {
		t.Fatal("the token must not have reached the provider")
	}
}
