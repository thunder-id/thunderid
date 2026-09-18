// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	oauthutils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/ou"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// DCRServiceInterface defines the interface for the DCR service.
type DCRServiceInterface interface {
	RegisterClient(
		ctx context.Context, request *DCRRegistrationRequest,
	) (*DCRRegistrationResponse, *tidcommon.ServiceError)
	// GetClient returns the current registration of a dynamically registered client (RFC 7592).
	GetClient(ctx context.Context, clientID string) (*DCRRegistrationResponse, *tidcommon.ServiceError)
	// UpdateClient replaces the registered metadata of a client, preserving its client_id (RFC 7592).
	UpdateClient(ctx context.Context, clientID string, request *DCRRegistrationRequest) (
		*DCRRegistrationResponse, *tidcommon.ServiceError)
	// DeleteClient removes a dynamically registered client (RFC 7592).
	DeleteClient(ctx context.Context, clientID string) *tidcommon.ServiceError
}

// dcrService is the default implementation of DCRServiceInterface.
type dcrService struct {
	appService    application.ApplicationServiceInterface
	ouService     ou.OrganizationUnitServiceInterface
	i18nService   i18nmgt.I18nServiceInterface
	transactioner providers.Transactioner
	cfg           oauthconfig.Config
}

// newDCRService creates a new instance of dcrService.
func newDCRService(
	appService application.ApplicationServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	i18nService i18nmgt.I18nServiceInterface,
	transactioner providers.Transactioner,
	cfg oauthconfig.Config,
) DCRServiceInterface {
	return &dcrService{
		appService:    appService,
		ouService:     ouService,
		i18nService:   i18nService,
		transactioner: transactioner,
		cfg:           cfg,
	}
}

// RegisterClient registers a new OAuth client using Dynamic Client Registration.
func (ds *dcrService) RegisterClient(ctx context.Context, request *DCRRegistrationRequest) (
	*DCRRegistrationResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DCRService"))

	if request == nil {
		return nil, &ErrorInvalidRequestFormat
	}

	if request.JWKSUri != "" && len(request.JWKS) > 0 {
		return nil, &ErrorJWKSConfigurationConflict
	}
	if request.JWKSUri != "" {
		parsedJWKSURI, err := sysutils.ParseURL(request.JWKSUri)
		if err != nil || parsedJWKSURI.Scheme != "https" || parsedJWKSURI.Host == "" {
			return nil, &ErrorInvalidClientMetadata
		}
	}

	// TODO: Revisit OU for DCR apps
	if request.OUID == "" {
		rootOUs, svcErr := ds.ouService.GetOrganizationUnitList(ctx, 1, 0, nil)
		if svcErr != nil {
			logger.Error(ctx, "Failed to retrieve root organization units for DCR",
				log.String("error", svcErr.Error.DefaultValue))
			return nil, &ErrorServerError
		}
		if rootOUs == nil || rootOUs.TotalResults == 0 || len(rootOUs.OrganizationUnits) == 0 {
			logger.Error(ctx, "No root organization unit available for DCR registration")
			return nil, &ErrorServerError
		}
		request.OUID = rootOUs.OrganizationUnits[0].ID
	}

	appDTO, svcErr := ds.convertDCRToApplication(request)
	if svcErr != nil {
		logger.Error(ctx, "Failed to convert DCR request to application DTO",
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &ErrorServerError
	}

	var response *DCRRegistrationResponse
	var capturedErr *tidcommon.ServiceError
	var createdAppID string

	err := ds.transactioner.Transact(ctx, func(txCtx context.Context) error {
		createdApp, svcErr := ds.appService.CreateApplication(txCtx, appDTO)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
				logger.Error(ctx, "Failed to create application via Application service",
					log.String("error_code", svcErr.Code))
				capturedErr = &ErrorServerError
				return errors.New("failed to create application")
			}
			logger.Debug(ctx, "Failed to create application via Application service",
				log.String("error_code", svcErr.Code))
			capturedErr = ds.mapApplicationErrorToDCRError(svcErr)
			return errors.New("failed to create application")
		}

		createdAppID = createdApp.ID

		var convErr *tidcommon.ServiceError
		response, convErr = ds.convertApplicationToDCRResponse(createdApp, request.ClientName)
		if convErr != nil {
			logger.Error(ctx, "Failed to convert application to DCR response",
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
		logger.Error(ctx, "Failed to write localized variants for DCR client; compensating by deleting app",
			log.String("appID", createdAppID), log.String("error", writeErr.Error.DefaultValue))
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cleanupCancel()
		for _, field := range []string{"name", "logo_uri", "tos_uri", "policy_uri"} {
			if cleanErr := ds.i18nService.DeleteTranslationsByKey(
				cleanupCtx, application.AppI18nNamespace(), application.AppI18nKey(createdAppID, field),
			); cleanErr != nil {
				logger.Error(ctx, "Failed to clean up partial i18n row after write failure",
					log.String("appID", createdAppID), log.String("field", field))
			}
		}
		if delSvcErr := ds.appService.DeleteApplication(cleanupCtx, createdAppID); delSvcErr != nil {
			logger.Error(ctx,
				"Compensation delete failed after i18n write failure; app record may be orphaned",
				log.String("appID", createdAppID))
		}
		return nil, writeErr
	}

	response.LocalizedClientName = request.LocalizedClientName
	response.LocalizedLogoURI = request.LocalizedLogoURI
	response.LocalizedTosURI = request.LocalizedTosURI
	response.LocalizedPolicyURI = request.LocalizedPolicyURI

	response.ClientIDIssuedAt = time.Now().Unix()

	return response, nil
}

// resolveClient looks up a registered client by client ID, returning both the OAuth client and the
// application that carries its human-readable metadata. A deleted or unknown client resolves to
// ErrorClientNotFound, so every operation on a client that no longer exists reports it as not
// found rather than acting on nothing.
func (ds *dcrService) resolveClient(ctx context.Context, clientID string) (
	*providers.OAuthClient, *providers.Application, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DCRService"))

	oauthClient, svcErr := ds.appService.GetOAuthApplication(ctx, clientID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			logger.Error(ctx, "Failed to retrieve OAuth application for client configuration request",
				log.String("error_code", svcErr.Code))
			return nil, nil, &ErrorServerError
		}
		return nil, nil, &ErrorClientNotFound
	}

	app, svcErr := ds.appService.GetApplication(ctx, oauthClient.ID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			logger.Error(ctx, "Failed to retrieve application for client configuration request",
				log.String("error_code", svcErr.Code))
			return nil, nil, &ErrorServerError
		}
		return nil, nil, &ErrorClientNotFound
	}

	return oauthClient, app, nil
}

// GetClient returns the current registration of a dynamically registered client.
func (ds *dcrService) GetClient(ctx context.Context, clientID string) (
	*DCRRegistrationResponse, *tidcommon.ServiceError) {
	oauthClient, app, svcErr := ds.resolveClient(ctx, clientID)
	if svcErr != nil {
		return nil, svcErr
	}
	return ds.buildClientConfigurationResponse(oauthClient, app)
}

// DeleteClient removes a dynamically registered client. Once deleted, the client no longer
// resolves, so every later operation on it reports the client as not found.
func (ds *dcrService) DeleteClient(ctx context.Context, clientID string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DCRService"))

	oauthClient, _, svcErr := ds.resolveClient(ctx, clientID)
	if svcErr != nil {
		return svcErr
	}

	if delErr := ds.appService.DeleteApplication(ctx, oauthClient.ID); delErr != nil {
		if delErr.Type == tidcommon.ServerErrorType {
			logger.Error(ctx, "Failed to delete application for client configuration request",
				log.String("error_code", delErr.Code))
			return &ErrorServerError
		}
		return ds.mapApplicationErrorToDCRError(delErr)
	}
	return nil
}

// UpdateClient replaces the registered metadata of a client. Per RFC 7592 the update is a full
// replacement of the client metadata, but the client_id and client_secret are preserved.
func (ds *dcrService) UpdateClient(ctx context.Context, clientID string, request *DCRRegistrationRequest) (
	*DCRRegistrationResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DCRService"))

	if request == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	if request.JWKSUri != "" && len(request.JWKS) > 0 {
		return nil, &ErrorJWKSConfigurationConflict
	}
	if request.JWKSUri != "" {
		parsedJWKSURI, err := sysutils.ParseURL(request.JWKSUri)
		if err != nil || parsedJWKSURI.Scheme != "https" || parsedJWKSURI.Host == "" {
			return nil, &ErrorInvalidClientMetadata
		}
	}

	oauthClient, app, svcErr := ds.resolveClient(ctx, clientID)
	if svcErr != nil {
		return nil, svcErr
	}

	appDTO, svcErr := ds.convertDCRToApplication(request)
	if svcErr != nil {
		logger.Error(ctx, "Failed to convert client configuration request to application DTO",
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &ErrorServerError
	}

	// Carry forward the identity of the existing registration. UpdateApplication replaces the
	// application wholesale, so anything not restored here would be lost: an omitted client ID is
	// regenerated (dropping the OAuth-app certificate), an omitted OU or name fails validation,
	// and an empty client secret is what preserves the existing one.
	appDTO.ID = app.ID
	appDTO.OUID = app.OUID
	appDTO.InboundAuthProfile = app.InboundAuthProfile
	// The application type is immutable; leaving it empty inherits the existing type.
	appDTO.Type = ""
	if len(appDTO.InboundAuthConfig) > 0 && appDTO.InboundAuthConfig[0].OAuthConfig != nil {
		appDTO.InboundAuthConfig[0].OAuthConfig.ClientID = oauthClient.ClientID
		appDTO.InboundAuthConfig[0].OAuthConfig.ClientSecret = ""
	}
	if request.ClientName == "" && len(request.LocalizedClientName) == 0 {
		appDTO.Name = app.Name
	}

	updatedApp, updErr := ds.appService.UpdateApplication(ctx, app.ID, appDTO)
	if updErr != nil {
		if updErr.Type == tidcommon.ServerErrorType {
			logger.Error(ctx, "Failed to update application via Application service",
				log.String("error_code", updErr.Code))
			return nil, &ErrorServerError
		}
		logger.Debug(ctx, "Failed to update application via Application service",
			log.String("error_code", updErr.Code))
		return nil, ds.mapApplicationErrorToDCRError(updErr)
	}

	if writeErr := ds.writeLocalizedVariants(ctx, app.ID, request); writeErr != nil {
		logger.Error(ctx, "Failed to write localized variants for updated client",
			log.String("appID", app.ID), log.String("error", writeErr.Error.DefaultValue))
		return nil, writeErr
	}

	// appDTO.Name carries the name that was actually applied, which is the inherited one when the
	// request omitted client_name. Passing request.ClientName here would fall back to the client ID
	// and report a name the registration does not have.
	response, convErr := ds.convertApplicationToDCRResponse(updatedApp, appDTO.Name)
	if convErr != nil {
		logger.Error(ctx, "Failed to convert updated application to DCR response",
			log.String("error", convErr.Error.DefaultValue))
		return nil, convErr
	}

	// The stored client secret cannot be read back, and this update does not rotate it.
	response.ClientSecret = ""
	response.LocalizedClientName = request.LocalizedClientName
	response.LocalizedLogoURI = request.LocalizedLogoURI
	response.LocalizedTosURI = request.LocalizedTosURI
	response.LocalizedPolicyURI = request.LocalizedPolicyURI

	return response, nil
}

// buildClientConfigurationResponse renders a registered client as an RFC 7592 client information
// response. The client secret is deliberately absent: ThunderID stores it write-only, so it cannot
// be read back after registration.
func (ds *dcrService) buildClientConfigurationResponse(
	oauthClient *providers.OAuthClient, app *providers.Application) (
	*DCRRegistrationResponse, *tidcommon.ServiceError) {
	var jwksURI string
	var jwks map[string]interface{}
	if oauthClient.Certificate != nil {
		switch oauthClient.Certificate.Type {
		case cert.CertificateTypeJWKSURI:
			jwksURI = oauthClient.Certificate.Value
		case cert.CertificateTypeJWKS:
			if err := json.Unmarshal([]byte(oauthClient.Certificate.Value), &jwks); err != nil {
				return nil, &ErrorServerError
			}
		}
	}

	var userInfoSignedAlg, userInfoEncryptedAlg, userInfoEncryptedEnc string
	if oauthClient.UserInfo != nil {
		userInfoSignedAlg = oauthClient.UserInfo.SigningAlg
		userInfoEncryptedAlg = oauthClient.UserInfo.EncryptionAlg
		userInfoEncryptedEnc = oauthClient.UserInfo.EncryptionEnc
	}

	var idTokenSignedAlg, idTokenEncryptedAlg, idTokenEncryptedEnc string
	if oauthClient.Token != nil && oauthClient.Token.IDToken != nil {
		idTokenSignedAlg = oauthClient.Token.IDToken.SigningAlg
		idTokenEncryptedAlg = oauthClient.Token.IDToken.EncryptionAlg
		idTokenEncryptedEnc = oauthClient.Token.IDToken.EncryptionEnc
	}

	response := &DCRRegistrationResponse{
		ClientID:                           oauthClient.ClientID,
		ClientSecretExpiresAt:              ClientSecretExpiresAtNever,
		RedirectURIs:                       oauthClient.RedirectURIs,
		PostLogoutRedirectURIs:             oauthClient.PostLogoutRedirectURIs,
		GrantTypes:                         oauthClient.GrantTypes,
		ResponseTypes:                      oauthClient.ResponseTypes,
		ClientName:                         app.Name,
		ClientURI:                          app.URL,
		LogoURI:                            app.LogoURL,
		TokenEndpointAuthMethod:            oauthClient.TokenEndpointAuthMethod,
		JWKSUri:                            jwksURI,
		JWKS:                               jwks,
		Scope:                              strings.Join(oauthClient.Scopes, " "),
		TosURI:                             app.TosURI,
		PolicyURI:                          app.PolicyURI,
		Contacts:                           app.Contacts,
		AppID:                              app.ID,
		RequirePushedAuthorizationRequests: oauthClient.RequirePushedAuthorizationRequests,
		DPoPBoundAccessTokens:              oauthClient.DPoPBoundAccessTokens,
		UserInfoSignedResponseAlg:          userInfoSignedAlg,
		UserInfoEncryptedResponseAlg:       userInfoEncryptedAlg,
		UserInfoEncryptedResponseEnc:       userInfoEncryptedEnc,
		IDTokenSignedResponseAlg:           idTokenSignedAlg,
		IDTokenEncryptedResponseAlg:        idTokenEncryptedAlg,
		IDTokenEncryptedResponseEnc:        idTokenEncryptedEnc,
	}

	return response, nil
}

// convertDCRToApplication converts DCR registration request to Application DTO.
func (ds *dcrService) convertDCRToApplication(request *DCRRegistrationRequest) (
	*model.ApplicationDTO, *tidcommon.ServiceError) {
	isPublicClient := request.TokenEndpointAuthMethod == providers.TokenEndpointAuthMethodNone

	var oauthCertificate *inboundmodel.Certificate
	if request.JWKSUri != "" {
		oauthCertificate = &inboundmodel.Certificate{
			Type:  cert.CertificateTypeJWKSURI,
			Value: request.JWKSUri,
		}
	} else if len(request.JWKS) > 0 {
		jwksBytes, err := json.Marshal(request.JWKS)
		if err != nil {
			return nil, &ErrorServerError
		}
		oauthCertificate = &inboundmodel.Certificate{
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

	// Generate client ID during DCR conversion so certificate-backed clients can persist
	// the OAuth-app certificate against the final client_id.
	// When localized variants are present without a client_name, use an i18n ref as the app name
	// so the UI resolves the display name from the i18n table rather than falling back to the clientID.
	clientID, err := oauthutils.GenerateOAuth2ClientID()
	if err != nil {
		return nil, &ErrorServerError
	}
	appName := request.ClientName
	if appName == "" {
		if len(request.LocalizedClientName) > 0 {
			appName = application.AppI18nRef(appID, "name")
		} else {
			appName = clientID
		}
	} else if len(request.LocalizedClientName) > 0 {
		// Store a template reference so the UI can resolve the name from the i18n table.
		appName = application.AppI18nRef(appID, "name")
	}

	oauthAppConfig := &providers.OAuthConfigWithSecret{
		ClientID:                           clientID,
		RedirectURIs:                       request.RedirectURIs,
		PostLogoutRedirectURIs:             request.PostLogoutRedirectURIs,
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
		Certificate:                        oauthCertificate,
	}

	inboundAuthConfig := []providers.InboundAuthConfigWithSecret{
		{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: oauthAppConfig,
		},
	}

	appDTO := &model.ApplicationDTO{
		ID:   appID,
		OUID: request.OUID,
		Name: appName,
		// Dynamic Client Registration (RFC 7591) has no concept of ThunderID's application type, and
		// a DCR-registered client can take any shape, so it is always registered as custom.
		Type:              model.ApplicationTypeCustom,
		URL:               request.ClientURI,
		LogoURL:           request.LogoURI,
		InboundAuthConfig: inboundAuthConfig,
		TosURI:            request.TosURI,
		PolicyURI:         request.PolicyURI,
		Contacts:          request.Contacts,
	}

	return appDTO, nil
}

// buildUserInfoConfig maps UserInfo alg fields from a DCR request to a UserInfoConfig.
// ResponseType is derived from the algorithm fields per OIDC DCR conventions.
func buildUserInfoConfig(request *DCRRegistrationRequest) *providers.UserInfoConfig {
	if request.UserInfoSignedResponseAlg == "" && request.UserInfoEncryptedResponseAlg == "" &&
		request.UserInfoEncryptedResponseEnc == "" {
		return nil
	}
	hasSign := request.UserInfoSignedResponseAlg != ""
	hasEnc := request.UserInfoEncryptedResponseAlg != ""
	var responseType providers.UserInfoResponseType
	switch {
	case hasSign && hasEnc:
		responseType = providers.UserInfoResponseTypeNESTEDJWT
	case hasEnc:
		responseType = providers.UserInfoResponseTypeJWE
	case hasSign:
		responseType = providers.UserInfoResponseTypeJWS
	default:
		responseType = providers.UserInfoResponseTypeJSON
	}
	return &providers.UserInfoConfig{
		ResponseType:  responseType,
		SigningAlg:    request.UserInfoSignedResponseAlg,
		EncryptionAlg: request.UserInfoEncryptedResponseAlg,
		EncryptionEnc: request.UserInfoEncryptedResponseEnc,
	}
}

// buildTokenConfig builds the OAuthTokenConfig from DCR request fields.
func buildTokenConfig(request *DCRRegistrationRequest) *providers.OAuthTokenConfig {
	idToken := buildIDTokenConfig(request)
	if idToken == nil {
		return nil
	}
	return &providers.OAuthTokenConfig{IDToken: idToken}
}

// buildIDTokenConfig maps ID token alg fields from a DCR request to an IDTokenConfig.
// ResponseType is derived from the algorithm fields per OIDC DCR conventions.
func buildIDTokenConfig(request *DCRRegistrationRequest) *providers.IDTokenConfig {
	if request.IDTokenSignedResponseAlg == "" && request.IDTokenEncryptedResponseAlg == "" &&
		request.IDTokenEncryptedResponseEnc == "" {
		return nil
	}
	hasEnc := request.IDTokenEncryptedResponseAlg != "" || request.IDTokenEncryptedResponseEnc != ""
	responseType := providers.IDTokenResponseTypeJWT
	switch {
	case hasEnc && request.IDTokenSignedResponseAlg != "":
		responseType = providers.IDTokenResponseTypeNESTEDJWT
	case hasEnc:
		responseType = providers.IDTokenResponseTypeJWE
	}
	return &providers.IDTokenConfig{
		ResponseType:  responseType,
		SigningAlg:    request.IDTokenSignedResponseAlg,
		EncryptionAlg: request.IDTokenEncryptedResponseAlg,
		EncryptionEnc: request.IDTokenEncryptedResponseEnc,
	}
}

// convertApplicationToDCRResponse converts Application DTO to DCR registration response.
func (ds *dcrService) convertApplicationToDCRResponse(appDTO *model.ApplicationDTO, originalClientName string) (
	*DCRRegistrationResponse, *tidcommon.ServiceError) {
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
	if oauthConfig.Certificate != nil {
		switch oauthConfig.Certificate.Type {
		case cert.CertificateTypeJWKSURI:
			jwksURI = oauthConfig.Certificate.Value
		case cert.CertificateTypeJWKS:
			if err := json.Unmarshal([]byte(oauthConfig.Certificate.Value), &jwks); err != nil {
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

	var idTokenSignedAlg, idTokenEncryptedAlg, idTokenEncryptedEnc string
	if oauthConfig.Token != nil && oauthConfig.Token.IDToken != nil {
		idTokenSignedAlg = oauthConfig.Token.IDToken.SigningAlg
		idTokenEncryptedAlg = oauthConfig.Token.IDToken.EncryptionAlg
		idTokenEncryptedEnc = oauthConfig.Token.IDToken.EncryptionEnc
	}

	response := &DCRRegistrationResponse{
		ClientID:                           oauthConfig.ClientID,
		ClientSecret:                       oauthConfig.ClientSecret,
		ClientSecretExpiresAt:              ClientSecretExpiresAtNever,
		RedirectURIs:                       oauthConfig.RedirectURIs,
		PostLogoutRedirectURIs:             oauthConfig.PostLogoutRedirectURIs,
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
		IDTokenSignedResponseAlg:           idTokenSignedAlg,
		IDTokenEncryptedResponseAlg:        idTokenEncryptedAlg,
		IDTokenEncryptedResponseEnc:        idTokenEncryptedEnc,
	}

	return response, nil
}

// writeLocalizedVariants persists all localized variants from a DCR request to the i18n table.
// The non-tagged default value for each field is also stored under SystemLanguage; an explicit
// #SystemLanguage-tagged variant in the same request takes priority over the default.
func (ds *dcrService) writeLocalizedVariants(
	ctx context.Context, appID string, request *DCRRegistrationRequest) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DCRService"))
	if ds.i18nService == nil {
		logger.Debug(ctx, "i18n service not configured, skipping localized variant writes")
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
		if svcErr.Type == tidcommon.ClientErrorType {
			logger.Debug(ctx, "Invalid client metadata in localized variants",
				log.String("appID", appID),
				log.String("errorCode", svcErr.Code),
				log.String("error", svcErr.Error.DefaultValue))
			return &ErrorServerError
		}
		logger.Error(ctx, "Failed to write localized variants",
			log.String("appID", appID),
			log.String("errorCode", svcErr.Code),
			log.String("error", svcErr.Error.DefaultValue))
		return &ErrorServerError
	}
	return nil
}

// mapApplicationErrorToDCRError maps Application service errors to DCR standard errors.
func (ds *dcrService) mapApplicationErrorToDCRError(
	appErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	dcrErr := &tidcommon.ServiceError{
		Type:             appErr.Type,
		Error:            appErr.Error,
		ErrorDescription: appErr.ErrorDescription,
	}

	switch appErr.Code {
	// Redirect URI validation errors
	case "APP-1012":
		dcrErr.Code = ErrorInvalidRedirectURI.Code
	// Server errors
	case tidcommon.InternalServerError.Code, tidcommon.ErrorEncodingError.Code:
		dcrErr.Code = ErrorServerError.Code
	// Default fallback for all other client errors
	default:
		dcrErr.Code = ErrorInvalidClientMetadata.Code
	}

	return dcrErr
}
