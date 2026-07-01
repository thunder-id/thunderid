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

package definition

import (
	"context"
	"errors"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeDefinitionStore implements a composite store that combines file-based
// (immutable) and database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative definitions (from YAML files) cannot be modified or deleted
type compositeDefinitionStore struct {
	fileStore definitionStoreInterface
	dbStore   definitionStoreInterface
}

// newCompositeDefinitionStore creates a new composite store with both file-based and database stores.
func newCompositeDefinitionStore(fileStore, dbStore definitionStoreInterface) *compositeDefinitionStore {
	return &compositeDefinitionStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// Create creates a new presentation definition in the database store only.
func (c *compositeDefinitionStore) CreatePresentationDefinition(
	ctx context.Context, dto PresentationDefinitionDTO,
) error {
	return c.dbStore.CreatePresentationDefinition(ctx, dto)
}

// GetByID retrieves a presentation definition by ID from either store.
// Checks the database store first, then falls back to the file store.
func (c *compositeDefinitionStore) GetPresentationDefinitionByID(
	ctx context.Context, id string,
) (*PresentationDefinitionDTO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*PresentationDefinitionDTO, error) { return c.dbStore.GetPresentationDefinitionByID(ctx, id) },
		func() (*PresentationDefinitionDTO, error) { return c.fileStore.GetPresentationDefinitionByID(ctx, id) },
		ErrNotFound,
	)
}

// GetByHandle retrieves a presentation definition by handle from either store.
// Checks the database store first, then falls back to the file store.
func (c *compositeDefinitionStore) GetPresentationDefinitionByHandle(
	ctx context.Context, handle string,
) (*PresentationDefinitionDTO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*PresentationDefinitionDTO, error) {
			return c.dbStore.GetPresentationDefinitionByHandle(ctx, handle)
		},
		func() (*PresentationDefinitionDTO, error) {
			return c.fileStore.GetPresentationDefinitionByHandle(ctx, handle)
		},
		ErrNotFound,
	)
}

// ListSummaries retrieves minimal listing data from both stores and merges the results.
func (c *compositeDefinitionStore) ListPresentationDefinitionSummaries(
	ctx context.Context,
) ([]PresentationDefinitionList, error) {
	dbSummaries, err := c.dbStore.ListPresentationDefinitionSummaries(ctx)
	if err != nil {
		return nil, err
	}
	fileSummaries, err := c.fileStore.ListPresentationDefinitionSummaries(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(dbSummaries))
	result := make([]PresentationDefinitionList, 0, len(dbSummaries)+len(fileSummaries))
	for _, s := range dbSummaries {
		if !seen[s.ID] {
			seen[s.ID] = true
			result = append(result, s)
		}
	}
	for _, s := range fileSummaries {
		if !seen[s.ID] {
			seen[s.ID] = true
			result = append(result, s)
		}
	}
	if len(result) > serverconst.MaxCompositeStoreRecords {
		return nil, ErrResultLimitExceededInCompositeMode
	}
	return result, nil
}

// List retrieves presentation definitions from both stores and merges the results.
func (c *compositeDefinitionStore) ListPresentationDefinitions(
	ctx context.Context,
) ([]PresentationDefinitionDTO, error) {
	dbDefs, err := c.dbStore.ListPresentationDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	fileDefs, err := c.fileStore.ListPresentationDefinitions(ctx)
	if err != nil {
		return nil, err
	}

	defs, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return len(dbDefs), nil },
		func() (int, error) { return len(fileDefs), nil },
		func(int) ([]PresentationDefinitionDTO, error) { return dbDefs, nil },
		func(int) ([]PresentationDefinitionDTO, error) { return fileDefs, nil },
		mergeAndDeduplicate,
		len(dbDefs)+len(fileDefs),
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, ErrResultLimitExceededInCompositeMode
	}

	return defs, nil
}

// Update updates a presentation definition in the database store only.
// Returns ErrDefinitionIsImmutable if the definition is declarative (exists in file store).
func (c *compositeDefinitionStore) UpdatePresentationDefinition(
	ctx context.Context, dto PresentationDefinitionDTO,
) error {
	return declarativeresource.CompositeUpdateHelper(
		dto,
		func(d PresentationDefinitionDTO) string { return d.ID },
		func(id string) (bool, error) { return c.existsInFileStore(ctx, id) },
		func(d PresentationDefinitionDTO) error { return c.dbStore.UpdatePresentationDefinition(ctx, d) },
		ErrDefinitionIsImmutable,
	)
}

// Delete deletes a presentation definition from the database store only.
// Returns ErrDefinitionIsImmutable if the definition is declarative (exists in file store).
func (c *compositeDefinitionStore) DeletePresentationDefinition(ctx context.Context, id string) error {
	return declarativeresource.CompositeDeleteHelper(
		id,
		func(id string) (bool, error) { return c.existsInFileStore(ctx, id) },
		func(id string) error { return c.dbStore.DeletePresentationDefinition(ctx, id) },
		ErrDefinitionIsImmutable,
	)
}

// IsDeclarative reports whether the presentation definition is file-based (immutable).
func (c *compositeDefinitionStore) IsPresentationDefinitionDeclarative(ctx context.Context, id string) (bool, error) {
	return c.existsInFileStore(ctx, id)
}

// existsInFileStore reports whether the id exists in the immutable file store.
func (c *compositeDefinitionStore) existsInFileStore(ctx context.Context, id string) (bool, error) {
	_, err := c.fileStore.GetPresentationDefinitionByID(ctx, id)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return false, err
}

// mergeAndDeduplicate merges definitions from both stores and removes duplicates by ID.
// Database definitions take precedence over file-based definitions with the same ID.
// While duplicates shouldn't exist by design, this provides defensive programming.
func mergeAndDeduplicate(dbDefs, fileDefs []PresentationDefinitionDTO) []PresentationDefinitionDTO {
	seen := make(map[string]bool, len(dbDefs))
	result := make([]PresentationDefinitionDTO, 0, len(dbDefs)+len(fileDefs))

	for i := range dbDefs {
		if !seen[dbDefs[i].ID] {
			seen[dbDefs[i].ID] = true
			result = append(result, dbDefs[i])
		}
	}

	for i := range fileDefs {
		if !seen[fileDefs[i].ID] {
			seen[fileDefs[i].ID] = true
			result = append(result, fileDefs[i])
		}
	}

	return result
}
