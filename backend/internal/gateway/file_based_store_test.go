// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"testing"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// newTestFileStore builds a file store over its own storage, so one test's declarations are not
// visible to the next.
func newTestFileStore() *gatewayFileStore {
	return &gatewayFileStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeGateway),
	}
}

func declaredFor(t *testing.T, store *gatewayFileStore, gateways ...Gateway) {
	t.Helper()
	for _, gw := range gateways {
		if err := store.put(gw); err != nil {
			t.Fatalf("loading %q: %v", gw.Name, err)
		}
	}
}

func TestFileStoreReadsBackWhatWasDeclared(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore()
	declaredFor(t, store, Gateway{
		ID: "gw-dev", Name: testGatewayName, BaseURL: "https://dp.dev.test:8090", Key: "k",
	})

	byID, err := store.GetByID(ctx, "gw-dev")
	if err != nil || byID == nil {
		t.Fatalf("GetByID: %v %v", byID, err)
	}
	byName, err := store.GetByName(ctx, testGatewayName)
	if err != nil || byName == nil {
		t.Fatalf("GetByName: %v %v", byName, err)
	}
	byURL, err := store.GetByBaseURL(ctx, "https://dp.dev.test:8090")
	if err != nil || byURL == nil {
		t.Fatalf("GetByBaseURL: %v %v", byURL, err)
	}
	count, err := store.Count(ctx)
	if err != nil || count != 1 {
		t.Fatalf("Count: %d %v", count, err)
	}
}

// The address is matched the way the database store matches it, so a declared gateway and a
// registered one cannot both claim the same data plane under different spellings of its address.
func TestFileStoreMatchesAnAddressTheWayTheDatabaseDoes(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore()
	declaredFor(t, store, Gateway{
		ID: "gw-dev", Name: testGatewayName, BaseURL: "https://dp.dev.test:8090/", Key: "k",
	})

	found, err := store.GetByBaseURL(ctx, "https://dp.dev.test:8090")
	if err != nil {
		t.Fatalf("GetByBaseURL: %v", err)
	}
	if found == nil {
		t.Error("a trailing slash made the same address look like a different one")
	}
}

// Absent is not an error. The composite store asks both sources, and a miss here is the ordinary
// answer for anything registered through the API.
func TestFileStoreReportsAnAbsentGatewayAsAbsent(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore()

	for name, read := range map[string]func() (*Gateway, error){
		"by id":   func() (*Gateway, error) { return store.GetByID(ctx, "nothing") },
		"by name": func() (*Gateway, error) { return store.GetByName(ctx, "nothing") },
		"by url":  func() (*Gateway, error) { return store.GetByBaseURL(ctx, "https://nothing.test") },
	} {
		found, err := read()
		if err != nil {
			t.Errorf("%s: expected absence to be no error, got %v", name, err)
		}
		if found != nil {
			t.Errorf("%s: expected nothing, got %+v", name, found)
		}
	}
}

// A declarative store is written by its files and by nothing else. A write that appeared to succeed
// would be undone by the next start, which reads the file again.
func TestFileStoreRefusesEveryWrite(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore()
	declaredFor(t, store, Gateway{ID: "gw-dev", Name: testGatewayName, BaseURL: "https://dp.dev.test", Key: "k"})

	if _, err := store.Create(ctx, &Gateway{Name: "new"}, 5); err == nil {
		t.Error("expected Create to be refused")
	}
	if err := store.Update(ctx, &Gateway{ID: "gw-dev", Name: "renamed"}); err == nil {
		t.Error("expected Update to be refused")
	}
	if err := store.Delete(ctx, "gw-dev"); err == nil {
		t.Error("expected Delete to be refused")
	}
}

// A read returns a copy, so a caller that changes what it was given does not change what the next
// read sees.
func TestFileStoreHandsOutCopies(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore()
	declaredFor(t, store, Gateway{ID: "gw-dev", Name: testGatewayName, BaseURL: "https://dp.dev.test", Key: "k"})

	first, err := store.GetByID(ctx, "gw-dev")
	if err != nil || first == nil {
		t.Fatalf("GetByID: %v %v", first, err)
	}
	first.Name = "mutated"

	second, err := store.GetByID(ctx, "gw-dev")
	if err != nil || second == nil {
		t.Fatalf("GetByID: %v %v", second, err)
	}
	if second.Name != testGatewayName {
		t.Errorf("a caller's change reached the store: %q", second.Name)
	}
}
