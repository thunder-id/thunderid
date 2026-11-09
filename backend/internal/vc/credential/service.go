// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// maxProjectIDLength is the length of the PROJECT_ID column. A longer project ID names no project.
const maxProjectIDLength = 36

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
	ListCredentialConfigurationSummariesByOUID(
		ctx context.Context, ouID string,
	) ([]CredentialConfigurationList, *tidcommon.ServiceError)
	// ListCredentialConfigurationSummariesByProject returns summaries of the credential configurations of a
	// project. A project that does not exist has none.
	ListCredentialConfigurationSummariesByProject(
		ctx context.Context, projectID string,
	) ([]CredentialConfigurationList, *tidcommon.ServiceError)
	// ProjectHasCredentialConfigurations reports whether a credential configuration belongs to the project.
	ProjectHasCredentialConfigurations(ctx context.Context, projectID string) (bool, *tidcommon.ServiceError)
	UpdateCredentialConfiguration(ctx context.Context, id string, dto *CredentialConfigurationDTO) (
		*CredentialConfigurationDTO, *tidcommon.ServiceError)
	DeleteCredentialConfiguration(ctx context.Context, id string) *tidcommon.ServiceError
	IsCredentialConfigurationDeclarative(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
}

// credentialConfigurationDeclarativeService resolves organization-unit handles
// while loading declarative credential configurations.
type credentialConfigurationDeclarativeService interface {
	ResolveCredentialConfigurationOUHandle(
		ctx context.Context, dto *CredentialConfigurationDTO,
	) *tidcommon.ServiceError
}

type configurationService struct {
	store     credentialStoreInterface
	ouService ou.OrganizationUnitServiceInterface
	authzSvc  sysauthz.SystemAuthorizationServiceInterface
	// projectValidator is nil on a deployment that serves no projects.
	projectValidator projectOUValidator
	// projectRequired is set on a deployment that serves projects, where every configuration belongs to one.
	projectRequired bool
	logger          *log.Logger
	uuid            func() (string, error)
}

// projectOUValidator checks that a resource scoped to a project is in an organization unit the project holds.
type projectOUValidator interface {
	ValidateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError
}

// newCredentialConfigurationService builds a credential-configuration service over the given store. A nil
// projectValidator serves no projects, and a project ID is kept unchecked; projectRequired refuses a
// configuration created without a project.
func newCredentialConfigurationService(
	store credentialStoreInterface, ouService ou.OrganizationUnitServiceInterface,
	authzSvc sysauthz.SystemAuthorizationServiceInterface, projectValidator projectOUValidator,
	projectRequired bool,
) *configurationService {
	return &configurationService{
		store:            store,
		ouService:        ouService,
		authzSvc:         authzSvc,
		projectValidator: projectValidator,
		projectRequired:  projectRequired,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "OpenID4VCIConfigurationService")),
		uuid: utils.GenerateUUIDv7,
	}
}

// ResolveCredentialConfigurationOUHandle resolves a declarative OU handle to its ID. A declarative configuration
// needs a project where one is required, and one in a project without an organization unit is placed in the
// system organization unit.
func (s *configurationService) ResolveCredentialConfigurationOUHandle(
	ctx context.Context, dto *CredentialConfigurationDTO,
) *tidcommon.ServiceError {
	if strings.TrimSpace(dto.ProjectID) == "" && s.projectRequired {
		return &ErrorConfigurationProjectRequired
	}
	if dto.ProjectID != "" && dto.OUID == "" && strings.TrimSpace(dto.OUHandle) == "" {
		return s.placeInSystemOU(ctx, dto)
	}
	if dto.OUID != "" {
		return s.resolveOU(ctx, dto)
	}
	if strings.TrimSpace(dto.OUHandle) == "" {
		return nil
	}
	if s.ouService == nil {
		return &ErrorConfigurationInvalidOU
	}
	resolved, svcErr := s.ouService.GetOrganizationUnitByPath(security.WithRuntimeContext(ctx), dto.OUHandle)
	if svcErr != nil {
		return &ErrorConfigurationInvalidOU
	}
	dto.OUID = resolved.ID
	return s.resolveOU(ctx, dto)
}

// resolveOU verifies the requested organization unit exists.
func (s *configurationService) resolveOU(
	ctx context.Context, dto *CredentialConfigurationDTO,
) *tidcommon.ServiceError {
	if s.ouService == nil {
		return nil
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
	if isDeclarativeModeEnabled() {
		return nil, &ErrorConfigurationDeclarativeModeCreateNotAllowed
	}
	if svcErr := validateConfiguration(dto); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.resolvePlacement(ctx, dto); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkAccess(ctx, security.ActionCreateVC, dto.OUID); svcErr != nil {
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
	} else {
		_, err = s.store.GetCredentialConfigurationByID(ctx, id)
		if err == nil {
			return nil, &ErrorConfigurationIDConflict
		}
		if !errors.Is(err, ErrNotFound) {
			s.logger.Error(ctx, "Failed to check existing configuration ID", log.Error(err))
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
	if svcErr := s.checkAccess(ctx, security.ActionReadVC, dto.OUID); svcErr != nil {
		return nil, svcErr
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
	if svcErr := s.checkAccess(ctx, security.ActionReadVC, dto.OUID); svcErr != nil {
		return nil, svcErr
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
	ouIDs, all, svcErr := sysauthz.ResolveListOUs(ctx, s.authzSvc, s.logger, security.ActionListVCs, "")
	if svcErr != nil {
		return nil, svcErr
	}
	configs = sysauthz.FilterByListOUs(configs, ouIDs, all,
		func(d CredentialConfigurationDTO) string { return d.OUID })
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
	ouIDs, all, svcErr := sysauthz.ResolveListOUs(ctx, s.authzSvc, s.logger, security.ActionListVCs, "")
	if svcErr != nil {
		return nil, svcErr
	}
	if !all && len(ouIDs) == 0 {
		return []CredentialConfigurationList{}, nil
	}
	var summaries []CredentialConfigurationList
	var err error
	if len(ouIDs) == 1 {
		summaries, err = s.store.ListCredentialConfigurationSummariesByOUID(ctx, ouIDs[0])
	} else {
		summaries, err = s.store.ListCredentialConfigurationSummaries(ctx)
	}
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorConfigurationResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list credential configuration summaries", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	summaries = sysauthz.FilterByListOUs(summaries, ouIDs, all,
		func(d CredentialConfigurationList) string { return d.OUID })
	s.populateSummaryOUHandles(ctx, summaries)
	return summaries, nil
}

// ListCredentialConfigurationSummariesByOUID returns summaries of the credential configurations an organization
// unit owns, with resolved OU handles.
func (s *configurationService) ListCredentialConfigurationSummariesByOUID(
	ctx context.Context, ouID string,
) ([]CredentialConfigurationList, *tidcommon.ServiceError) {
	if ouID == "" {
		return nil, &ErrorConfigurationInvalidRequest
	}
	summaries, err := s.store.ListCredentialConfigurationSummariesByOUID(ctx, ouID)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorConfigurationResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list credential configuration summaries in organization unit",
			log.String("ouID", ouID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	ouIDs, all, svcErr := sysauthz.ResolveListOUs(ctx, s.authzSvc, s.logger, security.ActionListVCs, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	summaries = sysauthz.FilterByListOUs(summaries, ouIDs, all,
		func(d CredentialConfigurationList) string { return d.OUID })
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

// ListCredentialConfigurationSummariesByProject returns summaries of the credential configurations of a project,
// with resolved OU handles. An empty project ID names no project, so it lists none rather than every
// configuration.
func (s *configurationService) ListCredentialConfigurationSummariesByProject(
	ctx context.Context, projectID string,
) ([]CredentialConfigurationList, *tidcommon.ServiceError) {
	if projectID == "" {
		return []CredentialConfigurationList{}, nil
	}
	summaries, err := s.store.ListCredentialConfigurationSummariesByProject(ctx, projectID)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorConfigurationResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list credential configuration summaries in project",
			log.String("projectID", projectID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	s.populateSummaryOUHandles(ctx, summaries)
	return summaries, nil
}

// ProjectHasCredentialConfigurations reports whether a credential configuration belongs to the project.
func (s *configurationService) ProjectHasCredentialConfigurations(
	ctx context.Context, projectID string,
) (bool, *tidcommon.ServiceError) {
	count, err := s.store.CountCredentialConfigurationsByProject(ctx, projectID)
	if err != nil {
		s.logger.Error(ctx, "Failed to check the credential configurations of a project",
			log.String("projectID", projectID), log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	return count > 0, nil
}

// resolvePlacement sets the organization unit and checks the project of a new credential configuration. A
// deployment that requires a project refuses a configuration without one. A configuration in a project may
// leave the organization unit out to be placed in the system organization unit, and one it names has to belong
// to the project.
func (s *configurationService) resolvePlacement(
	ctx context.Context, dto *CredentialConfigurationDTO,
) *tidcommon.ServiceError {
	if dto.ProjectID == "" {
		if s.projectRequired {
			return &ErrorConfigurationProjectRequired
		}
		return s.resolveOU(ctx, dto)
	}
	if strings.TrimSpace(dto.OUID) == "" {
		if svcErr := s.placeInSystemOU(ctx, dto); svcErr != nil {
			return svcErr
		}
	} else if svcErr := s.resolveOU(ctx, dto); svcErr != nil {
		return svcErr
	}
	return s.validateProjectOU(ctx, dto.ProjectID, dto.OUID)
}

// placeInSystemOU places a credential configuration in the system organization unit. Without an OU service,
// as in tests, the organization unit is left as given.
func (s *configurationService) placeInSystemOU(
	ctx context.Context, dto *CredentialConfigurationDTO,
) *tidcommon.ServiceError {
	if s.ouService == nil {
		return nil
	}
	systemOUID, svcErr := s.ouService.GetSystemOUID(ctx)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to get the system organization unit", log.String("code", svcErr.Code))
		return &tidcommon.InternalServerError
	}
	dto.OUID = systemOUID
	return nil
}

// validateProjectOU checks that a credential configuration's organization unit belongs to its project. It returns
// ErrorConfigurationInvalidProject when no such project exists, and ErrorConfigurationOUNotInProject when the
// organization unit is outside the project.
//
// A deployment that serves no projects, such as a Data Plane, cannot check the project. It keeps the project ID
// as given so that imported configuration retains it, but refuses one longer than the column holds.
func (s *configurationService) validateProjectOU(
	ctx context.Context, projectID, ouID string,
) *tidcommon.ServiceError {
	if utf8.RuneCountInString(projectID) > maxProjectIDLength {
		return &ErrorConfigurationInvalidProject
	}
	if s.projectValidator == nil {
		return nil
	}
	svcErr := s.projectValidator.ValidateProjectOU(ctx, projectID, ouID)
	switch {
	case svcErr == nil:
		return nil
	case svcErr.Code == projectscope.ProjectNotFoundErrorCode:
		return &ErrorConfigurationInvalidProject
	case svcErr.Code == projectscope.OUNotInProjectErrorCode:
		return &ErrorConfigurationOUNotInProject
	default:
		s.logger.Error(ctx, "Failed to check the project of a credential configuration",
			log.String("projectID", projectID), log.String("code", svcErr.Code))
		return &tidcommon.InternalServerError
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

	existing, err := s.store.GetCredentialConfigurationByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, &ErrorConfigurationNotFound
		}
		s.logger.Error(ctx, "Failed to load credential configuration", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if svcErr := s.checkAccess(ctx, security.ActionUpdateVC, existing.OUID); svcErr != nil {
		return nil, svcErr
	}

	// An update that names no project keeps the configuration's project, and one in a project that names no
	// organization unit keeps its organization unit.
	if dto.ProjectID == "" {
		dto.ProjectID = existing.ProjectID
	}
	if dto.ProjectID != "" && strings.TrimSpace(dto.OUID) == "" {
		dto.OUID = existing.OUID
	}
	if svcErr := s.resolveOU(ctx, dto); svcErr != nil {
		return nil, svcErr
	}
	// The project is checked again only when the configuration moves to another project or organization unit.
	if dto.ProjectID != "" && (dto.ProjectID != existing.ProjectID || dto.OUID != existing.OUID) {
		if svcErr := s.validateProjectOU(ctx, dto.ProjectID, dto.OUID); svcErr != nil {
			return nil, svcErr
		}
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
	// Moving the credential configuration places it in another organization unit, which the caller must be
	// allowed to act for too.
	if dto.OUID != existing.OUID {
		if svcErr := s.checkAccess(ctx, security.ActionUpdateVC, dto.OUID); svcErr != nil {
			return nil, svcErr
		}
	}
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
	existing, err := s.store.GetCredentialConfigurationByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil // idempotent
		}
		s.logger.Error(ctx, "Failed to load credential configuration", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if svcErr := s.checkAccess(ctx, security.ActionDeleteVC, existing.OUID); svcErr != nil {
		return svcErr
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

// checkAccess checks the caller may perform the action on a credential configuration of the organization unit.
func (s *configurationService) checkAccess(ctx context.Context, action security.Action,
	ouID string) *tidcommon.ServiceError {
	allowed, svcErr := s.authzSvc.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeVerifiableCredential, OUID: ouID})
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to check authorization for action",
			log.String("action", string(action)), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}
