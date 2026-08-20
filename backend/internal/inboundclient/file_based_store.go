// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package inboundclient

import (
	"context"
	"errors"
	"fmt"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// PropOAuthProfile is the key under InboundClient.Properties used by the file store to embed
// the typed OAuthProfile for a declaratively-loaded inbound client.
const PropOAuthProfile = "oauth_profile"

// fileBasedStore is a read-only in-memory inboundClientStoreInterface backed by declaratively-loaded
// YAML resources. Create is the only write path and is invoked by the declarative loader
// framework; update/delete/CreateOAuthProfile/DeleteOAuthProfile all return errors.
type fileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newFileBasedStore creates a fileBasedStore scoped to the given key type (e.g.
// entity.KeyTypeApplication). The key type namespaces entries in the shared generic store so
// multiple callers (application, agent, ...) can coexist without colliding.
func newFileBasedStore(keyType entity.KeyType) *fileBasedStore {
	return &fileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(keyType),
	}
}

// Create implements declarativeresource.Storer. It is called by the declarative loader
// framework with the InboundClient produced by the caller's parser.
func (f *fileBasedStore) Create(id string, data interface{}) error {
	client, ok := data.(*inboundmodel.InboundClient)
	if !ok {
		return fmt.Errorf("unexpected data type for inbound client: %T", data)
	}
	return f.GenericFileBasedStore.Create(id, client)
}

// CreateInboundClient stores an inbound client directly. Used by tests; the production path
// goes via Create (the declarative loader).
func (f *fileBasedStore) CreateInboundClient(ctx context.Context, client inboundmodel.InboundClient) error {
	return f.GenericFileBasedStore.Create(client.ID, &client)
}

// CreateOAuthProfile is not supported in the file store — OAuth profile is embedded in the
// inbound client's Properties under PropOAuthProfile.
func (f *fileBasedStore) CreateOAuthProfile(ctx context.Context, _ string, _ *providers.OAuthProfile) error {
	return errors.New("CreateOAuthProfile is not supported in file-based store")
}

// GetInboundClientByEntityID retrieves an inbound client from the file store by entity ID.
func (f *fileBasedStore) GetInboundClientByEntityID(ctx context.Context, entityID string) (
	*inboundmodel.InboundClient, error) {
	data, err := f.GenericFileBasedStore.Get(ctx, entityID)
	if err != nil {
		return nil, ErrInboundClientNotFound
	}
	client, ok := data.(*inboundmodel.InboundClient)
	if !ok {
		declarativeresource.LogTypeAssertionError("inbound client", entityID)
		return nil, ErrInboundClientDataCorrupted
	}
	return client, nil
}

// GetOAuthProfileByEntityID extracts the OAuth profile embedded in the inbound client's Properties.
func (f *fileBasedStore) GetOAuthProfileByEntityID(ctx context.Context, entityID string) (
	*providers.OAuthProfile, error) {
	client, err := f.GetInboundClientByEntityID(ctx, entityID)
	if err != nil {
		return nil, err
	}
	if client == nil || client.Properties == nil {
		return nil, nil
	}

	raw, ok := client.Properties[PropOAuthProfile]
	if !ok || raw == nil {
		return nil, nil
	}

	var oauthProfile providers.OAuthProfile
	switch p := raw.(type) {
	case providers.OAuthProfile:
		oauthProfile = p
	case *providers.OAuthProfile:
		if p == nil {
			return nil, nil
		}
		oauthProfile = *p
	default:
		declarativeresource.LogTypeAssertionError("inbound OAuth profile", entityID)
		return nil, ErrInboundClientDataCorrupted
	}

	return &oauthProfile, nil
}

// GetInboundClientList returns all inbound clients in the file store with IsReadOnly set.
func (f *fileBasedStore) GetInboundClientList(ctx context.Context, limit int) ([]inboundmodel.InboundClient, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, err
	}

	clients := make([]inboundmodel.InboundClient, 0, len(list))
	for _, item := range list {
		if limit > 0 && len(clients) >= limit {
			break
		}
		if c, ok := item.Data.(*inboundmodel.InboundClient); ok {
			copy := *c
			copy.IsReadOnly = true
			clients = append(clients, copy)
		}
	}
	return clients, nil
}

// GetTotalInboundClientCount returns the count of inbound clients in the file store.
func (f *fileBasedStore) GetTotalInboundClientCount(ctx context.Context) (int, error) {
	return f.GenericFileBasedStore.Count(ctx)
}

// UpdateInboundClient is not supported in the file store.
func (f *fileBasedStore) UpdateInboundClient(ctx context.Context, _ inboundmodel.InboundClient) error {
	return errors.New("UpdateInboundClient is not supported in file-based store")
}

// UpdateOAuthProfile is not supported in the file store.
func (f *fileBasedStore) UpdateOAuthProfile(ctx context.Context, _ string, _ *providers.OAuthProfile) error {
	return errors.New("UpdateOAuthProfile is not supported in file-based store")
}

// DeleteInboundClient is not supported in the file store.
func (f *fileBasedStore) DeleteInboundClient(ctx context.Context, _ string) error {
	return errors.New("DeleteInboundClient is not supported in file-based store")
}

// DeleteOAuthProfile is not supported in the file store.
func (f *fileBasedStore) DeleteOAuthProfile(ctx context.Context, _ string) error {
	return errors.New("DeleteOAuthProfile is not supported in file-based store")
}

// InboundClientExists reports whether an inbound client with the given entity ID is present
// in the file store.
func (f *fileBasedStore) InboundClientExists(ctx context.Context, entityID string) (bool, error) {
	_, err := f.GenericFileBasedStore.Get(ctx, entityID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// IsDeclarative returns true when the given entity ID corresponds to a declaratively-loaded
// inbound client. All inbound clients held by the file store are declarative.
func (f *fileBasedStore) IsDeclarative(ctx context.Context, entityID string) bool {
	_, err := f.GenericFileBasedStore.Get(ctx, entityID)
	return err == nil
}

func (f *fileBasedStore) GetEntityIDsByReference(ctx context.Context, refType, refID string, limit,
	offset int) ([]string, int, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, 0, err
	}

	matched := make([]string, 0)
	for _, item := range list {
		if c, ok := item.Data.(*inboundmodel.InboundClient); ok && clientReferences(c, refType, refID) {
			matched = append(matched, c.ID)
		}
	}

	total := len(matched)
	if offset >= total || limit == 0 {
		return []string{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
}
