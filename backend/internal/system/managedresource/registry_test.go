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

package managedresource

import (
	"context"
	"errors"
	"testing"
)

// fakeStore is an in-memory registry.
type fakeStore struct {
	marked map[string]bool
	err    error
}

func newFakeStore() *fakeStore { return &fakeStore{marked: map[string]bool{}} }

func (f *fakeStore) key(t, id string) string { return t + "/" + id }

func (f *fakeStore) Mark(_ context.Context, resourceType, resourceID string) error {
	f.marked[f.key(resourceType, resourceID)] = true
	return nil
}

func (f *fakeStore) Unmark(_ context.Context, resourceType, resourceID string) error {
	delete(f.marked, f.key(resourceType, resourceID))
	return nil
}

func (f *fakeStore) IsManaged(_ context.Context, resourceType, resourceID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.marked[f.key(resourceType, resourceID)], nil
}

func (f *fakeStore) ManagedIDs(_ context.Context, resourceType string) (map[string]bool, error) {
	if f.err != nil {
		return nil, f.err
	}
	ids := map[string]bool{}
	for k := range f.marked {
		if len(k) > len(resourceType) && k[:len(resourceType)+1] == resourceType+"/" {
			ids[k[len(resourceType)+1:]] = true
		}
	}
	return ids, nil
}

func enabledRegistry(store storeInterface) *Registry {
	return &Registry{store: store, enabled: true}
}

func TestGuardRefusesAChangeToAnImportedResource(t *testing.T) {
	store := newFakeStore()
	r := enabledRegistry(store)
	SetDefault(r)
	t.Cleanup(func() { SetDefault(nil) })

	if err := r.Mark(WithImport(context.Background()), TypeApplication, "app-1"); err != nil {
		t.Fatalf("mark: %v", err)
	}

	svcErr := Guard(context.Background(), TypeApplication, "app-1")
	if svcErr == nil {
		t.Fatal("a resource written by an import must not be changeable here")
	}
	if svcErr.Code != ErrorResourceManaged.Code {
		t.Fatalf("unexpected error code %q", svcErr.Code)
	}
}

func TestGuardAllowsAResourceCreatedOnThisDeployment(t *testing.T) {
	r := enabledRegistry(newFakeStore())
	SetDefault(r)
	t.Cleanup(func() { SetDefault(nil) })

	// A user who signed up here is owned here and stays editable.
	if svcErr := Guard(context.Background(), TypeUser, "local-user"); svcErr != nil {
		t.Fatalf("a locally created resource must stay editable, got %v", svcErr.Code)
	}
}

func TestGuardAllowsTheImportItself(t *testing.T) {
	store := newFakeStore()
	r := enabledRegistry(store)
	SetDefault(r)
	t.Cleanup(func() { SetDefault(nil) })

	ctx := WithImport(context.Background())
	if err := r.Mark(ctx, TypeApplication, "app-1"); err != nil {
		t.Fatalf("mark: %v", err)
	}

	// The control plane has to be able to update what it owns, or a promotion could never run twice.
	if svcErr := Guard(ctx, TypeApplication, "app-1"); svcErr != nil {
		t.Fatalf("an import must be able to change a managed resource, got %v", svcErr.Code)
	}
}

func TestGuardIsInertWhenNotControlPlaneManaged(t *testing.T) {
	SetDefault(New(false, ""))
	t.Cleanup(func() { SetDefault(nil) })

	// A standalone server has no control plane, so nothing is owned elsewhere.
	if svcErr := Guard(context.Background(), TypeApplication, "app-1"); svcErr != nil {
		t.Fatalf("ownership tracking must stay off unless configured, got %v", svcErr.Code)
	}
}

func TestGuardRefusesWhenTheRegistryCannotBeRead(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("database is down")
	SetDefault(enabledRegistry(store))
	t.Cleanup(func() { SetDefault(nil) })

	// Refusing a write that should have been allowed is recoverable. Allowing one that should have
	// been refused is the thing this exists to prevent, so an unreadable registry refuses.
	if svcErr := Guard(context.Background(), TypeApplication, "app-1"); svcErr == nil {
		t.Fatal("an unreadable registry must refuse rather than allow")
	}
}

func TestUnmarkReleasesAResource(t *testing.T) {
	store := newFakeStore()
	r := enabledRegistry(store)
	SetDefault(r)
	t.Cleanup(func() { SetDefault(nil) })

	ctx := WithImport(context.Background())
	if err := r.Mark(ctx, TypeGroup, "group-1"); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if err := r.Unmark(ctx, TypeGroup, "group-1"); err != nil {
		t.Fatalf("unmark: %v", err)
	}

	// An id reused by a resource created here later must not inherit the old ownership.
	if svcErr := Guard(context.Background(), TypeGroup, "group-1"); svcErr != nil {
		t.Fatalf("a released resource must be editable again, got %v", svcErr.Code)
	}
}

func TestDefaultIsNeverNil(t *testing.T) {
	SetDefault(nil)
	if Default() == nil || Default().Enabled() {
		t.Fatal("the default registry must exist and be inert")
	}
}
