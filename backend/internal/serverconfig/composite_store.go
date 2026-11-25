// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
)

// compositeServerConfigStore combines the file-based (read-only) and database (writable) stores.
// Reads take the readOnly layer from the file store and the writable layer from the db store; writes
// go to the db store only.
type compositeServerConfigStore struct {
	fileStore serverConfigStoreInterface
	dbStore   serverConfigStoreInterface
}

// newCompositeServerConfigStore creates a composite store over the file and database stores.
func newCompositeServerConfigStore(fileStore, dbStore serverConfigStoreInterface) serverConfigStoreInterface {
	return &compositeServerConfigStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

func (c *compositeServerConfigStore) GetServerConfig(ctx context.Context,
	name ConfigName) (storeLayers, error) {
	fileLayers, err := c.fileStore.GetServerConfig(ctx, name)
	if err != nil {
		return storeLayers{}, err
	}
	dbLayers, err := c.dbStore.GetServerConfig(ctx, name)
	if err != nil {
		return storeLayers{}, err
	}
	return storeLayers{ReadOnly: fileLayers.ReadOnly, Writable: dbLayers.Writable}, nil
}

func (c *compositeServerConfigStore) UpsertServerConfig(ctx context.Context, cfg ServerConfig) error {
	return c.dbStore.UpsertServerConfig(ctx, cfg)
}

func (c *compositeServerConfigStore) ReplaceServerConfigIf(ctx context.Context, cfg ServerConfig,
	expected json.RawMessage) (bool, error) {
	return c.dbStore.ReplaceServerConfigIf(ctx, cfg, expected)
}
