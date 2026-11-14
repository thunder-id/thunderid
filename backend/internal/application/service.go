// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/entity"
	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/filter"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ApplicationServiceInterface defines the interface for the application service.
type ApplicationServiceInterface interface {
	CreateApplication(
		ctx context.Context, app *model.ApplicationDTO) (*model.ApplicationDTO, *tidcommon.ServiceError)
	ValidateApplication(ctx context.Context, app *model.ApplicationDTO) (
		*model.ApplicationProcessedDTO, *providers.InboundAuthConfigWithSecret, *tidcommon.ServiceError)
	GetApplicationList(ctx context.Context, filters map[string]interface{}) (
		*model.ApplicationListResponse, *tidcommon.ServiceError)
	GetApplicationListByProject(ctx context.Context, projectID string, filters map[string]interface{}) (
		*model.ApplicationListResponse, *tidcommon.ServiceError)
	GetOAuthApplication(
		ctx context.Context, clientID string) (*providers.OAuthClient, *tidcommon.ServiceError)
	GetApplication(ctx context.Context, appID string) (*providers.Application, *tidcommon.ServiceError)
	UpdateApplication(
		ctx context.Context, appID string, app *model.ApplicationDTO) (
		*model.ApplicationDTO, *tidcommon.ServiceError)
	DeleteApplication(ctx context.Context, appID string) *tidcommon.ServiceError
	ValidateDeleteApplication(ctx context.Context, appID string) (
		*model.ApplicationArtifactProfile, *tidcommon.ServiceError)
	ValidateClientSecretRegeneration(ctx context.Context, appID string) (
		*entity.ClientSecretProfile, *tidcommon.ServiceError)
	RegenerateClientSecret(ctx context.Context, appID string) (*model.ClientSecretResponse, *tidcommon.ServiceError)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	SetDependencyRegistry(r resourcedependency.Registry)

	// Sharing policies. Each resolves the application first, so a policy is only ever reached
	// through the application it governs.
	CreateSharingPolicy(ctx context.Context, appID string, req sharing.PolicyRequest) (
		sharing.Policy, *tidcommon.ServiceError)
	GetSharingPolicy(ctx context.Context, appID, policyID string) (
		sharing.Policy, *tidcommon.ServiceError)
	ListSharingPolicies(ctx context.Context, appID string, limit, offset int) (
		sharing.PolicyList, *tidcommon.ServiceError)
	UpdateSharingPolicy(ctx context.Context, appID, policyID string, req sharing.PolicyRequest) (
		sharing.Policy, *tidcommon.ServiceError)
	DeleteSharingPolicy(ctx context.Context, appID, policyID string) *tidcommon.ServiceError
	ResolveSharingOverlay(ctx context.Context, appID, ouID string) (
		sharing.ResolvedOverlay, *tidcommon.ServiceError)
}

// artifactLifetimeResolver reports how long an artifact issued to the given OAuth client can remain
// valid. A revocation against an application sizes its deny-list row from it. The composition root
// supplies it, so this package depends on no OAuth configuration of its own; a nil resolver, or a zero
// duration, leaves the revocation service on its configured default.
type artifactLifetimeResolver func(client *providers.OAuthClient) time.Duration

// projectOUValidator checks that the organization unit of a resource scoped to a project belongs to it.
type projectOUValidator interface {
	ValidateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError
}

// ApplicationService is the default implementation of the ApplicationServiceInterface.
type applicationService struct {
	logger               *log.Logger
	inboundClientService inboundclient.InboundClientServiceInterface
	entityService        entity.EntityServiceInterface
	ouService            oupkg.OrganizationUnitServiceInterface
	i18nService          i18nmgt.I18nServiceInterface
	cryptoSvc            providers.RuntimeCryptoProvider
	dependencyRegistry   resourcedependency.Registry
	serverConfigService  serverconfig.ServerConfigService
	resolveLifetime      artifactLifetimeResolver
	valueCapturer        declarativeresource.ValueCapturer
	sharingService       sharing.SharingServiceInterface
	authzService         sysauthz.SystemAuthorizationServiceInterface
	// governance derives the access status.
	governance identitygovernance.IdentityGovernanceServiceInterface
	// projectValidator is nil on a deployment that serves no projects.
	projectValidator projectOUValidator
	// projectRequired is set when a deployment that serves projects refuses an application created without
	// one.
	projectRequired bool
	// systemFlows is nil where system flows are off.
	systemFlows systemFlowProvider
	bus         eventbus.EventBusInterface
	// managed says which applications a control plane deployed, which stay read only here. Nil where
	// nothing is deployed from one.
	managed managedresource.StoreInterface
}

// systemFlowProvider runs a system flow to completion. The flow engine implements it.
type systemFlowProvider interface {
	ExecuteSystemFlow(ctx context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
		*tidcommon.ServiceError)
}

// newApplicationService creates a new instance of ApplicationService.
func newApplicationService(
	inboundClientSvc inboundclient.InboundClientServiceInterface,
	entityService entity.EntityServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	i18nService i18nmgt.I18nServiceInterface,
	cryptoSvc providers.RuntimeCryptoProvider,
	serverConfigSvc serverconfig.ServerConfigService,
	artifactLifetime artifactLifetimeResolver,
	valueCapturer declarativeresource.ValueCapturer,
	sharingService sharing.SharingServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	governance identitygovernance.IdentityGovernanceServiceInterface,
	projectValidator projectOUValidator,
	bus eventbus.EventBusInterface,
) ApplicationServiceInterface {
	return &applicationService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ApplicationService")),
		inboundClientService: inboundClientSvc,
		entityService:        entityService,
		ouService:            ouService,
		i18nService:          i18nService,
		cryptoSvc:            cryptoSvc,
		serverConfigService:  serverConfigSvc,
		resolveLifetime:      artifactLifetime,
		valueCapturer:        valueCapturer,
		sharingService:       sharingService,
		authzService:         authzService,
		governance:           governance,
		projectValidator:     projectValidator,
		projectRequired:      projectValidator != nil && projectscope.ProjectRequired(security.ResourceTypeApplication),
		bus:                  bus,
	}
}

func (as *applicationService) deleteEntityCompensation(ctx context.Context, appID string) {
	if delErr := as.entityService.DeleteEntity(ctx, appID); delErr != nil {
		as.logger.Error(ctx, "Failed to delete entity during compensation", log.Error(delErr),
			log.String("appID", appID))
	}
}

// CreateApplication creates the application.
func (as *applicationService) CreateApplication(ctx context.Context, app *model.ApplicationDTO) (*model.ApplicationDTO,
	*tidcommon.ServiceError) {
	created, svcErr := as.createApplication(ctx, app)
	var appID string
	if svcErr == nil {
		appID = created.ID
	}
	as.publishApplicationEvent(ctx, audit.EventApplicationCreate, audit.ActionCreate, appID, svcErr)
	return created, svcErr
}

// createApplication creates the application without publishing its audit event.
func (as *applicationService) createApplication(ctx context.Context, app *model.ApplicationDTO) (
	*model.ApplicationDTO, *tidcommon.ServiceError) {
	if app == nil {
		return nil, &ErrorApplicationNil
	}
	// Check if store is in pure declarative mode
	if isDeclarativeModeEnabled() {
		return nil, &ErrorCannotModifyDeclarativeResource
	}
	if svcErr := as.checkProjectGiven(ctx, app.ProjectID); svcErr != nil {
		return nil, svcErr
	}

	// Check if an application with the same ID exists and is declarative (in composite mode). An id
	// no entity holds yet is the ordinary case for a create.
	if app.ID != "" {
		if svcErr := as.checkApplicationDeclarative(ctx, app.ID); svcErr != nil &&
			svcErr.Code != ErrorApplicationNotFound.Code {
			return nil, svcErr
		}
	}

	processedDTO, inboundAuthConfig, svcErr := as.ValidateApplication(ctx, app)
	if svcErr != nil {
		return nil, svcErr
	}
	// Validation resolves the organization unit the application is created in.
	if svcErr := as.checkApplicationAccess(ctx, security.ActionCreateApplication, app.OUID); svcErr != nil {
		return nil, svcErr
	}

	appID := processedDTO.ID

	seedClientSubTypeAttribute(processedDTO)

	inboundClient := toInboundClient(processedDTO)
	oauthProfile := toOAuthProfile(processedDTO)
	if svcErr := as.resolveAttestationCredentialsForPersist(ctx, appID, &inboundClient); svcErr != nil {
		return nil, svcErr
	}

	// Create entity.
	var clientID string
	var clientSecret string
	if inboundAuthConfig != nil && inboundAuthConfig.OAuthConfig != nil {
		clientID = inboundAuthConfig.OAuthConfig.ClientID
		clientSecret = inboundAuthConfig.OAuthConfig.ClientSecret
	}

	// Issue an Flow Secret only to applications that can initiate a flow directly via the Flow
	// Execution API — i.e. backend / server-side apps with the token-exchange grant, or with no OAuth
	// config. Public clients (browser SPAs, mobile apps) and redirect-based (authorization_code)
	// clients initiate their flows through the OAuth component, and clients without the
	// token-exchange grant cannot redeem a flow assertion, so they have no use for an Flow Secret and
	// never receive one — any caller-supplied value for such apps is ignored. For eligible apps, an
	// explicitly provided value (e.g. declarative resources) is preserved; otherwise one is generated.
	flowSecret := ""
	if isFlowSecretEligible(app.Type, inboundAuthConfig) {
		flowSecret = app.FlowSecret
		if flowSecret == "" {
			generatedFlowSecret, secretErr := oauthutils.GenerateOAuth2ClientSecret()
			if secretErr != nil {
				as.logger.Error(ctx, "Failed to generate flow secret", log.Error(secretErr))
				return nil, &tidcommon.InternalServerError
			}
			flowSecret = generatedFlowSecret
		}
	}

	appEntity, sysCredsJSON, buildErr := buildAppEntity(appID, app, clientID, clientSecret, flowSecret)
	if buildErr != nil {
		as.logger.Error(ctx, "Failed to build entity for create", log.Error(buildErr))
		return nil, &tidcommon.InternalServerError
	}

	_, entErr := as.entityService.CreateEntity(ctx, appEntity, sysCredsJSON, nil)
	if entErr != nil {
		if svcErr := mapEntityError(entErr); svcErr != nil {
			return nil, svcErr
		}
		as.logger.Error(ctx, "Failed to create application entity",
			log.String("appID", appID), log.Error(entErr))
		return nil, &tidcommon.InternalServerError
	}

	// Create config (with compensation if it fails).
	if err := as.inboundClientService.CreateInboundClient(ctx, &inboundClient, oauthProfile,
		clientSecret != ""); err != nil {
		// Compensate: delete entity since config creation failed.
		as.deleteEntityCompensation(ctx, appID)
		if svcErr := as.translateInboundClientError(ctx, err); svcErr != nil {
			return nil, svcErr
		}
		as.logger.Error(ctx, "Failed to create application", log.Error(err), log.String("appID", appID))
		return nil, &tidcommon.InternalServerError
	}

	as.syncPasskeyOriginsToCORS(ctx, processedDTO.PasskeyAllowedOrigins)

	appForReturn := *app
	appForReturn.AuthFlowID = inboundClient.AuthFlowID
	appForReturn.DeviceGrantFlowID = inboundClient.DeviceGrantFlowID
	appForReturn.RegistrationFlowID = inboundClient.RegistrationFlowID
	appForReturn.RecoveryFlowID = inboundClient.RecoveryFlowID
	appForReturn.SignOutFlowID = inboundClient.SignOutFlowID
	var oauthToken *providers.OAuthTokenConfig
	var userInfo *providers.UserInfoConfig
	var scopeClaims map[string][]string
	if inboundAuthConfig != nil && oauthProfile != nil {
		oauthToken = oauthProfile.Token
		userInfo = oauthProfile.UserInfo
		scopeClaims = oauthProfile.ScopeClaims
		oauthCfg := inboundAuthConfig.OAuthConfig
		if oauthCfg != nil &&
			(oauthCfg.Certificate == nil || oauthCfg.Certificate.Type == "") {
			oauthCfg.Certificate = nil
		}
	}
	returnDTO := buildReturnApplicationDTO(appID, &appForReturn, inboundClient.Assertion, processedDTO.Metadata,
		inboundAuthConfig, oauthToken, userInfo, scopeClaims)
	// Surface the Flow Secret once, on creation only.
	returnDTO.FlowSecret = flowSecret

	as.captureValues(ctx, returnDTO)
	return returnDTO, nil
}

// ValidateApplication validates the application data transfer object.
func (as *applicationService) ValidateApplication(ctx context.Context, app *model.ApplicationDTO) (
	*model.ApplicationProcessedDTO, *providers.InboundAuthConfigWithSecret, *tidcommon.ServiceError) {
	if app == nil {
		return nil, nil, &ErrorApplicationNil
	}
	if app.Name == "" {
		return nil, nil, &ErrorInvalidApplicationName
	}
	nameExists, nameCheckErr := as.isIdentifierTaken(ctx, fieldName, app.Name, app.ID)
	if nameCheckErr != nil {
		return nil, nil, nameCheckErr
	}
	if nameExists {
		return nil, nil, &ErrorApplicationAlreadyExistsWithName
	}

	inboundAuthConfig, svcErr := as.processInboundAuthConfig(ctx, app, nil)
	if svcErr != nil {
		return nil, nil, svcErr
	}

	if svcErr := as.validateApplicationFields(ctx, app); svcErr != nil {
		return nil, nil, svcErr
	}

	// A type is required on creation; there is no implicit default.
	if app.Type == "" {
		return nil, nil, &ErrorApplicationTypeRequired
	}

	appID := app.ID
	if appID == "" {
		var err error
		appID, err = sysutils.GenerateUUIDv7()
		if err != nil {
			as.logger.Error(ctx, "Failed to generate UUID", log.Error(err))
			return nil, nil, &tidcommon.InternalServerError
		}
	}
	processedDTO := buildBaseApplicationProcessedDTO(appID, app, app.Assertion)
	if inboundAuthConfig != nil {
		oa := inboundAuthConfig.OAuthConfig
		processedInboundAuthConfig := buildOAuthInboundAuthConfigProcessedDTO(
			appID, inboundAuthConfig, oa.Token, oa.UserInfo, oa.ScopeClaims, oa.Certificate,
		)
		processedDTO.InboundAuthConfig = []inboundmodel.InboundAuthConfigProcessed{processedInboundAuthConfig}
	}

	// Validate FK constraints (flow, theme, layout, user-type) and OAuth profile.
	// This runs the same checks as Create/Update so declarative resources are validated consistently.
	inboundClient := toInboundClient(processedDTO)
	oauthProfile := toOAuthProfile(processedDTO)
	var hasClientSecret bool
	var oauthClientID string
	if inboundAuthConfig != nil && inboundAuthConfig.OAuthConfig != nil {
		hasClientSecret = inboundAuthConfig.OAuthConfig.ClientSecret != ""
		oauthClientID = inboundAuthConfig.OAuthConfig.ClientID
	}
	if err := as.inboundClientService.Validate(
		ctx, &inboundClient, oauthProfile, hasClientSecret, oauthClientID); err != nil {
		if svcErr := as.translateInboundClientError(ctx, err); svcErr != nil {
			return nil, nil, svcErr
		}
		as.logger.Error(ctx, "Inbound client validation failed", log.Error(err))
		return nil, nil, &tidcommon.InternalServerError
	}
	processedDTO.AuthFlowID = inboundClient.AuthFlowID
	processedDTO.DeviceGrantFlowID = inboundClient.DeviceGrantFlowID
	processedDTO.RegistrationFlowID = inboundClient.RegistrationFlowID
	processedDTO.RecoveryFlowID = inboundClient.RecoveryFlowID
	processedDTO.SignOutFlowID = inboundClient.SignOutFlowID

	return processedDTO, inboundAuthConfig, nil
}

// GetApplicationList list the applications. An organization unit filter lists the applications that
// unit owns, followed by the ones shared into it, as does a list without the filter by a caller confined to
// an organization unit.
func (as *applicationService) GetApplicationList(
	ctx context.Context, filters map[string]interface{}) (*model.ApplicationListResponse, *tidcommon.ServiceError) {
	requestedOUID, rest, err := filter.SplitOUIDFilter(filters)
	if err != nil || len(rest) > 0 {
		return nil, &ErrorInvalidFilter
	}
	ouID, visible, svcErr := sysauthz.ResolveListOU(ctx, as.authzService, as.logger, security.ActionListApplications,
		requestedOUID)
	if svcErr != nil {
		return nil, svcErr
	}
	if !visible {
		return as.buildApplicationList(ctx, nil, 0, 0)
	}
	if ouID != "" {
		return as.listApplicationsInOU(ctx, ouID)
	}

	totalResults, entErr := as.entityService.GetEntityListCount(ctx, providers.EntityCategoryApp, nil)
	if entErr != nil {
		as.logger.Error(ctx, "Failed to count application entities", log.Error(entErr))
		return nil, &tidcommon.InternalServerError
	}

	entities, entErr := as.entityService.GetEntityList(
		ctx, providers.EntityCategoryApp, serverconst.MaxCompositeStoreRecords, 0, nil)
	if entErr != nil {
		as.logger.Error(ctx, "Failed to list application entities", log.Error(entErr))
		return nil, &tidcommon.InternalServerError
	}
	return as.buildApplicationList(ctx, entities, len(entities), totalResults)
}

// GetApplicationListByProject lists the applications of a project, narrowed to those an organization
// unit owns when the filter names one. A project that does not exist has none.
func (as *applicationService) GetApplicationListByProject(ctx context.Context, projectID string,
	filters map[string]interface{}) (*model.ApplicationListResponse, *tidcommon.ServiceError) {
	ouID, rest, err := filter.SplitOUIDFilter(filters)
	if err != nil || len(rest) > 0 {
		return nil, &ErrorInvalidFilter
	}
	if projectID == "" {
		return as.buildApplicationList(ctx, nil, 0, 0)
	}
	ouID, visible, svcErr := sysauthz.ResolveListOU(ctx, as.authzService, as.logger, security.ActionListApplications,
		ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	if !visible {
		return as.buildApplicationList(ctx, nil, 0, 0)
	}
	var ouIDs []string
	if ouID != "" {
		ouIDs = []string{ouID}
	}

	total, err := as.entityService.GetEntityListCountByProject(ctx, providers.EntityCategoryApp, projectID, ouIDs)
	if err != nil {
		as.logger.Error(ctx, "Failed to count project application entities",
			log.String("projectID", projectID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	entities, err := as.entityService.GetEntityListByProject(ctx, providers.EntityCategoryApp, projectID, ouIDs,
		serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		as.logger.Error(ctx, "Failed to list project application entities",
			log.String("projectID", projectID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return as.buildApplicationList(ctx, entities, len(entities), total)
}

// checkProjectGiven refuses an application created without a project where projects are served and one is
// required, and warns about it where one is not required yet.
func (as *applicationService) checkProjectGiven(ctx context.Context, projectID string) *tidcommon.ServiceError {
	if projectID != "" || as.projectValidator == nil {
		return nil
	}
	if as.projectRequired {
		return &ErrorProjectRequired
	}
	as.logger.Warn(ctx, "Application created without a project; a project will be required")
	return nil
}

// validateProjectOU checks that an application's organization unit belongs to its project. It returns
// ErrorInvalidProject when no such project exists, and ErrorOUNotInProject when the organization unit is
// outside the project.
//
// A deployment that serves no projects, such as a Data Plane, cannot check the project. It keeps the
// project ID and organization unit as given so that imported configuration retains them.
func (as *applicationService) validateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError {
	if as.projectValidator == nil {
		return nil
	}
	svcErr := as.projectValidator.ValidateProjectOU(ctx, projectID, ouID)
	switch {
	case svcErr == nil:
		return nil
	case svcErr.Code == projectscope.ProjectNotFoundErrorCode:
		return &ErrorInvalidProject
	case svcErr.Code == projectscope.OUNotInProjectErrorCode:
		return &ErrorOUNotInProject
	default:
		return &tidcommon.InternalServerError
	}
}

// listApplicationsInOU lists the applications an organization unit owns, followed by the ones shared
// into it.
func (as *applicationService) listApplicationsInOU(
	ctx context.Context, ouID string) (*model.ApplicationListResponse, *tidcommon.ServiceError) {
	page, err := as.entityService.GetEntityPageInOU(ctx, providers.EntityCategoryApp, ouID,
		entity.OUEntityScopeAll, serverconst.MaxCompositeStoreRecords, 0, nil)
	if err != nil {
		as.logger.Error(ctx, "Failed to list application entities in organization unit",
			log.String("ouID", ouID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return as.buildApplicationList(ctx, slices.Concat(page.Owned, page.Shared), len(page.Owned),
		page.TotalResults)
}

// buildApplicationList builds the list response for application entities, of which the first
// ownedCount are owned by the listed organization unit and the rest are shared into it.
func (as *applicationService) buildApplicationList(ctx context.Context, entities []providers.Entity,
	ownedCount, totalResults int) (*model.ApplicationListResponse, *tidcommon.ServiceError) {
	if len(entities) == 0 {
		return &model.ApplicationListResponse{
			TotalResults: totalResults,
			Count:        0,
			Applications: []model.BasicApplicationResponse{},
		}, nil
	}

	// Get all inbound clients and filter to app entities.
	configs, err := as.inboundClientService.GetInboundClientList(ctx)
	if err != nil {
		if errors.Is(err, inboundclient.ErrCompositeResultLimitExceeded) {
			return nil, &ErrorResultLimitExceeded
		}
		as.logger.Error(ctx, "Failed to list inbound clients", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	appIDs := make(map[string]struct{}, len(entities))
	for i := range entities {
		appIDs[entities[i].ID] = struct{}{}
	}
	configMap := make(map[string]*inboundmodel.InboundClient, len(entities))
	for i := range configs {
		if _, ok := appIDs[configs[i].ID]; ok {
			configMap[configs[i].ID] = &configs[i]
		}
	}

	statuses := as.accessStatuses(ctx, entities)
	applicationList := make([]model.BasicApplicationResponse, 0, len(entities))
	for i := range entities {
		cfg := configMap[entities[i].ID]
		if cfg == nil {
			as.logger.Warn(ctx, "Application entity has no inbound-client row; skipping in list",
				log.String("appID", entities[i].ID))
			continue
		}
		resp := buildBasicApplicationResponse(*cfg, &entities[i])
		resp.Status = statuses[i]
		owned := i < ownedCount
		resp.IsOwnedByOU = new(owned)
		// The organization unit an application is shared into cannot change it until templated configs are
		// supported for applications.
		if !owned {
			resp.IsReadOnly = true
		}
		applicationList = append(applicationList, resp)
	}

	if err := managedresource.MarkDeployed(ctx, as.managed, applicationList,
		func(a *model.BasicApplicationResponse) string { return a.ID },
		func(a *model.BasicApplicationResponse) { a.IsReadOnly = true },
		managedresource.TypeApplication); err != nil {
		// Every write to such an application is refused whatever the list says, so it is served without the
		// flag.
		as.logger.Error(ctx, "Failed to read which applications a control plane deployed", log.Error(err))
	}

	return &model.ApplicationListResponse{
		TotalResults: totalResults,
		Count:        len(applicationList),
		Applications: applicationList,
	}, nil
}

// GetOAuthApplication retrieves the OAuth application based on the client id.
func (as *applicationService) GetOAuthApplication(
	ctx context.Context, clientID string) (*providers.OAuthClient, *tidcommon.ServiceError) {
	if clientID == "" {
		return nil, &ErrorInvalidClientID
	}

	client, err := as.inboundClientService.GetOAuthClientByClientID(ctx, clientID)
	if err != nil {
		as.logger.Error(ctx, "Failed to retrieve OAuth client", log.Error(err),
			log.MaskedString("clientID", clientID))
		return nil, &tidcommon.InternalServerError
	}
	if client == nil {
		return nil, &ErrorApplicationNotFound
	}

	appEntity, entErr := as.entityService.GetEntityProfile(ctx, client.ID)
	if entErr != nil && !errors.Is(entErr, entity.ErrEntityNotFound) {
		as.logger.Error(ctx, "Failed to load entity for OAuth client",
			log.MaskedString(log.LoggerKeyEntityID, client.ID), log.Error(entErr))
		return nil, &tidcommon.InternalServerError
	}
	if appEntity == nil || appEntity.Category != providers.EntityCategoryApp {
		return nil, &ErrorApplicationNotFound
	}
	return client, nil
}

// GetApplication get the application for given app id.
func (as *applicationService) GetApplication(ctx context.Context, appID string) (*providers.Application,
	*tidcommon.ServiceError) {
	if appID == "" {
		return nil, &ErrorInvalidApplicationID
	}

	fullApp, svcErr := as.getApplication(ctx, appID)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := as.checkApplicationAccess(ctx, security.ActionReadApplication, fullApp.OUID); svcErr != nil {
		return as.getSharedApplication(ctx, fullApp, svcErr)
	}
	app, svcErr := as.enrichApplicationWithCertificate(ctx, buildApplicationResponse(fullApp))
	if svcErr != nil {
		return nil, svcErr
	}
	// Read in its owning organization unit, as a shared application is read in the one it is shared into.
	app.IsOwnedByOU = new(true)
	return app, nil
}

// getSharedApplication answers a read refused in the application's owning organization unit with the
// application as the organization unit the caller is confined to sees it, when the application is shared
// there, and with the refusal otherwise. That unit cannot change it, as applications declare no fields it
// may override.
func (as *applicationService) getSharedApplication(ctx context.Context, fullApp *model.ApplicationProcessedDTO,
	refusal *tidcommon.ServiceError) (*providers.Application, *tidcommon.ServiceError) {
	ouID, svcErr := sharing.SharedIntoCallerOU(ctx, as.authzService, as.sharingService, as.logger,
		ApplicationSharingType, fullApp.ID, fullApp.OUID, refusal)
	if svcErr != nil {
		return nil, svcErr
	}
	if ouID == "" {
		return nil, refusal
	}
	if svcErr := as.checkApplicationAccess(ctx, security.ActionReadApplication, ouID); svcErr != nil {
		return nil, svcErr
	}
	app, svcErr := as.enrichApplicationWithCertificate(ctx, buildApplicationResponse(fullApp))
	if svcErr != nil {
		return nil, svcErr
	}
	app.OUID = ouID
	app.IsOwnedByOU = new(false)
	app.IsReadOnly = true
	return app, nil
}

// sharedApplicationRefusal answers a write refused in the application's owning organization unit with
// ErrorSharedApplicationReadOnly when the application is shared into the organization unit the caller is
// confined to, and with the refusal otherwise.
func (as *applicationService) sharedApplicationRefusal(ctx context.Context, appID, ownerOUID string,
	refusal *tidcommon.ServiceError) *tidcommon.ServiceError {
	ouID, svcErr := sharing.SharedIntoCallerOU(ctx, as.authzService, as.sharingService, as.logger,
		ApplicationSharingType, appID, ownerOUID, refusal)
	if svcErr != nil {
		return svcErr
	}
	if ouID == "" {
		return refusal
	}
	return &ErrorSharedApplicationReadOnly
}

// checkApplicationDeclarative refuses an application declared in a resource file, which cannot be
// changed or removed through the API. An id no entity holds is answered as not found.
func (as *applicationService) checkApplicationDeclarative(
	ctx context.Context, appID string,
) *tidcommon.ServiceError {
	isDeclarative, err := as.entityService.IsEntityDeclarative(ctx, appID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return &ErrorApplicationNotFound
		}
		as.logger.Error(ctx, "Failed to check whether the application is declarative",
			log.String("appID", appID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if isDeclarative {
		return &ErrorCannotModifyDeclarativeResource
	}
	return nil
}

// checkApplicationMutable refuses a change to an application declared in a file, or deployed by a control
// plane unless the change is the managed import's. An id no entity holds is answered as not found.
func (as *applicationService) checkApplicationMutable(
	ctx context.Context, appID string,
) *tidcommon.ServiceError {
	if svcErr := as.checkApplicationDeclarative(ctx, appID); svcErr != nil {
		return svcErr
	}
	return as.refuseManaged(ctx, appID)
}

// refuseManaged refuses a change to an application a control plane deployed, unless the change is the
// managed import's: only a deploy from the control plane changes or removes it.
func (as *applicationService) refuseManaged(ctx context.Context, appID string) *tidcommon.ServiceError {
	managed, err := managedresource.IsReadOnly(ctx, as.managed, appID, managedresource.TypeApplication)
	if err != nil {
		as.logger.Error(ctx, "Failed to read whether a control plane deployed the application",
			log.String("appID", appID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if managed {
		return &ErrorManagedApplication
	}
	return nil
}

// UpdateApplication update the application for given app id.
func (as *applicationService) UpdateApplication(ctx context.Context, appID string, app *model.ApplicationDTO) (
	*model.ApplicationDTO, *tidcommon.ServiceError) {
	updated, svcErr := as.updateApplication(ctx, appID, app)
	as.publishApplicationEvent(ctx, audit.EventApplicationUpdate, audit.ActionUpdate, appID, svcErr)
	return updated, svcErr
}

// updateApplication updates the application for the given app id without publishing its audit event.
func (as *applicationService) updateApplication(ctx context.Context, appID string, app *model.ApplicationDTO) (
	*model.ApplicationDTO, *tidcommon.ServiceError) {
	if appID == "" {
		return nil, &ErrorInvalidApplicationID
	}
	if svcErr := as.checkApplicationMutable(ctx, appID); svcErr != nil {
		return nil, svcErr
	}
	// An update can set the client secret, switch the authentication method or replace the
	// certificate, so it is control over the application.
	if svcErr := as.authzService.CanControlEntity(ctx, appID); svcErr != nil {
		return nil, svcErr
	}
	existingApp, inboundAuthConfig, svcErr := as.validateApplicationForUpdate(ctx, appID, app)

	if svcErr != nil {
		return nil, svcErr
	}

	secretChange, svcErr := as.resolveClientSecretDisposition(ctx, appID, existingApp, inboundAuthConfig)
	if svcErr != nil {
		return nil, svcErr
	}
	processedDTO := as.buildProcessedDTOForUpdate(appID, app, inboundAuthConfig)

	inboundClient := toInboundClient(processedDTO)
	oauthProfile := toOAuthProfile(processedDTO)
	if svcErr := as.resolveAttestationCredentialsForPersist(ctx, appID, &inboundClient); svcErr != nil {
		return nil, svcErr
	}

	var newOAuthClientID string
	if inboundAuthConfig != nil && inboundAuthConfig.OAuthConfig != nil {
		newOAuthClientID = inboundAuthConfig.OAuthConfig.ClientID
	}
	oauthSecretSupplied := inboundAuthConfig != nil &&
		inboundAuthConfig.OAuthConfig != nil &&
		inboundAuthConfig.OAuthConfig.ClientSecret != ""

	write := func() *tidcommon.ServiceError {
		// Update config first, while entity attributes still hold the previous client_id so the
		// inbound client service can clean up the old OAuth-app cert.
		if err := as.inboundClientService.UpdateInboundClient(
			ctx, &inboundClient, oauthProfile, oauthSecretSupplied, newOAuthClientID,
		); err != nil {
			if svcErr := as.translateInboundClientError(ctx, err); svcErr != nil {
				return svcErr
			}
			as.logger.Error(ctx, "Failed to update application", log.Error(err), log.String("appID", appID))
			return &tidcommon.InternalServerError
		}
		return as.updateEntityDataForApplicationUpdate(ctx, appID, app, inboundAuthConfig,
			secretChange != clientSecretUnchanged)
	}
	if secretChange == clientSecretRotated {
		// The caller chose the secret, so a failed post flow is reported as the update's failure.
		var postErr *tidcommon.ServiceError
		if svcErr, postErr = as.rotateClientSecret(ctx, appID, write); svcErr == nil {
			svcErr = postErr
		}
	} else {
		svcErr = write()
	}
	if svcErr != nil {
		return nil, svcErr
	}

	if svcErr := as.cleanupStaleI18nKeys(ctx, appID, existingApp, app); svcErr != nil {
		return nil, svcErr
	}

	as.syncPasskeyOriginsToCORS(ctx, processedDTO.PasskeyAllowedOrigins)

	appForReturn := *app
	appForReturn.AuthFlowID = inboundClient.AuthFlowID
	appForReturn.DeviceGrantFlowID = inboundClient.DeviceGrantFlowID
	appForReturn.RegistrationFlowID = inboundClient.RegistrationFlowID
	appForReturn.RecoveryFlowID = inboundClient.RecoveryFlowID
	appForReturn.SignOutFlowID = inboundClient.SignOutFlowID
	var oauthToken *providers.OAuthTokenConfig
	var userInfo *providers.UserInfoConfig
	var scopeClaims map[string][]string
	if oauthProfile != nil {
		oauthToken = oauthProfile.Token
		userInfo = oauthProfile.UserInfo
		scopeClaims = oauthProfile.ScopeClaims
	}
	if inboundAuthConfig != nil && inboundAuthConfig.OAuthConfig != nil {
		c := inboundAuthConfig.OAuthConfig.Certificate
		if c == nil || c.Type == "" {
			inboundAuthConfig.OAuthConfig.Certificate = nil
		}
	}
	returnDTO := buildReturnApplicationDTO(appID, &appForReturn, inboundClient.Assertion, processedDTO.Metadata,
		inboundAuthConfig, oauthToken, userInfo, scopeClaims)
	// The update changes no runtime state, so the status read before the write still holds.
	returnDTO.Status = existingApp.Status

	as.captureValues(ctx, returnDTO)
	return returnDTO, nil
}

func (as *applicationService) updateEntityDataForApplicationUpdate(ctx context.Context,
	appID string,
	app *model.ApplicationDTO,
	inboundAuthConfig *providers.InboundAuthConfigWithSecret,
	writeClientSecret bool,
) *tidcommon.ServiceError {
	var clientID string
	if inboundAuthConfig != nil && inboundAuthConfig.OAuthConfig != nil {
		clientID = inboundAuthConfig.OAuthConfig.ClientID
	}

	sysAttrsJSON, marshalErr := buildSystemAttributes(app, clientID)
	if marshalErr != nil {
		as.logger.Error(ctx, "Failed to build entity system attributes for update", log.Error(marshalErr))
		return &tidcommon.InternalServerError
	}

	if entErr := as.entityService.UpdateSystemAttributes(ctx, appID, sysAttrsJSON); entErr != nil {
		if svcErr := mapEntityError(entErr); svcErr != nil {
			return svcErr
		}
		as.logger.Error(ctx, "Failed to update entity system attributes",
			log.String("appID", appID), log.Error(entErr))
		return &tidcommon.InternalServerError
	}

	if entErr := as.entityService.UpdateOwnership(ctx, appID, app.OUID, app.ProjectID); entErr != nil {
		if svcErr := mapEntityError(entErr); svcErr != nil {
			return svcErr
		}
		as.logger.Error(ctx, "Failed to update entity organization unit and project",
			log.String("appID", appID), log.Error(entErr))
		return &tidcommon.InternalServerError
	}

	// Rotate the Flow Secret when a new value is supplied (e.g. a regenerate request). Only
	// applications eligible to hold an Flow Secret — backend / server-side apps that are neither
	// public nor redirect-based and have the token-exchange grant — may have one set; a value
	// supplied for an ineligible app is ignored. Credential updates merge, so this preserves the
	// stored client secret, and an empty value leaves the existing Flow Secret intact.
	if app.FlowSecret != "" && isFlowSecretEligible(app.Type, inboundAuthConfig) {
		flowSecretJSON, marshalErr := buildSystemCredentials("", app.FlowSecret)
		if marshalErr != nil {
			as.logger.Error(ctx, "Failed to build flow secret credentials for update", log.Error(marshalErr))
			return &tidcommon.InternalServerError
		}
		if entErr := as.entityService.UpdateSystemCredentials(ctx, appID, flowSecretJSON); entErr != nil {
			if svcErr := mapEntityError(entErr); svcErr != nil {
				return svcErr
			}
			as.logger.Error(ctx, "Failed to update flow secret credentials",
				log.String("appID", appID), log.Error(entErr))
			return &tidcommon.InternalServerError
		}
	}

	// The secret is written only when the client uses one and the caller decided it changed.
	if inboundAuthConfig == nil || inboundAuthConfig.OAuthConfig == nil ||
		!inboundclient.AuthMethodUsesClientSecret(inboundAuthConfig.OAuthConfig.PublicClient,
			inboundAuthConfig.OAuthConfig.TokenEndpointAuthMethod) {
		return nil
	}
	if inboundAuthConfig.OAuthConfig.ClientSecret == "" || !writeClientSecret {
		return nil
	}

	sysCredsJSON, marshalErr := buildSystemCredentials(inboundAuthConfig.OAuthConfig.ClientSecret, "")
	if marshalErr != nil {
		as.logger.Error(ctx, "Failed to build entity system credentials for update", log.Error(marshalErr))
		return &tidcommon.InternalServerError
	}

	if entErr := as.entityService.UpdateSystemCredentials(ctx, appID, sysCredsJSON); entErr != nil {
		if svcErr := mapEntityError(entErr); svcErr != nil {
			return svcErr
		}
		as.logger.Error(ctx, "Failed to update entity system credentials",
			log.String("appID", appID), log.Error(entErr))
		return &tidcommon.InternalServerError
	}

	return nil
}

// isFlowSecretEligible reports whether an application may hold a Flow Secret. Browser (public
// redirect), mobile (attestation), and m2m (direct token) apps never hold one. Full-stack, custom,
// and mcp apps derive eligibility from the OAuth config shape: only confidential, flow-native
// clients (or embedded apps with no OAuth config) are eligible.
func isFlowSecretEligible(appType model.ApplicationType,
	inboundAuthConfig *providers.InboundAuthConfigWithSecret) bool {
	switch appType {
	case model.ApplicationTypeBrowser, model.ApplicationTypeMobile, model.ApplicationTypeM2M:
		return false
	default:
		if inboundAuthConfig == nil || inboundAuthConfig.OAuthConfig == nil {
			return true
		}
		oauthConfig := inboundAuthConfig.OAuthConfig
		return !oauthConfig.PublicClient && model.IsFlowNativeGrantSet(oauthConfig.GrantTypes)
	}
}

// seedClientSubTypeAttribute selects the sub_type claim for a new application that can use the
// client_credentials grant, so its own tokens identify it without being reconfigured. Keyed on the
// grant, not the type: a fullstack or custom application can hold the grant too.
func seedClientSubTypeAttribute(processedDTO *model.ApplicationProcessedDTO) {
	if processedDTO == nil {
		return
	}
	oauthProcessed := getOAuthInboundAuthConfigProcessedDTO(processedDTO.InboundAuthConfig)
	if oauthProcessed == nil || oauthProcessed.OAuthConfig == nil {
		return
	}
	if !slices.Contains(oauthProcessed.OAuthConfig.GrantTypes, providers.GrantTypeClientCredentials) {
		return
	}
	oauthProcessed.OAuthConfig.Token = oauthutils.EnsureClientSubTypeAttribute(oauthProcessed.OAuthConfig.Token)
}

// DeleteApplication delete the application for given app id.
// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (as *applicationService) SetDependencyRegistry(r resourcedependency.Registry) {
	as.dependencyRegistry = r
}

func (as *applicationService) DeleteApplication(ctx context.Context, appID string) *tidcommon.ServiceError {
	svcErr := as.deleteApplication(ctx, appID)
	// A delete that found nothing to remove succeeds without an event, so no caller can record a
	// deletion of an ID it made up.
	if svcErr == errApplicationNotFoundNoop {
		return nil
	}
	as.publishApplicationEvent(ctx, audit.EventApplicationDelete, audit.ActionDelete, appID, svcErr)
	return svcErr
}

// deleteApplication deletes the application for the given app id without publishing its audit event.
func (as *applicationService) deleteApplication(ctx context.Context, appID string) *tidcommon.ServiceError {
	if appID == "" {
		return &ErrorInvalidApplicationID
	}

	inputs := map[string]string{systemFlowInputApplication: appID}
	var shared map[string]string
	ranPreFlow := false
	if existing, entErr := as.entityService.GetEntityProfile(ctx, appID); entErr != nil {
		if !errors.Is(entErr, entity.ErrEntityNotFound) {
			as.logger.Error(ctx, "Failed to load entity before delete",
				log.String("appID", appID), log.Error(entErr))
			return &tidcommon.InternalServerError
		}
	} else if existing != nil {
		if existing.Category != providers.EntityCategoryApp {
			return &ErrorApplicationNotFound
		}
		if svcErr := as.checkApplicationAccess(ctx, security.ActionDeleteApplication,
			existing.OUID); svcErr != nil {
			return as.sharedApplicationRefusal(ctx, appID, existing.OUID, svcErr)
		}
		// Refused before anything is removed. Only an entity that exists can be declared.
		if svcErr := as.checkApplicationMutable(ctx, appID); svcErr != nil {
			return svcErr
		}
		if svcErr := as.authzService.CanControlEntity(ctx, appID); svcErr != nil {
			return svcErr
		}
		// Revokes what the application issued before anything of it is removed.
		var svcErr *tidcommon.ServiceError
		if shared, svcErr = as.runSystemFlow(ctx, systemFlowApplicationDeletePre, inputs, nil); svcErr != nil {
			return svcErr
		}
		ranPreFlow = true
	}

	// Remove dependents that must be deleted with the application (e.g. its role assignments and
	// group memberships). Run before the deletes so a cleanup failure aborts and leaves the
	// application retriable. Fails closed when the registry is unavailable.
	if as.dependencyRegistry == nil {
		as.logger.Error(ctx, "Dependency registry not set; refusing to delete application",
			log.String("appID", appID))
		return &tidcommon.InternalServerError
	}
	if _, err := as.dependencyRegistry.CascadeDelete(
		ctx, resourcedependency.ResourceTypeApplication, appID); err != nil {
		as.logger.Error(ctx, "Failed to cascade-delete application dependencies",
			log.String("appID", appID), log.Error(err))
		return &tidcommon.InternalServerError
	}

	// The sharing policies recorded against the application go with it, for the same reason and in
	// the same position as the cascade above: nothing else removes them, and an application later
	// created with this id would come back already shared. Reshares beneath a policy go with that
	// policy, and each organization unit that loses sight of the application has its own state
	// cleaned up.
	if svcErr := as.sharingService.DeletePoliciesForResource(
		ctx, ApplicationSharingType, appID); svcErr != nil {
		return translateSharingError(svcErr)
	}

	// Delete config. A missing inbound client is non-fatal (e.g. on a retry after a partial
	// delete) so the remaining delete steps still run.
	if appErr := as.inboundClientService.DeleteInboundClient(ctx, appID); appErr != nil &&
		!errors.Is(appErr, inboundclient.ErrInboundClientNotFound) {
		if svcErr := as.translateInboundClientError(ctx, appErr); svcErr != nil {
			return svcErr
		}
		as.logger.Error(ctx, "Failed to delete application", log.Error(appErr), log.String("appID", appID))
		return &tidcommon.InternalServerError
	}
	// Nothing more can be issued to the client once it is gone, so the cutoff moves past that point
	// here, while the entity still exists: a failure keeps the record, and deleting again runs both
	// flows before removing it.
	if ranPreFlow {
		if _, svcErr := as.runSystemFlow(ctx, systemFlowRevocationPost, inputs, shared); svcErr != nil {
			return svcErr
		}
	}
	// Delete entity. A missing entity is non-fatal (e.g. on a retry after a partial delete) so the
	// remaining delete steps still run.
	if entErr := as.entityService.DeleteEntity(ctx, appID); entErr != nil &&
		!errors.Is(entErr, entity.ErrEntityNotFound) {
		if svcErr := mapEntityError(entErr); svcErr != nil {
			return svcErr
		}
		as.logger.Error(ctx, "Failed to delete application entity",
			log.String("appID", appID), log.Error(entErr))
		return &tidcommon.InternalServerError
	}

	if svcErr := as.deleteLocalizedVariants(ctx, appID); svcErr != nil || ranPreFlow {
		return svcErr
	}
	// The pre-deletion flow runs only once the application is found.
	return errApplicationNotFoundNoop
}

// publishApplicationEvent publishes the audit event of an attempt on the application with appID, after
// the change committed or was refused. An update or a delete is refused by a system flow only once the
// application is found. An update or a delete that fails after part of it is stored, a rotated client
// secret whose revocation flow then fails or localized variants left after the entity is deleted, is
// recorded as failed: a known limitation.
func (as *applicationService) publishApplicationEvent(ctx context.Context, name string, action audit.Action,
	appID string, svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the ID the caller asked for, even one that matches no application, as
	// forensic evidence of what was probed; the payload caps it and the sink escapes it.
	target := audit.Ref{Type: audit.TargetTypeApplication, ID: appID}
	eventbus.TryPublish(ctx, as.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))
}

// runSystemFlow runs a system flow with the given inputs, starting from what an earlier run shared, and
// returns what this run shared. With system flows off it runs nothing.
func (as *applicationService) runSystemFlow(ctx context.Context, handle string,
	inputs, runtimeData map[string]string) (map[string]string, *tidcommon.ServiceError) {
	if as.systemFlows == nil {
		return nil, nil
	}
	result, svcErr := as.systemFlows.ExecuteSystemFlow(ctx, &flowcommon.SystemFlowContext{
		Handle: handle, Inputs: inputs, RuntimeData: runtimeData})
	if svcErr != nil {
		return nil, svcErr
	}
	return result.RuntimeData, nil
}

// ValidateDeleteApplication reports whether the application may be deleted and returns the profile of
// the artifacts it has issued, without changing state. Refusals are decided here so the cascade in
// DeleteApplication cannot fail after the deny-list row is written.
func (as *applicationService) ValidateDeleteApplication(ctx context.Context, appID string) (
	*model.ApplicationArtifactProfile, *tidcommon.ServiceError) {
	existing, svcErr := as.loadApplicationEntity(ctx, appID)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := as.checkApplicationAccess(ctx, security.ActionDeleteApplication, existing.OUID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := as.checkApplicationMutable(ctx, appID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := as.authzService.CanControlEntity(ctx, appID); svcErr != nil {
		return nil, svcErr
	}
	if as.dependencyRegistry == nil {
		as.logger.Error(ctx, "Dependency registry not set; refusing to delete application",
			log.String("appID", appID))
		return nil, &tidcommon.InternalServerError
	}
	clientID, err := clientIDFromEntity(existing)
	if err != nil {
		as.logger.Error(ctx, "Failed to read the application's OAuth client id; refusing to delete",
			log.String("appID", appID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if svcErr := as.ensureNoBlockingDependencies(ctx, appID); svcErr != nil {
		return nil, svcErr
	}
	return as.artifactProfile(ctx, clientID), nil
}

// ensureNoBlockingDependencies refuses the deletion while a dependent forbids it, as userService does
// before a user is deleted. Fails closed when a provider cannot report its usage.
func (as *applicationService) ensureNoBlockingDependencies(
	ctx context.Context, appID string) *tidcommon.ServiceError {
	deps, err := as.dependencyRegistry.GetDependencies(
		ctx, resourcedependency.ResourceTypeApplication, appID)
	if err != nil {
		as.logger.Error(ctx, "Failed to evaluate application dependencies",
			log.String("appID", appID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if deps == nil || deps.TotalResults == nil {
		as.logger.Error(ctx, "Application dependency data unavailable; refusing to delete application",
			log.String("appID", appID))
		return &tidcommon.InternalServerError
	}

	blocking := resourcedependency.BlockingUsages(deps)
	if len(blocking) == 0 {
		return nil
	}
	dependencies := resourcedependency.SummarizeBlockingUsages(blocking)
	return tidcommon.CustomServiceError(ErrorApplicationHasBlockingDependencies, tidcommon.I18nMessage{
		Key: "error.applicationservice.application_has_blocking_dependencies_description",
		DefaultValue: fmt.Sprintf(
			"The application cannot be deleted because %s depend on it. Remove or reassign them first.",
			dependencies),
		Params: map[string]string{"dependencies": dependencies},
	})
}

// ValidateClientSecretRegeneration reports whether the caller may regenerate the application's client
// secret and returns the profile of the artifacts its client has issued, without changing state. A client
// that authenticates without a secret has nothing to rotate. The entity service asks it before a
// regeneration, and the flow run before the secret is replaced asks it again.
func (as *applicationService) ValidateClientSecretRegeneration(ctx context.Context, appID string) (
	*entity.ClientSecretProfile, *tidcommon.ServiceError) {
	existing, svcErr := as.loadApplicationEntity(ctx, appID)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := as.checkApplicationAccess(ctx, security.ActionUpdateApplication, existing.OUID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := as.checkApplicationMutable(ctx, appID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := as.authzService.CanControlEntity(ctx, appID); svcErr != nil {
		return nil, svcErr
	}

	clientID, err := clientIDFromEntity(existing)
	if err != nil {
		as.logger.Error(ctx, "Failed to read the application's OAuth client id",
			log.String("appID", appID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if clientID == "" {
		return nil, &ErrorApplicationHasNoClientSecret
	}
	client, err := as.inboundClientService.GetOAuthClientByClientID(ctx, clientID)
	if err != nil || client == nil {
		as.logger.Error(ctx, "Failed to load OAuth client before secret regeneration",
			log.String("appID", appID))
		return nil, &tidcommon.InternalServerError
	}
	if !inboundclient.AuthMethodUsesClientSecret(client.PublicClient, client.TokenEndpointAuthMethod) {
		return nil, &ErrorApplicationHasNoClientSecret
	}
	profile := as.artifactProfileForClient(clientID, client)
	return &entity.ClientSecretProfile{ClientID: profile.ClientKey, MaxLifetimeSeconds: profile.MaxLifetimeSeconds}, nil
}

// RegenerateClientSecret replaces the application's client secret with one generated server-side and
// returns it. This return is the only time it is readable: it is hashed on write and no read path
// returns it. The entity service generates and stores it, revoking what the old secret issued.
func (as *applicationService) RegenerateClientSecret(ctx context.Context, appID string) (
	*model.ClientSecretResponse, *tidcommon.ServiceError) {
	regenerated, svcErr := as.entityService.RegenerateClientSecret(ctx, providers.EntityCategoryApp, appID)
	// A failed revocation after the write is audited as the failure it is, but the secret is still
	// returned: it is already stored, and nobody could read it again, so withholding it would leave the
	// application with a secret no one holds. The response says the revocation is incomplete, so the
	// caller can regenerate again.
	audited := svcErr
	if audited == nil {
		audited = regenerated.RevocationError
	}
	as.publishApplicationEvent(ctx, audit.EventApplicationUpdate, audit.ActionUpdate, appID, audited)
	if svcErr != nil {
		return nil, svcErr
	}
	as.captureRegeneratedSecret(ctx, appID, regenerated.ClientSecret)
	return &model.ClientSecretResponse{ClientSecret: regenerated.ClientSecret,
		RevocationIncomplete: regenerated.RevocationError != nil}, nil
}

// captureRegeneratedSecret hands the application, with the secret just written, to the value capturer, as
// an update that sets the secret does. Only the regeneration still holds the secret, since it is hashed on
// write. A failure to read the application back is logged and leaves the capture out, as any failure to
// capture does: the apply that later needs the value reports it missing.
func (as *applicationService) captureRegeneratedSecret(ctx context.Context, appID, secret string) {
	if as.valueCapturer == nil {
		return
	}
	fullApp, svcErr := as.getApplication(ctx, appID)
	if svcErr != nil {
		as.logger.Warn(ctx, "Failed to read the application back, so its regenerated secret was not captured",
			log.String("appID", appID))
		return
	}
	app := buildApplicationResponse(fullApp)
	for i := range app.InboundAuthConfig {
		if app.InboundAuthConfig[i].OAuthConfig != nil {
			app.InboundAuthConfig[i].OAuthConfig.ClientSecret = secret
		}
	}
	as.valueCapturer.CaptureValues(ctx, resourceTypeApplication, app)
}

// rotateClientSecret writes a client secret the caller chose to replace the application's current one,
// with the system flows a regeneration runs around its write: the pre flow revokes what the old secret
// issued before it is replaced, and the post flow moves the cutoff past the write, so a token issued
// while it ran is revoked too. A refusal of the pre flow writes nothing. A failure of the post flow is
// returned as postErr, after the write, so the caller decides what it means.
func (as *applicationService) rotateClientSecret(ctx context.Context, appID string,
	write func() *tidcommon.ServiceError) (svcErr, postErr *tidcommon.ServiceError) {
	inputs := map[string]string{
		entity.SystemFlowInputEntity:         appID,
		entity.SystemFlowInputEntityCategory: string(providers.EntityCategoryApp),
	}
	shared, svcErr := as.runSystemFlow(ctx, entity.SystemFlowSecretRotatePre, inputs, nil)
	if svcErr != nil {
		return svcErr, nil
	}
	if svcErr := write(); svcErr != nil {
		return svcErr, nil
	}
	_, postErr = as.runSystemFlow(ctx, systemFlowRevocationPost, inputs, shared)
	return nil, postErr
}

// resolveClientSecretDisposition tells what an update does with the supplied client secret. The stored
// secret is a hash, so the supplied value is verified against it the way the token endpoint
// authenticates the client. A client that authenticated without a secret is given one as its first,
// whatever an earlier configuration left stored.
func (as *applicationService) resolveClientSecretDisposition(ctx context.Context, appID string,
	existing *model.ApplicationProcessedDTO, inboundAuthConfig *providers.InboundAuthConfigWithSecret,
) (clientSecretDisposition, *tidcommon.ServiceError) {
	if inboundAuthConfig == nil || inboundAuthConfig.OAuthConfig == nil ||
		inboundAuthConfig.OAuthConfig.ClientSecret == "" ||
		!inboundclient.AuthMethodUsesClientSecret(inboundAuthConfig.OAuthConfig.PublicClient,
			inboundAuthConfig.OAuthConfig.TokenEndpointAuthMethod) {
		return clientSecretUnchanged, nil
	}
	current := getOAuthInboundAuthConfigProcessedDTO(existing.InboundAuthConfig)
	if current == nil || current.OAuthConfig == nil ||
		!inboundclient.AuthMethodUsesClientSecret(current.OAuthConfig.PublicClient,
			current.OAuthConfig.TokenEndpointAuthMethod) {
		return clientSecretSet, nil
	}
	_, _, err := as.entityService.AuthenticateEntityByID(ctx, appID,
		map[string]interface{}{fieldClientSecret: inboundAuthConfig.OAuthConfig.ClientSecret})
	switch {
	case err == nil:
		return clientSecretUnchanged, nil
	case errors.Is(err, entity.ErrAuthenticationFailed):
		return clientSecretRotated, nil
	default:
		as.logger.Error(ctx, "Failed to compare the supplied client secret with the application's",
			log.String("appID", appID), log.Error(err))
		return clientSecretUnchanged, &tidcommon.InternalServerError
	}
}

// loadApplicationEntity returns the application's entity, mapping a missing or non-application record to
// the not-found error the API surfaces.
func (as *applicationService) loadApplicationEntity(ctx context.Context, appID string) (
	*providers.Entity, *tidcommon.ServiceError) {
	if appID == "" {
		return nil, &ErrorInvalidApplicationID
	}
	existing, entErr := as.entityService.GetEntityProfile(ctx, appID)
	if entErr != nil {
		if errors.Is(entErr, entity.ErrEntityNotFound) {
			return nil, &ErrorApplicationNotFound
		}
		as.logger.Error(ctx, "Failed to load application entity",
			log.String("appID", appID), log.Error(entErr))
		return nil, &tidcommon.InternalServerError
	}
	if existing == nil || existing.Category != providers.EntityCategoryApp {
		return nil, &ErrorApplicationNotFound
	}
	return existing, nil
}

// artifactProfile describes the artifacts issued to a client id, sizing their lifetime from that
// client's own token validity. An application with no OAuth client yields an empty profile: it issues
// no artifacts.
func (as *applicationService) artifactProfile(
	ctx context.Context, clientID string) *model.ApplicationArtifactProfile {
	if clientID == "" {
		return &model.ApplicationArtifactProfile{}
	}
	return &model.ApplicationArtifactProfile{
		ClientKey:          clientID,
		MaxLifetimeSeconds: int64(as.resolveArtifactLifetime(ctx, clientID) / time.Second),
	}
}

// artifactProfileForClient builds the same profile from a client the caller has already resolved.
func (as *applicationService) artifactProfileForClient(
	clientID string, client *providers.OAuthClient) *model.ApplicationArtifactProfile {
	return &model.ApplicationArtifactProfile{
		ClientKey:          clientID,
		MaxLifetimeSeconds: int64(as.artifactLifetime(client) / time.Second),
	}
}

// resolveArtifactLifetime returns how long a deny-list row against this application's OAuth client
// must survive: the longest validity any artifact issued to that client can carry, plus the
// authorization-code window. Sizing from the deployment default alone would let an application whose
// validity was raised outlive its own revocation.
//
// Zero means "no opinion", which leaves the revocation service on its configured default.
func (as *applicationService) resolveArtifactLifetime(ctx context.Context, clientID string) time.Duration {
	client, err := as.inboundClientService.GetOAuthClientByClientID(ctx, clientID)
	if err != nil || client == nil {
		as.logger.Warn(ctx, "Failed to resolve application token validity for revocation; "+
			"falling back to the deployment default", log.MaskedString("clientID", clientID))
		return 0
	}
	return as.artifactLifetime(client)
}

// artifactLifetime is resolveArtifactLifetime for a caller that already holds the client, so a path
// that has resolved it does not resolve it a second time.
func (as *applicationService) artifactLifetime(client *providers.OAuthClient) time.Duration {
	if as.resolveLifetime == nil {
		return 0
	}
	return as.resolveLifetime(client)
}

// clientIDFromEntity returns the OAuth client id recorded on the application entity, or an empty
// string when the application genuinely has none. It reads the entity rather than the inbound client so
// the value is still available while a delete is in progress.
//
// An unreadable attribute blob is an error rather than an empty id: callers size a revocation from this
// value, and an empty one means "nothing to revoke". Returning it for a blob that merely failed to parse
// would delete the application while its issued artifacts stay valid.
func clientIDFromEntity(e *providers.Entity) (string, error) {
	if e == nil || len(e.SystemAttributes) == 0 {
		return "", nil
	}
	var sysAttrs map[string]interface{}
	if err := json.Unmarshal(e.SystemAttributes, &sysAttrs); err != nil {
		return "", fmt.Errorf("failed to parse application system attributes: %w", err)
	}
	raw, ok := sysAttrs[fieldClientID]
	if !ok || raw == nil {
		return "", nil
	}
	clientID, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("application client id is not a string")
	}
	return clientID, nil
}

// GetResourceDependencies returns the applications that reference the resource identified
// by (resourceType, id). It implements the resourcedependency.Provider interface. The
// inbound-client store resolves which reference types are tracked. An application falls back from
// a reference it loses, except from an agent type it allows: a deleted handle could be recreated
// in another organization unit and inherit the allowance, so that reference blocks the delete. The
// number of referencing entities is bounded by MaxCompositeStoreRecords (the inbound-client store
// limit).
func (as *applicationService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	if resourceType == resourcedependency.ResourceTypeOU {
		return as.organizationUnitUsages(ctx, id)
	}
	ids, _, err := as.inboundClientService.GetEntityIDsByReference(
		ctx, resourceType, id, serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		as.logger.Error(ctx, "Failed to get entity IDs by reference", log.Error(err))
		return nil, err
	}
	if len(ids) == 0 {
		return []resourcedependency.ResourceDependency{}, nil
	}

	entities, entErr := as.entityService.GetEntitiesByIDs(ctx, ids)
	if entErr != nil {
		as.logger.Error(ctx, "Failed to get entities by IDs", log.Error(entErr))
		return nil, entErr
	}

	behavior := resourcedependency.BehaviorFallback
	if resourceType == resourcedependency.ResourceTypeAgentType {
		behavior = resourcedependency.BehaviorRestrict
	}
	usages := make([]resourcedependency.ResourceDependency, 0, len(entities))
	for _, e := range entities {
		// Applications and agents share the inbound-client store; only report applications.
		if e.Category != providers.EntityCategoryApp {
			continue
		}
		usages = append(usages, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeApplication,
			ID:               e.ID,
			DisplayName:      applicationName(e),
			BehaviorOnDelete: behavior,
		})
	}
	return usages, nil
}

// organizationUnitUsages reports the applications an organization unit owns, which block its deletion. The
// number scanned is bounded by MaxCompositeStoreRecords.
func (as *applicationService) organizationUnitUsages(
	ctx context.Context, ouID string) ([]resourcedependency.ResourceDependency, error) {
	entities, err := as.entityService.GetEntityListByOUIDs(
		ctx, providers.EntityCategoryApp, []string{ouID}, serverconst.MaxCompositeStoreRecords, 0, nil)
	if err != nil {
		as.logger.Error(ctx, "Failed to list applications in organization unit", log.Error(err))
		return nil, err
	}
	usages := make([]resourcedependency.ResourceDependency, 0, len(entities))
	for _, e := range entities {
		usages = append(usages, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeApplication,
			ID:               e.ID,
			DisplayName:      applicationName(e),
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}
	return usages, nil
}

// applicationName reads an application's name from its entity's system attributes, or returns an empty string
// when it has none.
func applicationName(e providers.Entity) string {
	var sysAttrs map[string]interface{}
	if len(e.SystemAttributes) > 0 {
		_ = json.Unmarshal(e.SystemAttributes, &sysAttrs)
	}
	if name, ok := sysAttrs[fieldName].(string); ok {
		return name
	}
	return ""
}

// ValidateReferenceUpdate re-validates every application that references (resourceType, id) after
// that resource has been updated. It implements resourcedependency.UpdateValidator so the registry
// can invoke it from within the update transaction. Only flow updates are handled as of now.
func (as *applicationService) ValidateReferenceUpdate(
	ctx context.Context, resourceType, id string) *tidcommon.ServiceError {
	if resourceType != resourcedependency.ResourceTypeFlow {
		return nil
	}

	ids, _, err := as.inboundClientService.GetEntityIDsByReference(
		ctx, resourceType, id, serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		as.logger.Error(ctx, "Failed to list applications referencing flow for revalidation",
			log.String("flowID", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	for _, entityID := range ids {
		if err := as.inboundClientService.RevalidateFKs(ctx, entityID); err != nil {
			as.logger.Debug(ctx, "Flow update rejected: application FK revalidation failed",
				log.String("flowID", id), log.String("appID", entityID), log.Error(err))

			if svcErr := translateInboundClientFKError(err); svcErr != nil {
				return svcErr
			}

			as.logger.Error(ctx, "Failed to revalidate application after flow update",
				log.String("flowID", id), log.String("appID", entityID), log.Error(err))
			return &tidcommon.InternalServerError
		}
	}

	return nil
}

// isIdentifierTaken checks if an entity with the given identifier already exists.
// If excludeID is non-empty, the entity with that ID is excluded from the check
// (used during declarative loading and updates where the entity already exists).
func (as *applicationService) isIdentifierTaken(
	ctx context.Context, key, value, excludeID string) (bool, *tidcommon.ServiceError) {
	entityID, entErr := as.entityService.IdentifyEntity(ctx, map[string]interface{}{key: value})
	if entErr != nil {
		if errors.Is(entErr, entity.ErrEntityNotFound) {
			return false, nil
		}
		as.logger.Error(ctx, "Failed to check identifier availability",
			log.String("key", key), log.String("value", value), log.Error(entErr))
		return false, &tidcommon.InternalServerError
	}
	if entityID == nil {
		return false, nil
	}
	if excludeID != "" && *entityID == excludeID {
		return false, nil
	}
	return true, nil
}

// getApplication loads entity + config + OAuth config and merges into ApplicationProcessedDTO.
func (as *applicationService) getApplication(
	ctx context.Context, appID string,
) (*model.ApplicationProcessedDTO, *tidcommon.ServiceError) {
	inboundClient, err := as.inboundClientService.GetInboundClientByEntityID(ctx, appID)
	if err != nil {
		return nil, as.mapStoreError(ctx, err)
	}
	if inboundClient == nil {
		return nil, &ErrorApplicationNotFound
	}

	appEntity, entErr := as.entityService.GetEntity(ctx, appID)
	if entErr != nil {
		if errors.Is(entErr, entity.ErrEntityNotFound) {
			appEntity = nil
		} else {
			as.logger.Error(ctx, "Failed to get entity for application",
				log.String("appID", appID), log.Error(entErr))
			return nil, &tidcommon.InternalServerError
		}
	}

	if appEntity != nil && appEntity.Category != providers.EntityCategoryApp {
		return nil, &ErrorApplicationNotFound
	}

	oauthProfile, err := as.inboundClientService.GetOAuthProfileByEntityID(ctx, appID)
	if err != nil && !errors.Is(err, inboundclient.ErrInboundClientNotFound) {
		as.logger.Error(ctx, "Failed to get OAuth profile for application",
			log.String("appID", appID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	dto := toProcessedDTO(appEntity, inboundClient, oauthProfile)
	if appEntity != nil {
		access := as.governance.ReadEntityAccess(ctx, *appEntity)
		dto.Status = &providers.ApplicationStatusReport{Value: access.Status.Value}
	}
	return dto, nil
}

// accessStatuses derives each application's status, in order, without the suspension details.
func (as *applicationService) accessStatuses(ctx context.Context,
	entities []providers.Entity) []*providers.ApplicationStatusReport {
	reads := as.governance.ReadAccess(ctx, entities)
	statuses := make([]*providers.ApplicationStatusReport, len(reads))
	for i := range reads {
		statuses[i] = &providers.ApplicationStatusReport{Value: reads[i].Status.Value}
	}
	return statuses
}

// mapEntityError maps entity service sentinel errors to application service errors. Schema
// validation and attribute conflict are omitted: the entity service raises them only for
// categories that use an entity type, which excludes applications.
func mapEntityError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, entity.ErrEntityNotFound):
		return &ErrorApplicationNotFound
	case errors.Is(err, entity.ErrInvalidCredential):
		return &ErrorInvalidCredential
	}
	return nil
}

// toInboundClient extracts gateway config fields from a full ApplicationProcessedDTO.
func toInboundClient(dto *model.ApplicationProcessedDTO) inboundmodel.InboundClient {
	dao := inboundmodel.InboundClient{
		ID:                        dto.ID,
		AuthFlowID:                dto.AuthFlowID,
		DeviceGrantFlowID:         dto.DeviceGrantFlowID,
		RegistrationFlowID:        dto.RegistrationFlowID,
		IsRegistrationFlowEnabled: dto.IsRegistrationFlowEnabled,
		RecoveryFlowID:            dto.RecoveryFlowID,
		IsRecoveryFlowEnabled:     dto.IsRecoveryFlowEnabled,
		SignOutFlowID:             dto.SignOutFlowID,
		ThemeID:                   dto.ThemeID,
		LayoutID:                  dto.LayoutID,
		Assertion:                 dto.Assertion,
		LoginConsent:              dto.LoginConsent,
		AllowedUserTypes:          dto.AllowedUserTypes,
		AllowedAgentTypes:         dto.AllowedAgentTypes,
		SubjectAttribute:          dto.SubjectAttribute,
		PasskeyAllowedOrigins:     dto.PasskeyAllowedOrigins,
		Attestation:               dto.Attestation,
	}

	// Pack remaining fields into Properties.
	props := make(map[string]interface{})
	if dto.URL != "" {
		props[propURL] = dto.URL
	}
	if dto.LogoURL != "" {
		props[propLogoURL] = dto.LogoURL
	}
	if dto.TosURI != "" {
		props[propTosURI] = dto.TosURI
	}
	if dto.PolicyURI != "" {
		props[propPolicyURI] = dto.PolicyURI
	}
	if len(dto.Contacts) > 0 {
		props[propContacts] = dto.Contacts
	}
	if dto.Type != "" {
		props[propType] = string(dto.Type)
	}
	if dto.Template != "" {
		props[propTemplate] = dto.Template
	}
	if dto.Metadata != nil {
		props[propMetadata] = dto.Metadata
	}
	if len(props) > 0 {
		dao.Properties = props
	}

	return dao
}

// toProcessedDTO merges entity identity data with store config into a full
// ApplicationProcessedDTO.
func toProcessedDTO(
	e *providers.Entity, dao *inboundmodel.InboundClient, oauthProfile *providers.OAuthProfile,
) *model.ApplicationProcessedDTO {
	dto := &model.ApplicationProcessedDTO{
		ID: dao.ID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                dao.AuthFlowID,
			DeviceGrantFlowID:         dao.DeviceGrantFlowID,
			RegistrationFlowID:        dao.RegistrationFlowID,
			IsRegistrationFlowEnabled: dao.IsRegistrationFlowEnabled,
			RecoveryFlowID:            dao.RecoveryFlowID,
			IsRecoveryFlowEnabled:     dao.IsRecoveryFlowEnabled,
			SignOutFlowID:             dao.SignOutFlowID,
			ThemeID:                   dao.ThemeID,
			LayoutID:                  dao.LayoutID,
			Assertion:                 dao.Assertion,
			LoginConsent:              dao.LoginConsent,
			AllowedUserTypes:          dao.AllowedUserTypes,
			AllowedAgentTypes:         dao.AllowedAgentTypes,
			SubjectAttribute:          dao.SubjectAttribute,
			PasskeyAllowedOrigins:     dao.PasskeyAllowedOrigins,
			Attestation:               dao.Attestation.WithoutCredentials(),
		},
	}

	// Extract identity fields from entity system attributes.
	if e != nil {
		dto.OUID = e.OUID
		dto.ProjectID = e.ProjectID
		var sysAttrs map[string]interface{}
		if len(e.SystemAttributes) > 0 {
			_ = json.Unmarshal(e.SystemAttributes, &sysAttrs)
		}
		if sysAttrs != nil {
			if name, ok := sysAttrs[fieldName].(string); ok {
				dto.Name = name
			}
			if desc, ok := sysAttrs[fieldDescription].(string); ok {
				dto.Description = desc
			}
		}
	}

	// Extract remaining fields from Properties.
	if dao.Properties != nil {
		if url, ok := dao.Properties[propURL].(string); ok {
			dto.URL = url
		}
		if logoURL, ok := dao.Properties[propLogoURL].(string); ok {
			dto.LogoURL = logoURL
		}
		if tosURI, ok := dao.Properties[propTosURI].(string); ok {
			dto.TosURI = tosURI
		}
		if policyURI, ok := dao.Properties[propPolicyURI].(string); ok {
			dto.PolicyURI = policyURI
		}
		switch contacts := dao.Properties[propContacts].(type) {
		case []string:
			dto.Contacts = append(dto.Contacts, contacts...)
		case []interface{}:
			for _, c := range contacts {
				if s, ok := c.(string); ok {
					dto.Contacts = append(dto.Contacts, s)
				}
			}
		}
		if t, ok := dao.Properties[propType].(string); ok {
			dto.Type = model.ApplicationType(t)
		}
		if template, ok := dao.Properties[propTemplate].(string); ok {
			dto.Template = template
		}
		if metadata, ok := dao.Properties[propMetadata].(map[string]interface{}); ok {
			dto.Metadata = metadata
		}
	}

	// Merge OAuth profile if present.
	if oauthProfile != nil {
		var clientID string
		if e != nil {
			var sysAttrs map[string]interface{}
			if len(e.SystemAttributes) > 0 {
				_ = json.Unmarshal(e.SystemAttributes, &sysAttrs)
			}
			if sysAttrs != nil {
				if cid, ok := sysAttrs[fieldClientID].(string); ok {
					clientID = cid
				}
			}
		}

		var ouID string
		if e != nil {
			ouID = e.OUID
		}
		oauthProcessed := inboundclient.BuildOAuthClient(
			dao.ID, clientID, ouID, providers.EntityCategoryApp, oauthProfile)
		dto.InboundAuthConfig = []inboundmodel.InboundAuthConfigProcessed{
			{Type: providers.OAuthInboundAuthType, OAuthConfig: oauthProcessed},
		}
	}

	return dto
}

// toOAuthProfile builds the typed OAuth config from a processed DTO for store persistence.
// Returns nil when no OAuth inbound config is present.
func toOAuthProfile(processedDTO *model.ApplicationProcessedDTO) *providers.OAuthProfile {
	oauthProcessed := getOAuthInboundAuthConfigProcessedDTO(processedDTO.InboundAuthConfig)
	if oauthProcessed == nil || oauthProcessed.OAuthConfig == nil {
		return nil
	}
	return buildOAuthProfileFromProcessed(*oauthProcessed)
}

// buildOAuthProfileFromProcessed builds a typed OAuthProfile from an InboundAuthConfigProcessed.
// Returns nil if the inbound auth config has no OAuth application config.
func buildOAuthProfileFromProcessed(inboundAuth inboundmodel.InboundAuthConfigProcessed) *providers.OAuthProfile {
	if inboundAuth.OAuthConfig == nil {
		return nil
	}
	oa := inboundAuth.OAuthConfig
	return &providers.OAuthProfile{
		RedirectURIs:                       oa.RedirectURIs,
		PostLogoutRedirectURIs:             oa.PostLogoutRedirectURIs,
		BackchannelLogoutURI:               oa.BackchannelLogoutURI,
		GrantTypes:                         sysutils.ConvertToStringSlice(oa.GrantTypes),
		ResponseTypes:                      sysutils.ConvertToStringSlice(oa.ResponseTypes),
		TokenEndpointAuthMethod:            string(oa.TokenEndpointAuthMethod),
		PKCERequired:                       oa.PKCERequired,
		PublicClient:                       oa.PublicClient,
		RequirePushedAuthorizationRequests: oa.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              oa.DPoPBoundAccessTokens,
		IncludeActClaim:                    oa.IncludeActClaim,
		ClientIDMetadataDocument:           oa.ClientIDMetadataDocument,
		Scopes:                             oa.Scopes,
		ScopeClaims:                        oa.ScopeClaims,
		Token:                              oa.Token,
		UserInfo:                           oa.UserInfo,
		Certificate:                        oa.Certificate,
		AcrValues:                          oa.AcrValues,
	}
}

// buildSystemAttributes builds the system attributes JSON for the entity.
func buildSystemAttributes(app *model.ApplicationDTO, clientID string) (json.RawMessage, error) {
	sysAttrs := map[string]interface{}{
		fieldName: app.Name,
	}
	if app.Description != "" {
		sysAttrs[fieldDescription] = app.Description
	}
	if clientID != "" {
		sysAttrs[fieldClientID] = clientID
	}
	return json.Marshal(sysAttrs)
}

// buildAppEntity constructs an entity and system credentials for entity creation.
func buildAppEntity(appID string, app *model.ApplicationDTO, clientID string, plaintextSecret string,
	flowSecret string) (*providers.Entity, json.RawMessage, error) {
	sysAttrsJSON, err := buildSystemAttributes(app, clientID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build entity system attributes: %w", err)
	}

	sysCredsJSON, err := buildSystemCredentials(plaintextSecret, flowSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build entity system credentials: %w", err)
	}

	e := &providers.Entity{
		ID:               appID,
		Category:         providers.EntityCategoryApp,
		Type:             "application",
		State:            providers.EntityStateActive,
		OUID:             app.OUID,
		ProjectID:        app.ProjectID,
		SystemAttributes: sysAttrsJSON,
	}
	return e, sysCredsJSON, nil
}

// buildSystemCredentials builds the system credentials JSON for the entity. Both the OAuth client
// secret and the Flow Secret are optional; only non-empty values are included.
func buildSystemCredentials(clientSecret string, flowSecret string) (json.RawMessage, error) {
	creds := map[string]interface{}{}
	if clientSecret != "" {
		creds[fieldClientSecret] = clientSecret
	}
	if flowSecret != "" {
		creds[fieldFlowSecret] = flowSecret
	}
	if len(creds) == 0 {
		return nil, nil
	}

	return json.Marshal(creds)
}

// getOAuthInboundAuthConfigDTO returns the single OAuth InboundAuthConfigDTO.
// It returns an error if multiple OAuth configs are found, nil if none exist.
func getOAuthInboundAuthConfigDTO(
	configs []providers.InboundAuthConfigWithSecret,
) (*providers.InboundAuthConfigWithSecret, *tidcommon.ServiceError) {
	var cfg *providers.InboundAuthConfigWithSecret
	for i := range configs {
		if configs[i].Type == providers.OAuthInboundAuthType {
			if cfg != nil {
				return nil, &ErrorInvalidInboundAuthConfig
			}
			cfg = &configs[i]
		}
	}
	return cfg, nil
}

// getOAuthInboundAuthConfigProcessedDTO returns the first OAuth InboundAuthConfigProcessedDTO, or nil.
func getOAuthInboundAuthConfigProcessedDTO(
	configs []inboundmodel.InboundAuthConfigProcessed,
) *inboundmodel.InboundAuthConfigProcessed {
	for i := range configs {
		if configs[i].Type == providers.OAuthInboundAuthType {
			return &configs[i]
		}
	}
	return nil
}

func (as *applicationService) validateApplicationForUpdate(
	ctx context.Context, appID string, app *model.ApplicationDTO) (
	*model.ApplicationProcessedDTO, *providers.InboundAuthConfigWithSecret, *tidcommon.ServiceError) {
	if appID == "" {
		return nil, nil, &ErrorInvalidApplicationID
	}
	if app == nil {
		return nil, nil, &ErrorApplicationNil
	}
	if app.Name == "" {
		return nil, nil, &ErrorInvalidApplicationName
	}

	existingApp, existingAppErr := as.getApplication(ctx, appID)
	if existingAppErr != nil {
		return nil, nil, existingAppErr
	}
	if svcErr := as.checkApplicationAccess(ctx, security.ActionUpdateApplication,
		existingApp.OUID); svcErr != nil {
		return nil, nil, as.sharedApplicationRefusal(ctx, appID, existingApp.OUID, svcErr)
	}

	// If the application name is changed, check if an application with the new name already exists.
	if existingApp.Name != app.Name {
		nameExists, nameCheckErr := as.isIdentifierTaken(ctx, fieldName, app.Name, appID)
		if nameCheckErr != nil {
			return nil, nil, nameCheckErr
		}
		if nameExists {
			return nil, nil, &ErrorApplicationAlreadyExistsWithName
		}
	}

	// An update that names no organization unit keeps the application in its own.
	if app.OUID == "" && app.OUHandle == "" {
		app.OUID = existingApp.OUID
	}
	requestedProjectID := app.ProjectID
	if svcErr := as.validateApplicationFields(ctx, app); svcErr != nil {
		return nil, nil, svcErr
	}
	// Moving the application places it in another organization unit, which the caller must be allowed
	// to act for too.
	if app.OUID != existingApp.OUID {
		if svcErr := as.checkApplicationAccess(ctx, security.ActionUpdateApplication, app.OUID); svcErr != nil {
			return nil, nil, svcErr
		}
	}
	// An update that names no project keeps the application in its project, so the organization unit
	// must still belong to it.
	if requestedProjectID == "" && existingApp.ProjectID != "" {
		app.ProjectID = existingApp.ProjectID
		if svcErr := as.validateProjectOU(ctx, app.ProjectID, app.OUID); svcErr != nil {
			return nil, nil, svcErr
		}
	}

	// The application type is immutable. Reject a request that attempts to change it; inherit the
	// existing type when the request omits it.
	if app.Type != "" && app.Type != existingApp.Type {
		return nil, nil, &ErrorApplicationTypeImmutable
	}
	app.Type = existingApp.Type

	// A shared application stays in its organization unit and project: its policies were issued
	// against that unit.
	if app.OUID != existingApp.OUID || app.ProjectID != existingApp.ProjectID {
		policies, svcErr := as.sharingService.ListPolicies(ctx, ApplicationSharingType, appID)
		if svcErr != nil {
			return nil, nil, translateSharingError(svcErr)
		}
		if len(policies) > 0 {
			return nil, nil, &ErrorSharedApplicationCannotMove
		}
	}

	inboundAuthConfig, svcErr := as.processInboundAuthConfig(ctx, app, existingApp)
	if svcErr != nil {
		return nil, nil, svcErr
	}

	return existingApp, inboundAuthConfig, nil
}

// validateApplicationFields validates application fields that are common to both create and update operations.
func (as *applicationService) validateApplicationFields(
	ctx context.Context, app *model.ApplicationDTO) *tidcommon.ServiceError {
	// Resolve ou_handle to an ID when the direct ID is absent.
	// If both are provided, ou_id wins and a warning is logged.
	if app.OUID != "" && app.OUHandle != "" {
		as.logger.Warn(ctx, "Both ou_id and ou_handle provided for application; ou_handle ignored",
			log.String("appID", app.ID), log.String("name", app.Name))
	} else if app.OUID == "" && app.OUHandle != "" {
		ou, svcErr := as.ouService.GetOrganizationUnitByPath(ctx, app.OUHandle)
		if svcErr != nil {
			return &ErrorInvalidRequestFormat
		}
		app.OUID = ou.ID
	}
	if app.OUID == "" {
		systemOUID, svcErr := as.ouService.GetSystemOUID(ctx)
		if svcErr != nil {
			return svcErr
		}
		app.OUID = systemOUID
	}
	// A project checks that the application's organization unit belongs to it.
	if app.ProjectID != "" {
		if svcErr := as.validateProjectOU(ctx, app.ProjectID, app.OUID); svcErr != nil {
			return svcErr
		}
	}
	// Resolve flow handles to IDs when the direct IDs are absent.
	if err := as.inboundClientService.ResolveInboundAuthProfileHandles(ctx, &app.InboundAuthProfile); err != nil {
		return &ErrorInvalidRequestFormat
	}
	// Validate organization unit ID.
	if app.OUID == "" {
		return &ErrorInvalidRequestFormat
	}
	if exists, err := as.ouService.IsOrganizationUnitExists(ctx, app.OUID); err != nil || !exists {
		return &ErrorInvalidRequestFormat
	}

	if app.URL != "" && !sysutils.IsValidURI(app.URL) {
		return &ErrorInvalidApplicationURL
	}
	if app.LogoURL != "" && !sysutils.IsValidLogoURI(app.LogoURL) {
		return &ErrorInvalidLogoURL
	}
	if app.TosURI != "" && !sysutils.IsValidURI(app.TosURI) {
		return &ErrorInvalidTosURI
	}
	if app.PolicyURI != "" && !sysutils.IsValidURI(app.PolicyURI) {
		return &ErrorInvalidPolicyURI
	}
	// Reject an unrecognized application type. Requiring a type (create) and enforcing
	// immutability (update) are handled by the respective callers.
	switch app.Type {
	case "", model.ApplicationTypeBrowser, model.ApplicationTypeFullStack,
		model.ApplicationTypeMobile, model.ApplicationTypeM2M, model.ApplicationTypeMCP,
		model.ApplicationTypeCustom:
	default:
		return &ErrorInvalidApplicationType
	}
	// Reject requests with more than one OAuth-typed inbound auth entry — at most one
	// inbound auth config per protocol per application is allowed.
	isOAuthConfig := false
	for i := range app.InboundAuthConfig {
		if app.InboundAuthConfig[i].Type != providers.OAuthInboundAuthType {
			continue
		}
		if isOAuthConfig {
			return &ErrorMultipleOAuthConfigs
		}
		isOAuthConfig = true
	}
	as.validateConsentConfig(app)

	// An attestation config identifies exactly one platform build of the app; the verifier dispatch
	// cannot pick between two simultaneously.
	if attestation := app.Attestation; attestation != nil &&
		attestation.Android != nil && attestation.Apple != nil {
		return &ErrorAmbiguousAttestationConfig
	}
	return nil
}

// validateConsentConfig validates the consent configuration for the application.
func (as *applicationService) validateConsentConfig(appDTO *model.ApplicationDTO) {
	if appDTO.LoginConsent == nil {
		appDTO.LoginConsent = &inboundmodel.LoginConsentConfig{
			ValidityPeriod: 0,
		}

		return
	}

	if appDTO.LoginConsent.ValidityPeriod < 0 {
		appDTO.LoginConsent.ValidityPeriod = 0
	}
}

// validateOAuthParamsForCreateAndUpdate validates the OAuth parameters for creating or updating an application.
func validateOAuthParamsForCreateAndUpdate(app *model.ApplicationDTO) (*providers.InboundAuthConfigWithSecret,
	*tidcommon.ServiceError) {
	if len(app.InboundAuthConfig) == 0 {
		return nil, nil
	}

	inboundAuthConfig, svcErr := getOAuthInboundAuthConfigDTO(app.InboundAuthConfig)
	if svcErr != nil {
		return nil, svcErr
	}
	if inboundAuthConfig == nil {
		return nil, &ErrorInvalidInboundAuthConfig
	}
	if inboundAuthConfig.OAuthConfig == nil {
		return nil, &ErrorInvalidInboundAuthConfig
	}

	oauthAppConfig := inboundAuthConfig.OAuthConfig

	if len(oauthAppConfig.GrantTypes) == 0 {
		oauthAppConfig.GrantTypes = []providers.GrantType{providers.GrantTypeAuthorizationCode}
	}
	// Browser-based SPAs (public clients) must use the redirect-based authorization_code flow.
	// A public client without the authorization_code grant is configured for direct (native)
	// flow execution, which is not permitted for single-page applications.
	if oauthAppConfig.PublicClient &&
		!slices.Contains(oauthAppConfig.GrantTypes, providers.GrantTypeAuthorizationCode) {
		return nil, &ErrorNativeFlowNotAllowedForSPA
	}
	if len(oauthAppConfig.ResponseTypes) == 0 {
		if slices.Contains(oauthAppConfig.GrantTypes, providers.GrantTypeAuthorizationCode) {
			oauthAppConfig.ResponseTypes = []providers.ResponseType{providers.ResponseTypeCode}
		}
	}
	if oauthAppConfig.TokenEndpointAuthMethod == "" {
		oauthAppConfig.TokenEndpointAuthMethod = providers.TokenEndpointAuthMethodClientSecretBasic
	}

	if err := validateAcrValues(oauthAppConfig.AcrValues); err != nil {
		return nil, err
	}

	return inboundAuthConfig, nil
}

// isValidACR reports whether acr is present in the deployment config ACR-AMR mapping.
func isValidACR(acr string) bool {
	mapping := config.GetServerRuntime().Config.OAuth.AuthClass
	_, ok := mapping.AcrAMR[acr]
	return ok
}

// validateAcrValues rejects acr values not registered in the ACR-AMR mapping.
func validateAcrValues(acrValues []string) *tidcommon.ServiceError {
	for _, acr := range acrValues {
		if !isValidACR(acr) {
			return tidcommon.CustomServiceError(ErrorInvalidAcrValues, tidcommon.I18nMessage{
				Key:          "error.applicationservice.invalid_acr_values_unrecognized",
				DefaultValue: "ACR value '{{param(acr)}}' is not recognized by the system",
				Params:       map[string]string{"acr": acr},
			})
		}
	}
	return nil
}

// translateInboundClientError maps inbound-client sentinel errors and typed wrappers to
// application-service errors. Returns nil when the input does not correspond to a known
// inbound-client error, allowing the caller to log and fall back to InternalServerError.
func (as *applicationService) translateInboundClientError(ctx context.Context, err error) *tidcommon.ServiceError {
	if err == nil {
		return nil
	}
	if errors.Is(err, inboundclient.ErrCannotModifyDeclarative) {
		return &ErrorCannotModifyDeclarativeResource
	}
	if svcErr := translateInboundClientFKError(err); svcErr != nil {
		return svcErr
	}
	if svcErr := translateOAuthValidationError(err); svcErr != nil {
		return svcErr
	}
	if svcErr := translateUserInfoValidationError(err); svcErr != nil {
		return svcErr
	}
	if svcErr := translateIDTokenValidationError(err); svcErr != nil {
		return svcErr
	}
	if svcErr := translateCertValidationError(err); svcErr != nil {
		return svcErr
	}
	var opErr *inboundclient.CertOperationError
	if errors.As(err, &opErr) {
		return as.translateCertOperationError(ctx, opErr)
	}
	if svcErr := translateCIMDValidationError(err); svcErr != nil {
		return svcErr
	}
	return nil
}

// translateCIMDValidationError maps a Client ID Metadata Document rule violation to an
// application-service error that keeps the description of the broken rule.
func translateCIMDValidationError(err error) *tidcommon.ServiceError {
	var cimdErr *inboundclient.CIMDValidationError
	if !errors.As(err, &cimdErr) {
		return nil
	}
	return tidcommon.CustomServiceError(ErrorInvalidCIMDClient, cimdErr.Underlying.ErrorDescription)
}

// translateOAuthValidationError maps OAuth redirect URI, grant/response type, token endpoint
// auth method, and public client validation sentinels to application-service errors.
func translateOAuthValidationError(err error) *tidcommon.ServiceError {
	switch {
	// OAuth: redirect URI
	case errors.Is(err, inboundclient.ErrOAuthInvalidRedirectURI):
		return &ErrorInvalidRedirectURI
	case errors.Is(err, inboundclient.ErrOAuthRedirectURIFragmentNotAllowed):
		return tidcommon.CustomServiceError(ErrorInvalidRedirectURI, tidcommon.I18nMessage{
			Key:          "error.applicationservice.redirect_uri_fragment_not_allowed_description",
			DefaultValue: "Redirect URIs must not contain a fragment component",
		})
	case errors.Is(err, inboundclient.ErrOAuthAuthCodeRequiresRedirectURIs):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.auth_code_requires_redirect_uris_description",
			DefaultValue: "authorization_code grant type requires redirect URIs",
		})
	case errors.Is(err, inboundclient.ErrOAuthInvalidBackchannelLogoutURI):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.applicationservice.invalid_backchannel_logout_uri_description",
			DefaultValue: "Backchannel logout URI must be an absolute http or https URL without userinfo, " +
				"a fragment or a wildcard",
		})
	case errors.Is(err, inboundclient.ErrOAuthBackchannelLogoutURIRequiresHTTPS):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.backchannel_logout_uri_requires_https_description",
			DefaultValue: "Backchannel logout URI must use https for a public client",
		})
	case errors.Is(err, inboundclient.ErrOAuthBackchannelLogoutURIPrivateHost):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.backchannel_logout_uri_private_host_description",
			DefaultValue: "Backchannel logout URI must not name a loopback, link-local or private address",
		})

	// OAuth: grant + response type
	case errors.Is(err, inboundclient.ErrOAuthInvalidGrantType):
		return &ErrorInvalidGrantType
	case errors.Is(err, inboundclient.ErrOAuthInvalidResponseType):
		return &ErrorInvalidResponseType
	case errors.Is(err, inboundclient.ErrOAuthClientCredentialsCannotUseResponseTypes):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.client_credentials_cannot_use_response_types_description",
			DefaultValue: "client_credentials grant type cannot be used with response types",
		})
	case errors.Is(err, inboundclient.ErrOAuthAuthCodeRequiresCodeResponseType):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.auth_code_requires_code_response_type_description",
			DefaultValue: "authorization_code grant type requires 'code' response type",
		})
	case errors.Is(err, inboundclient.ErrOAuthRefreshTokenRequiresTokenIssuingGrant):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.refresh_token_requires_token_issuing_grant_description",
			DefaultValue: "refresh_token grant type requires a token-issuing grant type",
		})
	case errors.Is(err, inboundclient.ErrOAuthPKCERequiresAuthCode):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.pkce_requires_authorization_code_description",
			DefaultValue: "PKCE can only be enabled when the authorization_code grant type is selected",
		})
	case errors.Is(err, inboundclient.ErrOAuthResponseTypesRequireAuthCode):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.response_types_require_authorization_code_description",
			DefaultValue: "Response types can only be configured with the authorization_code grant type",
		})

	// OAuth: token endpoint auth method
	case errors.Is(err, inboundclient.ErrOAuthInvalidTokenEndpointAuthMethod):
		return &ErrorInvalidTokenEndpointAuthMethod
	case errors.Is(err, inboundclient.ErrOAuthPrivateKeyJWTRequiresCertificate):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.private_key_jwt_requires_certificate_description",
			DefaultValue: "private_key_jwt authentication method requires a certificate",
		})
	case errors.Is(err, inboundclient.ErrOAuthCertificateRequiresClientID):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.certificate_requires_client_id_description",
			DefaultValue: "certificate configuration requires an OAuth client ID",
		})
	case errors.Is(err, inboundclient.ErrOAuthPrivateKeyJWTCannotHaveClientSecret):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.private_key_jwt_cannot_have_client_secret_description",
			DefaultValue: "private_key_jwt authentication method cannot have a client secret",
		})
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthRequiresPublicClient):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.none_auth_method_requires_public_client_description",
			DefaultValue: "'none' authentication method requires the client to be a public client",
		})
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthCannotHaveSecret):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.none_auth_method_cannot_have_secret_description",
			DefaultValue: "'none' authentication method cannot have a client secret",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientCredentialsCannotUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.client_credentials_cannot_use_none_auth_description",
			DefaultValue: "client_credentials grant type cannot use 'none' authentication method",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientJWTBearerCannotUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.applicationservice.jwt_bearer_cannot_use_none_auth_description",
			DefaultValue: "jwt-bearer grant type requires a confidential client and cannot use 'none' " +
				"token endpoint authentication method",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientIDJAGCannotUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.applicationservice.idjag_cannot_use_none_auth_description",
			DefaultValue: "ID-JAG configuration requires a confidential client and cannot use 'none' " +
				"token endpoint authentication method",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientIDJAGRequiresTokenExchange):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.applicationservice.idjag_requires_token_exchange_description",
			DefaultValue: "ID-JAG configuration requires the " +
				"urn:ietf:params:oauth:grant-type:token-exchange grant type",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientIDJAGRequiresAllowedAudience):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.applicationservice.idjag_requires_allowed_audience_description",
			DefaultValue: "ID-JAG configuration requires at least one allowed audience, and allowed " +
				"audiences must not be blank",
		})

	// OAuth: public client
	case errors.Is(err, inboundclient.ErrOAuthPublicClientMustUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidPublicClientConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.public_client_must_use_none_auth_description",
			DefaultValue: "Public clients must use 'none' as token endpoint authentication method",
		})
	case errors.Is(err, inboundclient.ErrOAuthPublicClientMustHavePKCE):
		return tidcommon.CustomServiceError(ErrorInvalidPublicClientConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.public_client_must_have_pkce_description",
			DefaultValue: "Public clients must have PKCE required set to true",
		})
	}
	return nil
}

// translateUserInfoValidationError maps OAuth userinfo validation sentinels to
// application-service errors.
func translateUserInfoValidationError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedSigningAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_unsupported_signing_alg_description",
			DefaultValue: "userinfo signing algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedEncryptionAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_unsupported_encryption_alg_description",
			DefaultValue: "userinfo encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedEncryptionEnc):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_unsupported_encryption_enc_description",
			DefaultValue: "userinfo content-encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoEncryptionAlgRequiresEnc):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_encryption_alg_requires_enc_description",
			DefaultValue: "userinfo encryptionEnc is required when encryptionAlg is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoEncryptionEncRequiresAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_encryption_enc_requires_alg_description",
			DefaultValue: "userinfo encryptionAlg is required when encryptionEnc is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoEncryptionRequiresCertificate):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_encryption_requires_certificate_description",
			DefaultValue: "a certificate (JWKS or JWKS_URI) is required when userinfo encryption is configured",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWKSURINotSSRFSafe):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_jwks_uri_not_ssrf_safe_description",
			DefaultValue: "userinfo JWKS URI must be a publicly reachable HTTPS URL",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoUnsupportedResponseType):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_unsupported_response_type_description",
			DefaultValue: "userinfo responseType is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWERequiresEncryption):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_jwe_requires_encryption_description",
			DefaultValue: "encryptionAlg and encryptionEnc are required when userinfo responseType is JWE",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoNestedJWTRequiresAll):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.applicationservice.userinfo_nested_jwt_requires_all_description",
			DefaultValue: "encryptionAlg and encryptionEnc are required " +
				"when userinfo responseType is NESTED_JWT",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoAlgRequiresResponseType):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_alg_requires_response_type_description",
			DefaultValue: "userinfo responseType is required when signingAlg or encryptionAlg is set",
		})
	}
	return nil
}

// translateIDTokenValidationError maps OAuth ID token validation sentinels to
// application-service errors.
func translateIDTokenValidationError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedSigningAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.idtoken_unsupported_signing_alg_description",
			DefaultValue: "idToken signing algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionFieldsNotAllowed):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.idtoken_encryption_fields_not_allowed_description",
			DefaultValue: "idToken encryptionAlg and encryptionEnc must not be set when responseType is JWT",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedResponseType):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.idtoken_unsupported_response_type_description",
			DefaultValue: "ID token responseType is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedEncryptionAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.idtoken_unsupported_encryption_alg_description",
			DefaultValue: "ID token encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenUnsupportedEncryptionEnc):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.idtoken_unsupported_encryption_enc_description",
			DefaultValue: "ID token content-encryption algorithm is not supported",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionAlgRequiresEnc):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.idtoken_encryption_alg_requires_enc_description",
			DefaultValue: "idToken encryptionEnc is required when encryptionAlg is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionEncRequiresAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.idtoken_encryption_enc_requires_alg_description",
			DefaultValue: "idToken encryptionAlg is required when encryptionEnc is set",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenEncryptionRequiresCertificate):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.idtoken_encryption_requires_certificate_description",
			DefaultValue: "a certificate (JWKS or JWKS_URI) is required when ID token encryption is configured",
		})
	case errors.Is(err, inboundclient.ErrOAuthIDTokenJWKSURINotSSRFSafe):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.idtoken_jwks_uri_not_ssrf_safe_description",
			DefaultValue: "idToken JWKS URI must be a publicly reachable HTTPS URL",
		})
	}
	return nil
}

// translateInboundClientFKError maps inbound-client foreign-key sentinel errors to
// application-service errors.
func translateInboundClientFKError(err error) *tidcommon.ServiceError {
	var fm *inboundclient.FlowMismatchError
	if errors.As(err, &fm) {
		return ErrorApplicationFlowMismatch.WithParams(map[string]string{
			"sourceFlowType": strings.ToLower(string(fm.SourceFlowType)),
			"flowType":       strings.ToLower(string(fm.FlowType)),
		})
	}

	switch {
	case errors.Is(err, inboundclient.ErrFKInvalidAuthFlow):
		return &ErrorInvalidAuthFlowID
	case errors.Is(err, inboundclient.ErrFKInvalidRegistrationFlow):
		return &ErrorInvalidRegistrationFlowID
	case errors.Is(err, inboundclient.ErrFKInvalidRecoveryFlow):
		return &ErrorInvalidRecoveryFlowID
	case errors.Is(err, inboundclient.ErrFKDeviceGrantFlowRequired):
		return &ErrorDeviceGrantFlowRequired
	case errors.Is(err, inboundclient.ErrFKInvalidDeviceGrantFlow):
		return &ErrorInvalidDeviceGrantFlowID
	case errors.Is(err, inboundclient.ErrFKFlowDefinitionRetrievalFailed):
		return &ErrorWhileRetrievingFlowDefinition
	case errors.Is(err, inboundclient.ErrFKFlowServerError):
		return &tidcommon.InternalServerError
	case errors.Is(err, inboundclient.ErrFKThemeNotFound):
		return &ErrorThemeNotFound
	case errors.Is(err, inboundclient.ErrFKLayoutNotFound):
		return &ErrorLayoutNotFound
	case errors.Is(err, inboundclient.ErrFKInvalidUserType):
		return &ErrorInvalidUserType
	case errors.Is(err, inboundclient.ErrFKInvalidAgentType):
		return &ErrorInvalidAgentType
	case errors.Is(err, inboundclient.ErrFKAnyAgentTypeNotAlone):
		return &ErrorAnyAgentTypeNotAlone
	case errors.Is(err, inboundclient.ErrUserSchemaLookupFailed):
		return &tidcommon.InternalServerError
	case errors.Is(err, inboundclient.ErrAgentSchemaLookupFailed):
		return &tidcommon.InternalServerError
	case errors.Is(err, inboundclient.ErrUniqueAttributeLookupFailed):
		return &tidcommon.InternalServerError
	case errors.Is(err, inboundclient.ErrFKInvalidSubjectAttributeMapping):
		return &ErrorInvalidSubjectAttributeMapping
	case errors.Is(err, inboundclient.ErrInvalidUserAttribute):
		return &ErrorInvalidUserAttribute
	}
	return nil
}

// translateCertValidationError maps inbound-client certificate validation sentinels to
// application-service errors.
func translateCertValidationError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, inboundclient.ErrCertValueRequired):
		return &ErrorInvalidCertificateValue
	case errors.Is(err, inboundclient.ErrCertInvalidJWKSURI):
		return &ErrorInvalidJWKSURI
	case errors.Is(err, inboundclient.ErrCertInvalidType):
		return &ErrorInvalidCertificateType
	}
	return nil
}

// translateCertOperationError maps a typed cert operation error from the inbound-client layer
// into an application-service ServiceError. Server-side failures are logged and surfaced as
// InternalServerError; client-side failures are wrapped in ErrorCertificateClientError with an
// operation-specific description.
func (as *applicationService) translateCertOperationError(ctx context.Context,
	err *inboundclient.CertOperationError) *tidcommon.ServiceError {
	if !err.IsClientError() {
		as.logger.Error(ctx, "Certificate operation failed",
			log.Any("operation", err.Operation),
			log.Any("refType", err.RefType),
			log.Any("serviceError", err.Underlying))
		return &tidcommon.InternalServerError
	}
	var key, prefix string
	switch err.Operation {
	case inboundclient.CertOpCreate:
		key, prefix = "error.applicationservice.create_certificate_failed_description",
			"Failed to create application certificate: "
	case inboundclient.CertOpUpdate:
		key, prefix = "error.applicationservice.update_certificate_failed_description",
			"Failed to update application certificate: "
	case inboundclient.CertOpRetrieve:
		key, prefix = "error.applicationservice.retrieve_certificate_failed_description",
			"Failed to retrieve application certificate: "
	case inboundclient.CertOpDelete:
		if err.RefType == cert.CertificateReferenceTypeOAuthApp {
			key, prefix = "error.applicationservice.delete_oauth_certificate_failed_description",
				"Failed to delete OAuth app certificate: "
		} else {
			key, prefix = "error.applicationservice.delete_certificate_failed_description",
				"Failed to delete application certificate: "
		}
	default:
		return &tidcommon.InternalServerError
	}
	return tidcommon.CustomServiceError(ErrorCertificateClientError, tidcommon.I18nMessage{
		Key:          key,
		DefaultValue: prefix + err.Underlying.ErrorDescription.DefaultValue,
	})
}

func (as *applicationService) processInboundAuthConfig(ctx context.Context, app *model.ApplicationDTO,
	existingApp *model.ApplicationProcessedDTO) (
	*providers.InboundAuthConfigWithSecret, *tidcommon.ServiceError) {
	inboundAuthConfig, err := validateOAuthParamsForCreateAndUpdate(app)
	if err != nil {
		return nil, err
	}

	if inboundAuthConfig == nil {
		return nil, nil
	}

	clientID := inboundAuthConfig.OAuthConfig.ClientID

	// For update operation
	if existingApp != nil {
		var existingClientID string
		if existingOAuthConfig := getOAuthInboundAuthConfigProcessedDTO(
			existingApp.InboundAuthConfig); existingOAuthConfig != nil &&
			existingOAuthConfig.OAuthConfig != nil {
			existingClientID = existingOAuthConfig.OAuthConfig.ClientID
		}

		if clientID == "" {
			if svcErr := generateAndAssignClientID(ctx, inboundAuthConfig); svcErr != nil {
				return nil, svcErr
			}
		} else if clientID != existingClientID {
			if taken, svcErr := as.isIdentifierTaken(ctx, fieldClientID, clientID, existingApp.ID); svcErr != nil {
				return nil, svcErr
			} else if taken {
				return nil, &ErrorApplicationAlreadyExistsWithClientID
			}
		}
	} else { // For create operation
		if clientID == "" {
			if svcErr := generateAndAssignClientID(ctx, inboundAuthConfig); svcErr != nil {
				return nil, svcErr
			}
		} else {
			if taken, svcErr := as.isIdentifierTaken(ctx, fieldClientID, clientID, app.ID); svcErr != nil {
				return nil, svcErr
			} else if taken {
				return nil, &ErrorApplicationAlreadyExistsWithClientID
			}
		}
	}

	if svcErr := resolveClientSecret(ctx, inboundAuthConfig, existingApp); svcErr != nil {
		return nil, svcErr
	}

	return inboundAuthConfig, nil
}

// generateAndAssignClientID generates an OAuth 2.0 compliant client ID and assigns it to the inbound auth config.
func generateAndAssignClientID(
	ctx context.Context, inboundAuthConfig *providers.InboundAuthConfigWithSecret,
) *tidcommon.ServiceError {
	generatedClientID, err := oauthutils.GenerateOAuth2ClientID()
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to generate OAuth client ID", log.Error(err))
		return &tidcommon.InternalServerError
	}
	inboundAuthConfig.OAuthConfig.ClientID = generatedClientID
	return nil
}

func resolveClientSecret(
	ctx context.Context,
	inboundAuthConfig *providers.InboundAuthConfigWithSecret,
	existingApp *model.ApplicationProcessedDTO,
) *tidcommon.ServiceError {
	if (inboundAuthConfig.OAuthConfig.TokenEndpointAuthMethod !=
		providers.TokenEndpointAuthMethodClientSecretBasic &&
		inboundAuthConfig.OAuthConfig.TokenEndpointAuthMethod !=
			providers.TokenEndpointAuthMethodClientSecretPost) ||
		inboundAuthConfig.OAuthConfig.ClientSecret != "" {
		return nil
	}

	if existingApp != nil {
		if existingInboundAuth := getOAuthInboundAuthConfigProcessedDTO(
			existingApp.InboundAuthConfig); existingInboundAuth != nil {
			existingOAuth := existingInboundAuth.OAuthConfig
			if existingOAuth != nil && !existingOAuth.PublicClient &&
				(existingOAuth.TokenEndpointAuthMethod == providers.TokenEndpointAuthMethodClientSecretBasic ||
					existingOAuth.TokenEndpointAuthMethod == providers.TokenEndpointAuthMethodClientSecretPost) {
				return nil
			}
		}
	}

	generatedClientSecret, err := oauthutils.GenerateOAuth2ClientSecret()
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to generate OAuth client secret", log.Error(err))
		return &tidcommon.InternalServerError
	}

	inboundAuthConfig.OAuthConfig.ClientSecret = generatedClientSecret
	return nil
}

// resolveAttestationCredentialsForPersist prepares the write-only Play Integrity service account
// credentials on the inbound client for persistence: newly supplied credentials are encrypted so
// they are never stored in plaintext, while an omitted value on an update falls back to the
// previously stored (encrypted) credentials. The client's Attestation is replaced with a fresh copy
// so the caller's input is not mutated.
func (as *applicationService) resolveAttestationCredentialsForPersist(
	ctx context.Context, appID string, inboundClient *inboundmodel.InboundClient,
) *tidcommon.ServiceError {
	if inboundClient == nil || inboundClient.Attestation == nil ||
		inboundClient.Attestation.Android == nil {
		return nil
	}

	android := *inboundClient.Attestation.Android
	android.CertificateSha256Digests = append([]string(nil),
		inboundClient.Attestation.Android.CertificateSha256Digests...)

	if android.ServiceAccountCredentials != "" {
		ciphertext, _, err := as.cryptoSvc.Encrypt(ctx, nil, string(cryptolib.AlgorithmAESGCM), nil,
			[]byte(android.ServiceAccountCredentials))
		if err != nil {
			as.logger.Error(ctx, "Failed to encrypt attestation credentials",
				log.String("appID", appID), log.Error(err))
			return &tidcommon.InternalServerError
		}
		android.ServiceAccountCredentials = string(ciphertext)
	} else {
		// No new credentials supplied: preserve the existing stored (encrypted) value. Distinguish a
		// missing record (nothing to preserve, e.g. on create) from a genuine lookup failure — the
		// latter must not silently overwrite stored credentials with an empty value.
		existing, err := as.inboundClientService.GetInboundClientByEntityID(ctx, appID)
		switch {
		case err == nil:
			if existing != nil && existing.Attestation != nil && existing.Attestation.Android != nil {
				android.ServiceAccountCredentials = existing.Attestation.Android.ServiceAccountCredentials
			}
		case errors.Is(err, inboundclient.ErrInboundClientNotFound):
			// No existing record; there is no stored credential to preserve.
		default:
			as.logger.Error(ctx, "Failed to load existing attestation credentials for preservation",
				log.String("appID", appID), log.Error(err))
			return &tidcommon.InternalServerError
		}
	}

	inboundClient.Attestation = &providers.AttestationConfig{
		Android: &android,
		Apple:   inboundClient.Attestation.Apple,
		DevMode: inboundClient.Attestation.DevMode,
	}
	return nil
}

// enrichApplicationWithCertificate retrieves and adds OAuth certificates to the application.
func (as *applicationService) enrichApplicationWithCertificate(
	ctx context.Context, application *providers.Application,
) (
	*providers.Application, *tidcommon.ServiceError) {
	for i, inboundAuthConfig := range application.InboundAuthConfig {
		if inboundAuthConfig.Type == providers.OAuthInboundAuthType && inboundAuthConfig.OAuthConfig != nil {
			oauthCert, oauthCertOpErr := as.inboundClientService.GetCertificate(ctx,
				cert.CertificateReferenceTypeOAuthApp, inboundAuthConfig.OAuthConfig.ClientID)
			if oauthCertOpErr != nil {
				if mapped := as.translateCertOperationError(ctx, oauthCertOpErr); mapped != nil {
					return nil, mapped
				}
				return nil, &tidcommon.InternalServerError
			}
			application.InboundAuthConfig[i].OAuthConfig.Certificate = oauthCert
		}
	}

	return application, nil
}

// buildApplicationResponse maps an ApplicationProcessedDTO to an Application response.
func buildApplicationResponse(dto *model.ApplicationProcessedDTO) *providers.Application {
	application := &providers.Application{
		ID:          dto.ID,
		OUID:        dto.OUID,
		ProjectID:   dto.ProjectID,
		Name:        dto.Name,
		Description: dto.Description,
		Status:      dto.Status,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                dto.AuthFlowID,
			DeviceGrantFlowID:         dto.DeviceGrantFlowID,
			RegistrationFlowID:        dto.RegistrationFlowID,
			IsRegistrationFlowEnabled: dto.IsRegistrationFlowEnabled,
			RecoveryFlowID:            dto.RecoveryFlowID,
			IsRecoveryFlowEnabled:     dto.IsRecoveryFlowEnabled,
			SignOutFlowID:             dto.SignOutFlowID,
			ThemeID:                   dto.ThemeID,
			LayoutID:                  dto.LayoutID,
			Assertion:                 dto.Assertion,
			AllowedUserTypes:          dto.AllowedUserTypes,
			AllowedAgentTypes:         dto.AllowedAgentTypes,
			SubjectAttribute:          dto.SubjectAttribute,
			PasskeyAllowedOrigins:     dto.PasskeyAllowedOrigins,
			LoginConsent:              dto.LoginConsent,
			Attestation:               dto.Attestation,
		},
		Type:      string(dto.Type),
		Template:  dto.Template,
		URL:       dto.URL,
		LogoURL:   dto.LogoURL,
		TosURI:    dto.TosURI,
		PolicyURI: dto.PolicyURI,
		Contacts:  dto.Contacts,
		Metadata:  dto.Metadata,
	}
	inboundAuthConfigs := make([]providers.InboundAuthConfigWithSecret, 0, len(dto.InboundAuthConfig))
	for _, config := range dto.InboundAuthConfig {
		if config.Type == providers.OAuthInboundAuthType && config.OAuthConfig != nil {
			oauthAppConfig := config.OAuthConfig
			inboundAuthConfigs = append(inboundAuthConfigs, providers.InboundAuthConfigWithSecret{
				Type: providers.OAuthInboundAuthType,
				OAuthConfig: &providers.OAuthConfigWithSecret{
					ClientID:                           oauthAppConfig.ClientID,
					RedirectURIs:                       oauthAppConfig.RedirectURIs,
					PostLogoutRedirectURIs:             oauthAppConfig.PostLogoutRedirectURIs,
					BackchannelLogoutURI:               oauthAppConfig.BackchannelLogoutURI,
					GrantTypes:                         oauthAppConfig.GrantTypes,
					ResponseTypes:                      oauthAppConfig.ResponseTypes,
					TokenEndpointAuthMethod:            oauthAppConfig.TokenEndpointAuthMethod,
					PKCERequired:                       oauthAppConfig.PKCERequired,
					PublicClient:                       oauthAppConfig.PublicClient,
					RequirePushedAuthorizationRequests: oauthAppConfig.RequirePushedAuthorizationRequests,
					DPoPBoundAccessTokens:              oauthAppConfig.DPoPBoundAccessTokens,
					IncludeActClaim:                    oauthAppConfig.IncludeActClaim,
					ClientIDMetadataDocument:           oauthAppConfig.ClientIDMetadataDocument,
					Token:                              oauthAppConfig.Token,
					Scopes:                             oauthAppConfig.Scopes,
					UserInfo:                           oauthAppConfig.UserInfo,
					ScopeClaims:                        oauthAppConfig.ScopeClaims,
					AcrValues:                          oauthAppConfig.AcrValues,
				},
			})
		}
	}
	application.InboundAuthConfig = inboundAuthConfigs
	return application
}

// buildBasicApplicationResponse builds a BasicApplicationResponse by merging config + entity data.
func buildBasicApplicationResponse(
	cfg inboundmodel.InboundClient, e *providers.Entity,
) model.BasicApplicationResponse {
	resp := model.BasicApplicationResponse{
		ID:                        cfg.ID,
		AuthFlowID:                cfg.AuthFlowID,
		DeviceGrantFlowID:         cfg.DeviceGrantFlowID,
		RegistrationFlowID:        cfg.RegistrationFlowID,
		IsRegistrationFlowEnabled: cfg.IsRegistrationFlowEnabled,
		RecoveryFlowID:            cfg.RecoveryFlowID,
		IsRecoveryFlowEnabled:     cfg.IsRecoveryFlowEnabled,
		SignOutFlowID:             cfg.SignOutFlowID,
		ThemeID:                   cfg.ThemeID,
		LayoutID:                  cfg.LayoutID,
		IsReadOnly:                cfg.IsReadOnly,
	}
	if cfg.Properties != nil {
		if t, ok := cfg.Properties[propType].(string); ok {
			resp.Type = model.ApplicationType(t)
		}
		if t, ok := cfg.Properties[propTemplate].(string); ok {
			resp.Template = t
		}
		if logoURL, ok := cfg.Properties[propLogoURL].(string); ok {
			resp.LogoURL = logoURL
		}
	}
	// Enrich from entity system attributes.
	if e != nil {
		resp.ProjectID = e.ProjectID
		var sysAttrs map[string]interface{}
		if len(e.SystemAttributes) > 0 {
			_ = json.Unmarshal(e.SystemAttributes, &sysAttrs)
		}
		if sysAttrs != nil {
			if name, ok := sysAttrs[fieldName].(string); ok {
				resp.Name = name
			}
			if desc, ok := sysAttrs[fieldDescription].(string); ok {
				resp.Description = desc
			}
			if clientID, ok := sysAttrs[fieldClientID].(string); ok {
				resp.ClientID = clientID
			}
		}
	}
	return resp
}

// buildBaseApplicationProcessedDTO constructs an ApplicationProcessedDTO with the common base fields.
// Callers are responsible for setting InboundAuthConfig on the returned DTO.
func buildBaseApplicationProcessedDTO(appID string, app *model.ApplicationDTO,
	assertion *inboundmodel.AssertionConfig) *model.ApplicationProcessedDTO {
	return &model.ApplicationProcessedDTO{
		ID:          appID,
		OUID:        app.OUID,
		ProjectID:   app.ProjectID,
		Name:        app.Name,
		Description: app.Description,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                app.AuthFlowID,
			DeviceGrantFlowID:         app.DeviceGrantFlowID,
			RegistrationFlowID:        app.RegistrationFlowID,
			IsRegistrationFlowEnabled: app.IsRegistrationFlowEnabled,
			RecoveryFlowID:            app.RecoveryFlowID,
			IsRecoveryFlowEnabled:     app.IsRecoveryFlowEnabled,
			SignOutFlowID:             app.SignOutFlowID,
			ThemeID:                   app.ThemeID,
			LayoutID:                  app.LayoutID,
			Assertion:                 assertion,
			AllowedUserTypes:          app.AllowedUserTypes,
			AllowedAgentTypes:         app.AllowedAgentTypes,
			SubjectAttribute:          app.SubjectAttribute,
			PasskeyAllowedOrigins:     app.PasskeyAllowedOrigins,
			LoginConsent:              app.LoginConsent,
			Attestation:               app.Attestation,
		},
		Type:      app.Type,
		Template:  app.Template,
		URL:       app.URL,
		LogoURL:   app.LogoURL,
		TosURI:    app.TosURI,
		PolicyURI: app.PolicyURI,
		Contacts:  app.Contacts,
		Metadata:  app.Metadata,
	}
}

// buildProcessedDTOForUpdate constructs the ApplicationProcessedDTO for an application
// update operation.
func (as *applicationService) buildProcessedDTOForUpdate(appID string, app *model.ApplicationDTO,
	inboundAuthConfig *providers.InboundAuthConfigWithSecret) *model.ApplicationProcessedDTO {
	processedDTO := buildBaseApplicationProcessedDTO(appID, app, app.Assertion)

	if inboundAuthConfig != nil {
		oa := inboundAuthConfig.OAuthConfig
		processedInboundAuthConfig := buildOAuthInboundAuthConfigProcessedDTO(
			appID, inboundAuthConfig, oa.Token, oa.UserInfo, oa.ScopeClaims, oa.Certificate,
		)
		processedDTO.InboundAuthConfig = []inboundmodel.InboundAuthConfigProcessed{processedInboundAuthConfig}
	}

	return processedDTO
}

// buildOAuthInboundAuthConfigProcessedDTO constructs the InboundAuthConfigProcessedDTO for an OAuth application.
func buildOAuthInboundAuthConfigProcessedDTO(
	appID string, inboundAuthConfig *providers.InboundAuthConfigWithSecret,
	oauthToken *providers.OAuthTokenConfig, userInfo *providers.UserInfoConfig,
	scopeClaims map[string][]string, certificate *inboundmodel.Certificate,
) inboundmodel.InboundAuthConfigProcessed {
	return inboundmodel.InboundAuthConfigProcessed{
		Type: providers.OAuthInboundAuthType,
		OAuthConfig: &providers.OAuthClient{
			ID:                                 appID,
			ClientID:                           inboundAuthConfig.OAuthConfig.ClientID,
			RedirectURIs:                       inboundAuthConfig.OAuthConfig.RedirectURIs,
			PostLogoutRedirectURIs:             inboundAuthConfig.OAuthConfig.PostLogoutRedirectURIs,
			BackchannelLogoutURI:               inboundAuthConfig.OAuthConfig.BackchannelLogoutURI,
			GrantTypes:                         inboundAuthConfig.OAuthConfig.GrantTypes,
			ResponseTypes:                      inboundAuthConfig.OAuthConfig.ResponseTypes,
			TokenEndpointAuthMethod:            inboundAuthConfig.OAuthConfig.TokenEndpointAuthMethod,
			PKCERequired:                       inboundAuthConfig.OAuthConfig.PKCERequired,
			PublicClient:                       inboundAuthConfig.OAuthConfig.PublicClient,
			RequirePushedAuthorizationRequests: inboundAuthConfig.OAuthConfig.RequirePushedAuthorizationRequests,
			DPoPBoundAccessTokens:              inboundAuthConfig.OAuthConfig.DPoPBoundAccessTokens,
			IncludeActClaim:                    inboundAuthConfig.OAuthConfig.IncludeActClaim,
			ClientIDMetadataDocument:           inboundAuthConfig.OAuthConfig.ClientIDMetadataDocument,
			Token:                              oauthToken,
			Scopes:                             inboundAuthConfig.OAuthConfig.Scopes,
			UserInfo:                           userInfo,
			ScopeClaims:                        scopeClaims,
			Certificate:                        certificate,
			AcrValues:                          inboundAuthConfig.OAuthConfig.AcrValues,
		},
	}
}

// buildReturnApplicationDTO constructs the ApplicationDTO returned from create and update operations.
func buildReturnApplicationDTO(
	appID string, app *model.ApplicationDTO, assertion *inboundmodel.AssertionConfig,
	metadata map[string]any, inboundAuthConfig *providers.InboundAuthConfigWithSecret,
	oauthToken *providers.OAuthTokenConfig, userInfo *providers.UserInfoConfig,
	scopeClaims map[string][]string) *model.ApplicationDTO {
	returnApp := &model.ApplicationDTO{
		ID:          appID,
		OUID:        app.OUID,
		ProjectID:   app.ProjectID,
		Name:        app.Name,
		Description: app.Description,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                app.AuthFlowID,
			DeviceGrantFlowID:         app.DeviceGrantFlowID,
			RegistrationFlowID:        app.RegistrationFlowID,
			IsRegistrationFlowEnabled: app.IsRegistrationFlowEnabled,
			RecoveryFlowID:            app.RecoveryFlowID,
			IsRecoveryFlowEnabled:     app.IsRecoveryFlowEnabled,
			SignOutFlowID:             app.SignOutFlowID,
			ThemeID:                   app.ThemeID,
			LayoutID:                  app.LayoutID,
			Assertion:                 assertion,
			AllowedUserTypes:          app.AllowedUserTypes,
			AllowedAgentTypes:         app.AllowedAgentTypes,
			SubjectAttribute:          app.SubjectAttribute,
			PasskeyAllowedOrigins:     app.PasskeyAllowedOrigins,
			LoginConsent:              app.LoginConsent,
			Attestation:               app.Attestation.WithoutCredentials(),
		},
		Type:      app.Type,
		Template:  app.Template,
		URL:       app.URL,
		LogoURL:   app.LogoURL,
		TosURI:    app.TosURI,
		PolicyURI: app.PolicyURI,
		Contacts:  app.Contacts,
		Metadata:  metadata,
	}
	if inboundAuthConfig != nil {
		var oauthCert *inboundmodel.Certificate
		if inboundAuthConfig.OAuthConfig != nil {
			oauthCert = inboundAuthConfig.OAuthConfig.Certificate
		}
		returnInboundAuthConfig := providers.InboundAuthConfigWithSecret{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:                           inboundAuthConfig.OAuthConfig.ClientID,
				ClientSecret:                       inboundAuthConfig.OAuthConfig.ClientSecret,
				RedirectURIs:                       inboundAuthConfig.OAuthConfig.RedirectURIs,
				PostLogoutRedirectURIs:             inboundAuthConfig.OAuthConfig.PostLogoutRedirectURIs,
				BackchannelLogoutURI:               inboundAuthConfig.OAuthConfig.BackchannelLogoutURI,
				GrantTypes:                         inboundAuthConfig.OAuthConfig.GrantTypes,
				ResponseTypes:                      inboundAuthConfig.OAuthConfig.ResponseTypes,
				TokenEndpointAuthMethod:            inboundAuthConfig.OAuthConfig.TokenEndpointAuthMethod,
				PKCERequired:                       inboundAuthConfig.OAuthConfig.PKCERequired,
				PublicClient:                       inboundAuthConfig.OAuthConfig.PublicClient,
				RequirePushedAuthorizationRequests: inboundAuthConfig.OAuthConfig.RequirePushedAuthorizationRequests,
				DPoPBoundAccessTokens:              inboundAuthConfig.OAuthConfig.DPoPBoundAccessTokens,
				IncludeActClaim:                    inboundAuthConfig.OAuthConfig.IncludeActClaim,
				ClientIDMetadataDocument:           inboundAuthConfig.OAuthConfig.ClientIDMetadataDocument,
				Token:                              oauthToken,
				Scopes:                             inboundAuthConfig.OAuthConfig.Scopes,
				UserInfo:                           userInfo,
				ScopeClaims:                        scopeClaims,
				Certificate:                        oauthCert,
				AcrValues:                          inboundAuthConfig.OAuthConfig.AcrValues,
			},
		}
		returnApp.InboundAuthConfig = []providers.InboundAuthConfigWithSecret{returnInboundAuthConfig}
	}
	return returnApp
}

// mapStoreError maps inbound client store errors to application service errors.
func (as *applicationService) mapStoreError(ctx context.Context, err error) *tidcommon.ServiceError {
	if errors.Is(err, inboundclient.ErrInboundClientNotFound) {
		return &ErrorApplicationNotFound
	}
	as.logger.Error(ctx, "Failed to retrieve application", log.Error(err))
	return &tidcommon.InternalServerError
}

// deleteLocalizedVariants removes all i18n translations for an application's fields.
// All fields are attempted; returns an internal server error if any deletion fails.
func (as *applicationService) deleteLocalizedVariants(ctx context.Context, appID string) *tidcommon.ServiceError {
	if as.i18nService == nil {
		return nil
	}
	var hasErr bool
	for _, field := range []string{"name", "logo_uri", "tos_uri", "policy_uri"} {
		if svcErr := as.i18nService.DeleteTranslationsByKey(
			ctx, AppI18nNamespace(), AppI18nKey(appID, field)); svcErr != nil {
			as.logger.Error(ctx, "Failed to delete localized variant on app deletion",
				log.String("appID", appID),
				log.String("field", field),
				log.String("namespace", AppI18nNamespace()))
			hasErr = true
		}
	}
	if hasErr {
		return &tidcommon.InternalServerError
	}
	return nil
}

// cleanupStaleI18nKeys removes i18n keys for fields that changed from an i18n ref back to plain text.
// Returns an internal server error if any deletion fails.
func (as *applicationService) cleanupStaleI18nKeys(
	ctx context.Context, appID string,
	existing *model.ApplicationProcessedDTO, updated *model.ApplicationDTO,
) *tidcommon.ServiceError {
	if as.i18nService == nil {
		return nil
	}
	type pair struct{ old, updated, field string }
	fields := []pair{
		{existing.Name, updated.Name, "name"},
		{existing.LogoURL, updated.LogoURL, "logo_uri"},
		{existing.TosURI, updated.TosURI, "tos_uri"},
		{existing.PolicyURI, updated.PolicyURI, "policy_uri"},
	}
	var hasErr bool
	for _, f := range fields {
		if isI18nRef(f.old) && !isI18nRef(f.updated) {
			if svcErr := as.i18nService.DeleteTranslationsByKey(
				ctx, AppI18nNamespace(), AppI18nKey(appID, f.field)); svcErr != nil {
				as.logger.Error(ctx, "Failed to delete stale i18n key",
					log.String("appID", appID),
					log.String("field", f.field),
					log.String("namespace", AppI18nNamespace()))
				hasErr = true
			}
		}
	}
	if hasErr {
		return &tidcommon.InternalServerError
	}
	return nil
}

// syncPasskeyOriginsToCORS merges the application's passkey allowed origins into the CORS writable
// layer so the browser can reach the ThunderID server from those origins. This is additive: origins
// already present are skipped; origins removed from an application are not pruned here. The write runs as
// an internal operation of the application write the caller is already authorized for, since a data plane
// refuses a token's own write to the server configuration.
func (as *applicationService) syncPasskeyOriginsToCORS(ctx context.Context, origins []string) {
	if as.serverConfigService == nil || len(origins) == 0 {
		return
	}

	writableAny, svcErr := as.serverConfigService.GetWritableConfig(ctx, string(serverconfig.ConfigNameCORS))
	if svcErr != nil {
		as.logger.Warn(ctx, "Failed to read CORS writable config for passkey origin sync",
			log.String("error", svcErr.ErrorDescription.DefaultValue))
		return
	}

	cfg, _ := writableAny.(cors.OriginConfig)

	existingJSON, err := json.Marshal(cfg.AllowedOrigins)
	if err != nil {
		as.logger.Warn(ctx, "Failed to marshal existing CORS allowed origins", log.Error(err))
		return
	}
	existingItems := make([]json.RawMessage, 0, len(cfg.AllowedOrigins))
	_ = json.Unmarshal(existingJSON, &existingItems)

	existingLiterals := make(map[string]struct{}, len(existingItems))
	for _, item := range existingItems {
		var s string
		if json.Unmarshal(item, &s) == nil {
			existingLiterals[s] = struct{}{}
		}
	}

	added := false
	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" || strings.Contains(trimmed, "*") || trimmed == "null" {
			continue
		}
		if _, err := cors.ParseOrigin(trimmed); err != nil {
			as.logger.Debug(ctx, "Skipping invalid passkey origin for CORS sync",
				log.String("origin", trimmed), log.String("error", err.Error()))
			continue
		}
		if _, found := existingLiterals[trimmed]; found {
			continue
		}
		originJSON, _ := json.Marshal(trimmed)
		existingItems = append(existingItems, json.RawMessage(originJSON))
		existingLiterals[trimmed] = struct{}{}
		added = true
	}

	if !added {
		return
	}

	newConfigJSON, err := json.Marshal(map[string]any{"allowedOrigins": existingItems})
	if err != nil {
		as.logger.Warn(ctx, "Failed to marshal updated CORS config", log.Error(err))
		return
	}
	if svcErr := as.serverConfigService.SetConfig(
		security.WithRuntimeContext(ctx), serverconfig.ConfigNameCORS, json.RawMessage(newConfigJSON),
	); svcErr != nil {
		as.logger.Warn(ctx, "Failed to update CORS config with passkey allowed origins",
			log.String("error", svcErr.ErrorDescription.DefaultValue))
	}
}

// sharingErrors answers the sharing framework's client errors in this service's own codes.
var sharingErrors = sharing.ErrorTranslation{
	PolicyNotFound:  ErrorSharingPolicyNotFound,
	PolicyExists:    ErrorSharingPolicyExists,
	VersionMismatch: ErrorSharingPolicyVersionMismatch,
	PolicyDeclared:  ErrorSharingPolicyDeclared,
	InvalidLimit:    ErrorInvalidLimit,
	InvalidOffset:   ErrorInvalidOffset,
	InvalidPolicy:   ErrorInvalidSharingPolicy,
}

// translateSharingError maps a sharing framework error into this service's own codes.
func translateSharingError(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	return sharingErrors.Translate(svcErr)
}

// requireSharing resolves the application a sharing call names and returns its owning organization
// unit, so an unknown application answers as one rather than as a sharing failure. The application is
// read without the caller's authorization, because a sharing call is authorized against the
// organization unit whose decision it concerns, which need not be the application's own.
func (as *applicationService) requireSharing(
	ctx context.Context, appID string,
) (string, *tidcommon.ServiceError) {
	if appID == "" {
		return "", &ErrorInvalidApplicationID
	}
	app, svcErr := as.getApplication(ctx, appID)
	if svcErr != nil {
		return "", svcErr
	}
	return app.OUID, nil
}

// requireOwnPolicy fetches a policy and refuses one belonging to another resource. Without it a
// policy id alone would reach any policy in the deployment through any application's path.
func (as *applicationService) requireOwnPolicy(
	ctx context.Context, appID, policyID string,
) (string, sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, svcErr := as.requireSharing(ctx, appID)
	if svcErr != nil {
		return "", sharing.Policy{}, svcErr
	}
	policy, svcErr := as.sharingService.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return "", sharing.Policy{}, translateSharingError(svcErr)
	}
	if policy.ResourceType != ApplicationSharingType || policy.ResourceID != appID {
		return "", sharing.Policy{}, &ErrorSharingPolicyNotFound
	}
	return owningOUID, policy, nil
}

// CreateSharingPolicy records which organization units may act for an application. A policy is the
// initiating organization unit's decision, so that unit is the one the caller must be allowed to act
// for.
func (as *applicationService) CreateSharingPolicy(
	ctx context.Context, appID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	policy, svcErr := as.createSharingPolicy(ctx, appID, req)
	var policyID string
	if svcErr == nil {
		policyID = policy.ID
	}
	as.publishSharingPolicyEvent(ctx, audit.EventApplicationSharingPolicyCreate, audit.ActionCreate, appID,
		policyID, svcErr)
	return policy, svcErr
}

// createSharingPolicy records a sharing policy of an application without publishing its audit event.
func (as *applicationService) createSharingPolicy(
	ctx context.Context, appID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, svcErr := as.requireSharing(ctx, appID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	initiatingOUID := req.InitiatingOUID
	if initiatingOUID == "" {
		initiatingOUID = owningOUID
	}
	if svcErr := as.checkApplicationAccess(ctx, security.ActionUpdateApplication, initiatingOUID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	// Applications that load from resource files only have all their sharing declared there.
	if isDeclarativeModeEnabled() {
		return sharing.Policy{}, &ErrorCannotModifyDeclarativeResource
	}
	// A declared application's own sharing is declared with it.
	if req.InitiatingOUID == "" || req.InitiatingOUID == owningOUID {
		if svcErr := as.checkApplicationMutable(ctx, appID); svcErr != nil {
			return sharing.Policy{}, svcErr
		}
	}
	// The framework drops the caller's id and version on an interactive create, so a request
	// carrying either is answered as though it had not.
	policy, svcErr := as.sharingService.CreatePolicy(ctx, ApplicationSharingType, appID, owningOUID, req)
	return policy, translateSharingError(svcErr)
}

// GetSharingPolicy returns one of the application's sharing policies. The owner sees every decision
// about its application; any other organization unit sees its own.
func (as *applicationService) GetSharingPolicy(
	ctx context.Context, appID, policyID string,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, policy, svcErr := as.requireOwnPolicy(ctx, appID, policyID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if svcErr := sysauthz.CheckActionInAnyOU(ctx, as.authzService, as.logger, security.ActionReadApplication,
		security.ResourceTypeApplication, owningOUID, policy.InitiatingOUID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	return policy, nil
}

// ListSharingPolicies returns one page of the application's sharing policies. A caller who may read
// applications in the owning organization unit sees them all; any other caller sees the ones its own
// organization units made.
func (as *applicationService) ListSharingPolicies(
	ctx context.Context, appID string, limit, offset int,
) (sharing.PolicyList, *tidcommon.ServiceError) {
	owningOUID, svcErr := as.requireSharing(ctx, appID)
	if svcErr != nil {
		return sharing.PolicyList{}, svcErr
	}
	return sharing.ListReadablePolicies(ctx, as.sharingService, ApplicationSharingType, appID, owningOUID, limit,
		offset, func(ouID string) *tidcommon.ServiceError {
			return as.checkApplicationAccess(ctx, security.ActionReadApplication, ouID)
		}, translateSharingError)
}

// UpdateSharingPolicy replaces a sharing policy's contents. Only the organization unit whose decision
// it is may change it.
func (as *applicationService) UpdateSharingPolicy(
	ctx context.Context, appID, policyID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	updated, svcErr := as.updateSharingPolicy(ctx, appID, policyID, req)
	as.publishSharingPolicyEvent(ctx, audit.EventApplicationSharingPolicyUpdate, audit.ActionUpdate, appID,
		policyID, svcErr)
	return updated, svcErr
}

// updateSharingPolicy replaces a sharing policy of an application without publishing its audit event.
func (as *applicationService) updateSharingPolicy(
	ctx context.Context, appID, policyID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, policy, svcErr := as.requireOwnPolicy(ctx, appID, policyID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if svcErr := as.checkApplicationAccess(ctx, security.ActionUpdateApplication,
		policy.InitiatingOUID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if isDeclarativeModeEnabled() {
		return sharing.Policy{}, &ErrorCannotModifyDeclarativeResource
	}
	// A deployed application's own sharing is deployed with it; a unit it is shared into may still reshare.
	if policy.InitiatingOUID == owningOUID {
		if svcErr := as.refuseManaged(ctx, appID); svcErr != nil {
			return sharing.Policy{}, svcErr
		}
	}
	updated, svcErr := as.sharingService.UpdatePolicy(ctx, policyID, req)
	return updated, translateSharingError(svcErr)
}

// DeleteSharingPolicy removes a sharing policy and everything that depended on it.
func (as *applicationService) DeleteSharingPolicy(
	ctx context.Context, appID, policyID string,
) *tidcommon.ServiceError {
	svcErr := as.deleteSharingPolicy(ctx, appID, policyID)
	// A delete that found no policy of the application succeeds without an event, so no caller can record
	// a deletion of an ID it made up.
	if svcErr == errSharingPolicyNotFoundNoop {
		return nil
	}
	as.publishSharingPolicyEvent(ctx, audit.EventApplicationSharingPolicyDelete, audit.ActionDelete, appID,
		policyID, svcErr)
	return svcErr
}

// deleteSharingPolicy removes a sharing policy of an application without publishing its audit event.
func (as *applicationService) deleteSharingPolicy(
	ctx context.Context, appID, policyID string,
) *tidcommon.ServiceError {
	owningOUID, policy, svcErr := as.requireOwnPolicy(ctx, appID, policyID)
	if svcErr != nil {
		if svcErr.Code == ErrorSharingPolicyNotFound.Code {
			return errSharingPolicyNotFoundNoop
		}
		return svcErr
	}
	if svcErr := as.checkApplicationAccess(ctx, security.ActionUpdateApplication,
		policy.InitiatingOUID); svcErr != nil {
		return svcErr
	}
	if isDeclarativeModeEnabled() {
		return &ErrorCannotModifyDeclarativeResource
	}
	if policy.InitiatingOUID == owningOUID {
		if svcErr := as.refuseManaged(ctx, appID); svcErr != nil {
			return svcErr
		}
	}
	if policy.Declared {
		return &ErrorSharingPolicyDeclared
	}
	return translateSharingError(as.sharingService.DeletePolicy(ctx, policyID))
}

// publishSharingPolicyEvent publishes the audit event of an attempt on the sharing policy with policyID
// of the application with appID, after the change committed or was refused. The policy is the target and
// the application the subject, which a succeeded or denied attempt names: a sharing call is refused only
// once the application is found. Reshares the framework re-narrows or removes beneath a changed policy
// write no event of their own.
func (as *applicationService) publishSharingPolicyEvent(ctx context.Context, name string,
	action audit.Action, appID, policyID string, svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the IDs the caller asked for, even ones that match nothing, as forensic
	// evidence of what was probed; the payload caps them and the sink escapes them.
	target := audit.Ref{Type: audit.TargetTypeApplicationSharingPolicy, ID: policyID}
	subject := audit.Ref{Type: audit.TargetTypeApplication, ID: appID}
	eventbus.TryPublish(ctx, as.bus, eventbus.NewEvent(ctx, name).With(
		audit.ChangePayloadWithSubject(ctx, target, subject, action, svcErr)))
}

// ResolveSharingOverlay answers what one organization unit may do with the application. An
// application declares no overlay fields, so the answer is visibility.
func (as *applicationService) ResolveSharingOverlay(
	ctx context.Context, appID, ouID string,
) (sharing.ResolvedOverlay, *tidcommon.ServiceError) {
	owningOUID, svcErr := as.requireSharing(ctx, appID)
	if svcErr != nil {
		return sharing.ResolvedOverlay{}, svcErr
	}
	if ouID == "" {
		return sharing.ResolvedOverlay{}, &ErrorMissingOUIDParameter
	}
	if svcErr := sysauthz.CheckActionInAnyOU(ctx, as.authzService, as.logger, security.ActionReadApplication,
		security.ResourceTypeApplication, owningOUID, ouID); svcErr != nil {
		return sharing.ResolvedOverlay{}, svcErr
	}
	overlay, svcErr := as.sharingService.ResolveOverlayRules(ctx, ApplicationSharingType, appID, ouID)
	if svcErr != nil {
		// An id naming no organization unit leaves nothing to resolve either, and answering it as
		// a malformed policy would describe a mistake the caller did not make.
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return sharing.ResolvedOverlay{}, &ErrorApplicationNotSharedToOU
		}
		return sharing.ResolvedOverlay{}, translateSharingError(svcErr)
	}
	// An organization unit the application never reached holds no terms to answer with.
	if !overlay.Visible {
		return sharing.ResolvedOverlay{}, &ErrorApplicationNotSharedToOU
	}
	return overlay, nil
}

// checkApplicationAccess checks the caller may perform the action on an application of the
// organization unit.
func (as *applicationService) checkApplicationAccess(ctx context.Context, action security.Action,
	ouID string) *tidcommon.ServiceError {
	allowed, svcErr := as.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeApplication, OUID: ouID})
	if svcErr != nil {
		as.logger.Error(ctx, "Failed to check authorization for action",
			log.String("action", string(action)), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}
