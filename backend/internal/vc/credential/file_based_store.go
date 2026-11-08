// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

type credentialFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newCredentialFileBasedStore creates a new instance of a file-based store.
func newCredentialFileBasedStore() *credentialFileBasedStore {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeCredentialConfiguration)
	return &credentialFileBasedStore{GenericFileBasedStore: genericStore}
}

// Create stores a credential configuration in the file-based store. Declarative resources
// are loaded through credentialStorer; the management API cannot reach this method because
// the service refuses creates while the store is in declarative-only mode.
func (f *credentialFileBasedStore) CreateCredentialConfiguration(
	_ context.Context, dto CredentialConfigurationDTO,
) error {
	return f.GenericFileBasedStore.Create(dto.ID, &dto)
}

// credentialStorer adapts the file-based store to declarativeresource.Storer so the
// resource loader can write parsed configurations through the (id, data) entry point.
type credentialStorer struct {
	store *credentialFileBasedStore
}

// Create implements declarativeresource.Storer for the resource loader.
func (s *credentialStorer) Create(id string, data interface{}) error {
	dto, ok := data.(*CredentialConfigurationDTO)
	if !ok {
		return ErrConfigurationDataCorrupted
	}
	if dto.ID == "" {
		dto.ID = id
	}
	return s.store.GenericFileBasedStore.Create(id, dto)
}

// GetByID retrieves a credential configuration by ID from the file-based store.
func (f *credentialFileBasedStore) GetCredentialConfigurationByID(
	_ context.Context, id string,
) (*CredentialConfigurationDTO, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		return nil, ErrNotFound
	}
	dto, ok := data.(*CredentialConfigurationDTO)
	if !ok {
		declarativeresource.LogTypeAssertionError("credential configuration", id)
		return nil, ErrConfigurationDataCorrupted
	}
	// Hand out a copy so a caller cannot mutate the shared declarative entry.
	stored := *dto
	return &stored, nil
}

// GetByHandle retrieves a credential configuration by handle from the file-based store.
func (f *credentialFileBasedStore) GetCredentialConfigurationByHandle(
	_ context.Context, handle string,
) (*CredentialConfigurationDTO, error) {
	data, err := f.GenericFileBasedStore.GetByField(handle, func(d interface{}) string {
		return d.(*CredentialConfigurationDTO).Handle
	})
	if err != nil {
		return nil, ErrNotFound
	}
	stored := *data.(*CredentialConfigurationDTO)
	return &stored, nil
}

// ListSummaries retrieves minimal listing data from the file-based store.
func (f *credentialFileBasedStore) ListCredentialConfigurationSummaries(
	_ context.Context,
) ([]CredentialConfigurationList, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}
	summaries := make([]CredentialConfigurationList, 0, len(list))
	for _, item := range list {
		if dto, ok := item.Data.(*CredentialConfigurationDTO); ok {
			summaries = append(summaries, toConfigSummary(*dto))
		}
	}
	return summaries, nil
}

// ListCredentialConfigurationSummariesByOUID retrieves minimal listing data for the credential configurations an
// organization unit owns from the file-based store.
func (f *credentialFileBasedStore) ListCredentialConfigurationSummariesByOUID(
	ctx context.Context, ouID string,
) ([]CredentialConfigurationList, error) {
	summaries, err := f.ListCredentialConfigurationSummaries(ctx)
	if err != nil {
		return nil, err
	}
	owned := make([]CredentialConfigurationList, 0, len(summaries))
	for _, summary := range summaries {
		if summary.OUID == ouID {
			owned = append(owned, summary)
		}
	}
	return owned, nil
}

// ListCredentialConfigurationSummariesByProject retrieves minimal listing data for the credential configurations
// of a project from the file-based store.
func (f *credentialFileBasedStore) ListCredentialConfigurationSummariesByProject(
	ctx context.Context, projectID string,
) ([]CredentialConfigurationList, error) {
	summaries, err := f.ListCredentialConfigurationSummaries(ctx)
	if err != nil {
		return nil, err
	}
	inProject := make([]CredentialConfigurationList, 0, len(summaries))
	for _, summary := range summaries {
		if summary.ProjectID == projectID {
			inProject = append(inProject, summary)
		}
	}
	return inProject, nil
}

// CountCredentialConfigurationsByProject returns the number of credential configurations of a project in the file-based
// store.
func (f *credentialFileBasedStore) CountCredentialConfigurationsByProject(
	ctx context.Context, projectID string,
) (int, error) {
	summaries, err := f.ListCredentialConfigurationSummariesByProject(ctx, projectID)
	if err != nil {
		return 0, err
	}
	return len(summaries), nil
}

// List retrieves all credential configurations from the file-based store.
func (f *credentialFileBasedStore) ListCredentialConfigurations(
	_ context.Context,
) ([]CredentialConfigurationDTO, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}
	configs := make([]CredentialConfigurationDTO, 0, len(list))
	for _, item := range list {
		if dto, ok := item.Data.(*CredentialConfigurationDTO); ok {
			configs = append(configs, *dto)
		}
	}
	return configs, nil
}

// Update is not supported in the file-based store.
func (f *credentialFileBasedStore) UpdateCredentialConfiguration(
	_ context.Context, _ CredentialConfigurationDTO,
) error {
	return ErrConfigurationIsImmutable
}

// Delete is not supported in the file-based store.
func (f *credentialFileBasedStore) DeleteCredentialConfiguration(_ context.Context, _ string) error {
	return ErrConfigurationIsImmutable
}

// IsDeclarative reports whether the given id exists in the file-based store.
func (f *credentialFileBasedStore) IsCredentialConfigurationDeclarative(_ context.Context, id string) (bool, error) {
	_, err := f.GenericFileBasedStore.Get(id)
	return err == nil, nil
}
