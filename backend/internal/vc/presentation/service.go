// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// maxProjectIDLength is the length of the PROJECT_ID column. A longer project ID names no project.
const maxProjectIDLength = 36

// PresentationDefinitionServiceInterface manages OpenID4VP presentation
// definitions in the configdb store, which the verifier engine reads on demand.
type PresentationDefinitionServiceInterface interface {
	// GetResourceDependencies reports the presentation definitions an organization unit owns, which block its deletion.
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	CreatePresentationDefinition(ctx context.Context, dto *PresentationDefinitionDTO) (
		*PresentationDefinitionDTO, *tidcommon.ServiceError)
	GetPresentationDefinition(ctx context.Context, id string) (*PresentationDefinitionDTO, *tidcommon.ServiceError)
	GetPresentationDefinitionByHandle(
		ctx context.Context, handle string,
	) (*PresentationDefinitionDTO, *tidcommon.ServiceError)
	ListPresentationDefinitions(ctx context.Context) ([]PresentationDefinitionDTO, *tidcommon.ServiceError)
	ListPresentationDefinitionSummaries(ctx context.Context) ([]PresentationDefinitionList, *tidcommon.ServiceError)
	ListPresentationDefinitionSummariesByOUID(
		ctx context.Context, ouID string,
	) ([]PresentationDefinitionList, *tidcommon.ServiceError)
	// ListPresentationDefinitionSummariesByProject returns summaries of the presentation definitions of a project.
	// A project that does not exist has none.
	ListPresentationDefinitionSummariesByProject(
		ctx context.Context, projectID string,
	) ([]PresentationDefinitionList, *tidcommon.ServiceError)
	// ProjectHasPresentationDefinitions reports whether a presentation definition belongs to the project.
	ProjectHasPresentationDefinitions(ctx context.Context, projectID string) (bool, *tidcommon.ServiceError)
	UpdatePresentationDefinition(ctx context.Context, id string, dto *PresentationDefinitionDTO) (
		*PresentationDefinitionDTO, *tidcommon.ServiceError)
	DeletePresentationDefinition(ctx context.Context, id string) *tidcommon.ServiceError
	IsPresentationDefinitionDeclarative(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
}

// presentationDefinitionDeclarativeService resolves organization-unit handles
// while loading declarative presentation definitions.
type presentationDefinitionDeclarativeService interface {
	ResolvePresentationDefinitionOUHandle(
		ctx context.Context, dto *PresentationDefinitionDTO,
	) *tidcommon.ServiceError
}

type definitionService struct {
	store     definitionStoreInterface
	ouService ou.OrganizationUnitServiceInterface
	authzSvc  sysauthz.SystemAuthorizationServiceInterface
	// projectValidator is nil on a deployment that serves no projects.
	projectValidator projectOUValidator
	// projectRequired is set on a deployment that serves projects, where every definition belongs to one.
	projectRequired bool
	logger          *log.Logger
	uuid            func() (string, error)
	// managed says which presentation definitions a control plane deployed, which stay read only here. Nil
	// where nothing is deployed from one.
	managed managedresource.StoreInterface
}

// projectOUValidator checks that a resource scoped to a project is in an organization unit the project holds.
type projectOUValidator interface {
	ValidateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError
}

// newPresentationDefinitionService builds a presentation-definition service over the given store. A nil
// projectValidator serves no projects, and a project ID is kept unchecked; projectRequired refuses a
// definition created without a project.
func newPresentationDefinitionService(
	store definitionStoreInterface, ouService ou.OrganizationUnitServiceInterface,
	authzSvc sysauthz.SystemAuthorizationServiceInterface, projectValidator projectOUValidator,
	projectRequired bool,
) *definitionService {
	return &definitionService{
		store:            store,
		ouService:        ouService,
		authzSvc:         authzSvc,
		projectValidator: projectValidator,
		projectRequired:  projectRequired,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VPDefinitionService")),
		uuid:             utils.GenerateUUIDv7,
	}
}

// ResolvePresentationDefinitionOUHandle resolves a declarative OU handle to its ID. A declarative definition
// needs a project where one is required, and one in a project without an organization unit is placed in the
// system organization unit.
func (s *definitionService) ResolvePresentationDefinitionOUHandle(
	ctx context.Context, dto *PresentationDefinitionDTO,
) *tidcommon.ServiceError {
	if strings.TrimSpace(dto.ProjectID) == "" && s.projectRequired {
		return &ErrorDefinitionProjectRequired
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
		return &ErrorDefinitionInvalidOU
	}
	resolved, svcErr := s.ouService.GetOrganizationUnitByPath(security.WithRuntimeContext(ctx), dto.OUHandle)
	if svcErr != nil {
		return &ErrorDefinitionInvalidOU
	}
	dto.OUID = resolved.ID
	return s.resolveOU(ctx, dto)
}

// resolveOU verifies the requested organization unit exists.
func (s *definitionService) resolveOU(
	ctx context.Context, dto *PresentationDefinitionDTO,
) *tidcommon.ServiceError {
	if s.ouService == nil {
		return nil
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
	if isDeclarativeModeEnabled() {
		return nil, &ErrorDefinitionDeclarativeModeCreateNotAllowed
	}
	if svcErr := validateDefinition(dto); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rejectExpiredCertificates(dto.TrustedAuthorities); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.resolvePlacement(ctx, dto); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.checkAccess(ctx, security.ActionCreateVC, dto.OUID); svcErr != nil {
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
	} else {
		_, err = s.store.GetPresentationDefinitionByID(ctx, id)
		if err == nil {
			return nil, &ErrorDefinitionIDConflict
		}
		if !errors.Is(err, ErrNotFound) {
			s.logger.Error(ctx, "Failed to check existing definition ID", log.Error(err))
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
	if svcErr := s.checkAccess(ctx, security.ActionReadVC, dto.OUID); svcErr != nil {
		return nil, svcErr
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
	if svcErr := s.checkAccess(ctx, security.ActionReadVC, dto.OUID); svcErr != nil {
		return nil, svcErr
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
	ouIDs, all, svcErr := sysauthz.ResolveListOUs(ctx, s.authzSvc, s.logger, security.ActionListVCs, "")
	if svcErr != nil {
		return nil, svcErr
	}
	defs = sysauthz.FilterByListOUs(defs, ouIDs, all, func(d PresentationDefinitionDTO) string { return d.OUID })
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
	ouIDs, all, svcErr := sysauthz.ResolveListOUs(ctx, s.authzSvc, s.logger, security.ActionListVCs, "")
	if svcErr != nil {
		return nil, svcErr
	}
	if !all && len(ouIDs) == 0 {
		return []PresentationDefinitionList{}, nil
	}
	var summaries []PresentationDefinitionList
	var err error
	if len(ouIDs) == 1 {
		summaries, err = s.store.ListPresentationDefinitionSummariesByOUID(ctx, ouIDs[0])
	} else {
		summaries, err = s.store.ListPresentationDefinitionSummaries(ctx)
	}
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorDefinitionResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list presentation definition summaries", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	summaries = sysauthz.FilterByListOUs(summaries, ouIDs, all,
		func(d PresentationDefinitionList) string { return d.OUID })
	s.populateSummaryOUHandles(ctx, summaries)
	return summaries, nil
}

// ListPresentationDefinitionSummariesByOUID returns summaries of the presentation definitions an organization
// unit owns, with resolved OU handles.
func (s *definitionService) ListPresentationDefinitionSummariesByOUID(
	ctx context.Context, ouID string,
) ([]PresentationDefinitionList, *tidcommon.ServiceError) {
	if ouID == "" {
		return nil, &ErrorDefinitionInvalidRequest
	}
	summaries, err := s.store.ListPresentationDefinitionSummariesByOUID(ctx, ouID)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorDefinitionResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list presentation definition summaries in organization unit",
			log.String("ouID", ouID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	ouIDs, all, svcErr := sysauthz.ResolveListOUs(ctx, s.authzSvc, s.logger, security.ActionListVCs, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	summaries = sysauthz.FilterByListOUs(summaries, ouIDs, all,
		func(d PresentationDefinitionList) string { return d.OUID })
	s.populateSummaryOUHandles(ctx, summaries)
	return summaries, nil
}

// ListPresentationDefinitionSummariesByProject returns summaries of the presentation definitions of a project,
// with resolved OU handles. An empty project ID names no project, so it lists none rather than every definition.
func (s *definitionService) ListPresentationDefinitionSummariesByProject(
	ctx context.Context, projectID string,
) ([]PresentationDefinitionList, *tidcommon.ServiceError) {
	if projectID == "" {
		return []PresentationDefinitionList{}, nil
	}
	summaries, err := s.store.ListPresentationDefinitionSummariesByProject(ctx, projectID)
	if err != nil {
		if errors.Is(err, ErrResultLimitExceededInCompositeMode) {
			return nil, &ErrorDefinitionResultLimitExceeded
		}
		s.logger.Error(ctx, "Failed to list presentation definition summaries in project",
			log.String("projectID", projectID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	s.populateSummaryOUHandles(ctx, summaries)
	return summaries, nil
}

// ProjectHasPresentationDefinitions reports whether a presentation definition belongs to the project.
func (s *definitionService) ProjectHasPresentationDefinitions(
	ctx context.Context, projectID string,
) (bool, *tidcommon.ServiceError) {
	count, err := s.store.CountPresentationDefinitionsByProject(ctx, projectID)
	if err != nil {
		s.logger.Error(ctx, "Failed to check the presentation definitions of a project",
			log.String("projectID", projectID), log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	return count > 0, nil
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
	if strings.TrimSpace(id) == "" || dto == nil {
		return nil, &ErrorDefinitionInvalidRequest
	}
	if svcErr := rejectExpiredCertificates(dto.TrustedAuthorities); svcErr != nil {
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
	if svcErr := s.checkAccess(ctx, security.ActionUpdateVC, existing.OUID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := carryStoredCertificates(dto.TrustedAuthorities, existing.TrustedAuthorities); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := validateDefinition(dto); svcErr != nil {
		return nil, svcErr
	}
	// An update that names no project keeps the definition's project, and one in a project that names no
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
	// The project is checked again only when the definition moves to another project or organization unit.
	if dto.ProjectID != "" && (dto.ProjectID != existing.ProjectID || dto.OUID != existing.OUID) {
		if svcErr := s.validateProjectOU(ctx, dto.ProjectID, dto.OUID); svcErr != nil {
			return nil, svcErr
		}
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
	// Moving the presentation definition places it in another organization unit, which the caller must be
	// allowed to act for too.
	if dto.OUID != existing.OUID {
		if svcErr := s.checkAccess(ctx, security.ActionUpdateVC, dto.OUID); svcErr != nil {
			return nil, svcErr
		}
	}
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
	existing, err := s.store.GetPresentationDefinitionByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil // idempotent
		}
		s.logger.Error(ctx, "Failed to load presentation definition", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if svcErr := s.checkAccess(ctx, security.ActionDeleteVC, existing.OUID); svcErr != nil {
		return svcErr
	}
	if svcErr := s.refuseManaged(ctx, id); svcErr != nil {
		return svcErr
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

// resolvePlacement sets the organization unit and checks the project of a new presentation definition. A
// deployment that requires a project refuses a definition without one. A definition in a project may leave the
// organization unit out to be placed in the system organization unit, and one it names has to belong to the
// project.
func (s *definitionService) resolvePlacement(
	ctx context.Context, dto *PresentationDefinitionDTO,
) *tidcommon.ServiceError {
	if dto.ProjectID == "" {
		if s.projectRequired {
			return &ErrorDefinitionProjectRequired
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

// placeInSystemOU places a presentation definition in the system organization unit. Without an OU service,
// as in tests, the organization unit is left as given.
func (s *definitionService) placeInSystemOU(
	ctx context.Context, dto *PresentationDefinitionDTO,
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

// validateProjectOU checks that a presentation definition's organization unit belongs to its project. It returns
// ErrorDefinitionInvalidProject when no such project exists, and ErrorDefinitionOUNotInProject when the
// organization unit is outside the project.
//
// A deployment that serves no projects, such as a Data Plane, cannot check the project. It keeps the project ID
// as given so that imported configuration retains it, but refuses one longer than the column holds.
func (s *definitionService) validateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError {
	if utf8.RuneCountInString(projectID) > maxProjectIDLength {
		return &ErrorDefinitionInvalidProject
	}
	if s.projectValidator == nil {
		return nil
	}
	svcErr := s.projectValidator.ValidateProjectOU(ctx, projectID, ouID)
	switch {
	case svcErr == nil:
		return nil
	case svcErr.Code == projectscope.ProjectNotFoundErrorCode:
		return &ErrorDefinitionInvalidProject
	case svcErr.Code == projectscope.OUNotInProjectErrorCode:
		return &ErrorDefinitionOUNotInProject
	default:
		s.logger.Error(ctx, "Failed to check the project of a presentation definition",
			log.String("projectID", projectID), log.String("code", svcErr.Code))
		return &tidcommon.InternalServerError
	}
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
	if dto.SubjectClaim != "" && !slices.Contains(dto.MandatoryClaims, dto.SubjectClaim) {
		return &ErrorDefinitionInvalidRequest
	}
	if svcErr := validateClaimNames(dto.RequestedClaims); svcErr != nil {
		return svcErr
	}
	if svcErr := validateClaimNames(slices.Concat(dto.MandatoryClaims, dto.OptionalClaims)); svcErr != nil {
		return svcErr
	}
	return validateTrustedAuthorities(dto)
}

// validateClaimNames enforces non-empty, unique claim names within one requested list. The
// mandatory and optional lists are checked as a single list, since a name may not appear as both:
// the two carry contradictory disclosure requirements. RequestedClaims is checked on its own,
// because it is the full set those two partition and so legitimately repeats their names.
func validateClaimNames(claims []string) *tidcommon.ServiceError {
	seen := make(map[string]bool, len(claims))
	for _, claim := range claims {
		name := strings.TrimSpace(claim)
		if name == "" {
			return &ErrorDefinitionEmptyClaimName
		}
		if seen[name] {
			return ErrorDefinitionDuplicateClaim.WithParams(map[string]string{"claim": name})
		}
		seen[name] = true
	}
	return nil
}

// validateTrustedAuthorities enforces named, unique trusted authorities that each carry one parsable
// certificate, within the supported limit, and requires at least one when issuer trust is enforced.
func validateTrustedAuthorities(dto *PresentationDefinitionDTO) *tidcommon.ServiceError {
	authorities := dto.TrustedAuthorities
	if len(authorities) > maxTrustedAuthorities {
		return ErrorDefinitionTooManyTrustedAuthorities.WithParams(
			map[string]string{"max": strconv.Itoa(maxTrustedAuthorities)})
	}
	if len(authorities) == 0 {
		if dto.EnforceTrustedIssuer != nil && *dto.EnforceTrustedIssuer {
			return &ErrorDefinitionTrustedIssuerWithoutAuthorities
		}
		return nil
	}
	names := make(map[string]bool, len(authorities))
	certificates := make(map[string]bool, len(authorities))
	for _, authority := range authorities {
		name := strings.TrimSpace(authority.Name)
		if name == "" {
			return &ErrorDefinitionEmptyTrustedAuthorityName
		}
		if names[name] {
			return ErrorDefinitionDuplicateTrustedAuthority.WithParams(map[string]string{"name": name})
		}
		names[name] = true
		cert, err := ParseTrustedAuthorityCertificate(authority.Certificate)
		if err != nil {
			return ErrorDefinitionInvalidTrustedAuthorityCertificate.WithParams(map[string]string{"name": name})
		}
		if certificates[string(cert.Raw)] {
			return ErrorDefinitionDuplicateTrustedAuthority.WithParams(map[string]string{"name": name})
		}
		certificates[string(cert.Raw)] = true
	}
	return nil
}

// rejectExpiredCertificates rejects an expired certificate among the trusted authorities a request
// supplies. Stored and declarative certificates are not checked here; an expired one fails verification.
func rejectExpiredCertificates(authorities []TrustedAuthority) *tidcommon.ServiceError {
	now := time.Now()
	for _, authority := range authorities {
		if authority.Certificate == "" {
			continue
		}
		cert, err := ParseTrustedAuthorityCertificate(authority.Certificate)
		if err != nil {
			return ErrorDefinitionInvalidTrustedAuthorityCertificate.WithParams(
				map[string]string{"name": authority.Name})
		}
		if now.After(cert.NotAfter) {
			return ErrorDefinitionExpiredTrustedAuthorityCertificate.WithParams(
				map[string]string{"name": authority.Name})
		}
	}
	return nil
}

// carryStoredCertificates fills in the certificate of each requested trusted authority that is named
// without one, from the stored authority of the same name. A name with no stored certificate is rejected.
func carryStoredCertificates(requested, stored []TrustedAuthority) *tidcommon.ServiceError {
	storedByName := make(map[string]string, len(stored))
	for _, authority := range stored {
		storedByName[authority.Name] = authority.Certificate
	}
	for i, authority := range requested {
		if authority.Certificate != "" {
			continue
		}
		certificate, ok := storedByName[authority.Name]
		if !ok {
			return ErrorDefinitionInvalidTrustedAuthorityCertificate.WithParams(
				map[string]string{"name": authority.Name})
		}
		requested[i].Certificate = certificate
	}
	return nil
}

// checkAccess checks the caller may perform the action on a presentation definition of the organization unit.
func (s *definitionService) checkAccess(ctx context.Context, action security.Action,
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

// refuseManaged refuses a change to a presentation definition a control plane deployed, unless the change is the
// managed import's: only a deploy from the control plane changes or removes it.
func (s *definitionService) refuseManaged(ctx context.Context, id string) *tidcommon.ServiceError {
	managed, err := managedresource.IsReadOnly(ctx, s.managed, id, managedresource.TypePresentationDefinition)
	if err != nil {
		s.logger.Error(ctx, "Failed to read whether a control plane deployed the presentation definition",
			log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if managed {
		return &ErrorManagedDefinition
	}
	return nil
}

// setManagedStore gives the service the record of what a control plane deployed.
func setManagedStore(service any, managed managedresource.StoreInterface) {
	if svc, ok := service.(*definitionService); ok {
		svc.managed = managed
	}
}

// GetResourceDependencies implements resourcedependency.Provider. It reports the presentation definitions an
// organization unit owns, which block its deletion. Other resource types have no dependencies here.
func (s *definitionService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	if resourceType != resourcedependency.ResourceTypeOU {
		return []resourcedependency.ResourceDependency{}, nil
	}
	summaries, err := s.store.ListPresentationDefinitionSummariesByOUID(ctx, id)
	if err != nil {
		return nil, err
	}
	deps := make([]resourcedependency.ResourceDependency, 0, len(summaries))
	for _, c := range summaries {
		deps = append(deps, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeVPDefinition,
			ID:               c.ID,
			DisplayName:      c.Handle,
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}
	return deps, nil
}
