// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// cacheBackedStore wraps a store with read-through caching for single-name reads.
//
// These values are read where a reference is resolved, and a credential held as a reference is
// resolved on every authentication against it. Without a cache here that is a database read per
// attempt: the resolver deliberately does not cache what it reads from a store in this process,
// on the grounds that the store is the cache.
//
// Only reads by name are cached. A listing is read by a person choosing a name, not on any hot
// path, and caching one would need invalidating whenever anything in it changed.
//
// Nothing cached here holds a secret's value. The API never returns one, so GetSecret answers with
// a name and whether it exists and nothing more, and that is all this keeps.
//
// Resolution is a different reader: it needs the value itself, and the read that gives it does not
// exist yet. When it arrives, what it caches has to be the sealed value exactly as the database
// holds it, decrypted per read and never written back here. A cache of decrypted credentials is a
// copy of every secret the deployment holds, sitting in memory and, under a shared backend, in
// another process entirely.
type cacheBackedStore struct {
	variables cache.CacheInterface[*Variable]
	secrets   cache.CacheInterface[*Secret]
	store     storeInterface
	logger    *log.Logger
}

// newCacheBackedStore wraps a store with read-through caching.
func newCacheBackedStore(store storeInterface, variables cache.CacheInterface[*Variable],
	secrets cache.CacheInterface[*Secret]) storeInterface {
	return &cacheBackedStore{
		variables: variables,
		secrets:   secrets,
		store:     store,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedVariableStore")),
	}
}

// Every write invalidates rather than refreshes the entry it touched.
//
// Invalidating is what makes a rotation take effect at once: the next read goes to the database and
// sees the new value. An expiry would leave a replaced credential authenticating until it lapsed,
// which is the failure this exists to avoid, and writing the new value into the cache instead would
// trust that what was written is what the database now holds.

func (s *cacheBackedStore) GetVariable(ctx context.Context, name string) (*Variable, error) {
	if held, found := s.variables.Get(ctx, cache.CacheKey{Key: name}); found {
		return held, nil
	}
	variable, err := s.store.GetVariable(ctx, name)
	if err != nil || variable == nil {
		return variable, err
	}
	if err := s.variables.Set(ctx, cache.CacheKey{Key: name}, variable); err != nil {
		// A value that could not be cached is still a value. The read succeeded, so the caller is
		// given it, and the next read tries the cache again.
		s.logger.Debug(ctx, "Failed to cache a variable", log.String("name", name), log.Error(err))
	}
	return variable, nil
}

func (s *cacheBackedStore) ListVariables(ctx context.Context, q listQuery) ([]Variable, int, error) {
	return s.store.ListVariables(ctx, q)
}

func (s *cacheBackedStore) InsertVariable(ctx context.Context, v Variable) (bool, error) {
	inserted, err := s.store.InsertVariable(ctx, v)
	if err == nil {
		s.forgetVariable(ctx, v.Name)
	}
	return inserted, err
}

func (s *cacheBackedStore) UpsertVariable(ctx context.Context, v Variable) (bool, error) {
	created, err := s.store.UpsertVariable(ctx, v)
	if err == nil {
		s.forgetVariable(ctx, v.Name)
	}
	return created, err
}

func (s *cacheBackedStore) DeleteVariable(ctx context.Context, name string) error {
	err := s.store.DeleteVariable(ctx, name)
	if err == nil {
		s.forgetVariable(ctx, name)
	}
	return err
}

func (s *cacheBackedStore) GetSecret(ctx context.Context, name string) (*Secret, error) {
	if held, found := s.secrets.Get(ctx, cache.CacheKey{Key: name}); found {
		return held, nil
	}
	secret, err := s.store.GetSecret(ctx, name)
	if err != nil || secret == nil {
		return secret, err
	}
	if err := s.secrets.Set(ctx, cache.CacheKey{Key: name}, secret); err != nil {
		s.logger.Debug(ctx, "Failed to cache a secret", log.String("name", name), log.Error(err))
	}
	return secret, nil
}

func (s *cacheBackedStore) ListSecrets(ctx context.Context, q listQuery) ([]Secret, int, error) {
	return s.store.ListSecrets(ctx, q)
}

func (s *cacheBackedStore) InsertSecret(ctx context.Context, name, sealed, description string) (bool, error) {
	inserted, err := s.store.InsertSecret(ctx, name, sealed, description)
	if err == nil {
		s.forgetSecret(ctx, name)
	}
	return inserted, err
}

func (s *cacheBackedStore) UpsertSecret(ctx context.Context, name, sealed, description string) (bool, error) {
	created, err := s.store.UpsertSecret(ctx, name, sealed, description)
	if err == nil {
		s.forgetSecret(ctx, name)
	}
	return created, err
}

func (s *cacheBackedStore) DeleteSecret(ctx context.Context, name string) error {
	err := s.store.DeleteSecret(ctx, name)
	if err == nil {
		s.forgetSecret(ctx, name)
	}
	return err
}

// forgetVariable drops a name so the next read goes to the database.
//
// A failure to invalidate is logged at warn rather than swallowed: the entry that could not be
// dropped is the one a caller has just changed, so a later read may answer with what it replaced.
func (s *cacheBackedStore) forgetVariable(ctx context.Context, name string) {
	if err := s.variables.Delete(ctx, cache.CacheKey{Key: name}); err != nil {
		s.logger.Warn(ctx, "Failed to invalidate a cached variable after a write",
			log.String("name", name), log.Error(err))
	}
}

func (s *cacheBackedStore) forgetSecret(ctx context.Context, name string) {
	if err := s.secrets.Delete(ctx, cache.CacheKey{Key: name}); err != nil {
		s.logger.Warn(ctx, "Failed to invalidate a cached secret after a write",
			log.String("name", name), log.Error(err))
	}
}
