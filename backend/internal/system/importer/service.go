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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"

	agentmodel "github.com/thunder-id/thunderid/internal/agent/model"
	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/user"
)

type applicationAdapter interface {
	CreateApplication(ctx context.Context, app *appmodel.ApplicationDTO) (
		*appmodel.ApplicationDTO,
		*serviceerror.ServiceError,
	)
	GetApplication(ctx context.Context, appID string) (*appmodel.Application, *serviceerror.ServiceError)
	UpdateApplication(ctx context.Context, appID string, app *appmodel.ApplicationDTO) (
		*appmodel.ApplicationDTO,
		*serviceerror.ServiceError,
	)
}

type idpAdapter interface {
	CreateIdentityProvider(ctx context.Context, idp *idp.IDPDTO) (*idp.IDPDTO, *serviceerror.ServiceError)
	GetIdentityProvider(ctx context.Context, idpID string) (*idp.IDPDTO, *serviceerror.ServiceError)
	GetIdentityProviderByName(ctx context.Context, idpName string) (*idp.IDPDTO, *serviceerror.ServiceError)
	UpdateIdentityProvider(ctx context.Context, idpID string, idpDTO *idp.IDPDTO) (
		*idp.IDPDTO,
		*serviceerror.ServiceError,
	)
}

type flowAdapter interface {
	CreateFlow(ctx context.Context, flowDef *flowmgt.FlowDefinition) (
		*flowmgt.CompleteFlowDefinition,
		*serviceerror.ServiceError,
	)
	GetFlow(ctx context.Context, flowID string) (*flowmgt.CompleteFlowDefinition, *serviceerror.ServiceError)
	GetFlowByHandle(ctx context.Context, handle string, flowType common.FlowType) (*flowmgt.CompleteFlowDefinition,
		*serviceerror.ServiceError)
	UpdateFlow(ctx context.Context, flowID string, flowDef *flowmgt.FlowDefinition) (*flowmgt.CompleteFlowDefinition,
		*serviceerror.ServiceError)
}

type ouAdapter interface {
	CreateOrganizationUnit(ctx context.Context, request ou.OrganizationUnitRequestWithID) (
		ou.OrganizationUnit,
		*serviceerror.ServiceError,
	)
	GetOrganizationUnit(ctx context.Context, id string) (ou.OrganizationUnit, *serviceerror.ServiceError)
	GetOrganizationUnitByPath(ctx context.Context, handlePath string) (ou.OrganizationUnit, *serviceerror.ServiceError)
	UpdateOrganizationUnit(ctx context.Context, id string, request ou.OrganizationUnitRequestWithID) (
		ou.OrganizationUnit,
		*serviceerror.ServiceError)
}

type entityTypeAdapter interface {
	CreateEntityType(ctx context.Context, category entitytype.TypeCategory,
		request entitytype.CreateEntityTypeRequestWithID) (*entitytype.EntityType,
		*serviceerror.ServiceError)
	GetEntityType(ctx context.Context, category entitytype.TypeCategory, schemaID string,
		includeDisplay bool) (*entitytype.EntityType,
		*serviceerror.ServiceError)
	GetEntityTypeByName(ctx context.Context, category entitytype.TypeCategory,
		schemaName string) (*entitytype.EntityType, *serviceerror.ServiceError)
	UpdateEntityType(ctx context.Context, category entitytype.TypeCategory, schemaID string,
		request entitytype.UpdateEntityTypeRequest) (
		*entitytype.EntityType,
		*serviceerror.ServiceError)
}

type roleAdapter interface {
	CreateRole(ctx context.Context, role role.RoleCreationDetail) (*role.RoleWithPermissionsAndAssignments,
		*serviceerror.ServiceError)
	GetRoleWithPermissions(ctx context.Context, id string) (*role.RoleWithPermissions, *serviceerror.ServiceError)
	UpdateRoleWithPermissions(ctx context.Context, id string, role role.RoleUpdateDetail) (*role.RoleWithPermissions,
		*serviceerror.ServiceError)
}

type roleAssignmentAdapter interface {
	AddAssignments(ctx context.Context, id string, assignments []role.RoleAssignment) *serviceerror.ServiceError
}

type groupAdapter interface {
	CreateGroup(ctx context.Context, request group.CreateGroupRequest) (*group.Group, *serviceerror.ServiceError)
	GetGroup(ctx context.Context, groupID string, includeDisplay bool) (*group.Group, *serviceerror.ServiceError)
	UpdateGroup(ctx context.Context, groupID string, request group.UpdateGroupRequest) (
		*group.Group, *serviceerror.ServiceError)
	AddGroupMembers(ctx context.Context, groupID string, members []group.Member) (
		*group.Group, *serviceerror.ServiceError)
}

type resourceServerAdapter interface {
	CreateResourceServer(ctx context.Context, rs resource.ResourceServer) (*resource.ResourceServer,
		*serviceerror.ServiceError)
	GetResourceServer(ctx context.Context, id string) (*resource.ResourceServer, *serviceerror.ServiceError)
	UpdateResourceServer(ctx context.Context, id string, rs resource.ResourceServer) (*resource.ResourceServer,
		*serviceerror.ServiceError)
	CreateResource(ctx context.Context, resourceServerID string, res resource.Resource) (
		*resource.Resource, *serviceerror.ServiceError)
	GetResourceList(ctx context.Context, resourceServerID string, parentID *string, limit, offset int) (
		*resource.ResourceList, *serviceerror.ServiceError)
	CreateAction(ctx context.Context, resourceServerID string, resourceID *string, action resource.Action) (
		*resource.Action, *serviceerror.ServiceError)
}

type themeAdapter interface {
	CreateTheme(ctx context.Context,
		theme thememgt.CreateThemeRequestWithID) (*thememgt.Theme, *serviceerror.ServiceError)
	GetTheme(ctx context.Context, id string) (*thememgt.Theme, *serviceerror.ServiceError)
	UpdateTheme(ctx context.Context,
		id string, theme thememgt.UpdateThemeRequest) (*thememgt.Theme, *serviceerror.ServiceError)
}

type layoutAdapter interface {
	CreateLayout(ctx context.Context,
		layout layoutmgt.CreateLayoutRequest) (*layoutmgt.Layout, *serviceerror.ServiceError)
	GetLayout(ctx context.Context, id string) (*layoutmgt.Layout, *serviceerror.ServiceError)
	UpdateLayout(ctx context.Context,
		id string, layout layoutmgt.UpdateLayoutRequest) (*layoutmgt.Layout, *serviceerror.ServiceError)
}

type userAdapter interface {
	CreateUser(ctx context.Context, user *user.User) (*user.User, *serviceerror.ServiceError)
	GetUser(ctx context.Context, userID string, includeDisplay bool) (*user.User, *serviceerror.ServiceError)
	UpdateUser(ctx context.Context, userID string, user *user.User) (*user.User, *serviceerror.ServiceError)
	DeleteUser(ctx context.Context, userID string) *serviceerror.ServiceError
	UpdateUserCredentials(ctx context.Context, userID string, credentials json.RawMessage) *serviceerror.ServiceError
}

type translationAdapter interface {
	SetTranslationOverrides(ctx context.Context, language string, translations map[string]map[string]string) (
		*i18nmgt.LanguageTranslationsResponse,
		*serviceerror.ServiceError)
}

type agentAdapter interface {
	CreateAgent(ctx context.Context, agent *agentmodel.Agent) (
		*agentmodel.AgentCompleteResponse, *serviceerror.ServiceError)
	GetAgent(ctx context.Context, agentID string, includeDisplay bool) (
		*agentmodel.AgentGetResponse, *serviceerror.ServiceError)
	UpdateAgent(ctx context.Context, agentID string, req *agentmodel.UpdateAgentRequest) (
		*agentmodel.AgentCompleteResponse, *serviceerror.ServiceError)
}

// ImportServiceInterface defines runtime resource import and declarative resource deletion operations.
type ImportServiceInterface interface {
	ImportResources(ctx context.Context, request *ImportRequest) (*ImportResponse, *serviceerror.ServiceError)
	DeleteResource(ctx context.Context, request *DeleteResourceRequest) (
		*DeleteResourceResponse,
		*serviceerror.ServiceError,
	)
}

const (
	importTargetRuntime           = "runtime"
	importTargetFile              = "file"
	invalidOAuthConfigurationCode = "APP-1024"
)

type importService struct {
	applicationService    applicationAdapter
	idpService            idpAdapter
	flowService           flowAdapter
	ouService             ouAdapter
	entityTypeService     entityTypeAdapter
	roleService           roleAdapter
	roleAssignmentService roleAssignmentAdapter
	groupService          groupAdapter
	resourceService       resourceServerAdapter
	themeService          themeAdapter
	layoutService         layoutAdapter
	userService           userAdapter
	translationService    translationAdapter
	agentService          agentAdapter
}

func newImportService(
	applicationService applicationAdapter,
	idpService idpAdapter,
	flowService flowAdapter,
	ouService ouAdapter,
	entityTypeService entityTypeAdapter,
	roleService roleAdapter,
	roleAssignmentService roleAssignmentAdapter,
	groupService groupAdapter,
	resourceService resourceServerAdapter,
	themeService themeAdapter,
	layoutService layoutAdapter,
	userService userAdapter,
	translationService translationAdapter,
	agentService agentAdapter,
) ImportServiceInterface {
	return &importService{
		applicationService:    applicationService,
		idpService:            idpService,
		flowService:           flowService,
		ouService:             ouService,
		entityTypeService:     entityTypeService,
		roleService:           roleService,
		roleAssignmentService: roleAssignmentService,
		groupService:          groupService,
		resourceService:       resourceService,
		themeService:          themeService,
		layoutService:         layoutService,
		userService:           userService,
		translationService:    translationService,
		agentService:          agentService,
	}
}

func (s *importService) ImportResources(
	ctx context.Context, request *ImportRequest,
) (*ImportResponse, *serviceerror.ServiceError) {
	if request == nil || request.Content == "" {
		return nil, serviceerror.CustomServiceError(ErrorInvalidImportRequest,
			core.I18nMessage{Key: "error.import.emptyContent", DefaultValue: "import content cannot be empty"})
	}

	options := request.Options
	if options == nil {
		options = &ImportOptions{}
	}
	if options.Upsert == nil {
		upsertEnabled := true
		options.Upsert = &upsertEnabled
	}
	if options.ContinueOnError == nil {
		continueOnErrorEnabled := true
		options.ContinueOnError = &continueOnErrorEnabled
	}
	if options.Target == "" {
		options.Target = importTargetRuntime
	}

	if options.Target == importTargetFile {
		return nil, serviceerror.CustomServiceError(
			ErrorInvalidImportRequest,
			core.I18nMessage{
				Key:          "error.import.fileTargetNotSupported",
				DefaultValue: "file target is not supported; use runtime target",
			},
		)
	}

	resolvedContent, err := resolveTemplate(request.Content, request.Variables)
	if err != nil {
		log.GetLogger().Warn(ctx, "Import template resolution failed", log.String("error", err.Error()))
		return nil, serviceerror.CustomServiceError(ErrorTemplateResolutionFailed,
			core.I18nMessage{Key: "error.import.dynamic", DefaultValue: err.Error()})
	}

	docs, err := parseDocuments(resolvedContent)
	if err != nil {
		log.GetLogger().Warn(ctx, "Import YAML parsing failed", log.String("error", err.Error()))
		return nil, serviceerror.CustomServiceError(ErrorInvalidYAMLContent,
			core.I18nMessage{Key: "error.import.dynamic", DefaultValue: err.Error()})
	}

	results := make([]ImportItemOutcome, 0, len(docs))
	imported := 0
	failed := 0
	flowIDAliases := make(map[string]string)

	orderedDocs := orderDocumentsByDependencies(docs)

	for _, doc := range orderedDocs {
		originalFlowID := ""
		if doc.ResourceType == resourceTypeFlow {
			var flowReq flowmgt.CompleteFlowDefinition
			if err := doc.Node.Decode(&flowReq); err == nil {
				originalFlowID = flowReq.ID
			}
		}

		outcome := s.importDocument(ctx, doc, options, request.DryRun, flowIDAliases)
		results = append(results, outcome)

		if doc.ResourceType == resourceTypeFlow && outcome.Status == statusSuccess && originalFlowID != "" &&
			outcome.ResourceID != "" && originalFlowID != outcome.ResourceID {
			flowIDAliases[originalFlowID] = outcome.ResourceID
		}

		if outcome.Status == statusSuccess {
			imported++
		} else {
			failed++
			if !options.IsContinueOnErrorEnabled() {
				break
			}
		}
	}

	return &ImportResponse{
		Summary: &ImportSummary{
			TotalDocuments: len(docs),
			Imported:       imported,
			Failed:         failed,
			ImportedAt:     time.Now().UTC(),
		},
		Results: results,
	}, nil
}

func (s *importService) DeleteResource(
	ctx context.Context, request *DeleteResourceRequest,
) (*DeleteResourceResponse, *serviceerror.ServiceError) {
	_ = ctx

	if request == nil || request.ResourceType == "" || request.ResourceKey == "" {
		return nil, serviceerror.CustomServiceError(
			ErrorInvalidImportRequest,
			core.I18nMessage{
				Key:          "error.import.missingDeleteFields",
				DefaultValue: "resourceType and resourceKey are required",
			},
		)
	}

	deletedFile, svcErr := deleteFileBackedResource(request.ResourceType, request.ResourceKey)
	if svcErr != nil {
		return nil, svcErr
	}

	return &DeleteResourceResponse{
		ResourceType: request.ResourceType,
		ResourceKey:  request.ResourceKey,
		DeletedFile:  deletedFile,
	}, nil
}

func (s *importService) importDocument(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool, flowIDAliases map[string]string,
) ImportItemOutcome {
	switch doc.ResourceType {
	case resourceTypeApplication:
		return s.importApplication(ctx, doc, options, dryRun, flowIDAliases)
	case resourceTypeIdentityProvider:
		return s.importIdentityProvider(ctx, doc, options, dryRun)
	case resourceTypeFlow:
		return s.importFlow(ctx, doc, options, dryRun)
	case resourceTypeOrganizationUnit:
		return s.importOrganizationUnit(ctx, doc, options, dryRun)
	case resourceTypeEntityType:
		return s.importEntityType(ctx, doc, options, dryRun)
	case resourceTypeRole:
		return s.importRole(ctx, doc, options, dryRun)
	case resourceTypeGroup:
		return s.importGroup(ctx, doc, options, dryRun)
	case resourceTypeResourceServer:
		return s.importResourceServer(ctx, doc, options, dryRun)
	case resourceTypeTheme:
		return s.importTheme(ctx, doc, options, dryRun)
	case resourceTypeLayout:
		return s.importLayout(ctx, doc, options, dryRun)
	case resourceTypeUser:
		return s.importUser(ctx, doc, options, dryRun)
	case resourceTypeTranslation:
		return s.importTranslation(ctx, doc, dryRun)
	case resourceTypeAgent:
		return s.importAgent(ctx, doc, options, dryRun, flowIDAliases)
	default:
		return ImportItemOutcome{
			ResourceType: doc.ResourceType,
			Status:       statusFailed,
			Code:         ErrorInvalidImportRequest.Code,
			Message:      "unsupported resource document",
		}
	}
}

func (s *importService) importIdentityProvider(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.idpService == nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeIdentityProvider,
			Status:       statusFailed,
			Code:         ErrorInvalidImportRequest.Code,
			Message:      "identity provider adapter is not configured",
		}
	}

	req, err := idp.ParseIDPDTOFromNode(doc.Node)
	if err != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeIdentityProvider,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("failed to decode identity provider document: %v", err),
		}
	}

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.idpService.GetIdentityProvider(ctx, req.ID)
			if svcErr == nil {
				return successOutcome(resourceTypeIdentityProvider, req.ID, req.Name, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeIdentityProvider, req.ID, req.Name, operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeIdentityProvider, req.ID, req.Name, operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		updated, svcErr := s.idpService.UpdateIdentityProvider(ctx, req.ID, req)
		if svcErr == nil {
			return ImportItemOutcome{
				ResourceType: resourceTypeIdentityProvider,
				ResourceID:   updated.ID,
				ResourceName: updated.Name,
				Operation:    operationUpdate,
				Status:       statusSuccess,
			}
		}

		if !isNotFoundServiceError(svcErr) {
			return ImportItemOutcome{
				ResourceType: resourceTypeIdentityProvider,
				ResourceID:   req.ID,
				ResourceName: req.Name,
				Operation:    operationUpdate,
				Status:       statusFailed,
				Code:         svcErr.Code,
				Message:      svcErr.Error.DefaultValue,
			}
		}
	}

	created, svcErr := s.idpService.CreateIdentityProvider(ctx, req)
	if svcErr != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeIdentityProvider,
			ResourceID:   req.ID,
			ResourceName: req.Name,
			Operation:    operationCreate,
			Status:       statusFailed,
			Code:         svcErr.Code,
			Message:      svcErr.Error.DefaultValue,
		}
	}

	return ImportItemOutcome{
		ResourceType: resourceTypeIdentityProvider,
		ResourceID:   created.ID,
		ResourceName: created.Name,
		Operation:    operationCreate,
		Status:       statusSuccess,
	}
}

func (s *importService) importFlow(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.flowService == nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeFlow,
			Status:       statusFailed,
			Code:         ErrorInvalidImportRequest.Code,
			Message:      "flow adapter is not configured",
		}
	}

	var req flowmgt.CompleteFlowDefinition
	if err := doc.Node.Decode(&req); err != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeFlow,
			ResourceID:   req.ID,
			ResourceName: req.Name,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("failed to decode flow document: %v", err),
		}
	}

	flowDef := &flowmgt.FlowDefinition{
		ID:       req.ID,
		Handle:   req.Handle,
		Name:     req.Name,
		FlowType: req.FlowType,
		Nodes:    req.Nodes,
	}

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.flowService.GetFlow(ctx, req.ID)
			if svcErr == nil {
				return successOutcome(resourceTypeFlow, req.ID, req.Name, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeFlow, req.ID, req.Name, operationUpdate, svcErr)
			}

			existingByHandle, handleErr := s.flowService.GetFlowByHandle(ctx, req.Handle, req.FlowType)
			if handleErr == nil {
				return successOutcome(resourceTypeFlow, existingByHandle.ID, req.Name, operationUpdate)
			}

			if !isNotFoundServiceError(handleErr) {
				return serviceErrorOutcome(resourceTypeFlow, req.ID, req.Name, operationUpdate, handleErr)
			}
		}

		return successOutcome(resourceTypeFlow, req.ID, req.Name, operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		updated, svcErr := s.flowService.UpdateFlow(ctx, req.ID, flowDef)
		if svcErr == nil {
			return ImportItemOutcome{
				ResourceType: resourceTypeFlow,
				ResourceID:   updated.ID,
				ResourceName: updated.Name,
				Operation:    operationUpdate,
				Status:       statusSuccess,
			}
		}

		if !isNotFoundServiceError(svcErr) {
			return ImportItemOutcome{
				ResourceType: resourceTypeFlow,
				ResourceID:   req.ID,
				ResourceName: req.Name,
				Operation:    operationUpdate,
				Status:       statusFailed,
				Code:         svcErr.Code,
				Message:      svcErr.Error.DefaultValue,
			}
		}
	}

	created, svcErr := s.flowService.CreateFlow(ctx, flowDef)
	if svcErr != nil {
		if options.IsUpsertEnabled() && req.ID != "" && svcErr.Code == flowmgt.ErrorDuplicateFlowHandle.Code {
			existingByHandle, handleErr := s.flowService.GetFlowByHandle(ctx, req.Handle, req.FlowType)
			if handleErr == nil {
				updated, updateErr := s.flowService.UpdateFlow(ctx, existingByHandle.ID, flowDef)
				if updateErr != nil {
					return ImportItemOutcome{
						ResourceType: resourceTypeFlow,
						ResourceID:   existingByHandle.ID,
						ResourceName: req.Name,
						Operation:    operationUpdate,
						Status:       statusFailed,
						Code:         updateErr.Code,
						Message:      updateErr.Error.DefaultValue,
					}
				}

				return ImportItemOutcome{
					ResourceType: resourceTypeFlow,
					ResourceID:   updated.ID,
					ResourceName: updated.Name,
					Operation:    operationUpdate,
					Status:       statusSuccess,
				}
			}

			if !isNotFoundServiceError(handleErr) {
				return ImportItemOutcome{
					ResourceType: resourceTypeFlow,
					ResourceID:   req.ID,
					ResourceName: req.Name,
					Operation:    operationUpdate,
					Status:       statusFailed,
					Code:         handleErr.Code,
					Message:      handleErr.Error.DefaultValue,
				}
			}
		}

		return ImportItemOutcome{
			ResourceType: resourceTypeFlow,
			ResourceID:   req.ID,
			ResourceName: req.Name,
			Operation:    operationCreate,
			Status:       statusFailed,
			Code:         svcErr.Code,
			Message:      svcErr.Error.DefaultValue,
		}
	}

	return ImportItemOutcome{
		ResourceType: resourceTypeFlow,
		ResourceID:   created.ID,
		ResourceName: created.Name,
		Operation:    operationCreate,
		Status:       statusSuccess,
	}
}

var resourceDependencyOrder = []string{
	resourceTypeOrganizationUnit,
	resourceTypeEntityType,
	resourceTypeResourceServer,
	resourceTypeIdentityProvider,
	resourceTypeNotificationSender,
	resourceTypeFlow,
	resourceTypeTheme,
	resourceTypeLayout,
	resourceTypeApplication,
	resourceTypeAgent,
	resourceTypeUser,
	resourceTypeGroup,
	resourceTypeRole,
	resourceTypeTranslation,
}

func orderDocumentsByDependencies(docs []parsedDocument) []parsedDocument {
	priority := make(map[string]int, len(resourceDependencyOrder))
	for i, resourceType := range resourceDependencyOrder {
		priority[resourceType] = i
	}

	ordered := make([]parsedDocument, len(docs))
	copy(ordered, docs)

	sort.SliceStable(ordered, func(i, j int) bool {
		pi, okI := priority[ordered[i].ResourceType]
		if !okI {
			pi = len(priority) + 1
		}
		pj, okJ := priority[ordered[j].ResourceType]
		if !okJ {
			pj = len(priority) + 1
		}
		if pi != pj {
			return pi < pj
		}
		return ordered[i].Sequence < ordered[j].Sequence
	})

	return ordered
}

func (s *importService) importApplication(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool, flowIDAliases map[string]string,
) ImportItemOutcome {
	var req appmodel.ApplicationRequestWithID
	if err := doc.Node.Decode(&req); err != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeApplication,
			ResourceID:   req.ID,
			ResourceName: req.Name,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("failed to decode application document: %v", err),
		}
	}

	if s.applicationService == nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeApplication,
			ResourceID:   req.ID,
			ResourceName: req.Name,
			Status:       statusFailed,
			Code:         ErrorAdapterNotConfigured.Code,
			Message:      "application adapter not configured",
		}
	}

	if mappedFlowID, ok := flowIDAliases[req.AuthFlowID]; ok {
		req.AuthFlowID = mappedFlowID
	}
	if mappedFlowID, ok := flowIDAliases[req.RegistrationFlowID]; ok {
		req.RegistrationFlowID = mappedFlowID
	}

	appDTO := applicationRequestToDTO(&req)
	normalizeOAuthConfigForImport(ctx, appDTO)
	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.applicationService.GetApplication(ctx, req.ID)
			if svcErr == nil {
				return successOutcome(resourceTypeApplication, req.ID, req.Name, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeApplication, req.ID, req.Name, operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeApplication, req.ID, req.Name, operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		updated, svcErr := s.applicationService.UpdateApplication(ctx, req.ID, appDTO)
		if svcErr == nil {
			return ImportItemOutcome{
				ResourceType: resourceTypeApplication,
				ResourceID:   updated.ID,
				ResourceName: updated.Name,
				Operation:    operationUpdate,
				Status:       statusSuccess,
			}
		}

		if !isNotFoundServiceError(svcErr) {
			return ImportItemOutcome{
				ResourceType: resourceTypeApplication,
				ResourceID:   req.ID,
				ResourceName: req.Name,
				Operation:    operationUpdate,
				Status:       statusFailed,
				Code:         svcErr.Code,
				Message:      svcErr.Error.DefaultValue,
			}
		}
	}

	created, svcErr := s.applicationService.CreateApplication(ctx, appDTO)
	if svcErr != nil {
		failureLogFields := []log.Field{
			log.String("appID", req.ID),
			log.String("name", req.Name),
			log.String("code", svcErr.Code),
			log.String("error", svcErr.Error.DefaultValue),
			log.String("authFlowID", appDTO.AuthFlowID),
			log.String("registrationFlowID", appDTO.RegistrationFlowID),
			log.Bool("isRegistrationFlowEnabled", appDTO.IsRegistrationFlowEnabled),
		}
		if svcErr.ErrorDescription.DefaultValue != "" {
			failureLogFields = append(failureLogFields,
				log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		}
		if oauthConfig := getOAuthConfigForImportLog(appDTO); oauthConfig != nil {
			failureLogFields = append(failureLogFields,
				log.String("clientID", oauthConfig.ClientID),
				log.Bool("hasClientSecret", oauthConfig.ClientSecret != ""),
				log.Bool("publicClient", oauthConfig.PublicClient),
				log.Bool("pkceRequired", oauthConfig.PKCERequired),
				log.String("tokenEndpointAuthMethod", string(oauthConfig.TokenEndpointAuthMethod)),
				log.Any("grantTypes", oauthConfig.GrantTypes),
				log.Any("responseTypes", oauthConfig.ResponseTypes),
				log.Any("redirectURIs", oauthConfig.RedirectURIs),
			)
		}

		log.GetLogger().Warn(ctx, "Application import create failed", failureLogFields...)

		if svcErr.Code == invalidOAuthConfigurationCode {
			log.GetLogger().Debug(ctx,
				"Application import failed due to invalid OAuth configuration", failureLogFields...)
		}

		return ImportItemOutcome{
			ResourceType: resourceTypeApplication,
			ResourceID:   req.ID,
			ResourceName: req.Name,
			Operation:    operationCreate,
			Status:       statusFailed,
			Code:         svcErr.Code,
			Message:      svcErr.Error.DefaultValue,
		}
	}

	return ImportItemOutcome{
		ResourceType: resourceTypeApplication,
		ResourceID:   created.ID,
		ResourceName: created.Name,
		Operation:    operationCreate,
		Status:       statusSuccess,
	}
}

func applicationRequestToDTO(req *appmodel.ApplicationRequestWithID) *appmodel.ApplicationDTO {
	appDTO := &appmodel.ApplicationDTO{
		ID:          req.ID,
		OUID:        req.OUID,
		OUHandle:    req.OUHandle,
		Name:        req.Name,
		Description: req.Description,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			AuthFlowID:                req.AuthFlowID,
			AuthFlowHandle:            req.AuthFlowHandle,
			RegistrationFlowID:        req.RegistrationFlowID,
			RegistrationFlowHandle:    req.RegistrationFlowHandle,
			IsRegistrationFlowEnabled: req.IsRegistrationFlowEnabled,
			RecoveryFlowID:            req.RecoveryFlowID,
			RecoveryFlowHandle:        req.RecoveryFlowHandle,
			IsRecoveryFlowEnabled:     req.IsRecoveryFlowEnabled,
			ThemeID:                   req.ThemeID,
			LayoutID:                  req.LayoutID,
			Assertion:                 req.Assertion,
			LoginConsent:              req.LoginConsent,
			AllowedUserTypes:          req.AllowedUserTypes,
			Certificate:               req.Certificate,
		},
		Template:  req.Template,
		URL:       req.URL,
		LogoURL:   req.LogoURL,
		TosURI:    req.TosURI,
		PolicyURI: req.PolicyURI,
		Contacts:  req.Contacts,
		Metadata:  req.Metadata,
	}

	if len(req.InboundAuthConfig) > 0 {
		inboundAuthConfigDTOs := make([]inboundmodel.InboundAuthConfigWithSecret, 0, len(req.InboundAuthConfig))
		for _, config := range req.InboundAuthConfig {
			if config.Type != inboundmodel.OAuthInboundAuthType || config.OAuthConfig == nil {
				continue
			}

			inboundAuthConfigDTOs = append(inboundAuthConfigDTOs, inboundmodel.InboundAuthConfigWithSecret{
				Type: config.Type,
				OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
					ClientID:                           config.OAuthConfig.ClientID,
					ClientSecret:                       config.OAuthConfig.ClientSecret,
					RedirectURIs:                       config.OAuthConfig.RedirectURIs,
					GrantTypes:                         config.OAuthConfig.GrantTypes,
					ResponseTypes:                      config.OAuthConfig.ResponseTypes,
					TokenEndpointAuthMethod:            config.OAuthConfig.TokenEndpointAuthMethod,
					PKCERequired:                       config.OAuthConfig.PKCERequired,
					PublicClient:                       config.OAuthConfig.PublicClient,
					RequirePushedAuthorizationRequests: config.OAuthConfig.RequirePushedAuthorizationRequests,
					Token:                              config.OAuthConfig.Token,
					Scopes:                             config.OAuthConfig.Scopes,
					UserInfo:                           config.OAuthConfig.UserInfo,
					ScopeClaims:                        config.OAuthConfig.ScopeClaims,
					Certificate:                        config.OAuthConfig.Certificate,
					AcrValues:                          config.OAuthConfig.AcrValues,
				},
			})
		}
		appDTO.InboundAuthConfig = inboundAuthConfigDTOs
	}

	return appDTO
}

func getOAuthConfigForImportLog(appDTO *appmodel.ApplicationDTO) *inboundmodel.OAuthConfigWithSecret {
	if appDTO == nil {
		return nil
	}

	for _, inboundAuth := range appDTO.InboundAuthConfig {
		if inboundAuth.Type == inboundmodel.OAuthInboundAuthType && inboundAuth.OAuthConfig != nil {
			return inboundAuth.OAuthConfig
		}
	}

	return nil
}

func normalizeOAuthConfigForImport(ctx context.Context, appDTO *appmodel.ApplicationDTO) {
	oauthConfig := getOAuthConfigForImportLog(appDTO)
	if oauthConfig == nil {
		return
	}

	if oauthConfig.PublicClient &&
		oauthConfig.TokenEndpointAuthMethod == oauth2const.TokenEndpointAuthMethodNone &&
		oauthConfig.ClientSecret != "" {
		log.GetLogger().Debug(ctx,
			"Dropping client_secret for public client import with token endpoint auth method 'none'",
			log.String("appID", appDTO.ID),
			log.String("name", appDTO.Name),
			log.String("clientID", oauthConfig.ClientID))
		oauthConfig.ClientSecret = ""
	}
}

func isNotFoundServiceError(svcErr *serviceerror.ServiceError) bool {
	if svcErr == nil {
		return false
	}
	_, ok := notFoundErrorCodes[svcErr.Code]
	return ok
}
