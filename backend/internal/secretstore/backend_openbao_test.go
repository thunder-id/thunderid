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

package secretstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeVault stands in for OpenBao's KV version 2 engine: secrets are held at
// {mount}/data/{path} and listed from {mount}/metadata/{prefix}.
type fakeVault struct {
	// data holds each secret's body, keyed by the full request path.
	data map[string]map[string]interface{}
	// requests records the method and path of every call, so a test can assert on what was sent.
	requests []string
	// tokens records the token presented on each call.
	tokens []string
	// namespaces records the namespace header presented on each call.
	namespaces []string
}

func newFakeVault() *fakeVault {
	return &fakeVault{data: map[string]map[string]interface{}{}}
}

func (v *fakeVault) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v1/")
		v.requests = append(v.requests, r.Method+" "+path)
		v.tokens = append(v.tokens, r.Header.Get(openBaoTokenHeader))
		v.namespaces = append(v.namespaces, r.Header.Get(openBaoNamespaceHeader))

		switch {
		case r.Method == "LIST":
			v.handleList(w, path)
		case r.Method == http.MethodGet:
			v.handleGet(w, path)
		case r.Method == http.MethodPost:
			v.handlePost(w, r, path)
		case r.Method == http.MethodDelete:
			v.handleDelete(w, path)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (v *fakeVault) handleList(w http.ResponseWriter, path string) {
	prefix := metadataToData(path)
	keys := []string{}
	for held := range v.data {
		if strings.HasPrefix(held, prefix+"/") {
			keys = append(keys, strings.TrimPrefix(held, prefix+"/"))
		}
	}
	if len(keys) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeVaultJSON(w, map[string]interface{}{"data": map[string]interface{}{"keys": keys}})
}

func (v *fakeVault) handleGet(w http.ResponseWriter, path string) {
	body, held := v.data[path]
	if !held {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeVaultJSON(w, map[string]interface{}{"data": map[string]interface{}{"data": body}})
}

func (v *fakeVault) handlePost(w http.ResponseWriter, r *http.Request, path string) {
	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	v.data[path] = body.Data
	writeVaultJSON(w, map[string]interface{}{"data": map[string]interface{}{"version": 1}})
}

func (v *fakeVault) handleDelete(w http.ResponseWriter, path string) {
	dataPath := metadataToData(path)
	if _, held := v.data[dataPath]; !held {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	delete(v.data, dataPath)
	w.WriteHeader(http.StatusNoContent)
}

// metadataToData maps a metadata path onto the data path holding the same secrets. The segment has no
// trailing slash when nothing is prefixed under it, so the whole segment is replaced rather than a
// slash-delimited one.
func metadataToData(path string) string {
	return strings.Replace(path, "/metadata", "/data", 1)
}

func writeVaultJSON(w http.ResponseWriter, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// newTestOpenBao builds a backend against the fake vault, with the defaults a deployment would use.
func newTestOpenBao(t *testing.T, vault *fakeVault, cfg KVConfig) Backend {
	t.Helper()
	srv := vault.server(t)
	if cfg.Address == "" {
		cfg.Address = srv.URL
	}
	if cfg.Token == "" {
		cfg.Token = "the-vault-token"
	}
	cfg.Type = KVTypeOpenBao
	backend, err := NewKVBackend(cfg)
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}
	return backend
}

func TestOpenBaoRoundTripsAReadableSecret(t *testing.T) {
	vault := newFakeVault()
	backend := newTestOpenBao(t, vault, KVConfig{PathPrefix: "thunderid/org1-dev"})
	ctx := context.Background()

	if err := backend.Put(ctx, Secret{Name: "VONAGE_API_SECRET", Kind: KindValue, Value: "the-value"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	secrets, err := backend.Load(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := secrets["VONAGE_API_SECRET"]
	if got.Value != "the-value" || got.Kind != KindValue {
		t.Fatalf("expected the stored secret back, got %+v", got)
	}
	if got.Name != "VONAGE_API_SECRET" {
		t.Fatalf("expected the secret to be named, got %q", got.Name)
	}
}

// A hash has to come back with the parameters that produced it, or the credential it stands for
// could never be verified against.
func TestOpenBaoRoundTripsAHashWithItsParameters(t *testing.T) {
	vault := newFakeVault()
	backend := newTestOpenBao(t, vault, KVConfig{})
	ctx := context.Background()
	secret := Secret{
		Name: "APP_CLIENT_SECRET", Kind: KindHash, Value: "the-hash", Algorithm: "PBKDF2",
		Parameters: HashParameters{Salt: "the-salt", Iterations: 600000, KeySize: 32},
	}

	if err := backend.Put(ctx, secret); err != nil {
		t.Fatalf("put: %v", err)
	}

	secrets, err := backend.Load(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := secrets["APP_CLIENT_SECRET"]
	if got.Kind != KindHash || got.Algorithm != "PBKDF2" {
		t.Fatalf("expected the hash back, got %+v", got)
	}
	if got.Parameters.Salt != "the-salt" || got.Parameters.Iterations != 600000 || got.Parameters.KeySize != 32 {
		t.Fatalf("expected the hash parameters to survive, got %+v", got.Parameters)
	}
}

// Deployments sharing one vault must not collide, so every path sits under the configured prefix.
func TestOpenBaoScopesEveryPathToTheConfiguredPrefix(t *testing.T) {
	vault := newFakeVault()
	backend := newTestOpenBao(t, vault, KVConfig{Mount: "kv", PathPrefix: "thunderid/org1-dev"})

	if err := backend.Put(context.Background(), Secret{Name: "A", Kind: KindValue, Value: "v"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if len(vault.requests) != 1 {
		t.Fatalf("expected one request, got %v", vault.requests)
	}
	if vault.requests[0] != "POST kv/data/thunderid/org1-dev/A" {
		t.Fatalf("expected the write to be scoped to the prefix, got %q", vault.requests[0])
	}
}

// A prefix that holds nothing yet is a 404 from the vault. That is an empty store, not a failure, or
// a deployment could never write its first secret.
func TestOpenBaoLoadsAnEmptyStoreWhenThePrefixIsAbsent(t *testing.T) {
	vault := newFakeVault()
	backend := newTestOpenBao(t, vault, KVConfig{PathPrefix: "thunderid/fresh"})

	secrets, err := backend.Load(context.Background())

	if err != nil {
		t.Fatalf("expected an absent prefix to load empty, got %v", err)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected no secrets, got %d", len(secrets))
	}
}

func TestOpenBaoDeleteRemovesASecretAndIsSafeToRepeat(t *testing.T) {
	vault := newFakeVault()
	backend := newTestOpenBao(t, vault, KVConfig{})
	ctx := context.Background()
	if err := backend.Put(ctx, Secret{Name: "GONE", Kind: KindValue, Value: "v"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := backend.Delete(ctx, "GONE"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// The vault answers 404 for a path it no longer holds, which a retried delete must tolerate.
	if err := backend.Delete(ctx, "GONE"); err != nil {
		t.Fatalf("repeated delete: %v", err)
	}

	secrets, err := backend.Load(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, held := secrets["GONE"]; held {
		t.Fatal("expected the secret to be gone")
	}
}

func TestOpenBaoPresentsTheTokenAndNamespace(t *testing.T) {
	vault := newFakeVault()
	backend := newTestOpenBao(t, vault, KVConfig{Token: "the-vault-token", Namespace: "team-a"})

	if err := backend.Put(context.Background(), Secret{Name: "A", Kind: KindValue, Value: "v"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if vault.tokens[0] != "the-vault-token" {
		t.Fatalf("expected the token to be presented, got %q", vault.tokens[0])
	}
	if vault.namespaces[0] != "team-a" {
		t.Fatalf("expected the namespace to be presented, got %q", vault.namespaces[0])
	}
}

// A rejected call must surface as an error rather than as an empty store: a deployment whose token
// has expired should say so, not appear to hold no secrets.
func TestOpenBaoReportsARejectedCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":["permission denied"]}`))
	}))
	defer srv.Close()
	backend, err := NewKVBackend(KVConfig{Type: KVTypeOpenBao, Address: srv.URL, Token: "stale"})
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}

	_, err = backend.Load(context.Background())

	if err == nil {
		t.Fatal("expected a rejected call to be reported")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected the vault's own message to be kept, got %v", err)
	}
}

// A secret written by hand with only a value is readable: it can only be a readable value, since a
// hash with no algorithm could never be verified against.
func TestOpenBaoReadsAHandWrittenSecretWithNoKind(t *testing.T) {
	vault := newFakeVault()
	backend := newTestOpenBao(t, vault, KVConfig{Mount: "secret"})
	vault.data["secret/data/SMS_KEY"] = map[string]interface{}{"value": "abc123"}

	secrets, err := backend.Load(context.Background())

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := secrets["SMS_KEY"]
	if got.Kind != KindValue || got.Value != "abc123" {
		t.Fatalf("expected a readable value, got %+v", got)
	}
}

// The vault is shared, so what one instance writes has to become visible to the others.
func TestOpenBaoReportsANonZeroCacheTTL(t *testing.T) {
	vault := newFakeVault()
	backend := newTestOpenBao(t, vault, KVConfig{})

	if backend.CacheTTL() <= 0 {
		t.Fatal("a shared backend must report a cache TTL, or another instance's write is never seen")
	}
}

// The name identifies where secrets are kept, for the startup log. It must not carry the token.
func TestOpenBaoNameOmitsTheToken(t *testing.T) {
	vault := newFakeVault()
	backend := newTestOpenBao(t, vault, KVConfig{Token: "the-vault-token", PathPrefix: "thunderid/org1-dev"})

	name := backend.Name()

	if strings.Contains(name, "the-vault-token") {
		t.Fatalf("the backend name must not carry the token, got %q", name)
	}
	if !strings.Contains(name, "thunderid/org1-dev") {
		t.Fatalf("expected the name to say where secrets are kept, got %q", name)
	}
}

func TestOpenBaoRequiresAnAddressAndToken(t *testing.T) {
	// Cleared so an injected token in the surrounding environment cannot stand in for the missing one.
	t.Setenv(EnvKVToken, "")

	if _, err := NewKVBackend(KVConfig{Type: KVTypeOpenBao, Token: "t"}); err == nil {
		t.Fatal("expected a vault with no address to be refused")
	}
	// Without a token every call is rejected, which would look like an empty store rather than a
	// configuration mistake.
	if _, err := NewKVBackend(KVConfig{Type: KVTypeOpenBao, Address: "https://vault:8200"}); err == nil {
		t.Fatal("expected a vault with no token to be refused")
	}
}
