// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package userinfo provides functionality for the OIDC UserInfo endpoint.
package userinfo

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/attributecache"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const serviceLoggerComponentName = "UserInfoService"

// userInfoServiceInterface defines the interface for OIDC UserInfo endpoint.
type userInfoServiceInterface interface {
	GetUserInfo(ctx context.Context, accessToken string) (*UserInfoResponse, *tidcommon.ServiceError)
	GetUserInfoForDPoP(ctx context.Context, accessToken, proof, htm, htu string) (
		*UserInfoResponse, *tidcommon.ServiceError)
}

// userInfoService implements the userInfoServiceInterface.
type userInfoService struct {
	cfg               oauthconfig.Config
	jwtService        jwt.JWTServiceInterface
	jweService        jwe.JWEServiceInterface
	jwksResolver      *jwksresolver.Resolver
	tokenValidator    tokenservice.TokenValidatorInterface
	inboundClient     providers.ActorProvider
	attributeCacheSvc attributecache.AttributeCacheServiceInterface
	dpopVerifier      dpop.VerifierInterface
	logger            *log.Logger
}

// newUserInfoService creates a new userInfoService instance.
func newUserInfoService(
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	resolver *jwksresolver.Resolver,
	tokenValidator tokenservice.TokenValidatorInterface,
	actorProvider providers.ActorProvider,
	attributeCacheSvc attributecache.AttributeCacheServiceInterface,
	dpopVerifier dpop.VerifierInterface,
	cfg oauthconfig.Config,
) userInfoServiceInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, serviceLoggerComponentName))
	return &userInfoService{
		cfg:               cfg,
		jwtService:        jwtService,
		jweService:        jweService,
		jwksResolver:      resolver,
		tokenValidator:    tokenValidator,
		inboundClient:     actorProvider,
		attributeCacheSvc: attributeCacheSvc,
		dpopVerifier:      dpopVerifier,
		logger:            logger,
	}
}

// GetUserInfo validates the access token under the Bearer scheme. A DPoP-bound
// access token presented under Bearer is rejected as a downgrade.
func (s *userInfoService) GetUserInfo(
	ctx context.Context, accessToken string,
) (*UserInfoResponse, *tidcommon.ServiceError) {
	if accessToken == "" {
		return nil, &errorInvalidAccessToken
	}

	accessTokenClaims, err := s.tokenValidator.ValidateAccessToken(ctx, accessToken)
	if err != nil {
		s.logger.Debug(ctx, "Failed to verify access token", log.Error(err))
		if errors.Is(err, revocation.ErrEnforcementUnavailable) {
			return nil, &errorRevocationUnavailable
		}
		return nil, &errorInvalidAccessToken
	}

	boundJkt, _ := dpop.ExtractCnfJkt(accessTokenClaims.Claims)
	if boundJkt != "" {
		s.logger.Debug(ctx, "DPoP-bound access token presented under Bearer scheme")
		return nil, &errorBearerDowngrade
	}

	return s.buildResponseFromClaims(ctx, accessTokenClaims)
}

// GetUserInfoForDPoP validates the access token under the DPoP scheme. The access
// token must be DPoP-bound and the proof must bind to the same key, htm, htu, and
// access token (via ath).
func (s *userInfoService) GetUserInfoForDPoP(
	ctx context.Context, accessToken, proof, htm, htu string,
) (*UserInfoResponse, *tidcommon.ServiceError) {
	if accessToken == "" {
		return nil, &errorInvalidAccessToken
	}

	accessTokenClaims, err := s.tokenValidator.ValidateAccessToken(ctx, accessToken)
	if err != nil {
		s.logger.Debug(ctx, "Failed to verify access token", log.Error(err))
		if errors.Is(err, revocation.ErrEnforcementUnavailable) {
			return nil, &errorRevocationUnavailable
		}
		return nil, &errorInvalidAccessToken
	}

	expectedJkt, _ := dpop.ExtractCnfJkt(accessTokenClaims.Claims)
	if expectedJkt == "" {
		s.logger.Debug(ctx, "DPoP scheme used with non-bound access token")
		return nil, &errorDPoPProofInvalid
	}

	if s.dpopVerifier == nil {
		s.logger.Error(ctx, "DPoP verifier not configured")
		return nil, &tidcommon.InternalServerError
	}
	if _, dpopErr := s.dpopVerifier.Verify(ctx, dpop.VerifyParams{
		Proof:       proof,
		HTM:         htm,
		HTU:         htu,
		AccessToken: accessToken,
		ExpectedJkt: expectedJkt,
	}); dpopErr != nil {
		s.logger.Debug(ctx, "DPoP proof verification failed", log.Error(dpopErr))
		return nil, &errorDPoPProofInvalid
	}

	return s.buildResponseFromClaims(ctx, accessTokenClaims)
}

// buildResponseFromClaims builds the UserInfo response from validated access token claims.
func (s *userInfoService) buildResponseFromClaims(
	ctx context.Context, accessTokenClaims *tokenservice.AccessTokenClaims,
) (*UserInfoResponse, *tidcommon.ServiceError) {
	tokenClaims := accessTokenClaims.Claims
	sub := accessTokenClaims.Sub

	if svcErr := s.validateGrantType(ctx, tokenClaims); svcErr != nil {
		return nil, svcErr
	}

	scopes := s.extractScopes(tokenClaims)

	// Validate that the 'openid' scope is present
	if svcErr := s.validateOpenIDScope(ctx, scopes); svcErr != nil {
		return nil, svcErr
	}

	oauthApp := s.getOAuthApp(ctx, tokenClaims)

	if svcErr := s.validateAudience(ctx, accessTokenClaims, oauthApp); svcErr != nil {
		return nil, svcErr
	}

	// Extract allowed user attributes
	var allowedUserAttributes []string
	if oauthApp != nil && oauthApp.UserInfo != nil {
		allowedUserAttributes = oauthApp.UserInfo.UserAttributes
	}

	attributeCacheID := ""
	if val, ok := tokenClaims["aci"].(string); ok {
		attributeCacheID = val
	}

	userAttributes, err := tokenservice.FetchUserAttributes(ctx, s.attributeCacheSvc,
		allowedUserAttributes, attributeCacheID)
	if err != nil {
		s.logger.Error(ctx, "Failed to fetch user attributes",
			log.MaskedString(log.LoggerKeyUserID, sub), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	var userInfoCfg *providers.UserInfoConfig
	var certificate *providers.Certificate
	if oauthApp != nil {
		userInfoCfg = oauthApp.UserInfo
		certificate = oauthApp.Certificate
	}

	// The authn provider returned an opaque JWT/JWE instead of individual claims: pass it
	// through as-is rather than running it through the claims/scope/allow-list pipeline.
	if rawToken, ok := userAttributes[providers.RawJWTAttributeKey].(string); ok && rawToken != "" {
		return s.buildRawJWTResponse(ctx, rawToken, userInfoCfg, certificate)
	}

	response, svcErr := s.buildUserInfoResponse(ctx, sub, scopes, userAttributes, oauthApp, tokenClaims)
	if svcErr != nil {
		return nil, svcErr
	}

	responseType := providers.UserInfoResponseTypeJSON
	if userInfoCfg != nil {
		responseType = userInfoCfg.ResponseType
	}
	switch responseType {
	case providers.UserInfoResponseTypeNESTEDJWT:
		return s.generateNestedJWTUserInfo(ctx, sub, tokenClaims, response, userInfoCfg, certificate)
	case providers.UserInfoResponseTypeJWE:
		return s.generateJWEUserInfo(ctx, response, userInfoCfg, certificate)
	case providers.UserInfoResponseTypeJWS:
		return s.generateJWSUserInfo(ctx, sub, tokenClaims, response, userInfoCfg)
	default:
		return &UserInfoResponse{Type: providers.UserInfoResponseTypeJSON, JSONBody: response}, nil
	}
}

// generateJWEUserInfo creates an encrypted JWE UserInfo response.
func (s *userInfoService) generateJWEUserInfo(
	ctx context.Context,
	response map[string]interface{},
	cfg *providers.UserInfoConfig,
	certificate *providers.Certificate,
) (*UserInfoResponse, *tidcommon.ServiceError) {
	rpKey, rpKID, svcErr := s.jwksResolver.ResolveEncryptionKey(
		ctx, certificate, cfg.EncryptionAlg, jwksresolver.KeyUseStrictEnc)
	if svcErr != nil {
		return nil, svcErr
	}

	payload, err := json.Marshal(response)
	if err != nil {
		s.logger.Error(ctx, "Failed to marshal userinfo claims for JWE")
		return nil, &tidcommon.InternalServerError
	}

	compact, svcErr := s.jweService.Encrypt(ctx,
		payload,
		&providers.KeyRef{PublicKeyJWK: rpKey},
		cfg.EncryptionAlg,
		jwe.ContentEncAlgorithm(cfg.EncryptionEnc),
		"json",
		rpKID,
	)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to encrypt userinfo JWE")
		return nil, svcErr
	}

	return &UserInfoResponse{Type: providers.UserInfoResponseTypeJWE, JWTBody: compact}, nil
}

// generateNestedJWTUserInfo creates a sign-then-encrypt Nested JWT UserInfo response.
func (s *userInfoService) generateNestedJWTUserInfo(
	ctx context.Context,
	sub string,
	tokenClaims map[string]interface{},
	response map[string]interface{},
	cfg *providers.UserInfoConfig,
	certificate *providers.Certificate,
) (*UserInfoResponse, *tidcommon.ServiceError) {
	jwsResp, svcErr := s.generateJWSUserInfo(ctx, sub, tokenClaims, response, cfg)
	if svcErr != nil {
		return nil, svcErr
	}

	compact, svcErr := s.encryptSignedJWT(ctx, jwsResp.JWTBody, cfg, certificate)
	if svcErr != nil {
		return nil, svcErr
	}

	return &UserInfoResponse{Type: providers.UserInfoResponseTypeNESTEDJWT, JWTBody: compact}, nil
}

// encryptSignedJWT encrypts an already-signed JWT into a nested-JWT (JWE with cty=JWT) compact
// serialization, using the client's encryption key.
func (s *userInfoService) encryptSignedJWT(
	ctx context.Context,
	signedJWT string,
	cfg *providers.UserInfoConfig,
	certificate *providers.Certificate,
) (string, *tidcommon.ServiceError) {
	rpKey, rpKID, svcErr := s.jwksResolver.ResolveEncryptionKey(
		ctx, certificate, cfg.EncryptionAlg, jwksresolver.KeyUseStrictEnc)
	if svcErr != nil {
		return "", svcErr
	}

	compact, svcErr := s.jweService.Encrypt(ctx,
		[]byte(signedJWT),
		&providers.KeyRef{PublicKeyJWK: rpKey},
		cfg.EncryptionAlg,
		jwe.ContentEncAlgorithm(cfg.EncryptionEnc),
		"JWT",
		rpKID,
	)
	if svcErr != nil {
		s.logger.Error(ctx, "Failed to encrypt nested JWT userinfo JWE")
		return "", svcErr
	}

	return compact, nil
}

// generateJWSUserInfo creates a signed JWT UserInfo response
// based on the application configuration.
func (s *userInfoService) generateJWSUserInfo(
	ctx context.Context,
	sub string,
	tokenClaims map[string]interface{},
	response map[string]interface{},
	cfg *providers.UserInfoConfig,
) (*UserInfoResponse, *tidcommon.ServiceError) {
	clientID := ""
	if cid, ok := tokenClaims["client_id"].(string); ok {
		clientID = cid
	}

	issuer := s.cfg.JWT.Issuer
	validity := s.cfg.JWT.ValidityPeriod

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
			s.logger.Error(ctx, "UserInfo signing algorithm is not supported by the server key",
				log.String("alg", signingAlg), log.String("error", err.Error.DefaultValue))
		} else {
			s.logger.Error(ctx, "Failed to generate signed UserInfo JWT",
				log.String("error", err.Error.DefaultValue))
		}
		return nil, &tidcommon.InternalServerError
	}

	return &UserInfoResponse{
		Type:    providers.UserInfoResponseTypeJWS,
		JWTBody: signedJWT,
	}, nil
}

// buildRawJWTResponse passes through an opaque JWT/JWE returned by the authn provider in place
// of individual claims. The token is never re-signed or decoded; it is only encrypted when the
// client's UserInfo response type requires it and it isn't encrypted already.
func (s *userInfoService) buildRawJWTResponse(
	ctx context.Context,
	rawToken string,
	cfg *providers.UserInfoConfig,
	certificate *providers.Certificate,
) (*UserInfoResponse, *tidcommon.ServiceError) {
	// JWE compact serialization has 5 dot-separated parts; a signed JWT (JWS) has 3, matching the
	// part-count checks used elsewhere for these formats (e.g. jwe.DecodeJWE).
	if len(strings.Split(rawToken, ".")) == 5 {
		return &UserInfoResponse{Type: providers.UserInfoResponseTypeJWE, JWTBody: rawToken}, nil
	}

	responseType := providers.UserInfoResponseTypeJWS
	if cfg != nil {
		responseType = cfg.ResponseType
	}
	if responseType != providers.UserInfoResponseTypeJWE && responseType != providers.UserInfoResponseTypeNESTEDJWT {
		return &UserInfoResponse{Type: providers.UserInfoResponseTypeJWS, JWTBody: rawToken}, nil
	}

	compact, svcErr := s.encryptSignedJWT(ctx, rawToken, cfg, certificate)
	if svcErr != nil {
		return nil, svcErr
	}

	return &UserInfoResponse{Type: providers.UserInfoResponseTypeNESTEDJWT, JWTBody: compact}, nil
}

// validateGrantType validates that the token was not issued using client_credentials grant.
func (s *userInfoService) validateGrantType(
	ctx context.Context, claims map[string]interface{}) *tidcommon.ServiceError {
	grantTypeValue, ok := claims["grant_type"]
	if !ok {
		return nil
	}

	grantTypeString, ok := grantTypeValue.(string)
	if !ok {
		return nil
	}

	if providers.GrantType(grantTypeString) == providers.GrantTypeClientCredentials {
		s.logger.Debug(ctx, "UserInfo endpoint called with client_credentials grant token",
			log.String("grant_type", grantTypeString))
		return &errorClientCredentialsNotSupported
	}

	return nil
}

// validateAudience validates that the access token's audience is the client's own default
// audience (its configured DefaultAudience, or the server issuer ID when unset). A token whose
// audience was bound to an external resource server via the 'resource' parameter is scoped to
// that resource server and must not be redeemable at the UserInfo endpoint.
func (s *userInfoService) validateAudience(
	ctx context.Context, accessTokenClaims *tokenservice.AccessTokenClaims, oauthApp *providers.OAuthClient,
) *tidcommon.ServiceError {
	expectedAud := oauthApp.ResolveDefaultAudience(s.cfg.JWT.Issuer)
	if !slices.Contains(accessTokenClaims.Aud, expectedAud) {
		s.logger.Debug(ctx, "UserInfo request token audience does not match the client's default audience",
			log.String("client_id", accessTokenClaims.ClientID))
		return &errorAudienceNotAccepted
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
func (s *userInfoService) validateOpenIDScope(ctx context.Context, scopes []string) *tidcommon.ServiceError {
	if !slices.Contains(scopes, constants.ScopeOpenID) {
		s.logger.Debug(ctx, "UserInfo request missing required 'openid' scope",
			log.String("scopes", tokenservice.JoinScopes(scopes)))
		return &errorInsufficientScope
	}
	return nil
}

// getOAuthApp retrieves the OAuth client configuration if client_id is present in claims.
// Returns nil when no client_id is present, on error, or when the app is not found.
func (s *userInfoService) getOAuthApp(
	ctx context.Context, claims map[string]interface{},
) *providers.OAuthClient {
	clientID, ok := claims["client_id"].(string)
	if !ok || clientID == "" {
		return nil
	}

	app, svcErr := s.inboundClient.GetOAuthClientByClientID(ctx, clientID)
	if svcErr != nil || app == nil {
		return nil
	}

	return app
}

// buildUserInfoResponse builds the final UserInfo response from sub, scopes, and user attributes.
// It also processes any explicit claims request embedded in the access token.
func (s *userInfoService) buildUserInfoResponse(ctx context.Context,
	sub string,
	scopes []string,
	userAttributes map[string]interface{},
	oauthApp *providers.OAuthClient,
	tokenClaims map[string]interface{},
) (map[string]interface{}, *tidcommon.ServiceError) {
	response := map[string]interface{}{
		"sub": sub,
	}

	// Build claims from scopes and explicit claims request
	// Extract only the UserInfo claims map from the access token
	claimsRequest, svcErr := s.extractClaimsRequest(ctx, tokenClaims)
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
func (s *userInfoService) extractClaimsRequest(ctx context.Context,
	tokenClaims map[string]interface{},
) (*model.ClaimsRequest, *tidcommon.ServiceError) {
	claimsRequestStr, ok := tokenClaims[constants.ClaimClaimsRequest].(string)
	if !ok || claimsRequestStr == "" {
		return nil, nil
	}

	claimsRequest, err := oauth2utils.ParseClaimsRequest(claimsRequestStr)
	if err != nil {
		s.logger.Error(ctx, "Failed to parse claims request from access token", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return claimsRequest, nil
}
