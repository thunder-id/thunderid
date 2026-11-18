// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cache"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// countingStore records how often it was read, so a test can tell a cache hit from a miss.
type countingStore struct {
	storeInterface
	variable  *Variable
	secret    *Secret
	reads     int
	writeErr  error
	deleteErr error
}

func (c *countingStore) GetVariable(context.Context, string) (*Variable, error) {
	c.reads++
	return c.variable, nil
}

func (c *countingStore) GetSecret(context.Context, string) (*Secret, error) {
	c.reads++
	return c.secret, nil
}

func (c *countingStore) UpsertVariable(_ context.Context, v Variable) (bool, error) {
	if c.writeErr != nil {
		return false, c.writeErr
	}
	c.variable = &v
	return false, nil
}

func (c *countingStore) DeleteVariable(context.Context, string) (bool, error) {
	if c.deleteErr != nil {
		return false, c.deleteErr
	}
	deleted := c.variable != nil
	c.variable = nil
	return deleted, nil
}

func (c *countingStore) DeleteSecret(context.Context, string) (bool, error) {
	if c.deleteErr != nil {
		return false, c.deleteErr
	}
	deleted := c.secret != nil
	c.secret = nil
	return deleted, nil
}

func (c *countingStore) UpsertSecret(_ context.Context, name, _, description string) (bool, error) {
	if c.writeErr != nil {
		return false, c.writeErr
	}
	c.secret = &Secret{Name: name, Exists: true, Description: description}
	return false, nil
}

// heldValue is what the database holds for the name these tests read.
const heldValue = "db.internal"

func newTestCaches() (cache.CacheInterface[*Variable], cache.CacheInterface[*Secret]) {
	manager := cache.Initialize(engineconfig.CacheConfig{Size: 100, TTL: 300}, "test-deployment")
	return cache.GetCache[*Variable](manager, "TestVariableCache"),
		cache.GetCache[*Secret](manager, "TestSecretCache")
}

// A second read of the same name is served without going to the database. This is the whole point:
// a value resolved on every authentication must not be a query each time.
func TestASecondReadIsServedFromTheCache(t *testing.T) {
	inner := &countingStore{variable: &Variable{Name: "DB_HOST", Value: heldValue}}
	vars, secrets := newTestCaches()
	store := newCacheBackedStore(inner, vars, secrets)
	ctx := context.Background()

	first, err := store.GetVariable(ctx, "DB_HOST")
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	second, err := store.GetVariable(ctx, "DB_HOST")
	if err != nil {
		t.Fatalf("second read: %v", err)
	}

	if first.Value != heldValue || second.Value != heldValue {
		t.Fatalf("read the wrong value: %q then %q", first.Value, second.Value)
	}
	if inner.reads != 1 {
		t.Fatalf("the database was read %d times for two reads of one name", inner.reads)
	}
}

// A write drops the cached entry, so the next read sees what was written rather than what it
// replaced. This is what makes rotating a value take effect at once.
func TestAWriteMakesTheNextReadSeeIt(t *testing.T) {
	inner := &countingStore{variable: &Variable{Name: "DB_HOST", Value: "old.internal"}}
	vars, secrets := newTestCaches()
	store := newCacheBackedStore(inner, vars, secrets)
	ctx := context.Background()

	if _, err := store.GetVariable(ctx, "DB_HOST"); err != nil {
		t.Fatalf("priming read: %v", err)
	}
	if _, err := store.UpsertVariable(ctx, Variable{Name: "DB_HOST", Value: "new.internal"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	after, err := store.GetVariable(ctx, "DB_HOST")
	if err != nil {
		t.Fatalf("read after write: %v", err)
	}
	if after.Value != "new.internal" {
		t.Fatalf("a read after a write answered with %q", after.Value)
	}
}

// Deleting drops the entry too, so a removed name does not keep resolving. What the store reports about
// the delete passes through, because the service records a deletion only when it removed something.
func TestADeleteMakesTheNextReadMiss(t *testing.T) {
	kinds := []struct {
		name    string
		inner   *countingStore
		resolve func(store storeInterface) (held bool, err error)
		remove  func(store storeInterface) (deleted bool, err error)
	}{
		{
			name:  "variable",
			inner: &countingStore{variable: &Variable{Name: "DB_HOST", Value: heldValue}},
			resolve: func(store storeInterface) (bool, error) {
				variable, err := store.GetVariable(context.Background(), "DB_HOST")
				return variable != nil, err
			},
			remove: func(store storeInterface) (bool, error) {
				return store.DeleteVariable(context.Background(), "DB_HOST")
			},
		},
		{
			name:  "secret",
			inner: &countingStore{secret: &Secret{Name: "API_KEY", Exists: true}},
			resolve: func(store storeInterface) (bool, error) {
				secret, err := store.GetSecret(context.Background(), "API_KEY")
				return secret != nil, err
			},
			remove: func(store storeInterface) (bool, error) {
				return store.DeleteSecret(context.Background(), "API_KEY")
			},
		},
	}
	for _, kind := range kinds {
		t.Run(kind.name, func(t *testing.T) {
			vars, secrets := newTestCaches()
			store := newCacheBackedStore(kind.inner, vars, secrets)

			if held, err := kind.resolve(store); err != nil || !held {
				t.Fatalf("priming read: held %v, err %v", held, err)
			}
			deleted, err := kind.remove(store)
			if err != nil {
				t.Fatalf("delete: %v", err)
			}
			if !deleted {
				t.Fatal("the delete of a held entry was reported as removing nothing")
			}
			if held, err := kind.resolve(store); err != nil || held {
				t.Fatalf("a deleted name still resolves: held %v, err %v", held, err)
			}

			deleted, err = kind.remove(store)
			if err != nil {
				t.Fatalf("second delete: %v", err)
			}
			if deleted {
				t.Fatal("the delete of an absent entry was reported as removing something")
			}
		})
	}
}

// A write that failed leaves the cache alone: the database still holds what it held, so dropping
// the entry would only cost a read, but answering from a refreshed one would be wrong.
func TestAFailedWriteDoesNotChangeWhatIsRead(t *testing.T) {
	inner := &countingStore{
		variable: &Variable{Name: "DB_HOST", Value: heldValue},
		writeErr: errors.New("the database is unreachable"),
	}
	vars, secrets := newTestCaches()
	store := newCacheBackedStore(inner, vars, secrets)
	ctx := context.Background()

	if _, err := store.GetVariable(ctx, "DB_HOST"); err != nil {
		t.Fatalf("priming read: %v", err)
	}
	if _, err := store.UpsertVariable(ctx, Variable{Name: "DB_HOST", Value: "never.stored"}); err == nil {
		t.Fatal("the write was reported as succeeding")
	}

	after, err := store.GetVariable(ctx, "DB_HOST")
	if err != nil {
		t.Fatalf("read after a failed write: %v", err)
	}
	if after.Value != heldValue {
		t.Fatalf("a failed write changed what is read: %q", after.Value)
	}
}

// Secrets are cached by the same rule, and a write to one is seen by the next read.
func TestASecretWriteMakesTheNextReadSeeIt(t *testing.T) {
	inner := &countingStore{secret: &Secret{Name: "API_KEY", Exists: true, Description: "old"}}
	vars, secrets := newTestCaches()
	store := newCacheBackedStore(inner, vars, secrets)
	ctx := context.Background()

	if _, err := store.GetSecret(ctx, "API_KEY"); err != nil {
		t.Fatalf("priming read: %v", err)
	}
	if _, err := store.UpsertSecret(ctx, "API_KEY", "sealed", "new"); err != nil {
		t.Fatalf("write: %v", err)
	}

	after, err := store.GetSecret(ctx, "API_KEY")
	if err != nil {
		t.Fatalf("read after write: %v", err)
	}
	if after.Description != "new" {
		t.Fatalf("a read after a write answered with %q", after.Description)
	}
}

// What is cached for a secret carries no value, because the store never reads one out. A cache of
// decrypted credentials is the thing this must never become.
func TestNothingCachedForASecretHoldsItsValue(t *testing.T) {
	inner := &countingStore{secret: &Secret{Name: "API_KEY", Exists: true}}
	vars, secrets := newTestCaches()
	store := newCacheBackedStore(inner, vars, secrets)
	ctx := context.Background()

	if _, err := store.GetSecret(ctx, "API_KEY"); err != nil {
		t.Fatalf("read: %v", err)
	}

	held, found := secrets.Get(ctx, cache.CacheKey{Key: "API_KEY"})
	if !found {
		t.Fatal("the secret was not cached")
	}
	if held.Name != "API_KEY" || !held.Exists {
		t.Fatalf("the cached secret is wrong: %+v", held)
	}
}
