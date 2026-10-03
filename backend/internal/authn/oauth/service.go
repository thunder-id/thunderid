// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package oauth implements an authentication service for authenticating via an OAuth 2.0 based identity provider.
package oauth

import (
	"context"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/idp"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	loggerComponentName = "OAuthAuthnService"
)

// OAuthAuthnCoreServiceInterface defines the core contract for OAuth based authenticator services.
type OAuthAuthnCoreServiceInterface interface {
	BuildAuthorizeURL(ctx context.Context, idpID string) (string, map[string]string, *tidcommon.ServiceError)
	ExchangeCodeForToken(ctx context.Context, idpID, code string, validateResponse bool) (
		*TokenResponse, *tidcommon.ServiceError)
	FetchUserInfo(ctx context.Context, idpID, accessToken string) (
		map[string]interface{}, *tidcommon.ServiceError)
	GetOAuthClientConfig(ctx context.Context, idpID string) (*OAuthClientConfig, *tidcommon.ServiceError)
	Authenticate(ctx context.Context, idpID string, authzData common.AuthorizationData) (
		*common.AuthnResult, *tidcommon.ServiceError)
	BuildFederatedAuthResult(ctx context.Context, idpID, sub string, claims map[string]interface{}) (
		*common.AuthnResult, *tidcommon.ServiceError)
}

// OAuthAuthnServiceInterface defines the contract for OAuth based authenticator services.
type OAuthAuthnServiceInterface interface {
	OAuthAuthnCoreServiceInterface
	ValidateTokenResponse(ctx context.Context, idpID string, tokenResp *TokenResponse) *tidcommon.ServiceError
	FetchUserInfoWithClientConfig(ctx context.Context, oAuthClientConfig *OAuthClientConfig, accessToken string) (
		map[string]interface{}, *tidcommon.ServiceError)
}

// oAuthAuthnService is the default implementation of OAuthAuthnServiceInterface.
type oAuthAuthnService struct {
	httpClient syshttp.HTTPClientInterface
	idpService idp.IDPServiceInterface
	logger     *log.Logger
}

// newOAuthAuthnService creates a new instance of OAuth authenticator service.
func newOAuthAuthnService(httpClient syshttp.HTTPClientInterface,
	idpSvc idp.IDPServiceInterface,
) OAuthAuthnServiceInterface {
	return &oAuthAuthnService{
		httpClient: httpClient,
		idpService: idpSvc,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// GetOAuthClientConfig retrieves the OAuth client configuration for the given identity provider ID.
func (s *oAuthAuthnService) GetOAuthClientConfig(ctx context.Context, idpID string) (
	*OAuthClientConfig, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("idpId", idpID))
	if strings.TrimSpace(idpID) == "" {
		return nil, &ErrorEmptyIdpID
	}

	idp, svcErr := s.idpService.GetIdentityProvider(ctx, idpID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return nil, tidcommon.CustomServiceError(ErrorClientErrorWhileRetrievingIDP, tidcommon.I18nMessage{
				Key:          "error.oauthauthnservice.error_retrieving_idp_description",
				DefaultValue: "Error while retrieving identity provider: " + svcErr.ErrorDescription.DefaultValue,
			})
		}
		logger.Error(ctx, "Error while retrieving identity provider", log.String("errorCode", svcErr.Code),
			log.String("description", svcErr.ErrorDescription.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}
	if idp == nil {
		return nil, &ErrorInvalidIDP
	}

	oAuthClientConfig, err := parseIDPConfig(idp)
	if err != nil {
		logger.Error(ctx, "Failed to parse identity provider configurations", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return oAuthClientConfig, nil
}

// BuildAuthorizeURL constructs the authorization request URL for the external identity provider.
func (s *oAuthAuthnService) BuildAuthorizeURL(
	ctx context.Context, idpID string) (string, map[string]string, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("idpId", idpID))
	logger.Debug(ctx, "Building authorize URL")

	oAuthClientConfig, svcErr := s.GetOAuthClientConfig(ctx, idpID)
	if svcErr != nil {
		return "", nil, svcErr
	}
	if oAuthClientConfig.OAuthEndpoints.AuthorizationEndpoint == "" {
		logger.Error(ctx, "Authorization endpoint is not configured for the identity provider")
		return "", nil, &tidcommon.InternalServerError
	}

	queryParams := map[string]string{
		oauth2const.RequestParamClientID:     oAuthClientConfig.ClientID,
		oauth2const.RequestParamRedirectURI:  oAuthClientConfig.RedirectURI,
		oauth2const.RequestParamResponseType: oauth2const.RequestParamCode,
	}
	if len(oAuthClientConfig.Scopes) > 0 {
		queryParams[oauth2const.RequestParamScope] = sysutils.StringifyStringArray(oAuthClientConfig.Scopes, " ")
	}

	for key, value := range oAuthClientConfig.AdditionalParams {
		if key == "" || value == "" {
			continue
		}
		queryParams[key] = value
	}

	// Generate a random state parameter for CSRF protection.
	state := sysutils.GenerateUUID()
	queryParams[oauth2const.RequestParamState] = state

	authZURL, err := sysutils.GetURIWithQueryParams(oAuthClientConfig.OAuthEndpoints.AuthorizationEndpoint,
		queryParams)
	if err != nil {
		logger.Error(ctx, "Failed to build authorize URL", log.Error(err))
		return "", nil, &tidcommon.InternalServerError
	}

	metadata := map[string]string{
		oauth2const.RequestParamState: state,
	}

	return authZURL, metadata, nil
}

// ExchangeCodeForToken exchanges the authorization code for a token with the external identity provider
// and validates the token response if validateResponse is true.
func (s *oAuthAuthnService) ExchangeCodeForToken(ctx context.Context, idpID, code string, validateResponse bool) (
	*TokenResponse, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("idpId", idpID))
	logger.Debug(ctx, "Exchanging authorization code for token")

	if strings.TrimSpace(code) == "" {
		return nil, &ErrorEmptyAuthorizationCode
	}

	oAuthClientConfig, svcErr := s.GetOAuthClientConfig(ctx, idpID)
	if svcErr != nil {
		return nil, svcErr
	}
	if oAuthClientConfig.OAuthEndpoints.TokenEndpoint == "" {
		logger.Error(ctx, "Token endpoint is not configured for the identity provider")
		return nil, &tidcommon.InternalServerError
	}

	httpReq, svcErr := buildTokenRequest(ctx, oAuthClientConfig, code, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	tokenResp, svcErr := sendTokenRequest(httpReq, s.httpClient, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	if validateResponse {
		svcErr = s.ValidateTokenResponse(ctx, idpID, tokenResp)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	return tokenResp, nil
}

// ValidateTokenResponse validates the token response returned by the identity provider.
// ExchangeCodeForToken method calls this method to validate the token response if validateResponse is set
// to true. Hence generally you may not need to call this method explicitly.
func (s *oAuthAuthnService) ValidateTokenResponse(ctx context.Context,
	idpID string, tokenResp *TokenResponse) *tidcommon.ServiceError {
	logger := s.logger.With(log.String("idpId", idpID))
	logger.Debug(ctx, "Validating token response")

	if tokenResp == nil {
		logger.Debug(ctx, "Empty token response received from identity provider")
		return &ErrorInvalidTokenResponse
	}
	if tokenResp.AccessToken == "" {
		logger.Debug(ctx, "Access token is empty in the token response")
		return &ErrorInvalidTokenResponse
	}

	return nil
}

// FetchUserInfo retrieves user information from the external identity provider.
func (s *oAuthAuthnService) FetchUserInfo(ctx context.Context, idpID, accessToken string) (
	map[string]interface{}, *tidcommon.ServiceError) {
	oAuthClientConfig, svcErr := s.GetOAuthClientConfig(ctx, idpID)
	if svcErr != nil {
		return nil, svcErr
	}

	return s.FetchUserInfoWithClientConfig(ctx, oAuthClientConfig, accessToken)
}

// FetchUserInfoWithClientConfig retrieves user information using the provided OAuth client configuration.
func (s *oAuthAuthnService) FetchUserInfoWithClientConfig(ctx context.Context, oAuthClientConfig *OAuthClientConfig,
	accessToken string) (map[string]interface{}, *tidcommon.ServiceError) {
	logger := s.logger
	logger.Debug(ctx, "Fetching user info")

	if strings.TrimSpace(accessToken) == "" {
		return nil, &ErrorEmptyAccessToken
	}

	if oAuthClientConfig.OAuthEndpoints.UserInfoEndpoint == "" {
		logger.Error(ctx, "User info endpoint is not configured for the identity provider")
		return nil, &tidcommon.InternalServerError
	}

	httpReq, svcErr := buildUserInfoRequest(ctx, oAuthClientConfig.OAuthEndpoints.UserInfoEndpoint,
		accessToken, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	userInfo, svcErr := sendUserInfoRequest(httpReq, s.httpClient, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	ProcessSubClaim(userInfo)
	return userInfo, nil
}

// Authenticate performs the full OAuth authentication flow: exchanges the code for a token,
// resolves the user attributes, extracts the subject claim, and resolves the internal user.
// A missing internal user is NOT an error — the caller decides how to handle it.
func (s *oAuthAuthnService) Authenticate(ctx context.Context, idpID string,
	authzData common.AuthorizationData) (*common.AuthnResult, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("idpId", idpID))
	logger.Debug(ctx, "Performing federated OAuth authentication")

	tokenResp, svcErr := s.ExchangeCodeForToken(ctx, idpID, authzData.Code, true)
	if svcErr != nil {
		return nil, svcErr
	}

	oAuthClientConfig, svcErr := s.GetOAuthClientConfig(ctx, idpID)
	if svcErr != nil {
		return nil, svcErr
	}

	userInfo, svcErr := s.resolveUserAttributes(ctx, oAuthClientConfig, tokenResp, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	sub := ""
	if subVal, ok := userInfo["sub"]; ok && subVal != nil {
		if subStr, ok := subVal.(string); ok && subStr != "" {
			sub = subStr
		}
	}
	if sub == "" {
		logger.Debug(ctx, "sub claim not found in user info")
		return nil, &common.ErrorSubClaimNotFound
	}

	return s.BuildFederatedAuthResult(ctx, idpID, sub, userInfo)
}

// resolveUserAttributes reads the federated user's attributes from the provider's profile endpoint,
// falling back to the subject in a JWT access token for providers that expose no profile API. Only
// the subject is taken from the token: its other claims address the resource server, not the user.
func (s *oAuthAuthnService) resolveUserAttributes(ctx context.Context, oAuthClientConfig *OAuthClientConfig,
	tokenResp *TokenResponse, logger *log.Logger) (map[string]interface{}, *tidcommon.ServiceError) {
	if oAuthClientConfig.OAuthEndpoints.UserInfoEndpoint != "" {
		return s.FetchUserInfoWithClientConfig(ctx, oAuthClientConfig, tokenResp.AccessToken)
	}

	logger.Debug(ctx, "User profile endpoint is not configured, deriving the subject from the access token")

	claims, err := jwt.DecodeJWTPayload(tokenResp.AccessToken)
	if err != nil {
		logger.Debug(ctx, "Access token is not a JWT and no user profile endpoint is configured")
		return nil, &ErrorNoUserProfileSource
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		logger.Debug(ctx, "Access token carries no string sub claim and no user profile endpoint is configured")
		return nil, &ErrorNoUserProfileSource
	}

	return map[string]interface{}{"sub": sub}, nil
}

// BuildFederatedAuthResult maps the federated identity's raw claims to local attributes and
// returns the token that names the identity. It is the shared entry point every federated
// authenticator (OAuth, OIDC, Google, GitHub) calls, so mapping is applied uniformly.
//
// It resolves nothing. The token describes the identity, and the authn provider turns it into an
// entity.
func (s *oAuthAuthnService) BuildFederatedAuthResult(ctx context.Context, idpID, sub string,
	claims map[string]interface{}) (*common.AuthnResult, *tidcommon.ServiceError) {
	idpDTO, svcErr := s.getIDP(ctx, idpID)
	if svcErr != nil {
		return nil, svcErr
	}

	mappings := idp.GetAttributeMappings(idpDTO, claims)
	mappedClaims := idp.ApplyAttributeMappings(claims, mappings)

	token := map[string]interface{}{
		authnprovidercm.UserAttributeFederatedIdpID: idpID,
		authnprovidercm.UserAttributeSub:            sub,
	}

	// The account-linking lookups travel under their own key, so no attribute value can stand in for
	// the keys that name the identity or be read as one.
	if idpDTO.AttributeConfiguration != nil {
		filters := idp.AccountLinkingFilters(idpDTO.AttributeConfiguration.AccountLinking, mappedClaims, mappings)
		if len(filters) > 0 {
			token[authnprovidercm.AccountLinkingFiltersKey] = filters
		}
	}

	// A connection may map any claim onto the local attribute sub. The flow records the link from the
	// published sub, so it carries the verified subject whatever the mappings say.
	if mappedClaims == nil {
		mappedClaims = map[string]interface{}{}
	}
	mappedClaims[authnprovidercm.UserAttributeSub] = sub

	return &common.AuthnResult{
		Token:               token,
		AuthenticatedClaims: mappedClaims,
	}, nil
}

// getIDP loads the identity provider, wrapping IDP-retrieval errors in the authn domain so the IDP
// error code is not leaked to the caller.
func (s *oAuthAuthnService) getIDP(ctx context.Context, idpID string) (
	*providers.IDPDTO, *tidcommon.ServiceError) {
	idpDTO, svcErr := s.idpService.GetIdentityProvider(ctx, idpID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return nil, tidcommon.CustomServiceError(ErrorClientErrorWhileRetrievingIDP, tidcommon.I18nMessage{
				Key:          "error.oauthauthnservice.error_retrieving_idp_description",
				DefaultValue: "Error while retrieving identity provider: " + svcErr.ErrorDescription.DefaultValue,
			})
		}
		s.logger.Error(ctx, "Error while retrieving identity provider", log.String("errorCode", svcErr.Code),
			log.String("description", svcErr.ErrorDescription.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}
	if idpDTO == nil {
		return nil, &ErrorInvalidIDP
	}
	return idpDTO, nil
}
