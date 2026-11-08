// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"errors"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeCredentialStore combines a file-based (immutable) and a database
// (mutable) store: reads merge both, writes route to the database, and declarative
// configurations cannot be modified or deleted.
type compositeCredentialStore struct {
	fileStore credentialStoreInterface
	dbStore   credentialStoreInterface
}

// newCompositeCredentialStore creates a compositeCredentialStore combining the given file and database stores.
func newCompositeCredentialStore(fileStore, dbStore credentialStoreInterface) *compositeCredentialStore {
	return &compositeCredentialStore{fileStore: fileStore, dbStore: dbStore}
}

// Create creates a new credential configuration in the database store only.
func (c *compositeCredentialStore) CreateCredentialConfiguration(
	ctx context.Context, dto CredentialConfigurationDTO,
) error {
	return c.dbStore.CreateCredentialConfiguration(ctx, dto)
}

// GetByID retrieves a credential configuration, checking the database then the file store.
func (c *compositeCredentialStore) GetCredentialConfigurationByID(
	ctx context.Context, id string,
) (*CredentialConfigurationDTO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*CredentialConfigurationDTO, error) { return c.dbStore.GetCredentialConfigurationByID(ctx, id) },
		func() (*CredentialConfigurationDTO, error) {
			return c.fileStore.GetCredentialConfigurationByID(ctx, id)
		},
		ErrNotFound,
	)
}

// GetByHandle retrieves a credential configuration, checking the database then the file store.
func (c *compositeCredentialStore) GetCredentialConfigurationByHandle(
	ctx context.Context, handle string,
) (*CredentialConfigurationDTO, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*CredentialConfigurationDTO, error) {
			return c.dbStore.GetCredentialConfigurationByHandle(ctx, handle)
		},
		func() (*CredentialConfigurationDTO, error) {
			return c.fileStore.GetCredentialConfigurationByHandle(ctx, handle)
		},
		ErrNotFound,
	)
}

// ListSummaries retrieves minimal listing data from both stores and merges the results.
func (c *compositeCredentialStore) ListCredentialConfigurationSummaries(
	ctx context.Context,
) ([]CredentialConfigurationList, error) {
	dbSummaries, err := c.dbStore.ListCredentialConfigurationSummaries(ctx)
	if err != nil {
		return nil, err
	}
	fileSummaries, err := c.fileStore.ListCredentialConfigurationSummaries(ctx)
	if err != nil {
		return nil, err
	}
	return mergeSummaries(dbSummaries, fileSummaries)
}

// ListCredentialConfigurationSummariesByOUID retrieves minimal listing data for the credential configurations an
// organization unit owns from both stores and merges the results.
func (c *compositeCredentialStore) ListCredentialConfigurationSummariesByOUID(
	ctx context.Context, ouID string,
) ([]CredentialConfigurationList, error) {
	dbSummaries, err := c.dbStore.ListCredentialConfigurationSummariesByOUID(ctx, ouID)
	if err != nil {
		return nil, err
	}
	fileSummaries, err := c.fileStore.ListCredentialConfigurationSummariesByOUID(ctx, ouID)
	if err != nil {
		return nil, err
	}
	return mergeSummaries(dbSummaries, fileSummaries)
}

// ListCredentialConfigurationSummariesByProject retrieves minimal listing data for the credential configurations
// of a project from both stores and merges the results.
func (c *compositeCredentialStore) ListCredentialConfigurationSummariesByProject(
	ctx context.Context, projectID string,
) ([]CredentialConfigurationList, error) {
	dbSummaries, err := c.dbStore.ListCredentialConfigurationSummariesByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	fileSummaries, err := c.fileStore.ListCredentialConfigurationSummariesByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return mergeSummaries(dbSummaries, fileSummaries)
}

// CountCredentialConfigurationsByProject returns the number of credential configurations of a project in both stores.
// An ID is unique across the stores, so the counts add up exactly.
func (c *compositeCredentialStore) CountCredentialConfigurationsByProject(
	ctx context.Context, projectID string,
) (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return c.dbStore.CountCredentialConfigurationsByProject(ctx, projectID) },
		func() (int, error) { return c.fileStore.CountCredentialConfigurationsByProject(ctx, projectID) },
	)
}

// List retrieves credential configurations from both stores and merges the results.
func (c *compositeCredentialStore) ListCredentialConfigurations(
	ctx context.Context,
) ([]CredentialConfigurationDTO, error) {
	dbConfigs, err := c.dbStore.ListCredentialConfigurations(ctx)
	if err != nil {
		return nil, err
	}
	fileConfigs, err := c.fileStore.ListCredentialConfigurations(ctx)
	if err != nil {
		return nil, err
	}

	configs, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return len(dbConfigs), nil },
		func() (int, error) { return len(fileConfigs), nil },
		func(int) ([]CredentialConfigurationDTO, error) { return dbConfigs, nil },
		func(int) ([]CredentialConfigurationDTO, error) { return fileConfigs, nil },
		mergeAndDeduplicate,
		len(dbConfigs)+len(fileConfigs),
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, ErrResultLimitExceededInCompositeMode
	}
	return configs, nil
}

// Update updates a credential configuration in the database store only, returning
// ErrConfigurationIsImmutable when the configuration is declarative (file-based).
func (c *compositeCredentialStore) UpdateCredentialConfiguration(
	ctx context.Context, dto CredentialConfigurationDTO,
) error {
	return declarativeresource.CompositeUpdateHelper(
		dto,
		func(d CredentialConfigurationDTO) string { return d.ID },
		func(id string) (bool, error) { return c.existsInFileStore(ctx, id) },
		func(d CredentialConfigurationDTO) error { return c.dbStore.UpdateCredentialConfiguration(ctx, d) },
		ErrConfigurationIsImmutable,
	)
}

// Delete deletes a credential configuration from the database store only, returning
// ErrConfigurationIsImmutable when the configuration is declarative (file-based).
func (c *compositeCredentialStore) DeleteCredentialConfiguration(ctx context.Context, id string) error {
	return declarativeresource.CompositeDeleteHelper(
		id,
		func(id string) (bool, error) { return c.existsInFileStore(ctx, id) },
		func(id string) error { return c.dbStore.DeleteCredentialConfiguration(ctx, id) },
		ErrConfigurationIsImmutable,
	)
}

// IsDeclarative reports whether the credential configuration is file-based (immutable).
func (c *compositeCredentialStore) IsCredentialConfigurationDeclarative(
	ctx context.Context, id string,
) (bool, error) {
	return c.existsInFileStore(ctx, id)
}

// existsInFileStore reports whether the id exists in the immutable file store.
func (c *compositeCredentialStore) existsInFileStore(ctx context.Context, id string) (bool, error) {
	_, err := c.fileStore.GetCredentialConfigurationByID(ctx, id)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return false, err
}

// mergeSummaries merges database and file summaries, deduplicated by ID with the database entry
// winning, and rejects a merged list over the composite cap.
func mergeSummaries(dbSummaries, fileSummaries []CredentialConfigurationList) ([]CredentialConfigurationList, error) {
	seen := make(map[string]bool, len(dbSummaries))
	result := make([]CredentialConfigurationList, 0, len(dbSummaries)+len(fileSummaries))
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

// mergeAndDeduplicate merges configurations from both stores, removing duplicate IDs
// with database entries taking precedence.
func mergeAndDeduplicate(dbConfigs, fileConfigs []CredentialConfigurationDTO) []CredentialConfigurationDTO {
	seen := make(map[string]bool, len(dbConfigs))
	result := make([]CredentialConfigurationDTO, 0, len(dbConfigs)+len(fileConfigs))
	for i := range dbConfigs {
		if !seen[dbConfigs[i].ID] {
			seen[dbConfigs[i].ID] = true
			result = append(result, dbConfigs[i])
		}
	}
	for i := range fileConfigs {
		if !seen[fileConfigs[i].ID] {
			seen[fileConfigs[i].ID] = true
			result = append(result, fileConfigs[i])
		}
	}
	return result
}
