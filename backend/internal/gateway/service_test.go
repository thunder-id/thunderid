// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	kmcommon "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
)

// reversingCrypto stands in for the deployment's configuration key. It only has to be reversible
// and to leave the ciphertext different from the plaintext.
type reversingCrypto struct{}

func (reversingCrypto) Encrypt(_ context.Context, content []byte) ([]byte, error) {
	return append([]byte("enc:"), content...), nil
}

func (reversingCrypto) Decrypt(_ context.Context, content []byte) ([]byte, error) {
	return content[len("enc:"):], nil
}

var _ kmcommon.ConfigCryptoProvider = reversingCrypto{}

type fakeStore struct {
	gateways []Gateway
	err      error
}

func (f *fakeStore) List(context.Context) ([]Gateway, error) { return f.gateways, f.err }

func (f *fakeStore) GetByID(_ context.Context, id string) (*Gateway, error) {
	for i := range f.gateways {
		if f.gateways[i].ID == id {
			return &f.gateways[i], nil
		}
	}
	return nil, f.err
}

func (f *fakeStore) GetByName(_ context.Context, name string) (*Gateway, error) {
	for i := range f.gateways {
		if f.gateways[i].Name == name {
			return &f.gateways[i], nil
		}
	}
	return nil, f.err
}

func (f *fakeStore) Count(context.Context) (int, error) { return len(f.gateways), f.err }

func (f *fakeStore) Create(_ context.Context, gw *Gateway) error {
	if f.err != nil {
		return f.err
	}
	f.gateways = append(f.gateways, *gw)
	return nil
}

func (f *fakeStore) Update(_ context.Context, gw *Gateway) error {
	if f.err != nil {
		return f.err
	}
	for i := range f.gateways {
		if f.gateways[i].ID == gw.ID {
			f.gateways[i] = *gw
			return nil
		}
	}
	return nil
}

func (f *fakeStore) Delete(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	for i := range f.gateways {
		if f.gateways[i].ID == id {
			f.gateways = append(f.gateways[:i], f.gateways[i+1:]...)
			return nil
		}
	}
	return nil
}

func validRequest(name string) RegisterRequest {
	return RegisterRequest{
		Name:         name,
		BaseURL:      "https://dp.example.test",
		ClientID:     "system-app",
		ClientSecret: "shhh",
	}
}

func newTestService(t *testing.T, store storeInterface) ServiceInterface {
	t.Helper()
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })
	return newService(store, nil, nil)
}

func TestRegisterRecordsTheGateway(t *testing.T) {
	store := &fakeStore{}
	gw, svcErr := newTestService(t, store).Register(context.Background(), validRequest("dev"))
	if svcErr != nil {
		t.Fatalf("register: %v", svcErr)
	}
	if gw.ID == "" || gw.Name != "dev" {
		t.Fatalf("unexpected gateway: %+v", gw)
	}
	if len(store.gateways) != 1 {
		t.Fatalf("expected the gateway to be stored, got %d", len(store.gateways))
	}
}

// A deployment pairs with one data plane unless it is configured otherwise, so the second
// registration is refused rather than quietly accepted.
func TestRegisterRefusesMoreGatewaysThanConfigured(t *testing.T) {
	store := &fakeStore{}
	svc := newTestService(t, store)

	if _, svcErr := svc.Register(context.Background(), validRequest("dev")); svcErr != nil {
		t.Fatalf("first register: %v", svcErr)
	}
	_, svcErr := svc.Register(context.Background(), validRequest("prod"))
	if svcErr == nil || svcErr.Code != ErrorGatewayLimitReached.Code {
		t.Fatalf("expected the limit to be enforced, got %v", svcErr)
	}
	if len(store.gateways) != 1 {
		t.Fatalf("expected the second registration not to be stored, got %d", len(store.gateways))
	}
}

// The credential must not be readable in the database, so what is stored is not the plaintext.
func TestRegisterStoresTheCredentialEncrypted(t *testing.T) {
	store := &fakeStore{}
	if _, svcErr := newTestService(t, store).Register(context.Background(), validRequest("dev")); svcErr != nil {
		t.Fatalf("register: %v", svcErr)
	}

	stored := store.gateways[0].ClientSecret
	if stored == "" {
		t.Fatal("expected a stored credential")
	}
	if stored == "shhh" {
		t.Fatal("the client secret was stored in the clear")
	}

	// It must still be recoverable, or an apply could never authenticate.
	props, err := cmodels.DeserializePropertiesFromJSON(stored)
	if err != nil || len(props) != 1 {
		t.Fatalf("stored credential is unreadable: %v", err)
	}
	value, err := props[0].GetValue()
	if err != nil || value != "shhh" {
		t.Fatalf("expected the credential to decrypt, got %q err=%v", value, err)
	}
}

// A credential must never travel back out of the API.
func TestTheAPINeverReturnsTheCredential(t *testing.T) {
	store := &fakeStore{}
	svc := newTestService(t, store)
	created, _ := svc.Register(context.Background(), validRequest("dev"))
	if created.ClientSecret != "" {
		t.Error("register returned the credential")
	}

	listed, _ := svc.List(context.Background())
	if len(listed) != 1 || listed[0].ClientSecret != "" {
		t.Error("list returned the credential")
	}

	got, _ := svc.Get(context.Background(), created.ID)
	if got.ClientSecret != "" {
		t.Error("get returned the credential")
	}
}

func TestRegisterRefusesADuplicateName(t *testing.T) {
	store := &fakeStore{gateways: []Gateway{{ID: "a", Name: "dev"}}}
	_, svcErr := newTestService(t, store).Register(context.Background(), validRequest("dev"))
	if svcErr == nil {
		t.Fatal("expected a refusal")
	}
}

// Without all three the gateway cannot be reached, and a registration that cannot be applied to is
// worse than none because it looks configured.
func TestRegisterRefusesAnUnreachableGateway(t *testing.T) {
	svc := newTestService(t, &fakeStore{})
	for _, req := range []RegisterRequest{
		{Name: "dev", ClientID: "a", ClientSecret: "b"},
		{Name: "dev", BaseURL: "https://dp", ClientSecret: "b"},
		{Name: "dev", BaseURL: "https://dp", ClientID: "a"},
	} {
		if _, svcErr := svc.Register(context.Background(), req); svcErr == nil ||
			svcErr.Code != ErrorGatewayConnectionRequired.Code {
			t.Fatalf("expected a connection refusal for %+v, got %v", req, svcErr)
		}
	}
}

func TestRegisterRequiresAName(t *testing.T) {
	req := validRequest("   ")
	if _, svcErr := newTestService(t, &fakeStore{}).Register(context.Background(), req); svcErr == nil ||
		svcErr.Code != ErrorGatewayNameRequired.Code {
		t.Fatalf("expected a name refusal, got %v", svcErr)
	}
}

func TestGetReportsAnUnknownGateway(t *testing.T) {
	_, svcErr := newTestService(t, &fakeStore{}).Get(context.Background(), "missing")
	if svcErr == nil || svcErr.Code != ErrorGatewayNotFound.Code {
		t.Fatalf("expected not found, got %v", svcErr)
	}
}

func TestDeleteRemovesTheGateway(t *testing.T) {
	store := &fakeStore{gateways: []Gateway{{ID: "a", Name: "dev"}}}
	if svcErr := newTestService(t, store).Delete(context.Background(), "a"); svcErr != nil {
		t.Fatalf("delete: %v", svcErr)
	}
	if len(store.gateways) != 0 {
		t.Fatal("expected the gateway to be removed")
	}
}

func TestListReportsAStoreFailure(t *testing.T) {
	_, svcErr := newTestService(t, &fakeStore{err: errors.New("database is down")}).List(context.Background())
	if svcErr == nil {
		t.Fatal("expected a failure to surface")
	}
}
