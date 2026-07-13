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

// Package oauth implements an authentication service for authenticating via an OAuth 2.0 based identity provider.
package oauth

import (
	"context"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/idp"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	loggerComponentName = "OAuthAuthnService"
)

// OAuthAuthnCoreServiceInterface defines the core contract for OAuth based authenticator services.
type OAuthAuthnCoreServiceInterface interface {
	BuildAuthorizeURL(ctx context.Context, idpID string) (string, *tidcommon.ServiceError)
	ExchangeCodeForToken(ctx context.Context, idpID, code string, validateResponse bool) (
		*TokenResponse, *tidcommon.ServiceError)
	FetchUserInfo(ctx context.Context, idpID, accessToken string) (
		map[string]interface{}, *tidcommon.ServiceError)
	GetOAuthClientConfig(ctx context.Context, idpID string) (*OAuthClientConfig, *tidcommon.ServiceError)
	Authenticate(ctx context.Context, idpID, code string) (*common.AuthnResult, *tidcommon.ServiceError)
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
	httpClient     syshttp.HTTPClientInterface
	idpService     idp.IDPServiceInterface
	entityProvider entityprovider.EntityProviderInterface
	logger         *log.Logger
}

// newOAuthAuthnService creates a new instance of OAuth authenticator service.
func newOAuthAuthnService(httpClient syshttp.HTTPClientInterface,
	idpSvc idp.IDPServiceInterface, entityProvider entityprovider.EntityProviderInterface,
) OAuthAuthnServiceInterface {
	return &oAuthAuthnService{
		httpClient:     httpClient,
		idpService:     idpSvc,
		entityProvider: entityProvider,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
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
	ctx context.Context, idpID string) (string, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("idpId", idpID))
	logger.Debug(ctx, "Building authorize URL")

	oAuthClientConfig, svcErr := s.GetOAuthClientConfig(ctx, idpID)
	if svcErr != nil {
		return "", svcErr
	}
	if oAuthClientConfig.OAuthEndpoints.AuthorizationEndpoint == "" {
		logger.Error(ctx, "Authorization endpoint is not configured for the identity provider")
		return "", &tidcommon.InternalServerError
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

	authZURL, err := sysutils.GetURIWithQueryParams(oAuthClientConfig.OAuthEndpoints.AuthorizationEndpoint,
		queryParams)
	if err != nil {
		logger.Error(ctx, "Failed to build authorize URL", log.Error(err))
		return "", &tidcommon.InternalServerError
	}

	return authZURL, nil
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

// GetInternalUser retrieves the internal user based on the external subject identifier.
func (s *oAuthAuthnService) GetInternalUser(
	ctx context.Context, sub string) (*providers.Entity, *tidcommon.ServiceError) {
	logger := s.logger.With(log.MaskedString("sub", sub))
	logger.Debug(ctx, "Retrieving internal user for the given sub claim")

	if strings.TrimSpace(sub) == "" {
		return nil, &ErrorEmptySubClaim
	}

	filters := map[string]interface{}{
		"sub": sub,
	}
	userID, upErr := s.entityProvider.IdentifyEntity(filters)
	if upErr != nil {
		if upErr.Code == entityprovider.ErrorCodeEntityNotFound {
			logger.Debug(ctx, "No user found for the provided sub claim")
			return nil, &common.ErrorUserNotFound
		}
		if upErr.Code == entityprovider.ErrorCodeAmbiguousEntity {
			logger.Debug(ctx, "Multiple users found for the provided sub claim")
			return nil, &common.ErrorAmbiguousUser
		}
		logger.Error(ctx, "Error while identifying user", log.String("errorCode", string(upErr.Code)),
			log.String("description", upErr.Description))
		return nil, &tidcommon.InternalServerError
	}

	if userID == nil {
		logger.Debug(ctx, "User id is nil, no user found for the provided sub claim")
		return nil, &common.ErrorUserNotFound
	}

	user, upErr := s.entityProvider.GetEntity(*userID)
	if upErr != nil {
		if upErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return nil, &common.ErrorUserNotFound
		}
		logger.Error(ctx, "Error while retrieving user", log.String("errorCode", string(upErr.Code)),
			log.String("description", upErr.Description))
		return nil, &tidcommon.InternalServerError
	}

	return user, nil
}

// Authenticate performs the full OAuth authentication flow: exchanges the code for a token,
// fetches user info, extracts the subject claim, and resolves the internal user.
// A missing internal user is NOT an error — the caller decides how to handle it.
func (s *oAuthAuthnService) Authenticate(ctx context.Context, idpID, code string) (
	*common.AuthnResult, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("idpId", idpID))
	logger.Debug(ctx, "Performing federated OAuth authentication")

	tokenResp, svcErr := s.ExchangeCodeForToken(ctx, idpID, code, true)
	if svcErr != nil {
		return nil, svcErr
	}

	userInfo, svcErr := s.FetchUserInfo(ctx, idpID, tokenResp.AccessToken)
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

// BuildFederatedAuthResult maps the federated identity's raw claims to local attributes and derives
// the local-user lookup filter. It is the shared entry point every federated authenticator (OAuth,
// OIDC, Google, GitHub) calls, so mapping and account-linking resolution are applied uniformly.
func (s *oAuthAuthnService) BuildFederatedAuthResult(ctx context.Context, idpID, sub string,
	claims map[string]interface{}) (*common.AuthnResult, *tidcommon.ServiceError) {
	idpDTO, svcErr := s.getIDP(ctx, idpID)
	if svcErr != nil {
		return nil, svcErr
	}

	mappings := idp.GetAttributeMappings(idpDTO)
	mappedClaims := idp.ApplyAttributeMappings(claims, mappings)

	token, svcErr := s.buildAccountLinkingFilter(ctx, idpDTO, sub, mappedClaims, mappings)
	if svcErr != nil {
		return nil, svcErr
	}

	return &common.AuthnResult{
		Token:               token,
		AuthenticatedClaims: mappedClaims,
	}, nil
}

// buildAccountLinkingFilter resolves the local-user lookup filter for the federated identity: without
// account linking configured it returns the subject filter unchanged; otherwise it tries the subject
// first, then the configured account-linking attributes, falling back to the subject filter.
func (s *oAuthAuthnService) buildAccountLinkingFilter(ctx context.Context, idpDTO *providers.IDPDTO,
	sub string, mappedClaims map[string]interface{}, mappings []providers.AttributeMapping) (
	map[string]interface{}, *tidcommon.ServiceError) {
	subFilter := map[string]interface{}{"sub": sub}
	if idpDTO.AttributeConfiguration == nil || idpDTO.AttributeConfiguration.AccountLinking == nil {
		return subFilter, nil
	}

	resolved, ok, svcErr := s.resolveFilter(ctx, subFilter)
	if svcErr != nil {
		return nil, svcErr
	}
	if ok {
		return resolved, nil
	}

	externalToLocal := make(map[string]string)
	for _, m := range mappings {
		externalToLocal[m.ExternalAttribute] = m.LocalAttribute
	}

	linkFilter := make(map[string]interface{})
	for _, attr := range idpDTO.AttributeConfiguration.AccountLinking.Attributes {
		local := attr
		if mapped, ok := externalToLocal[attr]; ok {
			local = mapped
		}
		if value := sysutils.ConvertInterfaceValueToString(mappedClaims[local]); value != "" {
			linkFilter[local] = value
		}
	}
	if len(linkFilter) > 0 {
		return linkFilter, nil
	}

	return subFilter, nil
}

// resolveFilter looks up the filter and, on a unique match, returns a userID token so the caller need
// not repeat the lookup. "Not found" and "ambiguous" report ok=false with no error so the caller can
// try the next candidate filter; any other (server) error is surfaced.
func (s *oAuthAuthnService) resolveFilter(ctx context.Context, filter map[string]interface{}) (
	map[string]interface{}, bool, *tidcommon.ServiceError) {
	entityID, epErr := s.entityProvider.IdentifyEntity(filter)
	if epErr != nil {
		if epErr.Code == entityprovider.ErrorCodeEntityNotFound ||
			epErr.Code == entityprovider.ErrorCodeAmbiguousEntity {
			return nil, false, nil
		}
		s.logger.Error(ctx, "Error while identifying user for account linking",
			log.String("errorCode", string(epErr.Code)), log.String("description", epErr.Description))
		return nil, false, &tidcommon.InternalServerError
	}
	if entityID == nil {
		return nil, false, nil
	}
	return map[string]interface{}{common.UserAttributeUserID: *entityID}, true, nil
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
