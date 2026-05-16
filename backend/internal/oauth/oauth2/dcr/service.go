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

package dcr

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// DCRServiceInterface defines the interface for the DCR service.
type DCRServiceInterface interface {
	RegisterClient(
		ctx context.Context, request *DCRRegistrationRequest,
	) (*DCRRegistrationResponse, *serviceerror.ServiceError)
}

// dcrService is the default implementation of DCRServiceInterface.
type dcrService struct {
	appService    application.ApplicationServiceInterface
	ouService     ou.OrganizationUnitServiceInterface
	i18nService   i18nmgt.I18nServiceInterface
	transactioner transaction.Transactioner
}

// newDCRService creates a new instance of dcrService.
func newDCRService(
	appService application.ApplicationServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	i18nService i18nmgt.I18nServiceInterface,
	transactioner transaction.Transactioner,
) DCRServiceInterface {
	return &dcrService{
		appService:    appService,
		ouService:     ouService,
		i18nService:   i18nService,
		transactioner: transactioner,
	}
}

// RegisterClient registers a new OAuth client using Dynamic Client Registration.
func (ds *dcrService) RegisterClient(ctx context.Context, request *DCRRegistrationRequest) (
	*DCRRegistrationResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DCRService"))

	if request == nil {
		return nil, &ErrorInvalidRequestFormat
	}

	if request.JWKSUri != "" && len(request.JWKS) > 0 {
		return nil, &ErrorJWKSConfigurationConflict
	}

	// TODO: Revisit OU for DCR apps
	if request.OUID == "" {
		rootOUs, svcErr := ds.ouService.GetOrganizationUnitList(ctx, 1, 0, nil)
		if svcErr != nil {
			logger.Error("Failed to retrieve root organization units for DCR",
				log.String("error", svcErr.Error.DefaultValue))
			return nil, &ErrorServerError
		}
		if rootOUs == nil || rootOUs.TotalResults == 0 || len(rootOUs.OrganizationUnits) == 0 {
			logger.Error("No root organization unit available for DCR registration")
			return nil, &ErrorServerError
		}
		request.OUID = rootOUs.OrganizationUnits[0].ID
	}

	appDTO, svcErr := ds.convertDCRToApplication(request)
	if svcErr != nil {
		logger.Error("Failed to convert DCR request to application DTO", log.String("error", svcErr.Error.DefaultValue))
		return nil, &ErrorServerError
	}

	var response *DCRRegistrationResponse
	var capturedErr *serviceerror.ServiceError
	var createdAppID string

	err := ds.transactioner.Transact(ctx, func(txCtx context.Context) error {
		createdApp, svcErr := ds.appService.CreateApplication(txCtx, appDTO)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ServerErrorType {
				logger.Error("Failed to create application via Application service",
					log.String("error_code", svcErr.Code))
				capturedErr = &ErrorServerError
				return errors.New("failed to create application")
			}
			logger.Debug("Failed to create application via Application service",
				log.String("error_code", svcErr.Code))
			capturedErr = ds.mapApplicationErrorToDCRError(svcErr)
			return errors.New("failed to create application")
		}

		createdAppID = createdApp.ID

		var convErr *serviceerror.ServiceError
		response, convErr = ds.convertApplicationToDCRResponse(createdApp, request.ClientName)
		if convErr != nil {
			logger.Error("Failed to convert application to DCR response",
				log.String("error", convErr.Error.DefaultValue))
			capturedErr = convErr
			return errors.New("conversion failed")
		}

		return nil
	})

	if err != nil {
		if capturedErr != nil {
			return nil, capturedErr
		}
		return nil, &ErrorServerError
	}

	// Write localized variants outside the transaction above because the i18n service
	// uses a separate configDB connection and cannot join the same transaction.
	// If writing fails, clean up any partial i18n rows and compensate by deleting the created app.
	// Note: writeLocalizedVariants only returns an error when i18nService is non-nil,
	// so calling DeleteTranslationsByKey here without a nil guard is safe.
	// If the compensation DeleteApplication also fails, the app record is left without localized
	// metadata — an accepted gap that can be cleaned up manually or via a future sweep.
	if writeErr := ds.writeLocalizedVariants(ctx, createdAppID, request); writeErr != nil {
		logger.Error("Failed to write localized variants for DCR client; compensating by deleting app",
			log.String("appID", createdAppID), log.String("error", writeErr.Error.DefaultValue))
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cleanupCancel()
		for _, field := range []string{"name", "logo_uri", "tos_uri", "policy_uri"} {
			if cleanErr := ds.i18nService.DeleteTranslationsByKey(
				cleanupCtx, application.AppI18nNamespace(), application.AppI18nKey(createdAppID, field),
			); cleanErr != nil {
				logger.Error("Failed to clean up partial i18n row after write failure",
					log.String("appID", createdAppID), log.String("field", field))
			}
		}
		if delSvcErr := ds.appService.DeleteApplication(cleanupCtx, createdAppID); delSvcErr != nil {
			logger.Error("Compensation delete failed after i18n write failure; app record may be orphaned",
				log.String("appID", createdAppID))
		}
		return nil, writeErr
	}

	response.LocalizedClientName = request.LocalizedClientName
	response.LocalizedLogoURI = request.LocalizedLogoURI
	response.LocalizedTosURI = request.LocalizedTosURI
	response.LocalizedPolicyURI = request.LocalizedPolicyURI

	return response, nil
}

// convertDCRToApplication converts DCR registration request to Application DTO.
func (ds *dcrService) convertDCRToApplication(request *DCRRegistrationRequest) (
	*model.ApplicationDTO, *serviceerror.ServiceError) {
	isPublicClient := request.TokenEndpointAuthMethod == oauth2const.TokenEndpointAuthMethodNone

	// Map JWKS/JWKS_URI to application-level certificate
	var appCertificate *inboundmodel.Certificate
	if request.JWKSUri != "" {
		appCertificate = &inboundmodel.Certificate{
			Type:  cert.CertificateTypeJWKSURI,
			Value: request.JWKSUri,
		}
	} else if len(request.JWKS) > 0 {
		jwksBytes, err := json.Marshal(request.JWKS)
		if err != nil {
			return nil, &ErrorServerError
		}
		appCertificate = &inboundmodel.Certificate{
			Type:  cert.CertificateTypeJWKS,
			Value: string(jwksBytes),
		}
	}

	var scopes []string
	if request.Scope != "" {
		scopes = strings.Fields(request.Scope)
	}

	// Pre-generate the application ID so we can build an i18n template reference if needed.
	appID, uuidErr := sysutils.GenerateUUIDv7()
	if uuidErr != nil {
		return nil, &ErrorServerError
	}

	// Generate client ID if client_name is not provided and use it as both app name and client ID.
	// When localized variants are present without a client_name, use an i18n ref as the app name
	// so the UI resolves the display name from the i18n table rather than falling back to the clientID.
	var clientID string
	appName := request.ClientName
	if appName == "" {
		generatedClientID, err := oauthutils.GenerateOAuth2ClientID()
		if err != nil {
			return nil, &ErrorServerError
		}
		clientID = generatedClientID
		if len(request.LocalizedClientName) > 0 {
			appName = application.AppI18nRef(appID, "name")
		} else {
			appName = clientID
		}
	} else if len(request.LocalizedClientName) > 0 {
		// Store a template reference so the UI can resolve the name from the i18n table.
		appName = application.AppI18nRef(appID, "name")
	}

	oauthAppConfig := &inboundmodel.OAuthConfigWithSecret{
		ClientID:                           clientID,
		RedirectURIs:                       request.RedirectURIs,
		GrantTypes:                         request.GrantTypes,
		ResponseTypes:                      request.ResponseTypes,
		TokenEndpointAuthMethod:            request.TokenEndpointAuthMethod,
		PublicClient:                       isPublicClient,
		PKCERequired:                       isPublicClient,
		RequirePushedAuthorizationRequests: request.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              request.DPoPBoundAccessTokens,
		Scopes:                             scopes,
		UserInfo:                           buildUserInfoConfig(request),
		Token:                              buildTokenConfig(request),
	}

	inboundAuthConfig := []inboundmodel.InboundAuthConfigWithSecret{
		{
			Type:        inboundmodel.OAuthInboundAuthType,
			OAuthConfig: oauthAppConfig,
		},
	}

	appDTO := &model.ApplicationDTO{
		ID:                appID,
		OUID:              request.OUID,
		Name:              appName,
		URL:               request.ClientURI,
		LogoURL:           request.LogoURI,
		InboundAuthConfig: inboundAuthConfig,
		TosURI:            request.TosURI,
		PolicyURI:         request.PolicyURI,
		Contacts:          request.Contacts,
		InboundAuthProfile: inboundmodel.InboundAuthProfile{
			Certificate: appCertificate,
		},
	}

	return appDTO, nil
}

// buildUserInfoConfig maps UserInfo alg fields from a DCR request to a UserInfoConfig.
// ResponseType is derived from the algorithm fields per OIDC DCR conventions.
func buildUserInfoConfig(request *DCRRegistrationRequest) *inboundmodel.UserInfoConfig {
	if request.UserInfoSignedResponseAlg == "" && request.UserInfoEncryptedResponseAlg == "" &&
		request.UserInfoEncryptedResponseEnc == "" {
		return nil
	}
	hasSign := request.UserInfoSignedResponseAlg != ""
	hasEnc := request.UserInfoEncryptedResponseAlg != ""
	var responseType inboundmodel.UserInfoResponseType
	switch {
	case hasSign && hasEnc:
		responseType = inboundmodel.UserInfoResponseTypeNESTEDJWT
	case hasEnc:
		responseType = inboundmodel.UserInfoResponseTypeJWE
	case hasSign:
		responseType = inboundmodel.UserInfoResponseTypeJWS
	default:
		responseType = inboundmodel.UserInfoResponseTypeJSON
	}
	return &inboundmodel.UserInfoConfig{
		ResponseType:  responseType,
		SigningAlg:    request.UserInfoSignedResponseAlg,
		EncryptionAlg: request.UserInfoEncryptedResponseAlg,
		EncryptionEnc: request.UserInfoEncryptedResponseEnc,
	}
}

// buildTokenConfig builds the OAuthTokenConfig from DCR request fields.
func buildTokenConfig(request *DCRRegistrationRequest) *inboundmodel.OAuthTokenConfig {
	idToken := buildIDTokenConfig(request)
	if idToken == nil {
		return nil
	}
	return &inboundmodel.OAuthTokenConfig{IDToken: idToken}
}

// buildIDTokenConfig maps ID token encryption fields from a DCR request to an IDTokenConfig.
func buildIDTokenConfig(request *DCRRegistrationRequest) *inboundmodel.IDTokenConfig {
	if request.IDTokenEncryptedResponseAlg == "" && request.IDTokenEncryptedResponseEnc == "" {
		return nil
	}
	return &inboundmodel.IDTokenConfig{
		ResponseType:  inboundmodel.IDTokenResponseTypeJWE,
		EncryptionAlg: request.IDTokenEncryptedResponseAlg,
		EncryptionEnc: request.IDTokenEncryptedResponseEnc,
	}
}

// convertApplicationToDCRResponse converts Application DTO to DCR registration response.
func (ds *dcrService) convertApplicationToDCRResponse(appDTO *model.ApplicationDTO, originalClientName string) (
	*DCRRegistrationResponse, *serviceerror.ServiceError) {
	if len(appDTO.InboundAuthConfig) == 0 || appDTO.InboundAuthConfig[0].OAuthConfig == nil {
		return nil, &ErrorServerError
	}

	oauthConfig := appDTO.InboundAuthConfig[0].OAuthConfig

	clientName := originalClientName
	if clientName == "" {
		clientName = oauthConfig.ClientID
	}

	var jwksURI string
	var jwks map[string]interface{}
	if appDTO.Certificate != nil {
		switch appDTO.Certificate.Type {
		case cert.CertificateTypeJWKSURI:
			jwksURI = appDTO.Certificate.Value
		case cert.CertificateTypeJWKS:
			if err := json.Unmarshal([]byte(appDTO.Certificate.Value), &jwks); err != nil {
				return nil, &ErrorServerError
			}
		}
	}

	scopeString := strings.Join(oauthConfig.Scopes, " ")

	var userInfoSignedAlg, userInfoEncryptedAlg, userInfoEncryptedEnc string
	if oauthConfig.UserInfo != nil {
		userInfoSignedAlg = oauthConfig.UserInfo.SigningAlg
		userInfoEncryptedAlg = oauthConfig.UserInfo.EncryptionAlg
		userInfoEncryptedEnc = oauthConfig.UserInfo.EncryptionEnc
	}

	var idTokenEncryptedAlg, idTokenEncryptedEnc string
	if oauthConfig.Token != nil && oauthConfig.Token.IDToken != nil {
		idTokenEncryptedAlg = oauthConfig.Token.IDToken.EncryptionAlg
		idTokenEncryptedEnc = oauthConfig.Token.IDToken.EncryptionEnc
	}

	response := &DCRRegistrationResponse{
		ClientID:                           oauthConfig.ClientID,
		ClientSecret:                       oauthConfig.ClientSecret,
		ClientSecretExpiresAt:              ClientSecretExpiresAtNever,
		RedirectURIs:                       oauthConfig.RedirectURIs,
		GrantTypes:                         oauthConfig.GrantTypes,
		ResponseTypes:                      oauthConfig.ResponseTypes,
		ClientName:                         clientName,
		ClientURI:                          appDTO.URL,
		LogoURI:                            appDTO.LogoURL,
		TokenEndpointAuthMethod:            oauthConfig.TokenEndpointAuthMethod,
		JWKSUri:                            jwksURI,
		JWKS:                               jwks,
		Scope:                              scopeString,
		TosURI:                             appDTO.TosURI,
		PolicyURI:                          appDTO.PolicyURI,
		Contacts:                           appDTO.Contacts,
		AppID:                              appDTO.ID,
		RequirePushedAuthorizationRequests: oauthConfig.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              oauthConfig.DPoPBoundAccessTokens,
		UserInfoSignedResponseAlg:          userInfoSignedAlg,
		UserInfoEncryptedResponseAlg:       userInfoEncryptedAlg,
		UserInfoEncryptedResponseEnc:       userInfoEncryptedEnc,
		IDTokenEncryptedResponseAlg:        idTokenEncryptedAlg,
		IDTokenEncryptedResponseEnc:        idTokenEncryptedEnc,
	}

	return response, nil
}

// writeLocalizedVariants persists all localized variants from a DCR request to the i18n table.
// The non-tagged default value for each field is also stored under SystemLanguage; an explicit
// #SystemLanguage-tagged variant in the same request takes priority over the default.
func (ds *dcrService) writeLocalizedVariants(
	ctx context.Context, appID string, request *DCRRegistrationRequest) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DCRService"))
	if ds.i18nService == nil {
		logger.Debug("i18n service not configured, skipping localized variant writes")
		return nil
	}
	ns := application.AppI18nNamespace()
	type fieldSpec struct {
		variants    map[string]string
		defaultVal  string
		key         string
		validateURI func(string) bool
	}
	fields := []fieldSpec{
		{request.LocalizedClientName, request.ClientName, application.AppI18nKey(appID, "name"), nil},
		{request.LocalizedLogoURI, request.LogoURI, application.AppI18nKey(appID, "logo_uri"), sysutils.IsValidLogoURI},
		{request.LocalizedTosURI, request.TosURI, application.AppI18nKey(appID, "tos_uri"), sysutils.IsValidURI},
		{request.LocalizedPolicyURI, request.PolicyURI,
			application.AppI18nKey(appID, "policy_uri"), sysutils.IsValidURI},
	}
	entries := make(map[string]map[string]string)
	for _, f := range fields {
		for tag, val := range f.variants {
			if f.validateURI != nil && !f.validateURI(val) {
				return &ErrorInvalidClientMetadata
			}
			if entries[f.key] == nil {
				entries[f.key] = make(map[string]string)
			}
			entries[f.key][tag] = val
		}
	}
	for _, f := range fields {
		if f.defaultVal == "" {
			continue
		}
		if entries[f.key] == nil {
			entries[f.key] = make(map[string]string)
		}
		if _, exists := entries[f.key][i18nmgt.SystemLanguage]; !exists {
			entries[f.key][i18nmgt.SystemLanguage] = f.defaultVal
		}
	}
	if len(entries) == 0 {
		return nil
	}
	if svcErr := ds.i18nService.SetTranslationOverridesForNamespace(ctx, ns, entries); svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			logger.Debug("Invalid client metadata in localized variants",
				log.String("appID", appID),
				log.String("errorCode", svcErr.Code),
				log.String("error", svcErr.Error.DefaultValue))
			return &ErrorServerError
		}
		logger.Error("Failed to write localized variants",
			log.String("appID", appID),
			log.String("errorCode", svcErr.Code),
			log.String("error", svcErr.Error.DefaultValue))
		return &ErrorServerError
	}
	return nil
}

// mapApplicationErrorToDCRError maps Application service errors to DCR standard errors.
func (ds *dcrService) mapApplicationErrorToDCRError(
	appErr *serviceerror.ServiceError) *serviceerror.ServiceError {
	dcrErr := &serviceerror.ServiceError{
		Type:             appErr.Type,
		Error:            appErr.Error,
		ErrorDescription: appErr.ErrorDescription,
	}

	switch appErr.Code {
	// Redirect URI validation errors
	case "APP-1012":
		dcrErr.Code = ErrorInvalidRedirectURI.Code
	// Server errors
	case "APP-5001", "APP-5002":
		dcrErr.Code = ErrorServerError.Code
	// Default fallback for all other client errors
	default:
		dcrErr.Code = ErrorInvalidClientMetadata.Code
	}

	return dcrErr
}
