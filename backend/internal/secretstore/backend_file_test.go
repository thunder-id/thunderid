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
	"os"
	"path/filepath"
	"testing"
)

func tempFileBackend(t *testing.T) Backend {
	t.Helper()
	// A nested directory that does not exist yet, so the first write has to create it.
	path := filepath.Join(t.TempDir(), "store", "secrets.json")
	backend, err := NewFileBackend(path)
	if err != nil {
		t.Fatalf("new file backend: %v", err)
	}
	return backend
}

// A deployment that has never stored a secret starts empty rather than failing, so a first start
// needs nothing provisioned ahead of it.
func TestFileBackendLoadsAnEmptyStoreWhenTheFileIsAbsent(t *testing.T) {
	secrets, err := tempFileBackend(t).Load(context.Background())

	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected an empty store, got %d", len(secrets))
	}
}

func TestFileBackendRoundTripsASecret(t *testing.T) {
	backend := tempFileBackend(t)
	ctx := context.Background()
	secret := Secret{Name: "VONAGE_API_SECRET", Kind: KindValue, Value: "the-value"}

	if err := backend.Put(ctx, secret); err != nil {
		t.Fatalf("put: %v", err)
	}

	secrets, err := backend.Load(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := secrets["VONAGE_API_SECRET"]; got.Value != "the-value" || got.Kind != KindValue {
		t.Fatalf("expected the stored secret back, got %+v", got)
	}
}

// A hash keeps the parameters that produced it: they come from wherever the credential was created
// and need not match this server's own hashing configuration, so losing them would leave a credential
// that rejects every attempt.
func TestFileBackendKeepsHashParameters(t *testing.T) {
	backend := tempFileBackend(t)
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
	if got.Algorithm != "PBKDF2" || got.Parameters.Salt != "the-salt" || got.Parameters.Iterations != 600000 {
		t.Fatalf("expected the hash parameters to survive, got %+v", got)
	}
}

// One write must not drop the secrets already stored: the whole file is rewritten each time, so a
// read-modify-write that lost the rest would empty the store one entry at a time.
func TestFileBackendKeepsTheOtherSecretsOnAWrite(t *testing.T) {
	backend := tempFileBackend(t)
	ctx := context.Background()

	if err := backend.Put(ctx, Secret{Name: "FIRST", Kind: KindValue, Value: "1"}); err != nil {
		t.Fatalf("put first: %v", err)
	}
	if err := backend.Put(ctx, Secret{Name: "SECOND", Kind: KindValue, Value: "2"}); err != nil {
		t.Fatalf("put second: %v", err)
	}

	secrets, err := backend.Load(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(secrets) != 2 {
		t.Fatalf("expected both secrets, got %d", len(secrets))
	}
}

func TestFileBackendDeleteRemovesASecretAndIsSafeToRepeat(t *testing.T) {
	backend := tempFileBackend(t)
	ctx := context.Background()
	if err := backend.Put(ctx, Secret{Name: "GONE", Kind: KindValue, Value: "v"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	if err := backend.Delete(ctx, "GONE"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Deleting an absent name is not an error, so a retried delete does not fail.
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

// An operator may hand-write a plain name to value file rather than the full form.
func TestFileBackendReadsAHandWrittenPlainFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.json")
	if err := os.WriteFile(path, []byte(`{"SMS_KEY":"abc123"}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	backend, err := NewFileBackend(path)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	secrets, err := backend.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	got := secrets["SMS_KEY"]
	if got.Value != "abc123" || got.Kind != KindValue {
		t.Fatalf("expected a readable value, got %+v", got)
	}
	// The name is the key, so an entry that omits it still has to come back named.
	if got.Name != "SMS_KEY" {
		t.Fatalf("expected the key to name the secret, got %q", got.Name)
	}
}

// Nothing else writes this file, so the store may keep what it read for the life of the process.
func TestFileBackendReportsNoCacheTTL(t *testing.T) {
	if ttl := tempFileBackend(t).CacheTTL(); ttl != 0 {
		t.Fatalf("expected no cache TTL for a file backend, got %v", ttl)
	}
}
