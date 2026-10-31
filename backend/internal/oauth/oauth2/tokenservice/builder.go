// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package tokenservice

import (
	"context"
	"fmt"
	"slices"
	"time"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// TokenBuilderInterface defines the interface for building OAuth2 tokens.
type TokenBuilderInterface interface {
	BuildAccessToken(ctx context.Context, tokenCtx *AccessTokenBuildContext) (*oauth2model.TokenDTO, error)
	BuildRefreshToken(ctx context.Context, tokenCtx *RefreshTokenBuildContext) (*oauth2model.TokenDTO, error)
	BuildIDToken(ctx context.Context, tokenCtx *IDTokenBuildContext) (*oauth2model.TokenDTO, error)
	BuildIDJAG(ctx context.Context, tokenCtx *IDJAGBuildContext) (*oauth2model.TokenDTO, error)
	BuildLogoutToken(ctx context.Context, tokenCtx *LogoutTokenBuildContext) (*oauth2model.TokenDTO, error)
	BuildDeviceSecret(ctx context.Context, tokenCtx *DeviceSecretBuildContext) (*oauth2model.TokenDTO, error)
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
	// governance decides, for every token built, whether the entities it is issued to or for may be
	// issued tokens now, and how long the token may live. Optional: nil admits every entity and caps
	// nothing.
	governance providers.IdentityGovernanceProvider
}

// issuanceDecisionsKey keys the issuance decisions memoized for one token request.
type issuanceDecisionsKey struct{}

// WithIssuanceDecisions returns ctx with a memo of the issuance decisions taken while serving one token
// request, so an entity that several of its tokens name, such as the client and the subject of an
// access, refresh, and ID token, is checked once. The memo is not safe for concurrent use.
func WithIssuanceDecisions(ctx context.Context) context.Context {
	return context.WithValue(ctx, issuanceDecisionsKey{}, map[string]providers.AccessDecision{})
}

// newTokenBuilder creates a new TokenBuilder instance.
func newTokenBuilder(
	cfg oauthconfig.Config,
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	resolver *jwksresolver.Resolver,
	actorProvider providers.ActorProvider,
	governance providers.IdentityGovernanceProvider,
) TokenBuilderInterface {
	return &tokenBuilder{
		cfg:           cfg,
		jwtService:    jwtService,
		jweService:    jweService,
		jwksResolver:  resolver,
		actorProvider: actorProvider,
		governance:    governance,
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
	validityPeriod, capErr := tb.admitAndCap(ctx, tokenConfig.ValidityPeriod,
		accessTokenParties(tokenCtx, tokenConfig.Issuer)...)
	if capErr != nil {
		return nil, capErr
	}

	jwtClaims, claimsErr := tb.buildAccessTokenClaims(tokenCtx)
	if claimsErr != nil {
		return nil, fmt.Errorf("failed to build access token claims: %w", claimsErr)
	}

	tokenType := constants.TokenTypeBearer
	if tokenCtx.DPoPJkt != "" {
		tokenType = constants.TokenTypeDPoP
	}

	tokenDTO := &oauth2model.TokenDTO{
		TokenType:            tokenType,
		ExpiresIn:            validityPeriod,
		Scopes:               tokenCtx.Scopes,
		ClientID:             tokenCtx.ClientID,
		UserAttributes:       tokenCtx.SubjectAttributes,
		AttributeCacheID:     tokenCtx.AttributeCacheID,
		Subject:              tokenCtx.Subject,
		Audiences:            tokenCtx.Audiences,
		ClaimsRequest:        tokenCtx.ClaimsRequest,
		ClaimsLocales:        tokenCtx.ClaimsLocales,
		TokenFamilyID:        tokenCtx.TokenFamilyID,
		AuthorizationDetails: tokenCtx.AuthorizationDetails,
	}
	if tokenCtx.ActorClaims != nil {
		tokenDTO.Delegated = true
		tokenDTO.ActorSub = tb.resolveActorIdentity(tokenCtx, tokenCtx.ActorClaims.Sub)
	}
	tokenDTO.SubjectID, tokenDTO.SubjectCategory = tb.resolveSubjectIdentity(tokenCtx)

	token, iat, err := tb.jwtService.GenerateJWT(
		ctx,
		tokenCtx.Subject,
		tokenConfig.Issuer,
		validityPeriod,
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

// resolveActorIdentity returns the resource ID of the principal acting for the subject, for
// observability rather than for any token claim, and an empty string when the actor's sub does not
// name an entity this server knows. It applies the rule resolveSubjectIdentity applies to the
// subject: an act.sub arriving on a presented actor_token is that token's own subject, which the
// issuing application may have mapped to an attribute such as an email address, and reporting no
// actor is preferable to publishing one that identifies a person. Delegation itself stays visible
// regardless, because the token DTO records it separately from the actor's identifier.
func (tb *tokenBuilder) resolveActorIdentity(tokenCtx *AccessTokenBuildContext, actorSub string) string {
	if actorSub == "" {
		return ""
	}

	// A grant that resolved the actor itself (token exchange, for a mapped actor_token sub) says so.
	if tokenCtx.ActorClaims != nil && tokenCtx.ActorClaims.EntityID != "" {
		return tokenCtx.ActorClaims.EntityID
	}

	// Grants that name the authenticated client as the actor (authorization_code, CIBA, refresh, and
	// token exchange without an actor_token) already hold a resource ID, so no lookup is needed.
	if tokenCtx.OAuthApp != nil && actorSub == tokenCtx.OAuthApp.ID {
		return actorSub
	}

	if tb.lookupCategory(actorSub) == "" {
		return ""
	}
	return actorSub
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

// admitAndCap asks the identity governance authority, at the issuance plane, whether a token may be
// issued now for each entity it is issued to or for, and returns validityPeriod capped at the earliest
// lifespan end among them. A refused entity fails issuance with ErrIssuanceRefused, without saying why;
// an authority fault fails it as a server error. An ID that names no local entity, such as an external
// token's sub, is admitted by the authority. Every token is built through here, so no grant can issue a
// token that outlives an entity it names.
func (tb *tokenBuilder) admitAndCap(ctx context.Context, validityPeriod int64,
	entityIDs ...string) (int64, error) {
	if tb.governance == nil {
		return validityPeriod, nil
	}
	var notAfter time.Time
	seen := make(map[string]bool, len(entityIDs))
	for _, entityID := range entityIDs {
		if entityID == "" || seen[entityID] {
			continue
		}
		seen[entityID] = true

		decision, svcErr := tb.evaluateIssuance(ctx, entityID)
		if svcErr != nil {
			return 0, fmt.Errorf("failed to evaluate issuance for an entity involved in the token: %s",
				svcErr.Error.DefaultValue)
		}
		if !decision.Admitted {
			return 0, ErrIssuanceRefused
		}
		if !decision.NotAfter.IsZero() && (notAfter.IsZero() || decision.NotAfter.Before(notAfter)) {
			notAfter = decision.NotAfter
		}
	}
	return capAtLifespan(validityPeriod, notAfter)
}

// evaluateIssuance returns the issuance decision for entityID, served from the request's memo when
// WithIssuanceDecisions set one up and the entity was already checked. A fault is not memoized.
func (tb *tokenBuilder) evaluateIssuance(ctx context.Context,
	entityID string) (providers.AccessDecision, *tidcommon.ServiceError) {
	memo, _ := ctx.Value(issuanceDecisionsKey{}).(map[string]providers.AccessDecision)
	if decision, ok := memo[entityID]; ok {
		return decision, nil
	}
	decision, svcErr := tb.governance.EvaluateAccess(ctx, entityID, providers.AccessPlaneIssuance, "")
	if svcErr == nil && memo != nil {
		memo[entityID] = decision
	}
	return decision, svcErr
}

// capAtLifespan returns validityPeriod shortened so the token expires no later than notAfter, and
// ErrIssuanceRefused when that instant has already been reached. A zero notAfter caps nothing. The JWT
// is stamped a moment after this check, possibly in the next second, so one second is held back to
// keep exp at or before notAfter.
func capAtLifespan(validityPeriod int64, notAfter time.Time) (int64, error) {
	if notAfter.IsZero() {
		return validityPeriod, nil
	}
	remaining := notAfter.Unix() - time.Now().Unix() - 1
	if remaining <= 0 {
		return 0, ErrIssuanceRefused
	}
	return min(validityPeriod, remaining), nil
}

// accessTokenParties returns the entities an access token is issued to or for: the client, the
// subject, the actor, and every actor nested under it in the token's act claim. The subject and the
// actor are checked only when the grant resolved them to a local entity, and a nested actor only when
// this server recorded it, since an external issuer's sub names no local account even when it equals a
// local entity's ID. A client acting as the actor is already checked as the client.
func accessTokenParties(tokenCtx *AccessTokenBuildContext, serverIssuer string) []string {
	parties := []string{clientEntityID(tokenCtx.OAuthApp), tokenCtx.SubjectEntityID}
	if tokenCtx.ActorClaims == nil {
		return parties
	}
	parties = append(parties, tokenCtx.ActorClaims.EntityID)
	for act := tokenCtx.ActorClaims.NestedAct; act != nil; {
		sub, _ := act["sub"].(string)
		if iss, _ := act["iss"].(string); iss == "" || iss == serverIssuer {
			parties = append(parties, sub)
		}
		act, _ = act["act"].(map[string]interface{})
	}
	return parties
}

// clientEntityID returns the resource ID of the client a token is issued to, empty when unknown.
func clientEntityID(oauthApp *providers.OAuthClient) string {
	if oauthApp == nil {
		return ""
	}
	return oauthApp.ID
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
	validityPeriod, capErr := tb.admitAndCap(ctx, validityPeriod, clientEntityID(tokenCtx.OAuthApp), tokenCtx.Subject)
	if capErr != nil {
		return nil, capErr
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

	// Merge the subject's attributes (already resolved and filtered by the grant handler), skipping
	// claims the builder writes itself so a configured attribute cannot supply one it is trusted for.
	ownedClaims := builderOwnedClaimNames()
	for key, value := range ctx.SubjectAttributes {
		if ownedClaims[key] {
			continue
		}
		claims[key] = value
	}

	// Set after merging subject attributes so the organization the token was issued for wins over
	// any organization claim the subject's own attributes carried.
	for key, value := range ctx.OUAttributes {
		claims[key] = value
	}

	// Set after merging subject attributes to prevent them from overwriting this system claim.
	if ctx.AttributeCacheID != "" {
		claims["aci"] = ctx.AttributeCacheID
	}

	// set sub_type claim to the token if conditions are met.
	// This is used to distinguish between an agent and an M2M application.
	if ctx.GrantType == string(providers.GrantTypeClientCredentials) &&
		slices.Contains(clientConfigAttributeNames(ctx.OAuthApp), constants.ClaimSubType) {
		if subType := clientSubjectType(ctx.OAuthApp); subType != "" {
			claims[constants.ClaimSubType] = subType
		}
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

	if len(ctx.AuthorizationDetails) > 0 {
		claims[constants.ClaimAuthorizationDetails] = ctx.AuthorizationDetails
	}

	return claims, nil
}

// clientSubjectType maps an OAuth client's entity category to its sub_type claim value. Returns ""
// for a category that is not a client identity class, so the claim is omitted rather than guessed.
func clientSubjectType(oauthApp *providers.OAuthClient) string {
	if oauthApp == nil {
		return ""
	}

	switch oauthApp.EntityCategory {
	case providers.EntityCategoryAgent:
		return constants.SubTypeAgent
	case providers.EntityCategoryApp:
		return constants.SubTypeApp
	default:
		return ""
	}
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
	validityPeriod, capErr := tb.admitAndCap(ctx, validityPeriod, clientEntityID(tokenCtx.OAuthApp),
		tokenCtx.SubjectEntityID, tokenCtx.ActorSub)
	if capErr != nil {
		return nil, capErr
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
		jwt.TokenTypeRefreshToken,
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
	claims[constants.ClaimAccessTokenAudience] = ctx.AccessTokenAudiences
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

	if ctx.SessionID != "" {
		claims[constants.ClaimSessionID] = ctx.SessionID
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
	validityPeriod, capErr := tb.admitAndCap(ctx, tokenConfig.ValidityPeriod, clientEntityID(tokenCtx.OAuthApp),
		tokenCtx.SubjectEntityID)
	if capErr != nil {
		return nil, capErr
	}

	jwtClaims := tb.buildIDTokenClaims(tokenCtx)

	tokenDTO := &oauth2model.TokenDTO{
		ExpiresIn: validityPeriod,
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
		validityPeriod,
		jwtClaims,
		jwt.TokenTypeJWT,
		tokenConfig.SigningAlg,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID token: %v", err.Error)
	}

	token, encErr := tb.encryptForClient(ctx, tokenCtx.OAuthApp, token, "ID token")
	if encErr != nil {
		return nil, encErr
	}

	// Assign generated token and issued at time
	tokenDTO.Token = token
	tokenDTO.IssuedAt = iat

	return tokenDTO, nil
}

// BuildLogoutToken builds an OIDC Back-Channel Logout token for one client and one terminated session.
// It is signed like the client's ID tokens, and encrypted the same way when the client negotiated ID
// token encryption, with iss replicated in the JWE header (§2.4). Every call yields a fresh jti.
func (tb *tokenBuilder) BuildLogoutToken(
	ctx context.Context,
	tokenCtx *LogoutTokenBuildContext,
) (*oauth2model.TokenDTO, error) {
	if tokenCtx == nil || tokenCtx.OAuthApp == nil || tokenCtx.OAuthApp.ClientID == "" {
		return nil, fmt.Errorf("logout token needs a client with a client id")
	}
	if tokenCtx.SubjectID == "" || tokenCtx.SessionID == "" {
		return nil, fmt.Errorf("logout token needs a subject and a session id")
	}
	client := tokenCtx.OAuthApp
	tokenConfig := ResolveTokenConfig(tb.cfg, client, TokenTypeID, 0)
	// Validated positive at startup; the default lives in default.json.
	validity := tb.cfg.OAuth.Logout.Backchannel.TokenValidityPeriod

	// iss, iat, exp, nbf and jti come from the JWT service; nonce is never set (§2.4).
	claims := map[string]interface{}{
		constants.ClaimAud:       client.ClientID,
		constants.ClaimSessionID: tokenCtx.SessionID,
		claimEvents:              map[string]interface{}{eventBackchannelLogout: map[string]interface{}{}},
	}
	token, iat, svcErr := tb.jwtService.GenerateJWT(ctx, tokenCtx.SubjectID, tokenConfig.Issuer, validity, claims,
		jwt.TokenTypeLogout, tokenConfig.SigningAlg)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to generate logout token: %v", svcErr.Error)
	}
	token, err := tb.encryptForClient(ctx, client, token, "logout token",
		jwe.WithProtectedHeader("iss", tokenConfig.Issuer))
	if err != nil {
		return nil, err
	}
	return &oauth2model.TokenDTO{
		Token:     token,
		IssuedAt:  iat,
		ExpiresIn: validity,
		ClientID:  client.ClientID,
		Subject:   tokenCtx.SubjectID,
		Audiences: []string{client.ClientID},
	}, nil
}

// BuildDeviceSecret builds the token that carries the SSO sessions of a sign-in to the client on a code
// exchange: each session's flow identifier and handle, for the subject who signed in. It is signed with
// the client's ID token signing algorithm and addressed to this server, which accepts it back as a token
// exchange subject token.
func (tb *tokenBuilder) BuildDeviceSecret(
	ctx context.Context,
	tokenCtx *DeviceSecretBuildContext,
) (*oauth2model.TokenDTO, error) {
	if tokenCtx == nil || tokenCtx.OAuthApp == nil || tokenCtx.OAuthApp.ClientID == "" {
		return nil, fmt.Errorf("device secret needs a client with a client id")
	}
	if tokenCtx.SubjectID == "" || len(tokenCtx.SessionData) == 0 || tokenCtx.ValidityPeriod <= 0 {
		return nil, fmt.Errorf("device secret needs a subject, a session and a validity period")
	}
	client := tokenCtx.OAuthApp
	tokenConfig := ResolveTokenConfig(tb.cfg, client, TokenTypeID, 0)

	sessionData := make([]map[string]string, 0, len(tokenCtx.SessionData))
	for _, entry := range tokenCtx.SessionData {
		sessionData = append(sessionData, map[string]string{
			claimFlowIdentifier: entry.FlowIdentifier,
			claimSessionHandle:  entry.SessionHandle,
		})
	}
	// iss, iat, exp, nbf and jti come from the JWT service.
	claims := map[string]interface{}{
		constants.ClaimAud: tokenConfig.Issuer,
		claimSessionData:   sessionData,
	}
	token, iat, svcErr := tb.jwtService.GenerateJWT(ctx, tokenCtx.SubjectID, tokenConfig.Issuer,
		tokenCtx.ValidityPeriod, claims, jwt.TokenTypeDeviceSecret, tokenConfig.SigningAlg)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to generate device secret: %v", svcErr.Error)
	}
	return &oauth2model.TokenDTO{
		Token:     token,
		IssuedAt:  iat,
		ExpiresIn: tokenCtx.ValidityPeriod,
		ClientID:  client.ClientID,
		Subject:   tokenCtx.SubjectID,
		Audiences: []string{tokenConfig.Issuer},
	}, nil
}

// encryptForClient returns the signed token encrypted with the client's ID token encryption settings
// when the client negotiated JWE or NESTED_JWT ID tokens, and unchanged otherwise. kind names the
// token in errors.
func (tb *tokenBuilder) encryptForClient(ctx context.Context, client *providers.OAuthClient, token, kind string,
	opts ...jwe.EncryptOption) (string, error) {
	if client == nil || client.Token == nil || client.Token.IDToken == nil {
		return token, nil
	}
	idTokenCfg := client.Token.IDToken
	rt := idTokenCfg.ResponseType
	if rt != providers.IDTokenResponseTypeJWE && rt != providers.IDTokenResponseTypeNESTEDJWT {
		return token, nil
	}
	if tb.jweService == nil {
		return "", fmt.Errorf("JWE service is not configured")
	}
	rpKey, rpKID, svcErr := tb.jwksResolver.ResolveEncryptionKey(ctx, client.Certificate, idTokenCfg.EncryptionAlg,
		jwksresolver.KeyUseLenientEnc)
	if svcErr != nil {
		return "", fmt.Errorf("failed to resolve %s encryption key: %v", kind, svcErr)
	}
	// cty="JWT" indicates a nested JWT (signed JWS payload encrypted as JWE per OIDC spec)
	encrypted, svcErr := tb.jweService.Encrypt(ctx, []byte(token), &providers.KeyRef{PublicKeyJWK: rpKey},
		idTokenCfg.EncryptionAlg, jwe.ContentEncAlgorithm(idTokenCfg.EncryptionEnc), "JWT", rpKID, opts...)
	if svcErr != nil {
		return "", fmt.Errorf("failed to encrypt %s: %v", kind, svcErr)
	}
	return encrypted, nil
}

// buildIDTokenClaims builds the claims map for an ID token (OIDC).
func (tb *tokenBuilder) buildIDTokenClaims(ctx *IDTokenBuildContext) map[string]interface{} {
	claims := make(map[string]interface{})

	if ctx.AuthTime > 0 {
		claims[constants.ClaimAuthTime] = ctx.AuthTime
	}

	if ctx.Nonce != "" {
		claims[constants.RequestParamNonce] = ctx.Nonce
	}

	if ctx.CompletedACR != "" {
		claims[constants.ClaimACR] = ctx.CompletedACR
	}

	if ctx.SessionID != "" {
		claims[constants.ClaimSessionID] = ctx.SessionID
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

	// Merge the attribute claims, skipping the ones the builder writes itself so a configured
	// attribute can neither replace them nor supply one that was not set.
	ownedClaims := builderOwnedIDTokenClaimNames()
	for key, value := range claimData {
		if ownedClaims[key] {
			continue
		}
		claims[key] = value
	}

	return claims
}
