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

// Package userinfo provides functionality for the OIDC UserInfo endpoint.
package userinfo

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

const serviceLoggerComponentName = "UserInfoService"

// userInfoServiceInterface defines the interface for OIDC UserInfo endpoint.
type userInfoServiceInterface interface {
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfoResponse, *serviceerror.ServiceError)
}

// userInfoService implements the userInfoServiceInterface.
type userInfoService struct {
	jwtService        jwt.JWTServiceInterface
	jweService        jwe.JWEServiceInterface
	jwksResolver      *jwksresolver.Resolver
	tokenValidator    tokenservice.TokenValidatorInterface
	inboundClient     inboundclient.InboundClientServiceInterface
	ouService         ou.OrganizationUnitServiceInterface
	attributeCacheSvc attributecache.AttributeCacheServiceInterface
	transactioner     transaction.Transactioner
	logger            *log.Logger
}

// newUserInfoService creates a new userInfoService instance.
func newUserInfoService(
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	resolver *jwksresolver.Resolver,
	tokenValidator tokenservice.TokenValidatorInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	attributeCacheSvc attributecache.AttributeCacheServiceInterface,
	transactioner transaction.Transactioner,
) userInfoServiceInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, serviceLoggerComponentName))
	return &userInfoService{
		jwtService:        jwtService,
		jweService:        jweService,
		jwksResolver:      resolver,
		tokenValidator:    tokenValidator,
		inboundClient:     inboundClient,
		ouService:         ouService,
		attributeCacheSvc: attributeCacheSvc,
		transactioner:     transactioner,
		logger:            logger,
	}
}

// GetUserInfo validates the access token and returns user information based on authorized scopes.
func (s *userInfoService) GetUserInfo(
	ctx context.Context, accessToken string,
) (*UserInfoResponse, *serviceerror.ServiceError) {
	if accessToken == "" {
		return nil, &errorInvalidAccessToken
	}

	accessTokenClaims, err := s.tokenValidator.ValidateAccessToken(accessToken)
	if err != nil {
		s.logger.Debug("Failed to verify access token", log.Error(err))
		return nil, &errorInvalidAccessToken
	}
	tokenClaims := accessTokenClaims.Claims
	sub := accessTokenClaims.Sub

	if svcErr := s.validateGrantType(tokenClaims); svcErr != nil {
		return nil, svcErr
	}

	scopes := s.extractScopes(tokenClaims)

	// Validate that the 'openid' scope is present
	if svcErr := s.validateOpenIDScope(scopes); svcErr != nil {
		return nil, svcErr
	}

	oauthApp := s.getOAuthApp(ctx, tokenClaims)

	// Extract allowed user attributes
	var allowedUserAttributes []string
	if oauthApp != nil && oauthApp.UserInfo != nil {
		allowedUserAttributes = oauthApp.UserInfo.UserAttributes
	}

	attributeCacheID := ""
	if val, ok := tokenClaims["aci"].(string); ok {
		attributeCacheID = val
	}

	// Fetch user attributes with groups and default claims
	userAttributes, err := tokenservice.FetchUserAttributes(ctx, s.attributeCacheSvc,
		allowedUserAttributes, attributeCacheID)
	if err != nil {
		s.logger.Error("Failed to fetch user attributes", log.MaskedString(log.LoggerKeyUserID, sub), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	response, svcErr := s.buildUserInfoResponse(sub, scopes, userAttributes, oauthApp, tokenClaims)
	if svcErr != nil {
		return nil, svcErr
	}

	var userInfoCfg *inboundmodel.UserInfoConfig
	var certificate *inboundmodel.Certificate
	if oauthApp != nil {
		userInfoCfg = oauthApp.UserInfo
		certificate = oauthApp.Certificate
	}

	responseType := inboundmodel.UserInfoResponseTypeJSON
	if userInfoCfg != nil {
		responseType = userInfoCfg.ResponseType
	}
	switch responseType {
	case inboundmodel.UserInfoResponseTypeNESTEDJWT:
		return s.generateNestedJWTUserInfo(ctx, sub, tokenClaims, response, userInfoCfg, certificate)
	case inboundmodel.UserInfoResponseTypeJWE:
		return s.generateJWEUserInfo(ctx, response, userInfoCfg, certificate)
	case inboundmodel.UserInfoResponseTypeJWS:
		return s.generateJWSUserInfo(ctx, sub, tokenClaims, response, userInfoCfg)
	default:
		return &UserInfoResponse{Type: inboundmodel.UserInfoResponseTypeJSON, JSONBody: response}, nil
	}
}

// generateJWEUserInfo creates an encrypted JWE UserInfo response.
func (s *userInfoService) generateJWEUserInfo(
	ctx context.Context,
	response map[string]interface{},
	cfg *inboundmodel.UserInfoConfig,
	certificate *inboundmodel.Certificate,
) (*UserInfoResponse, *serviceerror.ServiceError) {
	rpKey, rpKID, svcErr := s.jwksResolver.ResolveEncryptionKey(
		ctx, certificate, cfg.EncryptionAlg, jwksresolver.KeyUseStrictEnc)
	if svcErr != nil {
		return nil, svcErr
	}

	payload, err := json.Marshal(response)
	if err != nil {
		s.logger.Error("Failed to marshal userinfo claims for JWE")
		return nil, &serviceerror.InternalServerError
	}

	compact, svcErr := s.jweService.Encrypt(
		payload, rpKey,
		jwe.KeyEncAlgorithm(cfg.EncryptionAlg),
		jwe.ContentEncAlgorithm(cfg.EncryptionEnc),
		"json",
		rpKID,
	)
	if svcErr != nil {
		s.logger.Error("Failed to encrypt userinfo JWE")
		return nil, svcErr
	}

	return &UserInfoResponse{Type: inboundmodel.UserInfoResponseTypeJWE, JWTBody: compact}, nil
}

// generateNestedJWTUserInfo creates a sign-then-encrypt Nested JWT UserInfo response.
func (s *userInfoService) generateNestedJWTUserInfo(
	ctx context.Context,
	sub string,
	tokenClaims map[string]interface{},
	response map[string]interface{},
	cfg *inboundmodel.UserInfoConfig,
	certificate *inboundmodel.Certificate,
) (*UserInfoResponse, *serviceerror.ServiceError) {
	jwsResp, svcErr := s.generateJWSUserInfo(ctx, sub, tokenClaims, response, cfg)
	if svcErr != nil {
		return nil, svcErr
	}

	rpKey, rpKID, svcErr := s.jwksResolver.ResolveEncryptionKey(
		ctx, certificate, cfg.EncryptionAlg, jwksresolver.KeyUseStrictEnc)
	if svcErr != nil {
		return nil, svcErr
	}

	compact, svcErr := s.jweService.Encrypt(
		[]byte(jwsResp.JWTBody), rpKey,
		jwe.KeyEncAlgorithm(cfg.EncryptionAlg),
		jwe.ContentEncAlgorithm(cfg.EncryptionEnc),
		"JWT",
		rpKID,
	)
	if svcErr != nil {
		s.logger.Error("Failed to encrypt nested JWT userinfo JWE")
		return nil, svcErr
	}

	return &UserInfoResponse{Type: inboundmodel.UserInfoResponseTypeNESTEDJWT, JWTBody: compact}, nil
}

// generateJWSUserInfo creates a signed JWT UserInfo response
// based on the application configuration.
func (s *userInfoService) generateJWSUserInfo(
	ctx context.Context,
	sub string,
	tokenClaims map[string]interface{},
	response map[string]interface{},
	cfg *inboundmodel.UserInfoConfig,
) (*UserInfoResponse, *serviceerror.ServiceError) {
	clientID := ""
	if cid, ok := tokenClaims["client_id"].(string); ok {
		clientID = cid
	}

	runtime := config.GetServerRuntime()

	issuer := runtime.Config.JWT.Issuer
	validity := runtime.Config.JWT.ValidityPeriod

	response["aud"] = clientID
	signingAlg := ""
	if cfg != nil {
		signingAlg = cfg.SigningAlg
	}

	signedJWT, _, err := s.jwtService.GenerateJWT(
		ctx,
		sub,
		issuer,
		validity,
		response,
		jwt.TokenTypeJWT,
		signingAlg,
	)
	if err != nil {
		if err.Code == jwt.ErrorUnsupportedJWSAlgorithm.Code {
			s.logger.Error("UserInfo signing algorithm is not supported by the server key",
				log.String("alg", signingAlg), log.String("error", err.Error.DefaultValue))
		} else {
			s.logger.Error("Failed to generate signed UserInfo JWT",
				log.String("error", err.Error.DefaultValue))
		}
		return nil, &serviceerror.InternalServerError
	}

	return &UserInfoResponse{
		Type:    inboundmodel.UserInfoResponseTypeJWS,
		JWTBody: signedJWT,
	}, nil
}

// validateGrantType validates that the token was not issued using client_credentials grant.
func (s *userInfoService) validateGrantType(claims map[string]interface{}) *serviceerror.ServiceError {
	grantTypeValue, ok := claims["grant_type"]
	if !ok {
		return nil
	}

	grantTypeString, ok := grantTypeValue.(string)
	if !ok {
		return nil
	}

	if constants.GrantType(grantTypeString) == constants.GrantTypeClientCredentials {
		s.logger.Debug("UserInfo endpoint called with client_credentials grant token",
			log.String("grant_type", grantTypeString))
		return &errorClientCredentialsNotSupported
	}

	return nil
}

// extractScopes extracts scopes from the token claims.
func (s *userInfoService) extractScopes(claims map[string]interface{}) []string {
	scopeValue, ok := claims["scope"]
	if !ok {
		return nil
	}

	scopeString, ok := scopeValue.(string)
	if !ok {
		return nil
	}

	return tokenservice.ParseScopes(scopeString)
}

// validateOpenIDScope validates that the access token contains the required 'openid' scope.
func (s *userInfoService) validateOpenIDScope(scopes []string) *serviceerror.ServiceError {
	if !slices.Contains(scopes, constants.ScopeOpenID) {
		s.logger.Debug("UserInfo request missing required 'openid' scope",
			log.String("scopes", tokenservice.JoinScopes(scopes)))
		return &errorInsufficientScope
	}
	return nil
}

// getOAuthApp retrieves the OAuth client configuration if client_id is present in claims.
// Returns nil when no client_id is present, on error, or when the app is not found.
func (s *userInfoService) getOAuthApp(
	ctx context.Context, claims map[string]interface{},
) *inboundmodel.OAuthClient {
	clientID, ok := claims["client_id"].(string)
	if !ok || clientID == "" {
		return nil
	}

	app, err := s.inboundClient.GetOAuthClientByClientID(ctx, clientID)
	if err != nil || app == nil {
		return nil
	}

	return app
}

// buildUserInfoResponse builds the final UserInfo response from sub, scopes, and user attributes.
// It also processes any explicit claims request embedded in the access token.
func (s *userInfoService) buildUserInfoResponse(
	sub string,
	scopes []string,
	userAttributes map[string]interface{},
	oauthApp *inboundmodel.OAuthClient,
	tokenClaims map[string]interface{},
) (map[string]interface{}, *serviceerror.ServiceError) {
	response := map[string]interface{}{
		"sub": sub,
	}

	// Build claims from scopes and explicit claims request
	// Extract only the UserInfo claims map from the access token
	claimsRequest, svcErr := s.extractClaimsRequest(tokenClaims)
	if svcErr != nil {
		return nil, svcErr
	}
	var userInfoClaims map[string]*model.IndividualClaimRequest
	if claimsRequest != nil {
		userInfoClaims = claimsRequest.UserInfo
	}

	// Get scope claims mapping and allowed user attributes from app config
	var scopeClaimsMapping map[string][]string
	var allowedUserAttributes []string
	if oauthApp != nil {
		scopeClaimsMapping = oauthApp.ScopeClaims
		if oauthApp.UserInfo != nil && len(oauthApp.UserInfo.UserAttributes) > 0 {
			allowedUserAttributes = oauthApp.UserInfo.UserAttributes
		}
	}

	claimData := tokenservice.BuildClaims(
		scopes,
		userInfoClaims,
		userAttributes,
		scopeClaimsMapping,
		allowedUserAttributes,
	)
	for key, value := range claimData {
		response[key] = value
	}

	return response, nil
}

// extractClaimsRequest extracts the claims request from the access token if present.
func (s *userInfoService) extractClaimsRequest(
	tokenClaims map[string]interface{},
) (*model.ClaimsRequest, *serviceerror.ServiceError) {
	claimsRequestStr, ok := tokenClaims[constants.ClaimClaimsRequest].(string)
	if !ok || claimsRequestStr == "" {
		return nil, nil
	}

	claimsRequest, err := oauth2utils.ParseClaimsRequest(claimsRequestStr)
	if err != nil {
		s.logger.Error("Failed to parse claims request from access token", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return claimsRequest, nil
}
