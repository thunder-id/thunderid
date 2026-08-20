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

// Create stores a credential configuration in the file-based store. In declarative
// and composite modes the loader writes resources through this method (resources
// loaded from YAML are immutable; management writes route to the database store).
func (f *credentialFileBasedStore) CreateCredentialConfiguration(ctx context.Context, dto CredentialConfigurationDTO,
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
func (f *credentialFileBasedStore) GetCredentialConfigurationByID(ctx context.Context, id string,
) (*CredentialConfigurationDTO, error) {
	data, err := f.GenericFileBasedStore.Get(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	dto, ok := data.(*CredentialConfigurationDTO)
	if !ok {
		declarativeresource.LogTypeAssertionError("credential configuration", id)
		return nil, ErrConfigurationDataCorrupted
	}
	return dto, nil
}

// GetByHandle retrieves a credential configuration by handle from the file-based store.
func (f *credentialFileBasedStore) GetCredentialConfigurationByHandle(ctx context.Context, handle string,
) (*CredentialConfigurationDTO, error) {
	data, err := f.GenericFileBasedStore.GetByField(ctx, handle, func(d interface{}) string {
		return d.(*CredentialConfigurationDTO).Handle
	})
	if err != nil {
		return nil, ErrNotFound
	}
	return data.(*CredentialConfigurationDTO), nil
}

// ListSummaries retrieves minimal listing data from the file-based store.
func (f *credentialFileBasedStore) ListCredentialConfigurationSummaries(ctx context.Context,
) ([]CredentialConfigurationList, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
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

// List retrieves all credential configurations from the file-based store.
func (f *credentialFileBasedStore) ListCredentialConfigurations(ctx context.Context,
) ([]CredentialConfigurationDTO, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
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
func (f *credentialFileBasedStore) UpdateCredentialConfiguration(ctx context.Context, _ CredentialConfigurationDTO,
) error {
	return ErrConfigurationIsImmutable
}

// Delete is not supported in the file-based store.
func (f *credentialFileBasedStore) DeleteCredentialConfiguration(ctx context.Context, _ string) error {
	return ErrConfigurationIsImmutable
}

// IsDeclarative reports whether the given id exists in the file-based store.
func (f *credentialFileBasedStore) IsCredentialConfigurationDeclarative(ctx context.Context, id string) (bool, error) {
	_, err := f.GenericFileBasedStore.Get(ctx, id)
	return err == nil, nil
}
