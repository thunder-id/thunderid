// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"errors"
	"strings"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// CredentialConfigurationServiceInterface manages OpenID4VCI credential configurations in the configdb
// store, which the issuer engine reads on demand.
type CredentialConfigurationServiceInterface interface {
	CreateCredentialConfiguration(ctx context.Context, dto *CredentialConfigurationDTO) (
		*CredentialConfigurationDTO, *tidcommon.ServiceError)
	GetCredentialConfiguration(ctx context.Context, id string) (*CredentialConfigurationDTO, *tidcommon.ServiceError)
	GetCredentialConfigurationByHandle(
		ctx context.Context, handle string,
	) (*CredentialConfigurationDTO, *tidcommon.ServiceError)
	ListCredentialConfigurations(ctx context.Context) ([]CredentialConfigurationDTO, *tidcommon.ServiceError)
	ListCredentialConfigurationSummaries(
		ctx context.Context,
	) ([]CredentialConfigurationList, *tidcommon.ServiceError)
	UpdateCredentialConfiguration(ctx context.Context, id string, dto *CredentialConfigurationDTO) (
		*CredentialConfigurationDTO, *tidcommon.ServiceError)
	DeleteCredentialConfiguration(ctx context.Context, id string) *tidcommon.ServiceError
	IsCredentialConfigurationDeclarative(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
}

type configurationService struct {
	store     credentialStoreInterface
	ouService ou.OrganizationUnitServiceInterface
	logger    *log.Logger
	uuid      func() (string, error)
}

// newCredentialConfigurationService builds a credential-configuration service over the given store.
func newCredentialConfigurationService(
	store credentialStoreInterface, ouService ou.OrganizationUnitServiceInterface,
) CredentialConfigurationServiceInterface {
	return &configurationService{
		store:     store,
		ouService: ouService,
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VCIConfigurationService")),
		uuid:      utils.GenerateUUIDv7,
	}
}

// resolveOU resolves ouHandle to ouId when needed and verifies the OU exists.
func (s *configurationService) resolveOU(
	ctx context.Context, dto *CredentialConfigurationDTO,
) *tidcommon.ServiceError {
	if s.ouService == nil {
		return nil
	}
	if dto.OUID == "" && strings.TrimSpace(dto.OUHandle) != "" {
		resolved, svcErr := s.ouService.GetOrganizationUnitByPath(ctx, dto.OUHandle)
		if svcErr != nil {
			return &ErrorConfigurationInvalidOU
		}
		dto.OUID = resolved.ID
	}
	if strings.TrimSpace(dto.OUID) == "" {
		return &ErrorConfigurationInvalidOU
	}
	exists, svcErr := s.ouService.IsOrganizationUnitExists(ctx, dto.OUID)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to verify organization unit", log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !exists {
		return &ErrorConfigurationInvalidOU
	}
	return nil
}

// populateOUHandle sets each DTO's owning OU handle for display.
func (s *configurationService) populateOUHandle(ctx context.Context, dtos ...*CredentialConfigurationDTO) {
	if s.ouService == nil {
		return
	}
	ids := make([]string, 0, len(dtos))
	seen := make(map[string]bool, len(dtos))
	for _, dto := range dtos {
		if dto.OUID != "" && !seen[dto.OUID] {
			seen[dto.OUID] = true
			ids = append(ids, dto.OUID)
		}
	}
	if len(ids) == 0 {
		return
	}
	handles, svcErr := s.ouService.GetOrganizationUnitHandlesByIDs(ctx, ids)
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to resolve OU handles for credential configurations", log.Any("error", svcErr))
		return
	}
	for _, dto := range dtos {
		if h, ok := handles[dto.OUID]; ok {
			dto.OUHandle = h
		}
	}
}

// CreateCredentialConfiguration validates, resolves the OU for, and persists a new credential configuration.
func (s *configurationService) CreateCredentialConfiguration(
	ctx context.Context, dto *CredentialConfigurationDTO,
) (*CredentialConfigurationDTO, *tidcommon.ServiceError) {
	if svcErr := validateConfiguration(dto); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.resolveOU(ctx, dto); svcErr != nil {
		return nil, svcErr
	}

	existing, err := s.store.GetCredentialConfigurationByHandle(ctx, dto.Handle)
	if err != nil && !errors.Is(err, ErrNotFound) {
		s.logger.Error(ctx, "Failed to check existing configuration", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if existing != nil {
		return nil, &ErrorConfigurationAlreadyExists
	}

	id := dto.ID
	if id == "" {
		var genErr error
		id, genErr = s.uuid()
		if genErr != nil {
			s.logger.Error(ctx, "Failed to generate configuration ID", log.Error(genErr))
			return nil, &tidcommon.InternalServerError
		}
	}
	dto.ID = id

	if err := s.store.CreateCredentialConfiguration(ctx, *dto); err != nil {
		s.logger.Error(ctx, "Failed to create credential configuration", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return dto, nil
}

// GetCredentialConfiguration retrieves a credential configuration by ID and resolves its OU handle.
func (s *configurationService) GetCredentialConfiguration(
	ctx context.Context, id string,
) (*CredentialConfigurationDTO, *tidcommon.ServiceError) {
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorConfigurationInvalidRequest
	}
	dto, err := s.store.GetCredentialConfigurationByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &ErrorConfigurationNotFound
		}
		s.logger.Error(ctx, "Failed to get credential configuration", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	s.populateOUHandle(ctx, dto)
	return dto, nil
}

// GetCredentialConfigurationByHandle retrieves a credential configuration by its handle.
func (s *configurationService) GetCredentialConfigurationByHandle(
	ctx context.Context, handle string,
) (*CredentialConfigurationDTO, *tidcommon.ServiceError) {
	dto, err := s.store.GetCredentialConfigurationByHandle(ctx, handle)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &ErrorConfigurationNotFound
		}
		s.logger.Error(ctx, "Failed to get credential configuration by handle", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return dto, nil
}

// ListCredentialConfigurations returns all credential configurations with resolved OU handles.
func (s *configurationService) ListCredentialConfigurations(
	ctx context.Context,
) ([]CredentialConfigurationDTO, *tidcommon.ServiceError) {
	configs, err := s.store.ListCredentialConfigurations(ctx)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorConfigurationResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list credential configurations", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	ptrs := make([]*CredentialConfigurationDTO, len(configs))
	for i := range configs {
		ptrs[i] = &configs[i]
	}
	s.populateOUHandle(ctx, ptrs...)
	return configs, nil
}

// ListCredentialConfigurationSummaries returns summary views of all credential configurations with resolved OU handles.
func (s *configurationService) ListCredentialConfigurationSummaries(
	ctx context.Context,
) ([]CredentialConfigurationList, *tidcommon.ServiceError) {
	summaries, err := s.store.ListCredentialConfigurationSummaries(ctx)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorConfigurationResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list credential configuration summaries", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	s.populateSummaryOUHandles(ctx, summaries)
	markManagedConfigurations(ctx, summaries)
	return summaries, nil
}

// markManagedConfigurations reports the control plane owned entries as read only, which is what a
// client renders its edit and delete controls from.
func markManagedConfigurations(ctx context.Context, items []CredentialConfigurationList) {
	managed := managedresource.Default().ManagedIDs(ctx, managedresource.TypeCredentialConfiguration)
	if len(managed) == 0 {
		return
	}
	for i := range items {
		if managed[items[i].ID] {
			items[i].IsReadOnly = true
		}
	}
}

// populateSummaryOUHandles resolves each summary's owning OU handle for display.
func (s *configurationService) populateSummaryOUHandles(
	ctx context.Context, summaries []CredentialConfigurationList,
) {
	if s.ouService == nil {
		return
	}
	ids := make([]string, 0, len(summaries))
	seen := make(map[string]bool, len(summaries))
	for _, sm := range summaries {
		if sm.OUID != "" && !seen[sm.OUID] {
			seen[sm.OUID] = true
			ids = append(ids, sm.OUID)
		}
	}
	if len(ids) == 0 {
		return
	}
	handles, svcErr := s.ouService.GetOrganizationUnitHandlesByIDs(ctx, ids)
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to resolve OU handles for credential configuration summaries",
			log.Any("error", svcErr))
		return
	}
	for i := range summaries {
		if h, ok := handles[summaries[i].OUID]; ok {
			summaries[i].OUHandle = h
		}
	}
}

// UpdateCredentialConfiguration validates and persists changes to an existing credential configuration.
func (s *configurationService) UpdateCredentialConfiguration(
	ctx context.Context, id string, dto *CredentialConfigurationDTO,
) (*CredentialConfigurationDTO, *tidcommon.ServiceError) {
	// A resource applied from the control plane is owned there. Changing it here would last only
	// until the next promotion overwrote it, so the change is refused instead.
	if svcErr := managedresource.Guard(ctx, managedresource.TypeCredentialConfiguration, id); svcErr != nil {
		return nil, svcErr
	}
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorConfigurationInvalidRequest
	}
	if svcErr := validateConfiguration(dto); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.resolveOU(ctx, dto); svcErr != nil {
		return nil, svcErr
	}

	existing, err := s.store.GetCredentialConfigurationByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &ErrorConfigurationNotFound
		}
		s.logger.Error(ctx, "Failed to load credential configuration", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if existing.Handle != dto.Handle {
		clash, err := s.store.GetCredentialConfigurationByHandle(ctx, dto.Handle)
		if err != nil && !errors.Is(err, ErrNotFound) {
			s.logger.Error(ctx, "Failed to check handle uniqueness", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if clash != nil {
			return nil, &ErrorConfigurationAlreadyExists
		}
	}

	dto.ID = id
	if err := s.store.UpdateCredentialConfiguration(ctx, *dto); err != nil {
		if errors.Is(err, ErrConfigurationIsImmutable) {
			return nil, &ErrorConfigurationImmutable
		}
		s.logger.Error(ctx, "Failed to update credential configuration", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return dto, nil
}

// DeleteCredentialConfiguration deletes a credential configuration by ID, succeeding idempotently when absent.
func (s *configurationService) DeleteCredentialConfiguration(
	ctx context.Context, id string,
) *tidcommon.ServiceError {
	if svcErr := managedresource.Guard(ctx, managedresource.TypeCredentialConfiguration, id); svcErr != nil {
		return svcErr
	}
	if strings.TrimSpace(id) == "" {
		return &ErrorConfigurationInvalidRequest
	}
	if _, err := s.store.GetCredentialConfigurationByID(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil // idempotent
		}
		s.logger.Error(ctx, "Failed to load credential configuration", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if err := s.store.DeleteCredentialConfiguration(ctx, id); err != nil {
		if errors.Is(err, ErrConfigurationIsImmutable) {
			return &ErrorConfigurationImmutable
		}
		s.logger.Error(ctx, "Failed to delete credential configuration", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// IsCredentialConfigurationDeclarative reports whether the credential configuration with the given ID is declarative.
func (s *configurationService) IsCredentialConfigurationDeclarative(
	ctx context.Context, id string,
) (bool, *tidcommon.ServiceError) {
	isDeclarative, err := s.store.IsCredentialConfigurationDeclarative(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to check if credential configuration is declarative", log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	return isDeclarative, nil
}

// validateConfiguration enforces the required fields of a credential configuration.
func validateConfiguration(dto *CredentialConfigurationDTO) *tidcommon.ServiceError {
	if dto == nil || strings.TrimSpace(dto.Handle) == "" || strings.TrimSpace(dto.VCT) == "" {
		return &ErrorConfigurationInvalidRequest
	}
	if dto.Format == "" {
		dto.Format = DefaultCredentialFormat
	}
	if dto.Format != DefaultCredentialFormat {
		return &ErrorConfigurationUnsupportedFormat
	}
	if dto.ValiditySeconds != nil && *dto.ValiditySeconds <= 0 {
		return &ErrorConfigurationInvalidRequest
	}
	return validateClaims(dto.Claims)
}

// reservedClaimNames are the claim names that cannot be selectively disclosed.
var reservedClaimNames = map[string]bool{
	"iss": true, "nbf": true, "exp": true, "cnf": true, "vct": true, "status": true,
	"_sd": true, "_sd_alg": true, "...": true,
	"sub": true, "iat": true,
}

// validateClaims enforces non-empty, unique and non-reserved claim names. Claim names are compared
// case-sensitively because they become JSON object keys in the issued credential.
func validateClaims(claims []ClaimMapping) *tidcommon.ServiceError {
	seen := make(map[string]bool, len(claims))
	for _, claim := range claims {
		name := strings.TrimSpace(claim.Name)
		if name == "" {
			return &ErrorConfigurationEmptyClaimName
		}
		if reservedClaimNames[name] {
			return ErrorConfigurationReservedClaim.WithParams(map[string]string{"claim": name})
		}
		if seen[name] {
			return ErrorConfigurationDuplicateClaim.WithParams(map[string]string{"claim": name})
		}
		seen[name] = true
	}
	return nil
}
