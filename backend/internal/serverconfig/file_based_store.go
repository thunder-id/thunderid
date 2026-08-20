// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// serverConfigDoc is a parsed declarative server-config document: a config name and its raw value.
type serverConfigDoc struct {
	Name  ConfigName
	Value json.RawMessage
}

// fileBasedStore is the in-memory, read-only declarative layer keyed by ConfigName, loaded at startup.
type fileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

var (
	_ declarativeresource.Storer = (*fileBasedStore)(nil)
	_ serverConfigStoreInterface = (*fileBasedStore)(nil)
)

// newFileBasedStore creates a new instance of a file-based store.
func newFileBasedStore() *fileBasedStore {
	return &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeServerConfig),
	}
}

// Create implements declarativeresource.Storer interface for the resource loader.
func (s *fileBasedStore) Create(id string, data any) error {
	doc, ok := data.(*serverConfigDoc)
	if !ok {
		return fmt.Errorf("serverconfig: file store got unexpected declarative type %T", data)
	}
	return s.GenericFileBasedStore.Create(id, doc)
}

// GetByName returns the declarative value for name and whether one is set.
func (s *fileBasedStore) GetByName(ctx context.Context, name ConfigName) (json.RawMessage, bool) {
	data, err := s.GenericFileBasedStore.Get(ctx, string(name))
	if err != nil || data == nil {
		return nil, false
	}
	doc, ok := data.(*serverConfigDoc)
	if !ok {
		return nil, false
	}
	return doc.Value, true
}

// GetServerConfig serves the declarative value as the read-only layer; the writable layer is never set.
func (s *fileBasedStore) GetServerConfig(ctx context.Context, name ConfigName) (storeLayers, error) {
	value, ok := s.GetByName(ctx, name)
	if !ok {
		return storeLayers{}, nil
	}
	return storeLayers{ReadOnly: value}, nil
}

// UpsertServerConfig is rejected: the declarative layer is read-only.
func (s *fileBasedStore) UpsertServerConfig(ctx context.Context, _ ServerConfig) error {
	return errors.New("serverconfig: declarative store is read-only")
}
