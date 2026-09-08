// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

const declaredYAML = `
name: dev
baseUrl: https://dp.example.test
clientId: system-app
clientSecret: shhh
scope: import
`

func TestParseDeclaredGateway(t *testing.T) {
	parsed, err := parseDeclaredGateway([]byte(declaredYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	declared, ok := parsed.(*declaredGateway)
	if !ok {
		t.Fatalf("unexpected type %T", parsed)
	}
	if declared.Name != "dev" || declared.BaseURL != "https://dp.example.test" ||
		declared.ClientID != "system-app" || declared.ClientSecret != testSecret || declared.Scope != "import" {
		t.Fatalf("unexpected gateway: %+v", declared)
	}
}

// A declaration that could not be applied to is refused as it is read, rather than registering
// something that looks configured and never works.
func TestValidateDeclaredGatewayRefusesAnIncompleteDeclaration(t *testing.T) {
	for name, declared := range map[string]*declaredGateway{
		"no name":     {BaseURL: "https://dp", ClientID: "a", ClientSecret: "b"},
		"no base url": {Name: "dev", ClientID: "a", ClientSecret: "b"},
		"no client":   {Name: "dev", BaseURL: "https://dp", ClientSecret: "b"},
		"no secret":   {Name: "dev", BaseURL: "https://dp", ClientID: "a"},
	} {
		if err := validateDeclaredGateway(declared); err == nil {
			t.Errorf("%s: expected the declaration to be refused", name)
		}
	}
}

func TestValidateDeclaredGatewayAcceptsACompleteDeclaration(t *testing.T) {
	declared := &declaredGateway{Name: "dev", BaseURL: "https://dp", ClientID: "a", ClientSecret: "b"}
	if err := validateDeclaredGateway(declared); err != nil {
		t.Fatalf("expected the declaration to be accepted, got %v", err)
	}
}

// The files are read on every start, so the same declaration must update the gateway it already
// registered rather than adding another or failing on the bound.
func TestAdoptIsIdempotentForTheSameName(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	store := &fakeStore{}
	svc := newService(store, nil, nil)
	req := RegisterRequest{Name: "dev", BaseURL: "https://dp.example.test",
		ClientID: "system-app", ClientSecret: testSecret}

	for i := 0; i < 3; i++ {
		if svcErr := svc.Adopt(context.Background(), req); svcErr != nil {
			t.Fatalf("adopt %d: %v", i, svcErr)
		}
	}
	if len(store.gateways) != 1 {
		t.Fatalf("expected one gateway after three reads, got %d", len(store.gateways))
	}
}

// An edited file changes the gateway it already described.
func TestAdoptUpdatesTheGatewayItAlreadyDescribed(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	store := &fakeStore{}
	svc := newService(store, nil, nil)
	base := RegisterRequest{Name: "dev", BaseURL: "https://old.example.test",
		ClientID: "old-app", ClientSecret: testSecret}
	if svcErr := svc.Adopt(context.Background(), base); svcErr != nil {
		t.Fatalf("first adopt: %v", svcErr)
	}

	moved := base
	moved.BaseURL = "https://new.example.test"
	moved.ClientID = "new-app"
	if svcErr := svc.Adopt(context.Background(), moved); svcErr != nil {
		t.Fatalf("second adopt: %v", svcErr)
	}

	if len(store.gateways) != 1 {
		t.Fatalf("expected one gateway, got %d", len(store.gateways))
	}
	if store.gateways[0].BaseURL != "https://new.example.test" ||
		store.gateways[0].ClientID != "new-app" {
		t.Fatalf("expected the gateway to be updated, got %+v", store.gateways[0])
	}
}

// A second, differently named declaration is still bounded by what the deployment allows.
func TestAdoptRespectsTheConfiguredBound(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	store := &fakeStore{}
	svc := newService(store, nil, nil)
	first := RegisterRequest{Name: "dev", BaseURL: "https://dp", ClientID: "a", ClientSecret: testSecret}
	if svcErr := svc.Adopt(context.Background(), first); svcErr != nil {
		t.Fatalf("first adopt: %v", svcErr)
	}

	second := first
	second.Name = "prod"
	svcErr := svc.Adopt(context.Background(), second)
	if svcErr == nil || svcErr.Code != ErrorGatewayLimitReached.Code {
		t.Fatalf("expected the bound to be enforced, got %v", svcErr)
	}
}

// The declared secret is encrypted before it is stored, exactly as a registration through the API is.
func TestAdoptStoresTheDeclaredCredentialEncrypted(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	store := &fakeStore{}
	svc := newService(store, nil, nil)
	req := RegisterRequest{Name: "dev", BaseURL: "https://dp", ClientID: "a", ClientSecret: testSecret}
	if svcErr := svc.Adopt(context.Background(), req); svcErr != nil {
		t.Fatalf("adopt: %v", svcErr)
	}
	if store.gateways[0].ClientSecret == testSecret {
		t.Fatal("the declared secret was stored in the clear")
	}
}
