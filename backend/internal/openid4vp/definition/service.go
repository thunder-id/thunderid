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
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// PresentationDefinitionServiceInterface manages OpenID4VP presentation
// definitions in the configdb store, which the verifier engine reads on demand.
type PresentationDefinitionServiceInterface interface {
	CreatePresentationDefinition(ctx context.Context, dto *PresentationDefinitionDTO) (
		*PresentationDefinitionDTO, *tidcommon.ServiceError)
	GetPresentationDefinition(ctx context.Context, id string) (*PresentationDefinitionDTO, *tidcommon.ServiceError)
	GetPresentationDefinitionByHandle(
		ctx context.Context, handle string,
	) (*PresentationDefinitionDTO, *tidcommon.ServiceError)
	ListPresentationDefinitions(ctx context.Context) ([]PresentationDefinitionDTO, *tidcommon.ServiceError)
	ListPresentationDefinitionSummaries(ctx context.Context) ([]PresentationDefinitionList, *tidcommon.ServiceError)
	UpdatePresentationDefinition(ctx context.Context, id string, dto *PresentationDefinitionDTO) (
		*PresentationDefinitionDTO, *tidcommon.ServiceError)
	DeletePresentationDefinition(ctx context.Context, id string) *tidcommon.ServiceError
	IsPresentationDefinitionDeclarative(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
}

type definitionService struct {
	store     definitionStoreInterface
	ouService ou.OrganizationUnitServiceInterface
	logger    *log.Logger
	uuid      func() (string, error)
}

// newPresentationDefinitionService builds a presentation-definition service over the given store.
func newPresentationDefinitionService(
	store definitionStoreInterface, ouService ou.OrganizationUnitServiceInterface,
) PresentationDefinitionServiceInterface {
	return &definitionService{
		store:     store,
		ouService: ouService,
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VPDefinitionService")),
		uuid:      utils.GenerateUUIDv7,
	}
}

// resolveOU resolves ouHandle to ouId when needed and verifies the OU exists.
func (s *definitionService) resolveOU(
	ctx context.Context, dto *PresentationDefinitionDTO,
) *tidcommon.ServiceError {
	if s.ouService == nil {
		return nil
	}
	if dto.OUID == "" && strings.TrimSpace(dto.OUHandle) != "" {
		resolved, svcErr := s.ouService.GetOrganizationUnitByPath(ctx, dto.OUHandle)
		if svcErr != nil {
			return &ErrorDefinitionInvalidOU
		}
		dto.OUID = resolved.ID
	}
	if strings.TrimSpace(dto.OUID) == "" {
		return &ErrorDefinitionInvalidOU
	}
	exists, svcErr := s.ouService.IsOrganizationUnitExists(ctx, dto.OUID)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to verify organization unit", log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !exists {
		return &ErrorDefinitionInvalidOU
	}
	return nil
}

// populateOUHandle sets each DTO's owning OU handle for display.
func (s *definitionService) populateOUHandle(ctx context.Context, dtos ...*PresentationDefinitionDTO) {
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
		s.logger.Warn(ctx, "Failed to resolve OU handles for presentation definitions", log.Any("error", svcErr))
		return
	}
	for _, dto := range dtos {
		if h, ok := handles[dto.OUID]; ok {
			dto.OUHandle = h
		}
	}
}

// CreatePresentationDefinition validates, assigns an ID, and persists a new presentation definition.
func (s *definitionService) CreatePresentationDefinition(
	ctx context.Context, dto *PresentationDefinitionDTO,
) (*PresentationDefinitionDTO, *tidcommon.ServiceError) {
	if svcErr := validateDefinition(dto); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.resolveOU(ctx, dto); svcErr != nil {
		return nil, svcErr
	}

	existing, err := s.store.GetPresentationDefinitionByHandle(ctx, dto.Handle)
	if err != nil && !errors.Is(err, ErrNotFound) {
		s.logger.Error(ctx, "Failed to check existing definition", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if existing != nil {
		return nil, &ErrorDefinitionAlreadyExists
	}

	id := dto.ID
	if id == "" {
		var genErr error
		id, genErr = s.uuid()
		if genErr != nil {
			s.logger.Error(ctx, "Failed to generate definition ID", log.Error(genErr))
			return nil, &tidcommon.InternalServerError
		}
	}
	dto.ID = id

	if err := s.store.CreatePresentationDefinition(ctx, *dto); err != nil {
		s.logger.Error(ctx, "Failed to create presentation definition", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return dto, nil
}

// GetPresentationDefinition returns the presentation definition with the given ID and resolves its OU handle.
func (s *definitionService) GetPresentationDefinition(
	ctx context.Context, id string,
) (*PresentationDefinitionDTO, *tidcommon.ServiceError) {
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorDefinitionInvalidRequest
	}
	dto, err := s.store.GetPresentationDefinitionByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &ErrorDefinitionNotFound
		}
		s.logger.Error(ctx, "Failed to get presentation definition", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	s.populateOUHandle(ctx, dto)
	return dto, nil
}

// GetPresentationDefinitionByHandle returns the presentation definition matching the given handle.
func (s *definitionService) GetPresentationDefinitionByHandle(
	ctx context.Context, handle string,
) (*PresentationDefinitionDTO, *tidcommon.ServiceError) {
	dto, err := s.store.GetPresentationDefinitionByHandle(ctx, handle)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &ErrorDefinitionNotFound
		}
		s.logger.Error(ctx, "Failed to get presentation definition by handle", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return dto, nil
}

// ListPresentationDefinitions returns all presentation definitions with resolved OU handles.
func (s *definitionService) ListPresentationDefinitions(
	ctx context.Context,
) ([]PresentationDefinitionDTO, *tidcommon.ServiceError) {
	defs, err := s.store.ListPresentationDefinitions(ctx)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorDefinitionResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list presentation definitions", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	ptrs := make([]*PresentationDefinitionDTO, len(defs))
	for i := range defs {
		ptrs[i] = &defs[i]
	}
	s.populateOUHandle(ctx, ptrs...)
	return defs, nil
}

// ListPresentationDefinitionSummaries returns summaries of all presentation definitions with resolved OU handles.
func (s *definitionService) ListPresentationDefinitionSummaries(
	ctx context.Context,
) ([]PresentationDefinitionList, *tidcommon.ServiceError) {
	summaries, err := s.store.ListPresentationDefinitionSummaries(ctx)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorDefinitionResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list presentation definition summaries", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	s.populateSummaryOUHandles(ctx, summaries)
	return summaries, nil
}

// populateSummaryOUHandles resolves each summary's owning OU handle for display.
func (s *definitionService) populateSummaryOUHandles(ctx context.Context, summaries []PresentationDefinitionList) {
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
		s.logger.Warn(ctx, "Failed to resolve OU handles for presentation definition summaries",
			log.Any("error", svcErr))
		return
	}
	for i := range summaries {
		if h, ok := handles[summaries[i].OUID]; ok {
			summaries[i].OUHandle = h
		}
	}
}

// UpdatePresentationDefinition validates and persists changes to the presentation definition with the given ID.
func (s *definitionService) UpdatePresentationDefinition(
	ctx context.Context, id string, dto *PresentationDefinitionDTO,
) (*PresentationDefinitionDTO, *tidcommon.ServiceError) {
	if strings.TrimSpace(id) == "" {
		return nil, &ErrorDefinitionInvalidRequest
	}
	if svcErr := validateDefinition(dto); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.resolveOU(ctx, dto); svcErr != nil {
		return nil, svcErr
	}

	existing, err := s.store.GetPresentationDefinitionByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &ErrorDefinitionNotFound
		}
		s.logger.Error(ctx, "Failed to load presentation definition", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if existing.Handle != dto.Handle {
		clash, err := s.store.GetPresentationDefinitionByHandle(ctx, dto.Handle)
		if err != nil && !errors.Is(err, ErrNotFound) {
			s.logger.Error(ctx, "Failed to check handle uniqueness", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if clash != nil {
			return nil, &ErrorDefinitionAlreadyExists
		}
	}

	dto.ID = id
	if err := s.store.UpdatePresentationDefinition(ctx, *dto); err != nil {
		if errors.Is(err, ErrDefinitionIsImmutable) {
			return nil, &ErrorDefinitionImmutable
		}
		s.logger.Error(ctx, "Failed to update presentation definition", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return dto, nil
}

// DeletePresentationDefinition deletes the presentation definition with the given ID.
func (s *definitionService) DeletePresentationDefinition(ctx context.Context, id string) *tidcommon.ServiceError {
	if strings.TrimSpace(id) == "" {
		return &ErrorDefinitionInvalidRequest
	}
	if _, err := s.store.GetPresentationDefinitionByID(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil // idempotent
		}
		s.logger.Error(ctx, "Failed to load presentation definition", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if err := s.store.DeletePresentationDefinition(ctx, id); err != nil {
		if errors.Is(err, ErrDefinitionIsImmutable) {
			return &ErrorDefinitionImmutable
		}
		s.logger.Error(ctx, "Failed to delete presentation definition", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// IsPresentationDefinitionDeclarative reports whether the presentation definition with the given ID is file-based.
func (s *definitionService) IsPresentationDefinitionDeclarative(
	ctx context.Context, id string,
) (bool, *tidcommon.ServiceError) {
	isDeclarative, err := s.store.IsPresentationDefinitionDeclarative(ctx, id)
	if err != nil {
		s.logger.Error(ctx, "Failed to check if presentation definition is declarative", log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	return isDeclarative, nil
}

// validateDefinition enforces the required fields of a presentation definition.
func validateDefinition(dto *PresentationDefinitionDTO) *tidcommon.ServiceError {
	if dto == nil || strings.TrimSpace(dto.Handle) == "" || strings.TrimSpace(dto.VCT) == "" {
		return &ErrorDefinitionInvalidRequest
	}
	if dto.Format == "" {
		dto.Format = DefaultCredentialFormat
	}
	if dto.Format != DefaultCredentialFormat {
		return &ErrorDefinitionUnsupportedFormat
	}
	return nil
}
