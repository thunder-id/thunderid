// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package tokenservice

import (
	"context"
	"fmt"
	"time"

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
	// actorProvider resolves the token subject's entity so its category can be recorded on the token
	// DTO. Optional: a nil provider leaves the category empty rather than failing issuance.
	actorProvider providers.ActorProvider
}

// newTokenBuilder creates a new TokenBuilder instance.
func newTokenBuilder(
	cfg oauthconfig.Config,
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	resolver *jwksresolver.Resolver,
	actorProvider providers.ActorProvider,
) TokenBuilderInterface {
	return &tokenBuilder{
		cfg:           cfg,
		jwtService:    jwtService,
		jweService:    jweService,
		jwksResolver:  resolver,
		actorProvider: actorProvider,
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
	if tokenCtx.ActorClaims != nil {
		tokenDTO.ActorSub = tokenCtx.ActorClaims.Sub
	}
	tokenDTO.SubjectID, tokenDTO.SubjectCategory = tb.resolveSubjectIdentity(tokenCtx)

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

// resolveSubjectIdentity returns the resource ID and entity category of the token's subject, for
// observability rather than for any token claim. Both are empty when the subject cannot be resolved
// to an entity, so consumers omit the fields rather than being told something the server does not
// know: an agent can be a token subject as much as a token requester, and guessing "user" would
// misreport agent-for-agent delegation.
//
// Every grant reaches this function, so none can omit the fields by forgetting to set them, but a
// grant whose upstream layer already knows the answer supplies it and skips the lookup entirely.
func (tb *tokenBuilder) resolveSubjectIdentity(tokenCtx *AccessTokenBuildContext) (id, category string) {
	// authorization_code: carried on the flow assertion, which resolved the entity during login.
	if tokenCtx.SubjectEntityID != "" {
		if tokenCtx.SubjectCategory != "" {
			return tokenCtx.SubjectEntityID, tokenCtx.SubjectCategory
		}
		return tokenCtx.SubjectEntityID, tb.lookupCategory(tokenCtx.SubjectEntityID)
	}

	if tokenCtx.Subject == "" {
		return "", ""
	}

	// client_credentials: the subject is the authenticated client, already loaded.
	if tokenCtx.OAuthApp != nil && tokenCtx.Subject == tokenCtx.OAuthApp.ID {
		return tokenCtx.OAuthApp.ID, string(tokenCtx.OAuthApp.EntityCategory)
	}

	// Exchange grants: the subject arrives on a presented token, whose sub is a resource ID on most
	// deployments but the mapped subject attribute where the issuing application configures one.
	// Resolving it decides which: a mapped attribute matches no entity, so an unresolvable subject is
	// reported as no subject rather than published as-is. This is what keeps a mapped attribute, which
	// may be an email address, off the events.
	category = tb.lookupCategory(tokenCtx.Subject)
	if category == "" {
		return "", ""
	}
	return tokenCtx.Subject, category
}

// lookupCategory returns the entity category of the given resource ID, or an empty string when the
// provider is absent or the ID resolves to no entity. Reads are served by the cache-backed entity
// store, so this is normally an in-memory hit.
func (tb *tokenBuilder) lookupCategory(entityID string) string {
	if tb.actorProvider == nil {
		return ""
	}
	entity, svcErr := tb.actorProvider.GetActor(entityID)
	if svcErr != nil || entity == nil {
		return ""
	}
	return string(entity.Category)
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

	// A rotated token inherits the expiry of the token it replaces, so refreshing extends access
	// but never the grant's lifetime. First issuance carries no expiry and starts a fresh period.
	validityPeriod := tokenConfig.ValidityPeriod
	if tokenCtx.ExpiresAt > 0 {
		validityPeriod = tokenCtx.ExpiresAt - time.Now().Unix()
		if validityPeriod <= 0 {
			return nil, fmt.Errorf("refresh token grant has reached its expiry")
		}
	}

	claims, claimsErr := tb.buildRefreshTokenClaims(tokenCtx)
	if claimsErr != nil {
		return nil, fmt.Errorf("failed to build refresh token claims: %w", claimsErr)
	}

	tokenDTO := &oauth2model.TokenDTO{
		ExpiresIn:     validityPeriod,
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
		validityPeriod,
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
				[]byte(token), &providers.KeyRef{PublicKeyJWK: rpKey},
				idTokenCfg.EncryptionAlg,
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
