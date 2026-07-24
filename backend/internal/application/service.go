/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"encoding/json"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// ApplicationServiceInterface defines the interface for the application service.
type ApplicationServiceInterface interface {
	CreateApplication(
		ctx context.Context, app *model.ApplicationDTO) (*model.ApplicationDTO, *tidcommon.ServiceError)
	ValidateApplication(ctx context.Context, app *model.ApplicationDTO) (
		*model.ApplicationProcessedDTO, *providers.InboundAuthConfigWithSecret, *tidcommon.ServiceError)
	GetApplicationList(ctx context.Context) (*model.ApplicationListResponse, *tidcommon.ServiceError)
	GetOAuthApplication(
		ctx context.Context, clientID string) (*providers.OAuthClient, *tidcommon.ServiceError)
	GetApplication(ctx context.Context, appID string) (*providers.Application, *tidcommon.ServiceError)
	UpdateApplication(
		ctx context.Context, appID string, app *model.ApplicationDTO) (
		*model.ApplicationDTO, *tidcommon.ServiceError)
	DeleteApplication(ctx context.Context, appID string) *tidcommon.ServiceError
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	SetDependencyRegistry(r resourcedependency.Registry)
}

// ApplicationService is the default implementation of the ApplicationServiceInterface.
type applicationService struct {
	logger               *log.Logger
	inboundClientService inboundclient.InboundClientServiceInterface
	entityProvider       entityprovider.EntityProviderInterface
	ouService            oupkg.OrganizationUnitServiceInterface
	i18nService          i18nmgt.I18nServiceInterface
	cryptoSvc            kmprovider.RuntimeCryptoProvider
	dependencyRegistry   resourcedependency.Registry
}

// newApplicationService creates a new instance of ApplicationService.
func newApplicationService(
	inboundClientSvc inboundclient.InboundClientServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	i18nService i18nmgt.I18nServiceInterface,
	cryptoSvc kmprovider.RuntimeCryptoProvider,
) ApplicationServiceInterface {
	return &applicationService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ApplicationService")),
		inboundClientService: inboundClientSvc,
		entityProvider:       entityProvider,
		ouService:            ouService,
		i18nService:          i18nService,
		cryptoSvc:            cryptoSvc,
	}
}

func (as *applicationService) deleteEntityCompensation(ctx context.Context, appID string) {
	if delErr := as.entityProvider.DeleteEntity(appID); delErr != nil {
		as.logger.Error(ctx, "Failed to delete entity during compensation", log.Error(delErr),
			log.String("appID", appID))
	}
}

// CreateApplication creates the application.
func (as *applicationService) CreateApplication(ctx context.Context, app *model.ApplicationDTO) (*model.ApplicationDTO,
	*tidcommon.ServiceError) {
	if app == nil {
		return nil, &ErrorApplicationNil
	}
	// Check if store is in pure declarative mode
	if isDeclarativeModeEnabled() {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	// Check if an application with the same ID exists and is declarative (in composite mode)
	if app.ID != "" && as.inboundClientService.IsDeclarative(ctx, app.ID) {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	processedDTO, inboundAuthConfig, svcErr := as.ValidateApplication(ctx, app)
	if svcErr != nil {
		return nil, svcErr
	}

	appID := processedDTO.ID

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
	// Execution API — i.e. backend / server-side apps. Public clients (browser SPAs, mobile apps)
	// and redirect-based (authorization_code) clients initiate their flows through the OAuth
	// component, so they have no use for an Flow Secret and never receive one — any caller-supplied
	// value for such apps is ignored. For eligible apps, an explicitly provided value (e.g.
	// declarative resources) is preserved; otherwise one is generated.
	flowSecret := ""
	if isFlowSecretEligible(inboundAuthConfig) {
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

	_, epErr := as.entityProvider.CreateEntity(appEntity, sysCredsJSON)
	if epErr != nil {
		if svcErr := mapEntityProviderError(epErr); svcErr != nil {
			return nil, svcErr
		}
		as.logger.Error(ctx, "Failed to create application entity",
			log.String("appID", appID), log.Error(epErr))
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

	appForReturn := *app
	appForReturn.AuthFlowID = inboundClient.AuthFlowID
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
	if inboundAuthConfig != nil && inboundAuthConfig.OAuthConfig != nil {
		hasClientSecret = inboundAuthConfig.OAuthConfig.ClientSecret != ""
	}
	if err := as.inboundClientService.Validate(ctx, &inboundClient, oauthProfile, hasClientSecret); err != nil {
		if svcErr := as.translateInboundClientError(ctx, err); svcErr != nil {
			return nil, nil, svcErr
		}
		as.logger.Error(ctx, "Inbound client validation failed", log.Error(err))
		return nil, nil, &tidcommon.InternalServerError
	}
	processedDTO.AuthFlowID = inboundClient.AuthFlowID
	processedDTO.RegistrationFlowID = inboundClient.RegistrationFlowID
	processedDTO.RecoveryFlowID = inboundClient.RecoveryFlowID
	processedDTO.SignOutFlowID = inboundClient.SignOutFlowID

	return processedDTO, inboundAuthConfig, nil
}

// GetApplicationList list the applications.
func (as *applicationService) GetApplicationList(
	ctx context.Context) (*model.ApplicationListResponse, *tidcommon.ServiceError) {
	totalResults, epErr := as.entityProvider.GetEntityListCount(providers.EntityCategoryApp, nil)
	if epErr != nil {
		as.logger.Error(ctx, "Failed to count application entities", log.Error(epErr))
		return nil, &tidcommon.InternalServerError
	}

	entities, epErr := as.entityProvider.GetEntityList(
		providers.EntityCategoryApp, serverconst.MaxCompositeStoreRecords, 0, nil)
	if epErr != nil {
		as.logger.Error(ctx, "Failed to list application entities", log.Error(epErr))
		return nil, &tidcommon.InternalServerError
	}
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

	applicationList := make([]model.BasicApplicationResponse, 0, len(entities))
	for i := range entities {
		cfg := configMap[entities[i].ID]
		if cfg == nil {
			as.logger.Warn(ctx, "Application entity has no inbound-client row; skipping in list",
				log.String("appID", entities[i].ID))
			continue
		}
		applicationList = append(applicationList, buildBasicApplicationResponse(*cfg, &entities[i]))
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

	entity, epErr := as.entityProvider.GetEntity(client.ID)
	if epErr != nil && epErr.Code != entityprovider.ErrorCodeEntityNotFound {
		as.logger.Error(ctx, "Failed to load entity for OAuth client",
			log.String("entityID", client.ID), log.Error(epErr))
		return nil, &tidcommon.InternalServerError
	}
	if entity == nil || entity.Category != providers.EntityCategoryApp {
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

	return as.enrichApplicationWithCertificate(ctx, buildApplicationResponse(fullApp))
}

// UpdateApplication update the application for given app id.
func (as *applicationService) UpdateApplication(ctx context.Context, appID string, app *model.ApplicationDTO) (
	*model.ApplicationDTO, *tidcommon.ServiceError) {
	if appID == "" {
		return nil, &ErrorInvalidApplicationID
	}
	if as.inboundClientService.IsDeclarative(ctx, appID) {
		return nil, &ErrorCannotModifyDeclarativeResource
	}
	existingApp, inboundAuthConfig, svcErr := as.validateApplicationForUpdate(ctx, appID, app)

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
	// Update config first, while entity attributes still hold the previous client_id so the
	// inbound client service can clean up the old OAuth-app cert.
	if err := as.inboundClientService.UpdateInboundClient(
		ctx, &inboundClient, oauthProfile, oauthSecretSupplied, newOAuthClientID,
	); err != nil {
		if svcErr := as.translateInboundClientError(ctx, err); svcErr != nil {
			return nil, svcErr
		}
		as.logger.Error(ctx, "Failed to update application", log.Error(err), log.String("appID", appID))
		return nil, &tidcommon.InternalServerError
	}

	if svcErr := as.updateEntityDataForApplicationUpdate(ctx, appID, app, inboundAuthConfig); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := as.cleanupStaleI18nKeys(ctx, appID, existingApp, app); svcErr != nil {
		return nil, svcErr
	}

	appForReturn := *app
	appForReturn.AuthFlowID = inboundClient.AuthFlowID
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
	return buildReturnApplicationDTO(appID, &appForReturn, inboundClient.Assertion, processedDTO.Metadata,
		inboundAuthConfig, oauthToken, userInfo, scopeClaims), nil
}

func (as *applicationService) updateEntityDataForApplicationUpdate(ctx context.Context,
	appID string,
	app *model.ApplicationDTO,
	inboundAuthConfig *providers.InboundAuthConfigWithSecret,
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

	if epErr := as.entityProvider.UpdateSystemAttributes(appID, sysAttrsJSON); epErr != nil {
		if svcErr := mapEntityProviderError(epErr); svcErr != nil {
			return svcErr
		}
		as.logger.Error(ctx, "Failed to update entity system attributes",
			log.String("appID", appID), log.Error(epErr))
		return &tidcommon.InternalServerError
	}

	// Rotate the Flow Secret when a new value is supplied (e.g. a regenerate request). Only
	// applications eligible to hold an Flow Secret — backend / server-side apps that are neither
	// public nor redirect-based — may have one set; a value supplied for an ineligible app is
	// ignored. Credential updates merge, so this preserves the stored client secret, and an empty
	// value leaves the existing Flow Secret intact.
	if app.FlowSecret != "" && isFlowSecretEligible(inboundAuthConfig) {
		flowSecretJSON, marshalErr := buildSystemCredentials("", app.FlowSecret)
		if marshalErr != nil {
			as.logger.Error(ctx, "Failed to build flow secret credentials for update", log.Error(marshalErr))
			return &tidcommon.InternalServerError
		}
		if epErr := as.entityProvider.UpdateSystemCredentials(appID, flowSecretJSON); epErr != nil {
			if svcErr := mapEntityProviderError(epErr); svcErr != nil {
				return svcErr
			}
			as.logger.Error(ctx, "Failed to update flow secret credentials",
				log.String("appID", appID), log.Error(epErr))
			return &tidcommon.InternalServerError
		}
	}

	// Decide client-secret disposition:
	// - No OAuth config, or OAuth method that doesn't use a client secret → leave credentials as-is.
	// - OAuth method requires a secret + new secret supplied → store the new secret.
	// - OAuth method requires a secret + no new secret supplied → leave existing secret intact (no rotation).
	if inboundAuthConfig == nil || inboundAuthConfig.OAuthConfig == nil ||
		!appRequiresClientSecret(inboundAuthConfig.OAuthConfig) {
		return nil
	}
	if inboundAuthConfig.OAuthConfig.ClientSecret == "" {
		return nil
	}

	sysCredsJSON, marshalErr := buildSystemCredentials(inboundAuthConfig.OAuthConfig.ClientSecret, "")
	if marshalErr != nil {
		as.logger.Error(ctx, "Failed to build entity system credentials for update", log.Error(marshalErr))
		return &tidcommon.InternalServerError
	}

	if epErr := as.entityProvider.UpdateSystemCredentials(appID, sysCredsJSON); epErr != nil {
		if svcErr := mapEntityProviderError(epErr); svcErr != nil {
			return svcErr
		}
		as.logger.Error(ctx, "Failed to update entity system credentials",
			log.String("appID", appID), log.Error(epErr))
		return &tidcommon.InternalServerError
	}

	return nil
}

// isFlowSecretEligible reports whether an application may hold a Flow Secret. Eligible apps initiate
// flows directly: embedded apps with no OAuth config, or confidential non-redirect apps. Public,
// redirect (authorization_code), and machine-to-machine (client_credentials as the only grant) apps
// are not eligible.
func isFlowSecretEligible(inboundAuthConfig *providers.InboundAuthConfigWithSecret) bool {
	if inboundAuthConfig == nil || inboundAuthConfig.OAuthConfig == nil {
		return true
	}
	oauthConfig := inboundAuthConfig.OAuthConfig
	if oauthConfig.PublicClient {
		return false
	}
	if slices.Contains(oauthConfig.GrantTypes, providers.GrantTypeAuthorizationCode) {
		return false
	}
	// Machine-to-machine apps use client_credentials as their only grant; they obtain tokens directly
	// and do not initiate flows, so they are not issued a Flow Secret.
	if isM2MGrantSet(oauthConfig.GrantTypes) {
		return false
	}
	return true
}

// isM2MGrantSet reports whether client_credentials is the only configured grant type.
func isM2MGrantSet(grantTypes []providers.GrantType) bool {
	return len(grantTypes) == 1 && grantTypes[0] == providers.GrantTypeClientCredentials
}

// appRequiresClientSecret reports whether the OAuth config implies a confidential client requiring a secret.
func appRequiresClientSecret(cfg *providers.OAuthConfigWithSecret) bool {
	if cfg == nil {
		return false
	}
	if cfg.PublicClient {
		return false
	}
	switch cfg.TokenEndpointAuthMethod {
	case providers.TokenEndpointAuthMethodClientSecretBasic,
		providers.TokenEndpointAuthMethodClientSecretPost:
		return true
	case providers.TokenEndpointAuthMethodNone,
		providers.TokenEndpointAuthMethodPrivateKeyJWT:
		return false
	}
	// Default to requiring a secret when method is unspecified.
	return true
}

// DeleteApplication delete the application for given app id.
// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (as *applicationService) SetDependencyRegistry(r resourcedependency.Registry) {
	as.dependencyRegistry = r
}

func (as *applicationService) DeleteApplication(ctx context.Context, appID string) *tidcommon.ServiceError {
	if appID == "" {
		return &ErrorInvalidApplicationID
	}

	if existing, epErr := as.entityProvider.GetEntity(appID); epErr != nil {
		if epErr.Code != entityprovider.ErrorCodeEntityNotFound {
			as.logger.Error(ctx, "Failed to load entity before delete",
				log.String("appID", appID), log.Error(epErr))
			return &tidcommon.InternalServerError
		}
	} else if existing != nil && existing.Category != providers.EntityCategoryApp {
		return &ErrorApplicationNotFound
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

	// Delete entity.
	if epErr := as.entityProvider.DeleteEntity(appID); epErr != nil {
		if svcErr := mapEntityProviderError(epErr); svcErr != nil {
			return svcErr
		}
		as.logger.Error(ctx, "Failed to delete application entity",
			log.String("appID", appID), log.Error(epErr))
		return &tidcommon.InternalServerError
	}

	return as.deleteLocalizedVariants(ctx, appID)
}

// GetResourceDependencies returns the applications that reference the resource identified
// by (resourceType, id). It implements the resourcedependency.Provider interface. The
// inbound-client store resolves which reference types are tracked, so no per-type handling is
// needed here. The number of referencing entities is bounded by MaxCompositeStoreRecords
// (the inbound-client store limit).
func (as *applicationService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	ids, _, err := as.inboundClientService.GetEntityIDsByReference(
		ctx, resourceType, id, serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		as.logger.Error(ctx, "Failed to get entity IDs by reference", log.Error(err))
		return nil, err
	}
	if len(ids) == 0 {
		return []resourcedependency.ResourceDependency{}, nil
	}

	entities, epErr := as.entityProvider.GetEntitiesByIDs(ids)
	if epErr != nil {
		as.logger.Error(ctx, "Failed to get entities by IDs", log.Error(epErr))
		return nil, epErr
	}

	usages := make([]resourcedependency.ResourceDependency, 0, len(entities))
	for _, e := range entities {
		// Applications and agents share the inbound-client store; only report applications.
		if e.Category != providers.EntityCategoryApp {
			continue
		}
		name := ""
		var sysAttrs map[string]interface{}
		if len(e.SystemAttributes) > 0 {
			_ = json.Unmarshal(e.SystemAttributes, &sysAttrs)
		}
		if sysAttrs != nil {
			if n, ok := sysAttrs[fieldName].(string); ok {
				name = n
			}
		}
		usages = append(usages, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeApplication,
			ID:               e.ID,
			DisplayName:      name,
			BehaviorOnDelete: resourcedependency.BehaviorFallback,
		})
	}
	return usages, nil
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
	entityID, epErr := as.entityProvider.IdentifyEntity(map[string]interface{}{key: value})
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return false, nil
		}
		as.logger.Error(ctx, "Failed to check identifier availability",
			log.String("key", key), log.String("value", value), log.Error(epErr))
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

	entity, epErr := as.entityProvider.GetEntity(appID)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeEntityNotFound {
			entity = nil
		} else {
			as.logger.Error(ctx, "Failed to get entity for application",
				log.String("appID", appID), log.Error(epErr))
			return nil, &tidcommon.InternalServerError
		}
	}

	if entity != nil && entity.Category != providers.EntityCategoryApp {
		return nil, &ErrorApplicationNotFound
	}

	oauthProfile, err := as.inboundClientService.GetOAuthProfileByEntityID(ctx, appID)
	if err != nil && !errors.Is(err, inboundclient.ErrInboundClientNotFound) {
		as.logger.Error(ctx, "Failed to get OAuth profile for application",
			log.String("appID", appID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	dto := toProcessedDTO(entity, inboundClient, oauthProfile)
	return dto, nil
}

// mapEntityProviderError maps entity provider error codes to application service errors.
func mapEntityProviderError(epErr *entityprovider.EntityProviderError) *tidcommon.ServiceError {
	if epErr == nil {
		return nil
	}
	switch epErr.Code {
	case entityprovider.ErrorCodeEntityNotFound:
		return &ErrorApplicationNotFound
	default:
		return nil
	}
}

// toInboundClient extracts gateway config fields from a full ApplicationProcessedDTO.
func toInboundClient(dto *model.ApplicationProcessedDTO) inboundmodel.InboundClient {
	dao := inboundmodel.InboundClient{
		ID:                        dto.ID,
		AuthFlowID:                dto.AuthFlowID,
		RegistrationFlowID:        dto.RegistrationFlowID,
		IsRegistrationFlowEnabled: dto.IsRegistrationFlowEnabled,
		RecoveryFlowID:            dto.RecoveryFlowID,
		IsRecoveryFlowEnabled:     dto.IsRecoveryFlowEnabled,
		SignOutFlowID:             dto.SignOutFlowID,
		IsSignOutFlowEnabled:      dto.IsSignOutFlowEnabled,
		ThemeID:                   dto.ThemeID,
		LayoutID:                  dto.LayoutID,
		Assertion:                 dto.Assertion,
		LoginConsent:              dto.LoginConsent,
		AllowedUserTypes:          dto.AllowedUserTypes,
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
			RegistrationFlowID:        dao.RegistrationFlowID,
			IsRegistrationFlowEnabled: dao.IsRegistrationFlowEnabled,
			RecoveryFlowID:            dao.RecoveryFlowID,
			IsRecoveryFlowEnabled:     dao.IsRecoveryFlowEnabled,
			SignOutFlowID:             dao.SignOutFlowID,
			IsSignOutFlowEnabled:      dao.IsSignOutFlowEnabled,
			ThemeID:                   dao.ThemeID,
			LayoutID:                  dao.LayoutID,
			Assertion:                 dao.Assertion,
			LoginConsent:              dao.LoginConsent,
			AllowedUserTypes:          dao.AllowedUserTypes,
			Attestation:               dao.Attestation.WithoutCredentials(),
		},
	}

	// Extract identity fields from entity system attributes.
	if e != nil {
		dto.OUID = e.OUID
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
		GrantTypes:                         sysutils.ConvertToStringSlice(oa.GrantTypes),
		ResponseTypes:                      sysutils.ConvertToStringSlice(oa.ResponseTypes),
		TokenEndpointAuthMethod:            string(oa.TokenEndpointAuthMethod),
		PKCERequired:                       oa.PKCERequired,
		PublicClient:                       oa.PublicClient,
		RequirePushedAuthorizationRequests: oa.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              oa.DPoPBoundAccessTokens,
		IncludeActClaim:                    oa.IncludeActClaim,
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

	if svcErr := as.validateApplicationFields(ctx, app); svcErr != nil {
		return nil, nil, svcErr
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
	return nil
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
	case errors.Is(err, inboundclient.ErrOAuthClientSecretCannotHaveCertificate):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.client_secret_cannot_have_certificate_description",
			DefaultValue: "client_secret authentication methods cannot have a certificate",
		})
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthRequiresPublicClient):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.none_auth_method_requires_public_client_description",
			DefaultValue: "'none' authentication method requires the client to be a public client",
		})
	case errors.Is(err, inboundclient.ErrOAuthNoneAuthCannotHaveCertOrSecret):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.none_auth_method_cannot_have_cert_or_secret_description",
			DefaultValue: "'none' authentication method cannot have a certificate or client secret",
		})
	case errors.Is(err, inboundclient.ErrOAuthClientCredentialsCannotUseNoneAuth):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.client_credentials_cannot_use_none_auth_description",
			DefaultValue: "client_credentials grant type cannot use 'none' authentication method",
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
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWSRequiresSigningAlg):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_jws_requires_signing_alg_description",
			DefaultValue: "signingAlg is required when userinfo responseType is JWS",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoJWERequiresEncryption):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key:          "error.applicationservice.userinfo_jwe_requires_encryption_description",
			DefaultValue: "encryptionAlg and encryptionEnc are required when userinfo responseType is JWE",
		})
	case errors.Is(err, inboundclient.ErrOAuthUserInfoNestedJWTRequiresAll):
		return tidcommon.CustomServiceError(ErrorInvalidOAuthConfiguration, tidcommon.I18nMessage{
			Key: "error.applicationservice.userinfo_nested_jwt_requires_all_description",
			DefaultValue: "signingAlg, encryptionAlg, and encryptionEnc are required " +
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
	case errors.Is(err, inboundclient.ErrUserSchemaLookupFailed):
		return &tidcommon.InternalServerError
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
		params := cryptolib.AlgorithmParams{Algorithm: cryptolib.AlgorithmAESGCM}
		ciphertext, _, err := as.cryptoSvc.Encrypt(ctx, nil, params,
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

	inboundClient.Attestation = &providers.AttestationConfig{Android: &android, Apple: inboundClient.Attestation.Apple}
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
		Name:        dto.Name,
		Description: dto.Description,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                dto.AuthFlowID,
			RegistrationFlowID:        dto.RegistrationFlowID,
			IsRegistrationFlowEnabled: dto.IsRegistrationFlowEnabled,
			RecoveryFlowID:            dto.RecoveryFlowID,
			IsRecoveryFlowEnabled:     dto.IsRecoveryFlowEnabled,
			SignOutFlowID:             dto.SignOutFlowID,
			IsSignOutFlowEnabled:      dto.IsSignOutFlowEnabled,
			ThemeID:                   dto.ThemeID,
			LayoutID:                  dto.LayoutID,
			Assertion:                 dto.Assertion,
			AllowedUserTypes:          dto.AllowedUserTypes,
			LoginConsent:              dto.LoginConsent,
			Attestation:               dto.Attestation,
		},
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
					GrantTypes:                         oauthAppConfig.GrantTypes,
					ResponseTypes:                      oauthAppConfig.ResponseTypes,
					TokenEndpointAuthMethod:            oauthAppConfig.TokenEndpointAuthMethod,
					PKCERequired:                       oauthAppConfig.PKCERequired,
					PublicClient:                       oauthAppConfig.PublicClient,
					RequirePushedAuthorizationRequests: oauthAppConfig.RequirePushedAuthorizationRequests,
					DPoPBoundAccessTokens:              oauthAppConfig.DPoPBoundAccessTokens,
					IncludeActClaim:                    oauthAppConfig.IncludeActClaim,
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
		RegistrationFlowID:        cfg.RegistrationFlowID,
		IsRegistrationFlowEnabled: cfg.IsRegistrationFlowEnabled,
		RecoveryFlowID:            cfg.RecoveryFlowID,
		IsRecoveryFlowEnabled:     cfg.IsRecoveryFlowEnabled,
		SignOutFlowID:             cfg.SignOutFlowID,
		IsSignOutFlowEnabled:      cfg.IsSignOutFlowEnabled,
		ThemeID:                   cfg.ThemeID,
		LayoutID:                  cfg.LayoutID,
		IsReadOnly:                cfg.IsReadOnly,
	}
	if cfg.Properties != nil {
		if t, ok := cfg.Properties[propTemplate].(string); ok {
			resp.Template = t
		}
		if logoURL, ok := cfg.Properties[propLogoURL].(string); ok {
			resp.LogoURL = logoURL
		}
	}
	// Enrich from entity system attributes.
	if e != nil {
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
		Name:        app.Name,
		Description: app.Description,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                app.AuthFlowID,
			RegistrationFlowID:        app.RegistrationFlowID,
			IsRegistrationFlowEnabled: app.IsRegistrationFlowEnabled,
			RecoveryFlowID:            app.RecoveryFlowID,
			IsRecoveryFlowEnabled:     app.IsRecoveryFlowEnabled,
			SignOutFlowID:             app.SignOutFlowID,
			IsSignOutFlowEnabled:      app.IsSignOutFlowEnabled,
			ThemeID:                   app.ThemeID,
			LayoutID:                  app.LayoutID,
			Assertion:                 assertion,
			AllowedUserTypes:          app.AllowedUserTypes,
			LoginConsent:              app.LoginConsent,
			Attestation:               app.Attestation,
		},
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
			GrantTypes:                         inboundAuthConfig.OAuthConfig.GrantTypes,
			ResponseTypes:                      inboundAuthConfig.OAuthConfig.ResponseTypes,
			TokenEndpointAuthMethod:            inboundAuthConfig.OAuthConfig.TokenEndpointAuthMethod,
			PKCERequired:                       inboundAuthConfig.OAuthConfig.PKCERequired,
			PublicClient:                       inboundAuthConfig.OAuthConfig.PublicClient,
			RequirePushedAuthorizationRequests: inboundAuthConfig.OAuthConfig.RequirePushedAuthorizationRequests,
			DPoPBoundAccessTokens:              inboundAuthConfig.OAuthConfig.DPoPBoundAccessTokens,
			IncludeActClaim:                    inboundAuthConfig.OAuthConfig.IncludeActClaim,
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
		Name:        app.Name,
		Description: app.Description,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                app.AuthFlowID,
			RegistrationFlowID:        app.RegistrationFlowID,
			IsRegistrationFlowEnabled: app.IsRegistrationFlowEnabled,
			RecoveryFlowID:            app.RecoveryFlowID,
			IsRecoveryFlowEnabled:     app.IsRecoveryFlowEnabled,
			SignOutFlowID:             app.SignOutFlowID,
			IsSignOutFlowEnabled:      app.IsSignOutFlowEnabled,
			ThemeID:                   app.ThemeID,
			LayoutID:                  app.LayoutID,
			Assertion:                 assertion,
			AllowedUserTypes:          app.AllowedUserTypes,
			LoginConsent:              app.LoginConsent,
			Attestation:               app.Attestation.WithoutCredentials(),
		},
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
				GrantTypes:                         inboundAuthConfig.OAuthConfig.GrantTypes,
				ResponseTypes:                      inboundAuthConfig.OAuthConfig.ResponseTypes,
				TokenEndpointAuthMethod:            inboundAuthConfig.OAuthConfig.TokenEndpointAuthMethod,
				PKCERequired:                       inboundAuthConfig.OAuthConfig.PKCERequired,
				PublicClient:                       inboundAuthConfig.OAuthConfig.PublicClient,
				RequirePushedAuthorizationRequests: inboundAuthConfig.OAuthConfig.RequirePushedAuthorizationRequests,
				DPoPBoundAccessTokens:              inboundAuthConfig.OAuthConfig.DPoPBoundAccessTokens,
				IncludeActClaim:                    inboundAuthConfig.OAuthConfig.IncludeActClaim,
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
