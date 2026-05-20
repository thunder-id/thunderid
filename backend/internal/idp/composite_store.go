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

package idp

import (
	"context"
	"errors"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

var (
	// ErrIDPIsImmutable is returned when trying to modify or delete an immutable (file-based) IDP.
	ErrIDPIsImmutable = errors.New("identity provider is immutable")
)

// compositeIDPStore implements a composite store that combines file-based (immutable) and database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative IDPs (from YAML files) cannot be modified or deleted
type compositeIDPStore struct {
	fileStore idpStoreInterface
	dbStore   idpStoreInterface
}

// newCompositeIDPStore creates a new composite store with both file-based and database stores.
func newCompositeIDPStore(fileStore, dbStore idpStoreInterface) *compositeIDPStore {
	return &compositeIDPStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// CreateIdentityProvider creates a new identity provider in the database store only.
func (c *compositeIDPStore) CreateIdentityProvider(ctx context.Context, idp IDPDTO) error {
	return c.dbStore.CreateIdentityProvider(ctx, idp)
}

// GetIdentityProviderList retrieves identity providers from both stores and merges the results.
// Database IDPs are marked as mutable (IsReadOnly=false), file-based IDPs as immutable (IsReadOnly=true).
func (c *compositeIDPStore) GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, error) {
	dbCount, err := c.dbStore.GetIdentityProviderListCount(ctx)
	if err != nil {
		return nil, err
	}

	fileCount, err := c.fileStore.GetIdentityProviderListCount(ctx)
	if err != nil {
		return nil, err
	}

	totalCount := dbCount + fileCount
	idps, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return dbCount, nil },
		func() (int, error) { return fileCount, nil },
		func(count int) ([]BasicIDPDTO, error) { return c.dbStore.GetIdentityProviderList(ctx) },
		func(count int) ([]BasicIDPDTO, error) { return c.fileStore.GetIdentityProviderList(ctx) },
		mergeAndDeduplicateIDPs,
		totalCount,
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, ErrResultLimitExceededInCompositeMode
	}

	return idps, nil
}

// GetIdentityProviderListCount retrieves the count of identity providers across both stores.
func (c *compositeIDPStore) GetIdentityProviderListCount(ctx context.Context) (int, error) {
	dbCount, err := c.dbStore.GetIdentityProviderListCount(ctx)
	if err != nil {
		return 0, err
	}

	fileCount, err := c.fileStore.GetIdentityProviderListCount(ctx)
	if err != nil {
		return 0, err
	}

	return dbCount + fileCount, nil
}

// GetIdentityProvider retrieves an identity provider by ID from either store.
// Checks database store first, then falls back to file store.
func (c *compositeIDPStore) GetIdentityProvider(ctx context.Context, idpID string) (*IDPDTO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*IDPDTO, error) { return c.dbStore.GetIdentityProvider(ctx, idpID) },
		func() (*IDPDTO, error) { return c.fileStore.GetIdentityProvider(ctx, idpID) },
		ErrIDPNotFound,
	)
}

// GetIdentityProviderByName retrieves an identity provider by name from either store.
// Checks database store first, then falls back to file store.
func (c *compositeIDPStore) GetIdentityProviderByName(ctx context.Context, idpName string) (*IDPDTO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*IDPDTO, error) { return c.dbStore.GetIdentityProviderByName(ctx, idpName) },
		func() (*IDPDTO, error) { return c.fileStore.GetIdentityProviderByName(ctx, idpName) },
		ErrIDPNotFound,
	)
}

// GetIdentityProviderByIssuer retrieves an identity provider by its issuer property from either store.
// Checks database store first, then falls back to file store.
func (c *compositeIDPStore) GetIdentityProviderByIssuer(ctx context.Context, issuer string) (*IDPDTO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*IDPDTO, error) { return c.dbStore.GetIdentityProviderByIssuer(ctx, issuer) },
		func() (*IDPDTO, error) { return c.fileStore.GetIdentityProviderByIssuer(ctx, issuer) },
		ErrIDPNotFound,
	)
}

// UpdateIdentityProvider updates an identity provider in the database store only.
// Returns an error if the IDP is declarative (exists in file store).
func (c *compositeIDPStore) UpdateIdentityProvider(ctx context.Context, idp *IDPDTO) error {
	return declarativeresource.CompositeUpdateHelper(
		idp,
		func(dto *IDPDTO) string { return dto.ID },
		func(idpID string) (bool, error) {
			_, err := c.fileStore.GetIdentityProvider(ctx, idpID)
			if err == nil {
				return true, nil // Found in file store
			}
			if errors.Is(err, ErrIDPNotFound) {
				return false, nil // Not found, safe to update
			}
			return false, err // Other error
		},
		func(dto *IDPDTO) error {
			return c.dbStore.UpdateIdentityProvider(ctx, dto)
		},
		ErrIDPIsImmutable,
	)
}

// DeleteIdentityProvider deletes an identity provider from the database store only.
// Returns an error if the IDP is declarative (exists in file store).
func (c *compositeIDPStore) DeleteIdentityProvider(ctx context.Context, idpID string) error {
	return declarativeresource.CompositeDeleteHelper(
		idpID,
		func(id string) (bool, error) {
			_, err := c.fileStore.GetIdentityProvider(ctx, id)
			if err == nil {
				return true, nil // Found in file store
			}
			if errors.Is(err, ErrIDPNotFound) {
				return false, nil // Not found, safe to delete
			}
			return false, err // Other error
		},
		func(id string) error {
			return c.dbStore.DeleteIdentityProvider(ctx, id)
		},
		ErrIDPIsImmutable,
	)
}

// mergeAndDeduplicateIDPs merges IDPs from both stores and removes duplicates by ID.
// Database IDPs are marked as mutable (IsReadOnly=false), file-based IDPs as immutable (IsReadOnly=true).
// While duplicates shouldn't exist by design, this provides defensive programming.
func mergeAndDeduplicateIDPs(dbIDPs, fileIDPs []BasicIDPDTO) []BasicIDPDTO {
	seen := make(map[string]bool)
	result := make([]BasicIDPDTO, 0, len(dbIDPs)+len(fileIDPs))

	// Add DB IDPs first (they take precedence) - mark as mutable (IsReadOnly=false)
	for i := range dbIDPs {
		if !seen[dbIDPs[i].ID] {
			seen[dbIDPs[i].ID] = true
			dbIDPs[i].IsReadOnly = false
			result = append(result, dbIDPs[i])
		}
	}

	// Add file IDPs if not already present - mark as immutable (IsReadOnly=true)
	for i := range fileIDPs {
		if !seen[fileIDPs[i].ID] {
			seen[fileIDPs[i].ID] = true
			fileIDPs[i].IsReadOnly = true
			result = append(result, fileIDPs[i])
		}
	}

	return result
}
