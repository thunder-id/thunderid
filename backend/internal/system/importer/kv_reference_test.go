package importer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/secretresolver"
)

// The whole chain: a control plane hashes a credential into the provider, the data plane loads it,
// the import stores a reference, and nothing anywhere holds the secret.
func TestEndToEnd_SecretStaysInTheProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"secrets": map[string]interface{}{
			"MY_APP_CLIENT_SECRET": map[string]interface{}{
				"kind": "hash", "value": "the-hash", "algorithm": "PBKDF2",
				"parameters": map[string]interface{}{"salt": "s", "iterations": 1000, "keySize": 32},
			},
		}})
	}))
	defer srv.Close()

	prev := secretresolver.Default()
	r := secretresolver.New(secretresolver.Config{BaseURL: srv.URL})
	if err := r.LoadAll(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	secretresolver.SetDefault(r)
	defer secretresolver.SetDefault(prev)

	content := "resource_type: application\nid: a\nclientSecret: {{.MY_APP_CLIENT_SECRET}}\n"
	filled := fillSecretPlaceholders(context.Background(), content, nil)

	if filled["MY_APP_CLIENT_SECRET"] != "kv:MY_APP_CLIENT_SECRET" {
		t.Fatalf("the import should store a reference, got %#v", filled["MY_APP_CLIENT_SECRET"])
	}
	if filled["MY_APP_CLIENT_SECRET"] == "the-hash" {
		t.Fatal("the hash must not be written into the configuration")
	}
}
