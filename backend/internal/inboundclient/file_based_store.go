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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package inboundclient

import (
	"context"
	"errors"
	"fmt"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
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
func (f *fileBasedStore) CreateInboundClient(_ context.Context, client inboundmodel.InboundClient) error {
	return f.GenericFileBasedStore.Create(client.ID, &client)
}

// CreateOAuthProfile is not supported in the file store — OAuth profile is embedded in the
// inbound client's Properties under PropOAuthProfile.
func (f *fileBasedStore) CreateOAuthProfile(_ context.Context, _ string, _ *inboundmodel.OAuthProfile) error {
	return errors.New("CreateOAuthProfile is not supported in file-based store")
}

// GetInboundClientByEntityID retrieves an inbound client from the file store by entity ID.
func (f *fileBasedStore) GetInboundClientByEntityID(_ context.Context, entityID string) (
	*inboundmodel.InboundClient, error) {
	data, err := f.GenericFileBasedStore.Get(entityID)
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
	*inboundmodel.OAuthProfile, error) {
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

	var oauthProfile inboundmodel.OAuthProfile
	switch p := raw.(type) {
	case inboundmodel.OAuthProfile:
		oauthProfile = p
	case *inboundmodel.OAuthProfile:
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
func (f *fileBasedStore) GetInboundClientList(_ context.Context, limit int) ([]inboundmodel.InboundClient, error) {
	list, err := f.GenericFileBasedStore.List()
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
func (f *fileBasedStore) GetTotalInboundClientCount(_ context.Context) (int, error) {
	return f.GenericFileBasedStore.Count()
}

// UpdateInboundClient is not supported in the file store.
func (f *fileBasedStore) UpdateInboundClient(_ context.Context, _ inboundmodel.InboundClient) error {
	return errors.New("UpdateInboundClient is not supported in file-based store")
}

// UpdateOAuthProfile is not supported in the file store.
func (f *fileBasedStore) UpdateOAuthProfile(_ context.Context, _ string, _ *inboundmodel.OAuthProfile) error {
	return errors.New("UpdateOAuthProfile is not supported in file-based store")
}

// DeleteInboundClient is not supported in the file store.
func (f *fileBasedStore) DeleteInboundClient(_ context.Context, _ string) error {
	return errors.New("DeleteInboundClient is not supported in file-based store")
}

// DeleteOAuthProfile is not supported in the file store.
func (f *fileBasedStore) DeleteOAuthProfile(_ context.Context, _ string) error {
	return errors.New("DeleteOAuthProfile is not supported in file-based store")
}

// InboundClientExists reports whether an inbound client with the given entity ID is present
// in the file store.
func (f *fileBasedStore) InboundClientExists(_ context.Context, entityID string) (bool, error) {
	_, err := f.GenericFileBasedStore.Get(entityID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// IsDeclarative returns true when the given entity ID corresponds to a declaratively-loaded
// inbound client. All inbound clients held by the file store are declarative.
func (f *fileBasedStore) IsDeclarative(_ context.Context, entityID string) bool {
	_, err := f.GenericFileBasedStore.Get(entityID)
	return err == nil
}
