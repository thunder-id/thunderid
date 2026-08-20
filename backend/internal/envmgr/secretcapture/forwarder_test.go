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

package secretcapture

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

const testSecretValue = "value"

// captured records what the fake secret service received.
type captured struct {
	mu    sync.Mutex
	path  string
	token string
	body  map[string]interface{}
}

func newFakeSecretService(t *testing.T) (*httptest.Server, *captured) {
	t.Helper()
	rec := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.path = r.URL.Path
		rec.token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		rec.body = map[string]interface{}{}
		_ = json.Unmarshal(raw, &rec.body)
		rec.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func TestForwarderSendsAReplayableCredentialAsAReadableValue(t *testing.T) {
	srv, rec := newFakeSecretService(t)
	f := &secretForwarder{baseURL: srv.URL, token: "tok", http: srv.Client()}

	// A Vonage secret has to be replayed to a third party, so hashing it would make it useless.
	f.CaptureSecret(tenantCtx(), "connection", "Corporate Vonage", "APISecret", "the-api-secret")

	rec.mu.Lock()
	defer rec.mu.Unlock()
	// The tenant is in the path, because the environment manager routes on it.
	if rec.path != "/tenants/dev-tenant/secrets/CONNECTION_CORPORATE_VONAGE_API_SECRET" {
		t.Fatalf("unexpected key path: %s", rec.path)
	}
	if rec.token != "tok" {
		t.Fatalf("the token was not sent: %q", rec.token)
	}
	if rec.body["kind"] != testSecretValue || rec.body[testSecretValue] != "the-api-secret" {
		t.Fatalf("expected a readable value, got %#v", rec.body)
	}
}

func TestForwarderKeepsAConnectionClientSecretReadable(t *testing.T) {
	srv, rec := newFakeSecretService(t)
	f := &secretForwarder{baseURL: srv.URL, http: srv.Client(), hashConfig: testHashConfig}

	// The field name is the same one an application uses, but a connection's client secret is sent on
	// to the upstream provider rather than verified here, so hashing it would break every login
	// through that connection.
	f.CaptureReplayableSecret(tenantCtx(), "connection", "My IdP", "ClientSecret", "the-client-secret")

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.body["kind"] != testSecretValue || rec.body[testSecretValue] != "the-client-secret" {
		t.Fatalf("expected a readable value, got %#v", rec.body)
	}
}

func TestForwarderNeverSendsAVerifiableCredentialInClear(t *testing.T) {
	srv, rec := newFakeSecretService(t)
	f := &secretForwarder{baseURL: srv.URL, http: srv.Client(), hashConfig: testHashConfig}

	const plaintext = "the-client-secret"
	f.CaptureSecret(tenantCtx(), "application", "My App", "ClientSecret", plaintext)

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.body["kind"] != "hash" {
		t.Fatalf("a client secret must be forwarded as a hash, got %#v", rec.body)
	}
	// The whole point: the plaintext must not appear anywhere in the payload.
	raw, _ := json.Marshal(rec.body)
	if strings.Contains(string(raw), plaintext) {
		t.Fatalf("the plaintext leaked into the payload: %s", raw)
	}
	if rec.body["algorithm"] == "" || rec.body["algorithm"] == nil {
		t.Fatalf("a hash needs its algorithm to be verifiable later: %#v", rec.body)
	}
}

func TestForwarderIgnoresAnEmptyValue(t *testing.T) {
	srv, rec := newFakeSecretService(t)
	f := &secretForwarder{baseURL: srv.URL, http: srv.Client(), hashConfig: testHashConfig}

	f.CaptureSecret(tenantCtx(), "application", "My App", "ClientSecret", "")

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.path != "" {
		t.Fatalf("nothing should have been sent, got a write to %s", rec.path)
	}
}

func TestForwarderSurvivesAnUnavailableService(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	f := &secretForwarder{baseURL: srv.URL, http: srv.Client()}

	// Creating a resource must not fail because the secret service is briefly unavailable, so the
	// failure is absorbed rather than propagated.
	f.CaptureSecret(tenantCtx(), "connection", "Corporate Vonage", "APISecret", "v")

	if calls != 1 {
		t.Fatalf("expected the write to have been attempted once, got %d", calls)
	}
}

func TestNewSecretForwarderIsDisabledWithoutAURL(t *testing.T) {
	if f := newSecretForwarder(configWithSecretProviderURL(""), testHashConfig); f != nil {
		t.Fatal("an empty URL must leave forwarding disabled so the local store is used")
	}
	if f := newSecretForwarder(configWithSecretProviderURL("http://localhost:9098"), testHashConfig); f == nil {
		t.Fatal("a configured URL should enable forwarding")
	}
}

// testHashConfig stands in for the server's configured hashing.
func testHashConfig() (cryptolib.HashConfig, error) {
	return cryptolib.HashConfig{Algorithm: cryptolib.PBKDF2, SaltSize: 16, Iterations: 1000, KeySize: 32}, nil
}

// tenantCtx carries the tenant a captured credential belongs to, as a request would.
func tenantCtx() context.Context {
	return deployment.WithID(context.Background(), "dev-tenant")
}

// configWithSecretProviderURL builds the minimum configuration the forwarder reads.
func configWithSecretProviderURL(url string) config.Config {
	var cfg config.Config
	cfg.Server.SecurityConfig.SecretProvider.Service.URL = url
	return cfg
}

func TestForwarderRefusesToRouteWithoutATenant(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	f := &secretForwarder{baseURL: srv.URL, http: srv.Client()}

	// Without a tenant the secret cannot be routed to the right provider, and sending it anywhere would
	// put a credential on the wrong data plane.
	f.CaptureSecret(context.Background(), "connection", "Corporate Vonage", "APISecret", "v")

	if calls != 0 {
		t.Fatalf("nothing should have been sent without a tenant, got %d call(s)", calls)
	}
}
