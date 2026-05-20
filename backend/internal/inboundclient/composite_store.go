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

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeStore combines a file-backed (immutable, declarative) store and a database-backed
// (mutable) store. Reads fuse results; writes go to the database store. On ID conflict the DB
// entry takes precedence and is emitted with IsReadOnly=false; file entries are surfaced with
// IsReadOnly=true only when no DB entry with the same ID exists.
type compositeStore struct {
	fileStore inboundClientStoreInterface
	dbStore   inboundClientStoreInterface
}

// newCompositeStore returns an inboundClientStoreInterface that fuses a file-backed store with
// a DB-backed store. Both must be non-nil.
func newCompositeStore(fileStore, dbStore inboundClientStoreInterface) inboundClientStoreInterface {
	return &compositeStore{fileStore: fileStore, dbStore: dbStore}
}

func (c *compositeStore) GetTotalInboundClientCount(ctx context.Context) (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return c.dbStore.GetTotalInboundClientCount(ctx) },
		func() (int, error) { return c.fileStore.GetTotalInboundClientCount(ctx) },
	)
}

func (c *compositeStore) GetInboundClientList(ctx context.Context, limit int) ([]inboundmodel.InboundClient, error) {
	if limit <= 0 {
		limit = serverconst.MaxCompositeStoreRecords
	}
	clients, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetTotalInboundClientCount(ctx) },
		func() (int, error) { return c.fileStore.GetTotalInboundClientCount(ctx) },
		func(lim int) ([]inboundmodel.InboundClient, error) { return c.dbStore.GetInboundClientList(ctx, lim) },
		func(lim int) ([]inboundmodel.InboundClient, error) { return c.fileStore.GetInboundClientList(ctx, lim) },
		mergeAndDeduplicateInboundClients,
		limit, 0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, ErrCompositeResultLimitExceeded
	}
	return clients, nil
}

func (c *compositeStore) CreateInboundClient(ctx context.Context, client inboundmodel.InboundClient) error {
	return c.dbStore.CreateInboundClient(ctx, client)
}

func (c *compositeStore) CreateOAuthProfile(ctx context.Context, entityID string,
	oauthProfile *inboundmodel.OAuthProfile) error {
	return c.dbStore.CreateOAuthProfile(ctx, entityID, oauthProfile)
}

func (c *compositeStore) GetInboundClientByEntityID(ctx context.Context, entityID string) (
	*inboundmodel.InboundClient, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*inboundmodel.InboundClient, error) {
			return c.dbStore.GetInboundClientByEntityID(ctx, entityID)
		},
		func() (*inboundmodel.InboundClient, error) {
			return c.fileStore.GetInboundClientByEntityID(ctx, entityID)
		},
		ErrInboundClientNotFound,
	)
}

func (c *compositeStore) GetOAuthProfileByEntityID(ctx context.Context, entityID string) (
	*inboundmodel.OAuthProfile, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*inboundmodel.OAuthProfile, error) { return c.dbStore.GetOAuthProfileByEntityID(ctx, entityID) },
		func() (*inboundmodel.OAuthProfile, error) {
			return c.fileStore.GetOAuthProfileByEntityID(ctx, entityID)
		},
		ErrInboundClientNotFound,
	)
}

func (c *compositeStore) UpdateInboundClient(ctx context.Context, client inboundmodel.InboundClient) error {
	return c.dbStore.UpdateInboundClient(ctx, client)
}

func (c *compositeStore) UpdateOAuthProfile(ctx context.Context, entityID string,
	oauthProfile *inboundmodel.OAuthProfile) error {
	return c.dbStore.UpdateOAuthProfile(ctx, entityID, oauthProfile)
}

func (c *compositeStore) DeleteInboundClient(ctx context.Context, entityID string) error {
	return c.dbStore.DeleteInboundClient(ctx, entityID)
}

func (c *compositeStore) DeleteOAuthProfile(ctx context.Context, entityID string) error {
	return c.dbStore.DeleteOAuthProfile(ctx, entityID)
}

func (c *compositeStore) InboundClientExists(ctx context.Context, entityID string) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.InboundClientExists(ctx, entityID) },
		func() (bool, error) { return c.dbStore.InboundClientExists(ctx, entityID) },
	)
}

func (c *compositeStore) IsDeclarative(ctx context.Context, entityID string) bool {
	return declarativeresource.CompositeIsDeclarativeHelper(
		entityID,
		func(id string) (bool, error) { return c.fileStore.InboundClientExists(ctx, id) },
	)
}

// mergeAndDeduplicateInboundClients merges inbound clients from the DB and file stores,
// deduplicating by entity ID. DB entries are marked mutable (IsReadOnly=false); file entries
// declarative (IsReadOnly=true).
func mergeAndDeduplicateInboundClients(
	dbClients, fileClients []inboundmodel.InboundClient,
) []inboundmodel.InboundClient {
	seen := make(map[string]bool)
	result := make([]inboundmodel.InboundClient, 0, len(dbClients)+len(fileClients))

	for i := range dbClients {
		if !seen[dbClients[i].ID] {
			seen[dbClients[i].ID] = true
			c := dbClients[i]
			c.IsReadOnly = false
			result = append(result, c)
		}
	}

	for i := range fileClients {
		if !seen[fileClients[i].ID] {
			seen[fileClients[i].ID] = true
			c := fileClients[i]
			c.IsReadOnly = true
			result = append(result, c)
		}
	}

	return result
}
