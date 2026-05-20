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
	"net/http"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"

	"github.com/thunder-id/thunderid/internal/application/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// ApplicationHandler defines the handler for managing application API requests.
type applicationHandler struct {
	service ApplicationServiceInterface
}

func newApplicationHandler(service ApplicationServiceInterface) *applicationHandler {
	return &applicationHandler{
		service: service,
	}
}

// HandleApplicationPostRequest handles the application request.
func (ah *applicationHandler) HandleApplicationPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ApplicationHandler"))

	appRequest, err := sysutils.DecodeJSONBody[model.ApplicationRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidRequestFormat.Code,
			Message:     ErrorInvalidRequestFormat.Error,
			Description: ErrorInvalidRequestFormat.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	appDTO := model.ApplicationDTO{
		OUID:        appRequest.OUID,
		Name:        appRequest.Name,
		Description: appRequest.Description,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			AuthFlowID:                appRequest.AuthFlowID,
			RegistrationFlowID:        appRequest.RegistrationFlowID,
			IsRegistrationFlowEnabled: appRequest.IsRegistrationFlowEnabled,
			RecoveryFlowID:            appRequest.RecoveryFlowID,
			IsRecoveryFlowEnabled:     appRequest.IsRecoveryFlowEnabled,
			ThemeID:                   appRequest.ThemeID,
			LayoutID:                  appRequest.LayoutID,
			Assertion:                 appRequest.Assertion,
			Certificate:               appRequest.Certificate,
			AllowedUserTypes:          appRequest.AllowedUserTypes,
			LoginConsent:              appRequest.LoginConsent,
		},
		Template:  appRequest.Template,
		URL:       appRequest.URL,
		LogoURL:   appRequest.LogoURL,
		TosURI:    appRequest.TosURI,
		PolicyURI: appRequest.PolicyURI,
		Contacts:  appRequest.Contacts,
		Metadata:  appRequest.Metadata,
	}
	appDTO.InboundAuthConfig = ah.processInboundAuthConfigFromRequest(appRequest.InboundAuthConfig)

	// Create the app using the application service.
	createdAppDTO, svcErr := ah.service.CreateApplication(ctx, &appDTO)
	if svcErr != nil {
		ah.handleError(w, r, svcErr)
		return
	}

	returnApp := model.ApplicationCompleteResponse{
		ID:          createdAppDTO.ID,
		OUID:        createdAppDTO.OUID,
		Name:        createdAppDTO.Name,
		Description: createdAppDTO.Description,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			AuthFlowID:                createdAppDTO.AuthFlowID,
			RegistrationFlowID:        createdAppDTO.RegistrationFlowID,
			IsRegistrationFlowEnabled: createdAppDTO.IsRegistrationFlowEnabled,
			RecoveryFlowID:            createdAppDTO.RecoveryFlowID,
			IsRecoveryFlowEnabled:     createdAppDTO.IsRecoveryFlowEnabled,
			ThemeID:                   createdAppDTO.ThemeID,
			LayoutID:                  createdAppDTO.LayoutID,
			Assertion:                 createdAppDTO.Assertion,
			Certificate:               createdAppDTO.Certificate,
			AllowedUserTypes:          createdAppDTO.AllowedUserTypes,
			LoginConsent:              createdAppDTO.LoginConsent,
		},
		Template:  createdAppDTO.Template,
		URL:       createdAppDTO.URL,
		LogoURL:   createdAppDTO.LogoURL,
		TosURI:    createdAppDTO.TosURI,
		PolicyURI: createdAppDTO.PolicyURI,
		Contacts:  createdAppDTO.Contacts,
		Metadata:  createdAppDTO.Metadata,
	}

	// TODO: Need to refactor when supporting other/multiple inbound auth types.
	if len(createdAppDTO.InboundAuthConfig) > 0 {
		success := ah.processInboundAuthConfig(logger, createdAppDTO, &returnApp)
		if !success {
			errResp := apierror.ErrorResponse{
				Code:        serviceerror.InternalServerError.Code,
				Message:     serviceerror.InternalServerError.Error,
				Description: serviceerror.InternalServerError.ErrorDescription,
			}
			sysutils.WriteErrorResponse(w, http.StatusInternalServerError, errResp)
			return
		}
	}

	sysutils.WriteSuccessResponse(w, http.StatusCreated, returnApp)
}

// HandleApplicationListRequest handles the application request.
func (ah *applicationHandler) HandleApplicationListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	listResponse, svcErr := ah.service.GetApplicationList(ctx)
	if svcErr != nil {
		ah.handleError(w, r, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, listResponse)
}

// HandleApplicationGetRequest handles the application request.
func (ah *applicationHandler) HandleApplicationGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ApplicationHandler"))

	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidApplicationID.Code,
			Message:     ErrorInvalidApplicationID.Error,
			Description: ErrorInvalidApplicationID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	appDTO, svcErr := ah.service.GetApplication(ctx, id)
	if svcErr != nil {
		ah.handleError(w, r, svcErr)
		return
	}

	returnApp := model.ApplicationGetResponse{
		ID:          appDTO.ID,
		OUID:        appDTO.OUID,
		Name:        appDTO.Name,
		Description: appDTO.Description,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			AuthFlowID:                appDTO.AuthFlowID,
			RegistrationFlowID:        appDTO.RegistrationFlowID,
			IsRegistrationFlowEnabled: appDTO.IsRegistrationFlowEnabled,
			RecoveryFlowID:            appDTO.RecoveryFlowID,
			IsRecoveryFlowEnabled:     appDTO.IsRecoveryFlowEnabled,
			ThemeID:                   appDTO.ThemeID,
			LayoutID:                  appDTO.LayoutID,
			Assertion:                 appDTO.Assertion,
			Certificate:               appDTO.Certificate,
			AllowedUserTypes:          appDTO.AllowedUserTypes,
			LoginConsent:              appDTO.LoginConsent,
		},
		Template:  appDTO.Template,
		URL:       appDTO.URL,
		LogoURL:   appDTO.LogoURL,
		TosURI:    appDTO.TosURI,
		PolicyURI: appDTO.PolicyURI,
		Contacts:  appDTO.Contacts,
		Metadata:  appDTO.Metadata,
	}

	// TODO: Need to refactor when supporting other/multiple inbound auth types.
	if len(appDTO.InboundAuthConfig) > 0 {
		if appDTO.InboundAuthConfig[0].Type != inboundmodel.OAuthInboundAuthType {
			logger.Error("Unsupported inbound authentication type returned",
				log.String("type", string(appDTO.InboundAuthConfig[0].Type)))

			errResp := apierror.ErrorResponse{
				Code:        serviceerror.InternalServerError.Code,
				Message:     serviceerror.InternalServerError.Error,
				Description: serviceerror.InternalServerError.ErrorDescription,
			}
			sysutils.WriteErrorResponse(w, http.StatusInternalServerError, errResp)
			return
		}

		if appDTO.InboundAuthConfig[0].OAuthConfig == nil {
			logger.Error("OAuth application configuration is nil")

			errResp := apierror.ErrorResponse{
				Code:        serviceerror.InternalServerError.Code,
				Message:     serviceerror.InternalServerError.Error,
				Description: serviceerror.InternalServerError.ErrorDescription,
			}
			sysutils.WriteErrorResponse(w, http.StatusInternalServerError, errResp)
			return
		}

		returnInboundAuthConfigs := make([]inboundmodel.InboundAuthConfig, 0, len(appDTO.InboundAuthConfig))
		for _, config := range appDTO.InboundAuthConfig {
			if config.OAuthConfig == nil {
				logger.Error("OAuth application configuration is nil")
				errResp := apierror.ErrorResponse{
					Code:        serviceerror.InternalServerError.Code,
					Message:     serviceerror.InternalServerError.Error,
					Description: serviceerror.InternalServerError.ErrorDescription,
				}
				sysutils.WriteErrorResponse(w, http.StatusInternalServerError, errResp)
				return
			}
			redirectURIs := config.OAuthConfig.RedirectURIs
			if len(redirectURIs) == 0 {
				redirectURIs = []string{}
			}
			grantTypes := config.OAuthConfig.GrantTypes
			if len(grantTypes) == 0 {
				grantTypes = []oauth2const.GrantType{}
			}
			responseTypes := config.OAuthConfig.ResponseTypes
			if len(responseTypes) == 0 {
				responseTypes = []oauth2const.ResponseType{}
			}
			oAuthAppConfig := inboundmodel.OAuthConfig{
				ClientID:                           config.OAuthConfig.ClientID,
				RedirectURIs:                       redirectURIs,
				GrantTypes:                         grantTypes,
				ResponseTypes:                      responseTypes,
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
			}
			returnInboundAuthConfigs = append(returnInboundAuthConfigs, inboundmodel.InboundAuthConfig{
				Type:        config.Type,
				OAuthConfig: &oAuthAppConfig,
			})
		}
		returnApp.InboundAuthConfig = returnInboundAuthConfigs
		returnApp.ClientID = appDTO.InboundAuthConfig[0].OAuthConfig.ClientID
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, returnApp)
}

// HandleApplicationPutRequest handles the application request.
func (ah *applicationHandler) HandleApplicationPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ApplicationHandler"))

	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidApplicationID.Code,
			Message:     ErrorInvalidApplicationID.Error,
			Description: ErrorInvalidApplicationID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	appRequest, err := sysutils.DecodeJSONBody[model.ApplicationRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidRequestFormat.Code,
			Message:     ErrorInvalidRequestFormat.Error,
			Description: ErrorInvalidRequestFormat.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	updateReqAppDTO := model.ApplicationDTO{
		ID:          id,
		OUID:        appRequest.OUID,
		Name:        appRequest.Name,
		Description: appRequest.Description,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			AuthFlowID:                appRequest.AuthFlowID,
			RegistrationFlowID:        appRequest.RegistrationFlowID,
			IsRegistrationFlowEnabled: appRequest.IsRegistrationFlowEnabled,
			RecoveryFlowID:            appRequest.RecoveryFlowID,
			IsRecoveryFlowEnabled:     appRequest.IsRecoveryFlowEnabled,
			ThemeID:                   appRequest.ThemeID,
			LayoutID:                  appRequest.LayoutID,
			Assertion:                 appRequest.Assertion,
			Certificate:               appRequest.Certificate,
			AllowedUserTypes:          appRequest.AllowedUserTypes,
			LoginConsent:              appRequest.LoginConsent,
		},
		Template:  appRequest.Template,
		URL:       appRequest.URL,
		LogoURL:   appRequest.LogoURL,
		TosURI:    appRequest.TosURI,
		PolicyURI: appRequest.PolicyURI,
		Contacts:  appRequest.Contacts,
		Metadata:  appRequest.Metadata,
	}
	updateReqAppDTO.InboundAuthConfig = ah.processInboundAuthConfigFromRequest(appRequest.InboundAuthConfig)

	// Update the application using the application service.
	updatedAppDTO, svcErr := ah.service.UpdateApplication(ctx, id, &updateReqAppDTO)
	if svcErr != nil {
		ah.handleError(w, r, svcErr)
		return
	}

	returnApp := model.ApplicationCompleteResponse{
		ID:          updatedAppDTO.ID,
		OUID:        updatedAppDTO.OUID,
		Name:        updatedAppDTO.Name,
		Description: updatedAppDTO.Description,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			AuthFlowID:                updatedAppDTO.AuthFlowID,
			RegistrationFlowID:        updatedAppDTO.RegistrationFlowID,
			IsRegistrationFlowEnabled: updatedAppDTO.IsRegistrationFlowEnabled,
			RecoveryFlowID:            updatedAppDTO.RecoveryFlowID,
			IsRecoveryFlowEnabled:     updatedAppDTO.IsRecoveryFlowEnabled,
			ThemeID:                   updatedAppDTO.ThemeID,
			LayoutID:                  updatedAppDTO.LayoutID,
			Assertion:                 updatedAppDTO.Assertion,
			Certificate:               updatedAppDTO.Certificate,
			AllowedUserTypes:          updatedAppDTO.AllowedUserTypes,
			LoginConsent:              updatedAppDTO.LoginConsent,
		},
		Template:  updatedAppDTO.Template,
		URL:       updatedAppDTO.URL,
		LogoURL:   updatedAppDTO.LogoURL,
		TosURI:    updatedAppDTO.TosURI,
		PolicyURI: updatedAppDTO.PolicyURI,
		Contacts:  updatedAppDTO.Contacts,
		Metadata:  updatedAppDTO.Metadata,
	}

	// TODO: Need to refactor when supporting other/multiple inbound auth types.
	if len(updatedAppDTO.InboundAuthConfig) > 0 {
		success := ah.processInboundAuthConfig(logger, updatedAppDTO, &returnApp)
		if !success {
			errResp := apierror.ErrorResponse{
				Code:        serviceerror.InternalServerError.Code,
				Message:     serviceerror.InternalServerError.Error,
				Description: serviceerror.InternalServerError.ErrorDescription,
			}
			sysutils.WriteErrorResponse(w, http.StatusInternalServerError, errResp)
			return
		}
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, returnApp)
}

// HandleApplicationDeleteRequest handles the application request.
func (ah *applicationHandler) HandleApplicationDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidApplicationID.Code,
			Message:     ErrorInvalidApplicationID.Error,
			Description: ErrorInvalidApplicationID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	svcErr := ah.service.DeleteApplication(ctx, id)
	if svcErr != nil {
		ah.handleError(w, r, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
}

// processInboundAuthConfig prepares the response for OAuth app configuration.
func (ah *applicationHandler) processInboundAuthConfig(logger *log.Logger, appDTO *model.ApplicationDTO,
	returnApp *model.ApplicationCompleteResponse) bool {
	if len(appDTO.InboundAuthConfig) > 0 {
		if appDTO.InboundAuthConfig[0].Type != inboundmodel.OAuthInboundAuthType {
			logger.Error("Unsupported inbound authentication type returned",
				log.String("type", string(appDTO.InboundAuthConfig[0].Type)))

			return false
		}

		if appDTO.InboundAuthConfig[0].OAuthConfig == nil {
			logger.Error("OAuth application configuration is nil")
			return false
		}

		returnInboundAuthConfigs := make([]inboundmodel.InboundAuthConfigWithSecret, 0, len(appDTO.InboundAuthConfig))
		for _, config := range appDTO.InboundAuthConfig {
			if config.OAuthConfig == nil {
				logger.Error("OAuth application configuration is nil")
				return false
			}
			redirectURIs := config.OAuthConfig.RedirectURIs
			if len(redirectURIs) == 0 {
				redirectURIs = []string{}
			}
			grantTypes := config.OAuthConfig.GrantTypes
			if len(grantTypes) == 0 {
				grantTypes = []oauth2const.GrantType{}
			}
			responseTypes := config.OAuthConfig.ResponseTypes
			if len(responseTypes) == 0 {
				responseTypes = []oauth2const.ResponseType{}
			}
			oAuthAppConfig := inboundmodel.OAuthConfigWithSecret{
				ClientID:                           config.OAuthConfig.ClientID,
				ClientSecret:                       config.OAuthConfig.ClientSecret,
				RedirectURIs:                       redirectURIs,
				GrantTypes:                         grantTypes,
				ResponseTypes:                      responseTypes,
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
			}
			returnInboundAuthConfigs = append(returnInboundAuthConfigs, inboundmodel.InboundAuthConfigWithSecret{
				Type:        config.Type,
				OAuthConfig: &oAuthAppConfig,
			})
		}
		returnApp.InboundAuthConfig = returnInboundAuthConfigs
		returnApp.ClientID = appDTO.InboundAuthConfig[0].OAuthConfig.ClientID
	}

	return true
}

// handleError handles service errors and returns appropriate HTTP responses.
// When the resolved status is 500, the error is logged with request context.
func (ah *applicationHandler) handleError(w http.ResponseWriter, r *http.Request,
	svcErr *serviceerror.ServiceError) {
	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		if svcErr.Code == ErrorApplicationNotFound.Code {
			statusCode = http.StatusNotFound
		} else {
			statusCode = http.StatusBadRequest
		}
	}

	if statusCode == http.StatusInternalServerError {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ApplicationHandler"))
		logger.Error("Internal server error processing application request",
			log.String("method", r.Method),
			log.String("path", r.URL.Path),
			log.String("error_code", svcErr.Code),
			log.String("error", svcErr.Error.DefaultValue),
		)
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}

// processInboundAuthConfigFromRequest processes inbound auth config from request to DTO.
func (ah *applicationHandler) processInboundAuthConfigFromRequest(
	configs []inboundmodel.InboundAuthConfigWithSecret) []inboundmodel.InboundAuthConfigWithSecret {
	if len(configs) == 0 {
		return nil
	}

	inboundAuthConfigDTOs := make([]inboundmodel.InboundAuthConfigWithSecret, 0)
	for _, config := range configs {
		if config.Type != inboundmodel.OAuthInboundAuthType || config.OAuthConfig == nil {
			continue
		}

		inboundAuthConfigDTO := inboundmodel.InboundAuthConfigWithSecret{
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
		}
		inboundAuthConfigDTOs = append(inboundAuthConfigDTOs, inboundAuthConfigDTO)
	}
	return inboundAuthConfigDTOs
}
