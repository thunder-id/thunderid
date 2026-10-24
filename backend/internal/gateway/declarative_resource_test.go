// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const declaredYAML = `
name: dev
baseUrl: https://dp.example.test
key: the-declared-key
`

// testGatewayName is the name the declarations in this file use.
const testGatewayName = "dev"

const testProdName = "prod"

func TestParseDeclaredGateway(t *testing.T) {
	parsed, err := parseDeclaredGateway([]byte(declaredYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	declared, ok := parsed.(*declaredGateway)
	if !ok {
		t.Fatalf("unexpected type %T", parsed)
	}
	if declared.Name != testGatewayName || declared.BaseURL != "https://dp.example.test" {
		t.Fatalf("unexpected gateway: %+v", declared)
	}
}

// A declaration that could not be applied to is refused as it is read, rather than registering
// something that looks configured and never works.
func TestValidateDeclaredGatewayRefusesAnIncompleteDeclaration(t *testing.T) {
	for name, declared := range map[string]*declaredGateway{
		"no name":     {BaseURL: "https://dp", Key: "k"},
		"no base url": {Name: "dev", Key: "k"},
		// A declared gateway holds its key in memory only, so one the file does not name is one
		// this control plane could never authenticate to.
		"no key":                     {Name: "dev", BaseURL: "https://dp"},
		"base url is http":           {Name: "dev", BaseURL: "http://dp.example.com:8090", Key: "k"},
		"base url is a scheme":       {Name: "dev", BaseURL: "https://", Key: "k"},
		"base url carries a user":    {Name: "dev", BaseURL: "https://user:secret@dp.example.com", Key: "k"},
		"name holds a slash":         {Name: "dev/prod", BaseURL: "https://dp", Key: "k"},
		"name is punctuation alone":  {Name: "- -", BaseURL: "https://dp", Key: "k"},
		"name is longer than a name": {Name: strings.Repeat("d", 65), BaseURL: "https://dp", Key: "k"},
	} {
		if err := validateDeclaredGateway(declared); err == nil {
			t.Errorf("%s: expected the declaration to be refused", name)
		}
	}

	err := validateDeclaredGateway(&declaredGateway{Name: "dev", BaseURL: "https://user:hunter2@dp.example.com",
		Key: "k"})
	if err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Errorf("the refusal shows the password the base URL carries: %v", err)
	}

	err = validateDeclaredGateway(&declaredGateway{Name: "dev", BaseURL: "http://dp.example.com:8090", Key: "k"})
	if err == nil || !strings.Contains(err.Error(), `but "http://dp.example.com:8090" is not https`) {
		t.Errorf("the refusal does not say what is wrong with the base URL: %v", err)
	}
}

// A declared gateway on a private address is refused unless the deployment allows such addresses.
func TestValidateDeclaredGatewayFollowsThePrivateAddressSetting(t *testing.T) {
	refusePrivateAddresses(t)
	declared := &declaredGateway{Name: "dev", BaseURL: "https://localhost:8090", Key: "k"}
	err := validateDeclaredGateway(declared)
	if err == nil || !strings.Contains(err.Error(), "gateway.allow_private_addresses") {
		t.Fatalf("expected the private address to be refused naming the setting, got %v", err)
	}

	configureGateway(5, true)
	if err := validateDeclaredGateway(declared); err != nil {
		t.Fatalf("an allowed private address was refused: %v", err)
	}
}

func TestValidateDeclaredGatewayAcceptsACompleteDeclaration(t *testing.T) {
	declared := &declaredGateway{Name: "dev", BaseURL: "https://dp", Key: "the-declared-key"}
	if err := validateDeclaredGateway(declared); err != nil {
		t.Fatalf("expected the declaration to be accepted, got %v", err)
	}
}

func TestDeclaredGatewayStoreLoadsWhatItReads(t *testing.T) {
	fileStore := newTestFileStore()
	declaredStore := &declaredGatewayStore{store: fileStore, logger: log.GetLogger()}

	err := declaredStore.Create("dev", &declaredGateway{
		Name:    testGatewayName,
		BaseURL: "https://dp.declared.test",
		Key:     "the-declared-key",
	})

	if err != nil {
		t.Fatalf("loading a declared gateway: %v", err)
	}
	loaded, err := fileStore.List(context.Background())
	if err != nil {
		t.Fatalf("listing declared gateways: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected one gateway, got %d", len(loaded))
	}
	if loaded[0].BaseURL != "https://dp.declared.test" || loaded[0].Key != "the-declared-key" {
		t.Fatalf("the declaration did not reach the store: %+v", loaded[0])
	}
}

// A declared gateway's id is derived from its name rather than generated, so it is the same on
// every start. Nothing about a declared gateway is written down between starts, so a generated id
// would change the address of every route naming it.
func TestADeclaredGatewayKeepsItsIDAcrossLoads(t *testing.T) {
	first := declaredGatewayID(testGatewayName)
	second := declaredGatewayID(testGatewayName)

	if first != second {
		t.Errorf("the same name produced two ids: %s and %s", first, second)
	}
	if first == declaredGatewayID(testProdName) {
		t.Error("two names produced the same id")
	}
	if len(first) != 36 {
		t.Errorf("expected a uuid, got %q", first)
	}
}

// Anything that is not a declaration is a programming error in the loader, not a file problem.
func TestDeclaredGatewayStoreRefusesTheWrongType(t *testing.T) {
	declaredStore := &declaredGatewayStore{store: newTestFileStore(), logger: log.GetLogger()}

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
