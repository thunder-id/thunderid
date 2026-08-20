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

package entity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/secretresolver"
)

// serveKVHash stands up a secret provider holding one hash backed secret.
func serveKVHash(t *testing.T, name string, hash cryptolib.Credential) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"secrets": map[string]interface{}{
			name: map[string]interface{}{
				"kind":      "hash",
				"value":     hash.Hash,
				"algorithm": string(hash.Algorithm),
				"parameters": map[string]interface{}{
					"salt":       hash.Parameters.Salt,
					"iterations": hash.Parameters.Iterations,
					"keySize":    hash.Parameters.KeySize,
				},
			},
		}})
	}))
	t.Cleanup(srv.Close)

	previous := secretresolver.Default()
	r := secretresolver.New(secretresolver.Config{BaseURL: srv.URL})
	if err := r.LoadAll(t.Context()); err != nil {
		t.Fatalf("load: %v", err)
	}
	secretresolver.SetDefault(r)
	t.Cleanup(func() { secretresolver.SetDefault(previous) })
}

// hashOf produces a credential the way the control plane would.
func hashOf(t *testing.T, plaintext string) cryptolib.Credential {
	t.Helper()
	svc, err := cryptolib.Initialize(cryptolib.HashConfig{
		Algorithm: cryptolib.PBKDF2, SaltSize: 16, Iterations: 1000, KeySize: 32,
	})
	if err != nil {
		t.Fatalf("hash service: %v", err)
	}
	cred, err := svc.Generate([]byte(plaintext))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return cred
}

func TestCredentialReference_ResolvesAPromotedCredentialFromTheSecretProvider(t *testing.T) {
	const plaintext = "the-client-secret"
	hash := hashOf(t, plaintext)
	serveKVHash(t, "MY_APP_CLIENT_SECRET", hash)

	// The database holds only a reference: the credential itself never reached this deployment's DB.
	ref, usable := credentialReference(StoredCredential{Value: "kv:MY_APP_CLIENT_SECRET"})
	if !usable {
		t.Fatal("a resolvable reference must be usable")
	}
	if ref.Hash != hash.Hash || ref.Parameters.Salt != hash.Parameters.Salt {
		t.Fatalf("the reference should carry the provider's hash and parameters, got %+v", ref)
	}
	// The parameters come from wherever the credential was made, not from this server's configuration.
	if ref.Parameters.Iterations != hash.Parameters.Iterations {
		t.Fatalf("iterations should come from the provider, got %d", ref.Parameters.Iterations)
	}
}

func TestCredentialReference_LeavesANativeCredentialAlone(t *testing.T) {
	hash := hashOf(t, "created-here")

	// A credential created on this deployment is stored normally and must keep working unchanged.
	ref, usable := credentialReference(StoredCredential{
		StorageAlgo:       hash.Algorithm,
		Value:             hash.Hash,
		StorageAlgoParams: hash.Parameters,
	})
	if !usable || ref.Hash != hash.Hash || ref.Algorithm != hash.Algorithm {
		t.Fatalf("a stored credential should be used as it stands, got %+v usable=%v", ref, usable)
	}
}

func TestCredentialReference_RejectsAnUnresolvableReference(t *testing.T) {
	serveKVHash(t, "SOMETHING_ELSE", hashOf(t, "x"))

	// An absent secret must reject rather than pass: treating it as a match would let any value in.
	if _, usable := credentialReference(StoredCredential{Value: "kv:NOT_IN_THE_PROVIDER"}); usable {
		t.Fatal("an unresolvable reference must not be usable")
	}
}

func TestHashPlaintextCredentials_KeepsAReferenceAsItIs(t *testing.T) {
	const plaintext = "the-client-secret"
	hash := hashOf(t, plaintext)
	serveKVHash(t, "MY_APP_CLIENT_SECRET", hash)

	svc := &entityService{hashService: mustHashService(t)}
	stored, err := svc.hashPlaintextCredentials(json.RawMessage(`{"clientSecret":"kv:MY_APP_CLIENT_SECRET"}`))
	if err != nil {
		t.Fatalf("hash credentials: %v", err)
	}

	var creds map[string][]StoredCredential
	if err := json.Unmarshal(stored, &creds); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Hashing the reference text would store the hash of "kv:..." and reject every authentication.
	if got := creds["clientSecret"][0].Value; got != "kv:MY_APP_CLIENT_SECRET" {
		t.Fatalf("the reference should be stored verbatim, got %q", got)
	}

	// And the reference still resolves to the provider's hash, so the real secret verifies.
	ref, usable := credentialReference(creds["clientSecret"][0])
	if !usable {
		t.Fatal("the stored reference must resolve")
	}
	ok, err := mustHashService(t).Verify([]byte(plaintext), ref)
	if err != nil || !ok {
		t.Fatalf("the promoted credential should verify, ok=%v err=%v", ok, err)
	}
}

// mustHashService builds the hashing this server would be configured with.
func mustHashService(t *testing.T) cryptolib.HashServiceInterface {
	t.Helper()
	svc, err := cryptolib.Initialize(cryptolib.HashConfig{
		Algorithm: cryptolib.PBKDF2, SaltSize: 16, Iterations: 1000, KeySize: 32,
	})
	if err != nil {
		t.Fatalf("hash service: %v", err)
	}
	return svc
}
