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

package tokenservice

import (
	"context"
	"fmt"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// TokenBuilderInterface defines the interface for building OAuth2 tokens.
type TokenBuilderInterface interface {
	BuildAccessToken(ctx context.Context, tokenCtx *AccessTokenBuildContext) (*oauth2model.TokenDTO, error)
	BuildRefreshToken(ctx context.Context, tokenCtx *RefreshTokenBuildContext) (*oauth2model.TokenDTO, error)
	BuildIDToken(ctx context.Context, tokenCtx *IDTokenBuildContext) (*oauth2model.TokenDTO, error)
	BuildIDJAG(ctx context.Context, tokenCtx *IDJAGBuildContext) (*oauth2model.TokenDTO, error)
}

// TokenBuilder implements TokenBuilderInterface.
type tokenBuilder struct {
	cfg          oauthconfig.Config
	jwtService   jwt.JWTServiceInterface
	jweService   jwe.JWEServiceInterface
	jwksResolver *jwksresolver.Resolver
}

// newTokenBuilder creates a new TokenBuilder instance.
func newTokenBuilder(
	cfg oauthconfig.Config,
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	resolver *jwksresolver.Resolver,
) TokenBuilderInterface {
	return &tokenBuilder{
		cfg:          cfg,
		jwtService:   jwtService,
		jweService:   jweService,
		jwksResolver: resolver,
	}
}

// BuildAccessToken builds an access token with all necessary claims.
func (tb *tokenBuilder) BuildAccessToken(
	ctx context.Context,
	tokenCtx *AccessTokenBuildContext,
) (*oauth2model.TokenDTO, error) {
	if tokenCtx == nil {
		return nil, fmt.Errorf("build context cannot be nil")
	}

	tokenConfig := ResolveTokenConfig(tb.cfg, tokenCtx.OAuthApp, TokenTypeAccess, tokenCtx.ValidityPeriod)

	jwtClaims, claimsErr := tb.buildAccessTokenClaims(tokenCtx)
	if claimsErr != nil {
		return nil, fmt.Errorf("failed to build access token claims: %w", claimsErr)
	}

	tokenType := constants.TokenTypeBearer
	if tokenCtx.DPoPJkt != "" {
		tokenType = constants.TokenTypeDPoP
	}

	tokenDTO := &oauth2model.TokenDTO{
		TokenType:        tokenType,
		ExpiresIn:        tokenConfig.ValidityPeriod,
		Scopes:           tokenCtx.Scopes,
		ClientID:         tokenCtx.ClientID,
		UserAttributes:   tokenCtx.SubjectAttributes,
		AttributeCacheID: tokenCtx.AttributeCacheID,
		Subject:          tokenCtx.Subject,
		Audiences:        tokenCtx.Audiences,
		ClaimsRequest:    tokenCtx.ClaimsRequest,
		ClaimsLocales:    tokenCtx.ClaimsLocales,
		TokenFamilyID:    tokenCtx.TokenFamilyID,
	}

	token, iat, err := tb.jwtService.GenerateJWT(
		ctx,
		tokenCtx.Subject,
		tokenConfig.Issuer,
		tokenConfig.ValidityPeriod,
		jwtClaims,
		jwt.TokenTypeAccessToken,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %v", err.Error)
	}

	// Assign generated token and issued at time
	tokenDTO.Token = token
	tokenDTO.IssuedAt = iat

	return tokenDTO, nil
}

// BuildIDJAG builds an Identity Assertion Authorization Grant (ID-JAG) JWT targeted at an external
// resource authorization server (draft-ietf-oauth-identity-assertion-authz-grant). The token carries
// typ=oauth-id-jag+jwt and is signed with the server's own key. token_type is "N_A" because the
// issued token is not an access token.
func (tb *tokenBuilder) BuildIDJAG(
	ctx context.Context,
	tokenCtx *IDJAGBuildContext,
) (*oauth2model.TokenDTO, error) {
	if tokenCtx == nil {
		return nil, fmt.Errorf("build context cannot be nil")
	}

	validityPeriod := providers.DefaultIDJAGValidityPeriod
	if tokenCtx.OAuthApp != nil && tokenCtx.OAuthApp.Token != nil && tokenCtx.OAuthApp.Token.IDJAG != nil &&
		tokenCtx.OAuthApp.Token.IDJAG.ValidityPeriod > 0 {
		validityPeriod = tokenCtx.OAuthApp.Token.IDJAG.ValidityPeriod
	}

	claims := map[string]interface{}{
		"aud":       tokenCtx.Audience,
		"client_id": tokenCtx.ClientID,
	}
	if len(tokenCtx.Scopes) > 0 {
		claims["scope"] = JoinScopes(tokenCtx.Scopes)
	}
	// RFC 8707: a single resource is embedded as a string, multiple resources as an array.
	if len(tokenCtx.Resources) == 1 {
		claims["resource"] = tokenCtx.Resources[0]
	} else if len(tokenCtx.Resources) > 1 {
		claims["resource"] = tokenCtx.Resources
	}

	token, iat, err := tb.jwtService.GenerateJWT(
		ctx,
		tokenCtx.Subject,
		tb.cfg.JWT.Issuer,
		validityPeriod,
		claims,
		jwt.TokenTypeIDJAG,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID-JAG: %v", err.Error)
	}

	return &oauth2model.TokenDTO{
		Token:     token,
		TokenType: constants.TokenTypeNA,
		IssuedAt:  iat,
		ExpiresIn: validityPeriod,
		Scopes:    tokenCtx.Scopes,
		ClientID:  tokenCtx.ClientID,
		Subject:   tokenCtx.Subject,
		Audiences: []string{tokenCtx.Audience},
	}, nil
}

// buildAccessTokenClaims builds the claims map for an access token.
func (tb *tokenBuilder) buildAccessTokenClaims(
	ctx *AccessTokenBuildContext,
) (map[string]interface{}, error) {
	claims := make(map[string]interface{})

	if len(ctx.Scopes) > 0 {
		claims["scope"] = JoinScopes(ctx.Scopes)
	}

	if ctx.ClientID != "" {
		claims["client_id"] = ctx.ClientID
	}

	if ctx.GrantType != "" {
		claims["grant_type"] = ctx.GrantType
	}

	// Merge the subject's attributes (already resolved and filtered by the grant handler).
	for key, value := range ctx.SubjectAttributes {
		claims[key] = value
	}

	// Set after merging subject attributes to prevent them from overwriting this system claim.
	if ctx.AttributeCacheID != "" {
		claims["aci"] = ctx.AttributeCacheID
	}

	// Set after merging user attributes so a federated principal's attributes cannot spoof the source
	// IdP. For a jwt-bearer-grant (ID-JAG) token the `sub` is an external IdP identifier and MUST be
	// interpreted together with this claim.
	if ctx.SourceIDP != "" {
		claims[constants.ClaimIDP] = ctx.SourceIDP
	}

	if ctx.ActorClaims != nil {
		actClaim := tb.buildActorClaim(ctx.ActorClaims)
		claims["act"] = actClaim
	}

	// Include only normal userinfo claims for UserInfo endpoint support.
	// verified_claims is never resolved or returned, so it is excluded from the access token.
	if ctx.ClaimsRequest != nil && len(ctx.ClaimsRequest.UserInfo) > 0 {
		userinfoClaims := &oauth2model.ClaimsRequest{UserInfo: ctx.ClaimsRequest.UserInfo}
		serialized, err := oauth2utils.SerializeClaimsRequest(userinfoClaims)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize userinfo claims request: %w", err)
		}
		if serialized != "" {
			claims[constants.ClaimClaimsRequest] = serialized
		}
	}

	// Include claims_locales if present
	if ctx.ClaimsLocales != "" {
		claims[constants.ClaimClaimsLocales] = ctx.ClaimsLocales
	}

	if len(ctx.Audiences) > 1 {
		claims["aud"] = ctx.Audiences
	} else if len(ctx.Audiences) == 1 {
		claims["aud"] = ctx.Audiences[0]
	}

	dpop.SetCnfJkt(claims, ctx.DPoPJkt)

	if ctx.TokenFamilyID != "" {
		claims[constants.ClaimTokenFamilyID] = ctx.TokenFamilyID
	}

	return claims, nil
}

// buildActorClaim builds the actor claim for token exchange.
func (tb *tokenBuilder) buildActorClaim(actorClaims *SubjectTokenClaims) map[string]interface{} {
	actClaim := map[string]interface{}{
		"sub": actorClaims.Sub,
	}

	if actorClaims.Iss != "" {
		actClaim["iss"] = actorClaims.Iss
	}

	if len(actorClaims.NestedAct) > 0 {
		actClaim["act"] = actorClaims.NestedAct
	}

	return actClaim
}

// BuildRefreshToken builds a refresh token with all necessary claims.
func (tb *tokenBuilder) BuildRefreshToken(
	ctx context.Context,
	tokenCtx *RefreshTokenBuildContext,
) (*oauth2model.TokenDTO, error) {
	if tokenCtx == nil {
		return nil, fmt.Errorf("build context cannot be nil")
	}

	tokenConfig := ResolveTokenConfig(tb.cfg, tokenCtx.OAuthApp, TokenTypeRefresh, 0)

	claims, claimsErr := tb.buildRefreshTokenClaims(tokenCtx)
	if claimsErr != nil {
		return nil, fmt.Errorf("failed to build refresh token claims: %w", claimsErr)
	}

	tokenDTO := &oauth2model.TokenDTO{
		ExpiresIn:     tokenConfig.ValidityPeriod,
		Scopes:        tokenCtx.Scopes,
		ClientID:      tokenCtx.ClientID,
		Subject:       tokenCtx.AccessTokenSubject,
		Audiences:     []string{tokenConfig.Issuer},
		ClaimsLocales: tokenCtx.ClaimsLocales,
	}

	claims["aud"] = tokenConfig.Issuer

	token, iat, err := tb.jwtService.GenerateJWT(
		ctx,
		tokenCtx.ClientID,
		tokenConfig.Issuer,
		tokenConfig.ValidityPeriod,
		claims,
		jwt.TokenTypeJWT,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %v", err.Error)
	}

	// Assign generated token and issued at time
	tokenDTO.Token = token
	tokenDTO.IssuedAt = iat

	return tokenDTO, nil
}

// buildRefreshTokenClaims builds the claims map for a refresh token.
func (tb *tokenBuilder) buildRefreshTokenClaims(ctx *RefreshTokenBuildContext) (map[string]interface{}, error) {
	claims := make(map[string]interface{})

	if len(ctx.Scopes) > 0 {
		claims["scope"] = JoinScopes(ctx.Scopes)
	}

	claims["access_token_sub"] = ctx.AccessTokenSubject
	claims["access_token_aud"] = ctx.AccessTokenAudiences
	claims["grant_type"] = ctx.GrantType

	if ctx.ActorSub != "" {
		claims["act_sub"] = ctx.ActorSub
	}

	if ctx.AttributeCacheID != "" {
		claims["aci"] = ctx.AttributeCacheID
	}

	// Include claims request if present
	if ctx.ClaimsRequest != nil && !ctx.ClaimsRequest.IsEmpty() {
		serialized, err := oauth2utils.SerializeClaimsRequest(ctx.ClaimsRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize claims request: %w", err)
		}
		if serialized != "" {
			claims["access_token_claims_request"] = serialized
		}
	}

	// Include claims_locales if present
	if ctx.ClaimsLocales != "" {
		claims["access_token_claims_locales"] = ctx.ClaimsLocales
	}

	if ctx.DPoPJkt != "" {
		claims[constants.ClaimDPoPJkt] = ctx.DPoPJkt
	}

	if ctx.TokenFamilyID != "" {
		claims[constants.ClaimTokenFamilyID] = ctx.TokenFamilyID
	}

	return claims, nil
}

// BuildIDToken builds an OIDC ID token with all necessary claims.
func (tb *tokenBuilder) BuildIDToken(
	ctx context.Context,
	tokenCtx *IDTokenBuildContext,
) (*oauth2model.TokenDTO, error) {
	if tokenCtx == nil {
		return nil, fmt.Errorf("build context cannot be nil")
	}

	tokenConfig := ResolveTokenConfig(tb.cfg, tokenCtx.OAuthApp, TokenTypeID, 0)

	jwtClaims := tb.buildIDTokenClaims(tokenCtx)

	tokenDTO := &oauth2model.TokenDTO{
		ExpiresIn: tokenConfig.ValidityPeriod,
		Scopes:    tokenCtx.Scopes,
		ClientID:  tokenCtx.Audience,
		Subject:   tokenCtx.Subject,
		Audiences: []string{tokenCtx.Audience},
	}

	jwtClaims["aud"] = tokenCtx.Audience

	token, iat, err := tb.jwtService.GenerateJWT(
		ctx,
		tokenCtx.Subject,
		tokenConfig.Issuer,
		tokenConfig.ValidityPeriod,
		jwtClaims,
		jwt.TokenTypeJWT,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID token: %v", err.Error)
	}

	// Optionally encrypt the signed ID token when responseType is JWE or NESTED_JWT.
	if tokenCtx.OAuthApp != nil && tokenCtx.OAuthApp.Token != nil && tokenCtx.OAuthApp.Token.IDToken != nil {
		idTokenCfg := tokenCtx.OAuthApp.Token.IDToken
		rt := idTokenCfg.ResponseType
		if rt == providers.IDTokenResponseTypeJWE || rt == providers.IDTokenResponseTypeNESTEDJWT {
			if tb.jweService == nil {
				return nil, fmt.Errorf("JWE service is not configured")
			}
			rpKey, rpKID, svcErr := tb.jwksResolver.ResolveEncryptionKey(
				ctx,
				tokenCtx.OAuthApp.Certificate,
				idTokenCfg.EncryptionAlg,
				jwksresolver.KeyUseLenientEnc,
			)
			if svcErr != nil {
				return nil, fmt.Errorf("failed to resolve ID token encryption key: %v", svcErr)
			}
			// cty="JWT" indicates a nested JWT (signed JWS payload encrypted as JWE per OIDC spec)
			encrypted, svcErr := tb.jweService.Encrypt(ctx,
				[]byte(token), rpKey,
				jwe.KeyEncAlgorithm(idTokenCfg.EncryptionAlg),
				jwe.ContentEncAlgorithm(idTokenCfg.EncryptionEnc),
				"JWT", rpKID,
			)
			if svcErr != nil {
				return nil, fmt.Errorf("failed to encrypt ID token: %v", svcErr)
			}
			token = encrypted
		}
	}

	// Assign generated token and issued at time
	tokenDTO.Token = token
	tokenDTO.IssuedAt = iat

	return tokenDTO, nil
}

// buildIDTokenClaims builds the claims map for an ID token (OIDC).
func (tb *tokenBuilder) buildIDTokenClaims(ctx *IDTokenBuildContext) map[string]interface{} {
	claims := make(map[string]interface{})

	if ctx.AuthTime > 0 {
		claims["auth_time"] = ctx.AuthTime
	}

	if ctx.Nonce != "" {
		claims[constants.RequestParamNonce] = ctx.Nonce
	}

	if ctx.CompletedACR != "" {
		claims["acr"] = ctx.CompletedACR
	}

	userAttributes := ctx.UserAttributes
	if userAttributes == nil {
		userAttributes = make(map[string]interface{})
	}

	// Get scope claims mapping and allowed user attributes from app config
	var scopeClaimsMapping map[string][]string
	var allowedUserAttributes []string
	if ctx.OAuthApp != nil {
		scopeClaimsMapping = ctx.OAuthApp.ScopeClaims
		if ctx.OAuthApp.Token != nil && ctx.OAuthApp.Token.IDToken != nil {
			allowedUserAttributes = ctx.OAuthApp.Token.IDToken.UserAttributes
		}
	}

	// Build claims from scopes and explicit claims parameter
	var idTokenClaims map[string]*oauth2model.IndividualClaimRequest
	if ctx.ClaimsRequest != nil {
		idTokenClaims = ctx.ClaimsRequest.IDToken
	}
	claimData := BuildClaims(
		ctx.Scopes,
		idTokenClaims,
		userAttributes,
		scopeClaimsMapping,
		allowedUserAttributes,
	)

	for key, value := range claimData {
		claims[key] = value
	}

	return claims
}
