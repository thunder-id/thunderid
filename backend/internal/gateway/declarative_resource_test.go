// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const declaredYAML = `
name: dev
baseUrl: https://dp.example.test
dataPlaneId: production-dp
key: shhh
`

// testGatewayName is the name the declarations in this file use.
const testGatewayName = "dev"

func TestParseDeclaredGateway(t *testing.T) {
	parsed, err := parseDeclaredGateway([]byte(declaredYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	declared, ok := parsed.(*declaredGateway)
	if !ok {
		t.Fatalf("unexpected type %T", parsed)
	}
	if declared.Name != testGatewayName || declared.BaseURL != "https://dp.example.test" ||
		declared.DataPlaneID != "production-dp" || declared.Key != testSecret {
		t.Fatalf("unexpected gateway: %+v", declared)
	}
}

// A declaration that could not be applied to is refused as it is read, rather than registering
// something that looks configured and never works.
func TestValidateDeclaredGatewayRefusesAnIncompleteDeclaration(t *testing.T) {
	for name, declared := range map[string]*declaredGateway{
		"no name":          {BaseURL: "https://dp", DataPlaneID: "a", Key: "b"},
		"no base url":      {Name: "dev", DataPlaneID: "a", Key: "b"},
		"no data plane id": {Name: "dev", BaseURL: "https://dp", Key: "b"},
		"no key":           {Name: "dev", BaseURL: "https://dp", DataPlaneID: "a"},
	} {
		if err := validateDeclaredGateway(declared); err == nil {
			t.Errorf("%s: expected the declaration to be refused", name)
		}
	}
}

func TestValidateDeclaredGatewayAcceptsACompleteDeclaration(t *testing.T) {
	declared := &declaredGateway{Name: "dev", BaseURL: "https://dp", DataPlaneID: "a", Key: "b"}
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
	svc := newService(store)
	req := RegisterRequest{Name: "dev", BaseURL: "https://dp.example.test",
		DataPlaneID: "system-app", Key: testSecret}

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
	svc := newService(store)
	base := RegisterRequest{Name: "dev", BaseURL: "https://old.example.test",
		DataPlaneID: "old-app", Key: testSecret}
	if svcErr := svc.Adopt(context.Background(), base); svcErr != nil {
		t.Fatalf("first adopt: %v", svcErr)
	}

	moved := base
	moved.BaseURL = "https://new.example.test"
	moved.DataPlaneID = "new-app"
	if svcErr := svc.Adopt(context.Background(), moved); svcErr != nil {
		t.Fatalf("second adopt: %v", svcErr)
	}

	if len(store.gateways) != 1 {
		t.Fatalf("expected one gateway, got %d", len(store.gateways))
	}
	if store.gateways[0].BaseURL != "https://new.example.test" ||
		store.gateways[0].DataPlaneID != "new-app" {
		t.Fatalf("expected the gateway to be updated, got %+v", store.gateways[0])
	}
}

// A second, differently named declaration is still bounded by what the deployment allows.
func TestAdoptRespectsTheConfiguredBound(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	store := &fakeStore{}
	svc := newService(store)
	first := RegisterRequest{Name: testGatewayName, BaseURL: "https://dp", DataPlaneID: "a", Key: testSecret}
	if svcErr := svc.Adopt(context.Background(), first); svcErr != nil {
		t.Fatalf("first adopt: %v", svcErr)
	}

	// A different data plane as well as a different name, so this tests the bound rather than the
	// rule that one data plane registers once.
	second := first
	second.Name = "prod"
	second.DataPlaneID = "b"
	svcErr := svc.Adopt(context.Background(), second)
	if svcErr == nil || svcErr.Code != ErrorGatewayLimitReached.Code {
		t.Fatalf("expected the bound to be enforced, got %v", svcErr)
	}
}

// Two declarations naming the same data plane are refused for that reason, not as a bound.
func TestAdoptRefusesASecondDeclarationOfTheSameDataPlane(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	store := &fakeStore{}
	svc := newService(store)
	first := RegisterRequest{Name: testGatewayName, BaseURL: "https://dp", DataPlaneID: "a", Key: testSecret}
	if svcErr := svc.Adopt(context.Background(), first); svcErr != nil {
		t.Fatalf("first adopt: %v", svcErr)
	}

	second := first
	second.Name = "prod"
	svcErr := svc.Adopt(context.Background(), second)
	if svcErr == nil || svcErr.Code != ErrorDataPlaneAlreadyRegistered.Code {
		t.Fatalf("expected the data plane conflict, got %v", svcErr)
	}
}

// The declared secret is encrypted before it is stored, exactly as a registration through the API is.
func TestAdoptStoresTheDeclaredCredentialEncrypted(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	store := &fakeStore{}
	svc := newService(store)
	req := RegisterRequest{Name: testGatewayName, BaseURL: "https://dp", DataPlaneID: "a", Key: testSecret}
	if svcErr := svc.Adopt(context.Background(), req); svcErr != nil {
		t.Fatalf("adopt: %v", svcErr)
	}
	if store.gateways[0].Key == testSecret {
		t.Fatal("the declared secret was stored in the clear")
	}
}

// Re-reading a declaration must not blank what it does not change. The lookup used to find the
// existing gateway reads the whole row for this reason: a partial one carried empty values into the
// write and erased the registered name on the next start.
func TestAdoptKeepsTheFieldsItDoesNotChange(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	store := &fakeStore{}
	svc := newService(store)
	declared := RegisterRequest{Name: testGatewayName, BaseURL: "https://dp", DataPlaneID: "a", Key: testSecret}
	if svcErr := svc.Adopt(context.Background(), declared); svcErr != nil {
		t.Fatalf("first adopt: %v", svcErr)
	}

	// The same file read again, as happens on every start.
	if svcErr := svc.Adopt(context.Background(), declared); svcErr != nil {
		t.Fatalf("second adopt: %v", svcErr)
	}

	if len(store.gateways) != 1 {
		t.Fatalf("expected one gateway, got %d", len(store.gateways))
	}
	if store.gateways[0].Name != testGatewayName {
		t.Fatalf("the name was lost on re-read: %q", store.gateways[0].Name)
	}
	if store.gateways[0].DataPlaneID != "a" {
		t.Fatalf("the data plane was lost on re-read: %q", store.gateways[0].DataPlaneID)
	}
}

// The loader hands what it read to the service, so a declared gateway is registered the same way an
// API call registers one.
func TestDeclaredGatewayStoreRegistersWhatItReads(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })
	allowSeveralGateways(t)

	store := &fakeStore{}
	declaredStore := &declaredGatewayStore{
		service: newService(store),
		logger:  log.GetLogger(),
	}

	err := declaredStore.Create("dev", &declaredGateway{
		Name:        testGatewayName,
		DataPlaneID: "dp-declared",
		BaseURL:     "https://dp.declared.test",
		Key:         testSecret,
	})

	if err != nil {
		t.Fatalf("registering a declared gateway: %v", err)
	}
	if len(store.gateways) != 1 {
		t.Fatalf("expected one gateway, got %d", len(store.gateways))
	}
	if store.gateways[0].DataPlaneID != "dp-declared" {
		t.Fatalf("the declaration did not reach the service: %+v", store.gateways[0])
	}
}

// A declaration the service refuses is reported rather than swallowed, so a server does not come up
// believing it registered something it did not.
func TestDeclaredGatewayStoreReportsARefusal(t *testing.T) {
	cmodels.SetConfigCryptoProvider(reversingCrypto{})
	t.Cleanup(func() { cmodels.SetConfigCryptoProvider(nil) })

	declaredStore := &declaredGatewayStore{
		service: newService(&fakeStore{}),
		logger:  log.GetLogger(),
	}

	err := declaredStore.Create("dev", &declaredGateway{
		Name:        testGatewayName,
		DataPlaneID: "dp-declared",
		BaseURL:     "not-a-url",
		Key:         testSecret,
	})

	if err == nil {
		t.Fatal("expected an unusable declaration to be reported")
	}
}

// Anything that is not a declaration is a programming error in the loader, not a file problem.
func TestDeclaredGatewayStoreRefusesTheWrongType(t *testing.T) {
	declaredStore := &declaredGatewayStore{service: newService(&fakeStore{}), logger: log.GetLogger()}

	if err := declaredStore.Create("dev", "not a declaration"); err == nil {
		t.Fatal("expected a type mismatch to be reported")
	}
}

// A declaration that is not readable YAML is refused as it is parsed.
func TestParseDeclaredGatewayRefusesUnreadableYAML(t *testing.T) {
	if _, err := parseDeclaredGateway([]byte("name: [unclosed")); err == nil {
		t.Fatal("expected unreadable YAML to be refused")
	}
}

// The validator refuses anything that is not a declaration, which would otherwise reach the service
// as a nil dereference.
func TestValidateDeclaredGatewayRefusesTheWrongType(t *testing.T) {
	if err := validateDeclaredGateway("not a declaration"); err == nil {
		t.Fatal("expected a type mismatch to be refused")
	}
}
