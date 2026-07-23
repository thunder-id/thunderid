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

// Package oidc implements an authentication service for authenticating via an OIDC-based identity provider.
package oidc

import (
	"context"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	loggerComponentName = "OIDCAuthnService"
)

// OIDCAuthnCoreServiceInterface defines the core contract for OIDC based authenticator services.
type OIDCAuthnCoreServiceInterface interface {
	authnoauth.OAuthAuthnCoreServiceInterface
	ValidateIDToken(ctx context.Context, idpID, idToken string) *tidcommon.ServiceError
	GetIDTokenClaims(ctx context.Context, idToken string) (map[string]interface{}, *tidcommon.ServiceError)
}

// OIDCAuthnServiceInterface defines the contract for OIDC based authenticator services.
type OIDCAuthnServiceInterface interface {
	OIDCAuthnCoreServiceInterface
	ValidateTokenResponse(ctx context.Context, idpID string, tokenResp *authnoauth.TokenResponse,
		validateIDToken bool) *tidcommon.ServiceError
}

// oidcAuthnService is the default implementation of OIDCAuthnServiceInterface.
type oidcAuthnService struct {
	internal   authnoauth.OAuthAuthnServiceInterface
	jwtService jwt.JWTServiceInterface
	logger     *log.Logger
}

// newOIDCAuthnService creates a new instance of OIDC authenticator service.
func newOIDCAuthnService(internal authnoauth.OAuthAuthnServiceInterface,
	jwtSvc jwt.JWTServiceInterface) OIDCAuthnServiceInterface {
	return &oidcAuthnService{
		internal:   internal,
		jwtService: jwtSvc,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// GetOAuthClientConfig retrieves the OAuth client configuration for the given identity provider ID.
func (s *oidcAuthnService) GetOAuthClientConfig(ctx context.Context, idpID string) (
	*authnoauth.OAuthClientConfig, *tidcommon.ServiceError) {
	return s.internal.GetOAuthClientConfig(ctx, idpID)
}

// BuildAuthorizeURL constructs the authorization request URL for the external identity provider
// with an OIDC nonce parameter appended.
func (s *oidcAuthnService) BuildAuthorizeURL(
	ctx context.Context, idpID string) (string, map[string]string, *tidcommon.ServiceError) {
	authorizeURL, metadata, svcErr := s.internal.BuildAuthorizeURL(ctx, idpID)
	if svcErr != nil {
		return "", nil, svcErr
	}

	nonce := systemutils.GenerateUUID()
	authorizeURL = authorizeURL + "&nonce=" + nonce

	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata[oauth2const.RequestParamNonce] = nonce

	return authorizeURL, metadata, nil
}

// ExchangeCodeForToken exchanges the authorization code for a token with the external identity provider
// and validates the token response if validateResponse is true.
func (s *oidcAuthnService) ExchangeCodeForToken(ctx context.Context, idpID, code string, validateResponse bool) (
	*authnoauth.TokenResponse, *tidcommon.ServiceError) {
	tokenResp, svcErr := s.internal.ExchangeCodeForToken(ctx, idpID, code, false)
	if svcErr != nil {
		return nil, svcErr
	}

	if validateResponse {
		svcErr = s.ValidateTokenResponse(ctx, idpID, tokenResp, true)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	return tokenResp, nil
}

// ValidateTokenResponse validates the token response returned by the identity provider.
// ExchangeCodeForToken method calls this method to validate the token response if validateResponse is set
// to true. Hence generally you may not need to call this method explicitly.
func (s *oidcAuthnService) ValidateTokenResponse(ctx context.Context, idpID string,
	tokenResp *authnoauth.TokenResponse, validateIDToken bool) *tidcommon.ServiceError {
	logger := s.logger
	logger.Debug(ctx, "Validating token response")

	if tokenResp == nil {
		logger.Debug(ctx, "Empty token response received from identity provider")
		return &authnoauth.ErrorInvalidTokenResponse
	}
	if tokenResp.AccessToken == "" {
		logger.Debug(ctx, "Access token is empty in the token response")
		return &authnoauth.ErrorInvalidTokenResponse
	}
	if tokenResp.IDToken == "" {
		logger.Debug(ctx, "ID token is empty in the token response")
		return &authnoauth.ErrorInvalidTokenResponse
	}

	if validateIDToken {
		svcErr := s.ValidateIDToken(ctx, idpID, tokenResp.IDToken)
		if svcErr != nil {
			return svcErr
		}
	}

	return nil
}

// ValidateIDToken validates the ID token from the OIDC provider.
// ValidateTokenResponse method calls this method to validate the token response if validateIDToken is set
// to true. Hence generally you may not need to call this method explicitly if ExchangeCodeForToken method
// is called with validateResponse set to true.
func (s *oidcAuthnService) ValidateIDToken(ctx context.Context, idpID, idToken string) *tidcommon.ServiceError {
	logger := s.logger.With(log.String("idpId", idpID))
	logger.Debug(ctx, "Validating ID token")

	if strings.TrimSpace(idToken) == "" {
		logger.Debug(ctx, "ID token is empty")
		return &ErrorInvalidIDToken
	}

	oAuthClientConfig, svcErr := s.GetOAuthClientConfig(ctx, idpID)
	if svcErr != nil {
		return svcErr
	}

	// Validate ID token signature using JWKS endpoint if available
	if oAuthClientConfig.OAuthEndpoints.JwksEndpoint != "" {
		err := s.jwtService.VerifyJWTWithJWKS(ctx, idToken, oAuthClientConfig.OAuthEndpoints.JwksEndpoint, "", "")
		if err != nil {
			logger.Debug(ctx, "ID token signature validation failed",
				log.String("error", err.Error.DefaultValue))
			return &ErrorInvalidIDTokenSignature
		}
	} else {
		logger.Debug(ctx, "Skipping ID token signature validation as JWKS endpoint is not configured")
	}

	// TODO: Should mandate ID token validation when the support is available through a IDP configuration.
	//  Additionally should switch the validation method based on the configurations.
	//  For now, assumes validation is only performed if the JWKS endpoint is available.

	return nil
}

// GetIDTokenClaims extracts and returns the claims from the ID token.
func (s *oidcAuthnService) GetIDTokenClaims(ctx context.Context, idToken string) (
	map[string]interface{}, *tidcommon.ServiceError) {
	logger := s.logger
	logger.Debug(ctx, "Extracting claims from ID token")

	if strings.TrimSpace(idToken) == "" {
		logger.Debug(ctx, "ID token is empty")
		return nil, &ErrorInvalidIDToken
	}

	claims, err := jwt.DecodeJWTPayload(idToken)
	if err != nil {
		logger.Debug(ctx, "Failed to decode ID token payload", log.Error(err))
		return nil, &ErrorInvalidIDToken
	}

	return claims, nil
}

// FetchUserInfo retrieves user information from the external identity provider.
func (s *oidcAuthnService) FetchUserInfo(ctx context.Context, idpID, accessToken string) (
	map[string]interface{}, *tidcommon.ServiceError) {
	return s.internal.FetchUserInfo(ctx, idpID, accessToken)
}

// Authenticate performs the full OIDC authentication flow: exchanges the code for a token,
// extracts ID token claims, validates the nonce, and resolves the internal user.
// A missing internal user is NOT an error — the caller decides how to handle it.
func (s *oidcAuthnService) Authenticate(ctx context.Context, idpID string,
	authzData authncm.AuthorizationData) (*authncm.AuthnResult, *tidcommon.ServiceError) {
	logger := s.logger.With(log.String("idpId", idpID))
	logger.Debug(ctx, "Performing federated OIDC authentication")

	tokenResp, svcErr := s.ExchangeCodeForToken(ctx, idpID, authzData.Code, true)
	if svcErr != nil {
		return nil, svcErr
	}

	claims, svcErr := s.GetIDTokenClaims(ctx, tokenResp.IDToken)
	if svcErr != nil {
		return nil, svcErr
	}

	// Validate that the ID token nonce matches the server-generated nonce.
	if svcErr := authnoauth.ValidateNonce(ctx, claims, authzData.Nonce, logger); svcErr != nil {
		logger.Debug(ctx, "OIDC nonce validation failed")
		return nil, svcErr
	}

	// Extract sub claim from the ID token claims.
	sub := ""
	if subVal, ok := claims["sub"]; ok && subVal != nil {
		if subStr, ok := subVal.(string); ok && subStr != "" {
			sub = subStr
		}
	}
	if sub == "" {
		logger.Debug(ctx, "sub claim not found in ID token")
		return nil, &authncm.ErrorSubClaimNotFound
	}

	// Fetch user info if additional scopes are configured so callers get the full attribute set.
	oauthConfig, svcErr := s.GetOAuthClientConfig(ctx, idpID)
	if svcErr == nil && len(oauthConfig.Scopes) > 1 {
		userInfo, infoErr := s.FetchUserInfo(ctx, idpID, tokenResp.AccessToken)
		if infoErr == nil {
			if userInfoSub, ok := userInfo["sub"].(string); !ok || userInfoSub == sub {
				for k, v := range userInfo {
					if _, exists := claims[k]; !exists {
						claims[k] = v
					}
				}
			} else {
				logger.Debug(ctx, "UserInfo sub mismatch, skipping attribute merge")
			}
		}
	}

	return s.internal.BuildFederatedAuthResult(ctx, idpID, sub, claims)
}

// BuildFederatedAuthResult delegates to the underlying OAuth service, which applies attribute mapping
// and account-linking resolution uniformly for all federated authenticators.
func (s *oidcAuthnService) BuildFederatedAuthResult(ctx context.Context, idpID, sub string,
	claims map[string]interface{}) (*authncm.AuthnResult, *tidcommon.ServiceError) {
	return s.internal.BuildFederatedAuthResult(ctx, idpID, sub, claims)
}
