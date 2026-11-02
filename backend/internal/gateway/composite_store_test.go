// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// newTestCompositeStore builds a composite over a real file store and the package's fake database
// store, so the merge is exercised against the same double the service tests use.
func newTestCompositeStore(t *testing.T, declared ...Gateway) (storeInterface, *fakeStore) {
	t.Helper()
	fileStore := newTestFileStore()
	declaredFor(t, fileStore, declared...)
	dbStore := &fakeStore{}
	return newCompositeStore(fileStore, dbStore), dbStore
}

// A listing shows both what a file declared and what the API registered.
func TestCompositeListsBothSources(t *testing.T) {
	ctx := context.Background()
	composite, dbStore := newTestCompositeStore(t, Gateway{
		ID: "gw-declared", Name: testGatewayName, BaseURL: "https://dp.declared.test", Key: "k",
	})
	dbStore.gateways = []Gateway{{ID: "gw-registered", Name: testProdName, BaseURL: "https://dp.prod.test"}}

	listed, err := composite.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected both sources, got %d: %+v", len(listed), listed)
	}
	if listed[0].Name != testGatewayName {
		t.Errorf("expected the declared gateway first, got %q", listed[0].Name)
	}
}

// The uniqueness checks have to see both sources, or a registration could take a name or an address
// a file already declared and the two would disagree on the next start.
func TestCompositeFindsDeclaredGatewaysInTheUniquenessChecks(t *testing.T) {
	ctx := context.Background()
	composite, _ := newTestCompositeStore(t, Gateway{
		ID: "gw-declared", Name: testGatewayName, BaseURL: "https://dp.declared.test", Key: "k",
	})

	byName, err := composite.GetByName(ctx, testGatewayName)
	if err != nil || byName == nil {
		t.Errorf("a declared name was invisible to the uniqueness check: %v %v", byName, err)
	}
	byURL, err := composite.GetByBaseURL(ctx, "https://dp.declared.test")
	if err != nil || byURL == nil {
		t.Errorf("a declared address was invisible to the uniqueness check: %v %v", byURL, err)
	}
}

// The configured bound counts what the deployment administers, however it came to administer it.
func TestCompositeCountsDeclaredGatewaysTowardsTheBound(t *testing.T) {
	ctx := context.Background()
	composite, dbStore := newTestCompositeStore(t, Gateway{
		ID: "gw-declared", Name: testGatewayName, BaseURL: "https://dp.declared.test", Key: "k",
	})
	dbStore.gateways = []Gateway{{ID: "gw-registered", Name: testProdName, BaseURL: "https://dp.prod.test"}}

	count, err := composite.Count(ctx)
	if err != nil || count != 2 {
		t.Fatalf("expected both counted, got %d %v", count, err)
	}

	// One declared and one registered, so a bound of three leaves room for one more.
	created, err := composite.Create(ctx, &Gateway{ID: "gw-new", Name: "new", BaseURL: "https://dp.new.test"}, 3)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created == nil {
		t.Fatal("expected the registration to be admitted")
	}

	// A bound of one is used up by the declared gateway alone, before the database is asked.
	refused, err := composite.Create(ctx, &Gateway{ID: "gw-nope", Name: "nope", BaseURL: "https://dp.nope.test"}, 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if refused != nil {
		t.Error("the bound did not count the declared gateway")
	}
}

// A gateway a file declared cannot be changed or removed through the API. The next start reads the
// file again, so a write that appeared to succeed would be undone.
func TestCompositeRefusesToChangeADeclaredGateway(t *testing.T) {
	ctx := context.Background()
	composite, dbStore := newTestCompositeStore(t, Gateway{
		ID: "gw-declared", Name: testGatewayName, BaseURL: "https://dp.declared.test", Key: "k",
	})

	updateErr := composite.Update(ctx, &Gateway{ID: "gw-declared", Name: "renamed"})
	if !errors.Is(updateErr, errDeclarativeGateway) {
		t.Errorf("expected the update to be refused, got %v", updateErr)
	}
	if err := composite.Delete(ctx, "gw-declared"); !errors.Is(err, errDeclarativeGateway) {
		t.Errorf("expected the delete to be refused, got %v", err)
	}
	if len(dbStore.deleted) != 0 {
		t.Error("a declared gateway reached the database store")
	}
}

// A registered gateway is still writable: the refusal is about where it came from, not about the
// composite store being read-only.
func TestCompositeStillWritesRegisteredGateways(t *testing.T) {
	ctx := context.Background()
	composite, dbStore := newTestCompositeStore(t, Gateway{
		ID: "gw-declared", Name: testGatewayName, BaseURL: "https://dp.declared.test", Key: "k",
	})
	dbStore.gateways = []Gateway{{ID: "gw-registered", Name: testProdName, BaseURL: "https://dp.prod.test"}}

	if err := composite.Update(ctx, &Gateway{ID: "gw-registered", Name: "renamed"}); err != nil {
		t.Errorf("expected a registered gateway to be updatable, got %v", err)
	}
	if err := composite.Delete(ctx, "gw-registered"); err != nil {
		t.Errorf("expected a registered gateway to be deletable, got %v", err)
	}
}

// The refusal reaches a caller as a client error naming the cause, not as an internal error. An
// operator who edited a declared gateway through the API needs to be told why it did not take.
func TestADeclaredGatewayIsRefusedAsAClientError(t *testing.T) {
	ctx := context.Background()
	fileStore := newTestFileStore()
	declaredFor(t, fileStore, Gateway{
		ID: "gw-declared", Name: testGatewayName, BaseURL: "https://dp.declared.test", Key: "k",
	})
	svc := newService(newCompositeStore(fileStore, &fakeStore{}))
	newName := "renamed"

	_, updateErr := svc.Update(ctx, "gw-declared", UpdateRequest{Name: &newName})
	if updateErr == nil || updateErr.Code != ErrorGatewayIsDeclarative.Code {
		t.Errorf("expected %s on update, got %v", ErrorGatewayIsDeclarative.Code, updateErr)
	}
	if updateErr != nil && updateErr.Type != tidcommon.ClientErrorType {
		t.Errorf("expected a client error, got %q", updateErr.Type)
	}

	deleteErr := svc.Delete(ctx, "gw-declared")
	if deleteErr == nil || deleteErr.Code != ErrorGatewayIsDeclarative.Code {
		t.Errorf("expected %s on delete, got %v", ErrorGatewayIsDeclarative.Code, deleteErr)
	}
}
