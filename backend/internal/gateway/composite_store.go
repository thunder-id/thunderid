// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import "context"

// compositeStore serves gateways declared in files alongside gateways registered through the API.
//
// Reads ask both and merge. Writes only ever reach the database, and a write aimed at something a
// file declared is refused rather than applied, because the next start would read the file again
// and undo it. This is the same contract every other composite store in the codebase keeps.
type compositeStore struct {
	fileStore storeInterface
	dbStore   storeInterface
}

func newCompositeStore(fileStore, dbStore storeInterface) storeInterface {
	return &compositeStore{fileStore: fileStore, dbStore: dbStore}
}

// List returns both sets, declared gateways first so the order is stable across restarts.
func (c *compositeStore) List(ctx context.Context) ([]Gateway, error) {
	declared, err := c.fileStore.List(ctx)
	if err != nil {
		return nil, err
	}
	registered, err := c.dbStore.List(ctx)
	if err != nil {
		return nil, err
	}
	return append(declared, registered...), nil
}

// GetByID asks the files first. A declared gateway wins a collision, which cannot normally happen
// because ids are generated, but makes the answer deterministic if it ever does.
func (c *compositeStore) GetByID(ctx context.Context, id string) (*Gateway, error) {
	return c.firstOf(
		func() (*Gateway, error) { return c.fileStore.GetByID(ctx, id) },
		func() (*Gateway, error) { return c.dbStore.GetByID(ctx, id) },
	)
}

// GetByName answers across both sources, which is what makes the uniqueness rule hold: a
// registration may not take a name a file already declared.
func (c *compositeStore) GetByName(ctx context.Context, name string) (*Gateway, error) {
	return c.firstOf(
		func() (*Gateway, error) { return c.fileStore.GetByName(ctx, name) },
		func() (*Gateway, error) { return c.dbStore.GetByName(ctx, name) },
	)
}

// GetByBaseURL answers across both sources, so one data plane still registers once however it was
// registered. Without this a file could declare an address the database already holds.
func (c *compositeStore) GetByBaseURL(ctx context.Context, baseURL string) (*Gateway, error) {
	return c.firstOf(
		func() (*Gateway, error) { return c.fileStore.GetByBaseURL(ctx, baseURL) },
		func() (*Gateway, error) { return c.dbStore.GetByBaseURL(ctx, baseURL) },
	)
}

// Count includes declared gateways, so the configured bound is a bound on what the deployment
// administers rather than on how it came to administer it.
func (c *compositeStore) Count(ctx context.Context) (int, error) {
	declared, err := c.fileStore.Count(ctx)
	if err != nil {
		return 0, err
	}
	registered, err := c.dbStore.Count(ctx)
	if err != nil {
		return 0, err
	}
	return declared + registered, nil
}

// Create writes to the database. The name and address checks the service makes before calling this
// already run against both sources, so a registration cannot take what a file declared.
//
// The limit is adjusted for what the files hold, because the database store enforces it against its
// own rows alone and would otherwise let the deployment exceed the configured number.
func (c *compositeStore) Create(ctx context.Context, gw *Gateway, limit int) (*Gateway, error) {
	declared, err := c.fileStore.Count(ctx)
	if err != nil {
		return nil, err
	}
	remaining := limit - declared
	if remaining <= 0 {
		// Refused by the bound, which the service reports. Nil with no error is how this store
		// says the limit turned it away.
		return nil, nil //nolint:nilnil // matches the database store's contract for a refused limit
	}
	return c.dbStore.Create(ctx, gw, remaining)
}

// Update refuses to change a declared gateway and otherwise writes to the database.
func (c *compositeStore) Update(ctx context.Context, gw *Gateway) error {
	declaredEntry, err := c.fileStore.GetByID(ctx, gw.ID)
	if err != nil {
		return err
	}
	if declaredEntry != nil {
		return errDeclarativeGateway
	}
	return c.dbStore.Update(ctx, gw)
}

// Delete refuses to remove a declared gateway and otherwise writes to the database.
func (c *compositeStore) Delete(ctx context.Context, id string) error {
	declaredEntry, err := c.fileStore.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if declaredEntry != nil {
		return errDeclarativeGateway
	}
	return c.dbStore.Delete(ctx, id)
}

// firstOf returns the first source that holds something, and stops at the first real failure. A
// source answering nothing is not a failure: it means the gateway came from the other one.
func (c *compositeStore) firstOf(sources ...func() (*Gateway, error)) (*Gateway, error) {
	for _, source := range sources {
		found, err := source()
		if err != nil {
			return nil, err
		}
		if found != nil {
			return found, nil
		}
	}
	return nil, nil //nolint:nilnil // absent is not an error, matching the database store
}
