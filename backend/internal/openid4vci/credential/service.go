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

package credential

import (
	"context"
	"errors"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
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
	return summaries, nil
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
	return nil
}
