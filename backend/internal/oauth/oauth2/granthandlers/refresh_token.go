// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/attributecache"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// refreshTokenGrantHandler handles the refresh token grant type.
type refreshTokenGrantHandler struct {
	cfg              oauthconfig.Config
	jwtService       jwt.JWTServiceInterface
	tokenBuilder     tokenservice.TokenBuilderInterface
	tokenValidator   tokenservice.TokenValidatorInterface
	attrCacheService attributecache.AttributeCacheServiceInterface
	resourceService  providers.ResourceServerProvider
	authzService     providers.AuthorizationProvider
	actorProvider    providers.ActorProvider
	refreshRevoker   revocation.RefreshTokenRevokerInterface
	criteriaRevoker  revocation.CriteriaRevokerInterface
}

// newRefreshTokenGrantHandler creates a new instance of RefreshTokenGrantHandler.
func newRefreshTokenGrantHandler(
	jwtService jwt.JWTServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	attrCacheService attributecache.AttributeCacheServiceInterface,
	resourceService providers.ResourceServerProvider,
	authzService providers.AuthorizationProvider,
	actorProvider providers.ActorProvider,
	refreshRevoker revocation.RefreshTokenRevokerInterface,
	criteriaRevoker revocation.CriteriaRevokerInterface,
	cfg oauthconfig.Config,
) RefreshTokenGrantHandlerInterface {
	return &refreshTokenGrantHandler{
		cfg:              cfg,
		jwtService:       jwtService,
		tokenBuilder:     tokenBuilder,
		tokenValidator:   tokenValidator,
		attrCacheService: attrCacheService,
		resourceService:  resourceService,
		authzService:     authzService,
		actorProvider:    actorProvider,
		refreshRevoker:   refreshRevoker,
		criteriaRevoker:  criteriaRevoker,
	}
}

// ValidateGrant validates the refresh token grant request.
func (h *refreshTokenGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) *model.ErrorResponse {
	if providers.GrantType(tokenRequest.GrantType) != providers.GrantTypeRefreshToken {
		return &model.ErrorResponse{
			Error:            constants.ErrorUnsupportedGrantType,
			ErrorDescription: "Unsupported grant type",
		}
	}
	if tokenRequest.RefreshToken == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Refresh token is required",
		}
	}
	if tokenRequest.ClientID == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Client ID is required",
		}
	}

	if errResp := resourceindicators.ValidateResourceURIs(tokenRequest.Resources); errResp != nil {
		return errResp
	}

	return nil
}

// resolveRefreshToken validates the presented refresh token and confirms it was issued to the
// requesting client. ValidateRefreshToken enforces the RFC 7009 deny list, so a revoked token is
// rejected as invalid_grant like any other invalid token and an unavailable deny list fails closed
// with a server_error.
func (h *refreshTokenGrantHandler) resolveRefreshToken(ctx context.Context,
	tokenRequest *model.TokenRequest, logger *log.Logger) (
	*tokenservice.RefreshTokenClaims, *model.ErrorResponse) {
	refreshTokenClaims, err := h.tokenValidator.ValidateRefreshToken(ctx, tokenRequest.RefreshToken)
	if err != nil {
		logger.Debug(ctx, "Failed to validate refresh token", log.Error(err))
		if errors.Is(err, revocation.ErrEnforcementUnavailable) {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Token revocation status could not be verified",
			}
		}
		// A revoked (already-rotated) refresh token presented again is a replay signal: revoke
		// the whole token family so the attacker's freshly rotated tokens die too (RFC 9700 §4.14.2).
		if errors.Is(err, revocation.ErrTokenRevoked) {
			h.revokeTokenFamilyOnReplay(ctx, tokenRequest.RefreshToken, logger)
		}
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid refresh token",
		}
	}

	// A client may only redeem refresh tokens issued to it.
	if refreshTokenClaims.ClientID != tokenRequest.ClientID {
		logger.Debug(ctx, "Refresh token does not belong to the requesting client")
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid refresh token",
		}
	}

	return refreshTokenClaims, nil
}

// HandleGrant processes the refresh token grant request and generates a new token response.
func (h *refreshTokenGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RefreshTokenGrantHandler"))

	refreshTokenClaims, errResp := h.resolveRefreshToken(ctx, tokenRequest, logger)
	if errResp != nil {
		return nil, errResp
	}

	if errResp := dpop.VerifyProofBinding(ctx, refreshTokenClaims.DPoPJkt, "refresh token"); errResp != nil {
		return nil, errResp
	}

	subjectEntity, errResp := h.verifyCredentialsUnchanged(ctx, refreshTokenClaims, oauthApp, logger)
	if errResp != nil {
		return nil, errResp
	}

	newTokenScopes, scopeErr := h.validateAndApplyScopes(ctx, tokenRequest.Scope, refreshTokenClaims.Scopes, logger)
	if scopeErr != nil {
		return nil, scopeErr
	}

	// The refresh token is bound to exactly one resource server audience. When the request supplies
	// a resource it must match that audience; when omitted, the bound audience is reused.
	if len(refreshTokenClaims.Audiences) != 1 {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Refresh token is not bound to a single resource server",
		}
	}
	audience := refreshTokenClaims.Audiences[0]
	if len(tokenRequest.Resources) > 1 {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "Only a single resource parameter is supported",
		}
	}
	if len(tokenRequest.Resources) == 1 && tokenRequest.Resources[0] != audience {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "Requested resource does not match the refresh token audience",
		}
	}
	audiences := []string{audience}

	oidcScopes, nonOidcScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
		strings.Join(newTokenScopes, " "), oauthApp.ScopeClaims)
	if audience == tokenRequest.ClientID {
		// The original token was not bound to a resource server (OIDC-only): its audience is the
		// client_id, which is not a resource server, so there are no permissions to downscope against.
		newTokenScopes = oidcScopes
	} else {
		// Resolve the bound resource server to downscope scopes to its currently defined permissions.
		targetRS, rsErr := h.resourceService.GetResourceServerByIdentifier(ctx, audience)
		if rsErr != nil {
			if rsErr.Type == tidcommon.ServerErrorType {
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorServerError,
					ErrorDescription: "Failed to resolve resource server",
				}
			}
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidTarget,
				ErrorDescription: "The resource server bound to the refresh token no longer exists",
			}
		}
		downscopedNonOidc, scopeErr := resourceindicators.DownscopeToResourceServer(
			ctx, h.resourceService, targetRS.ID, nonOidcScopes)
		if scopeErr != nil {
			return nil, scopeErr
		}
		authorizedNonOidc, authzErr := h.reauthorizeScopes(
			ctx, subjectEntity, targetRS.ID, targetRS.Identifier, downscopedNonOidc, logger)
		if authzErr != nil {
			return nil, authzErr
		}
		newTokenScopes = make([]string, 0, len(oidcScopes)+len(authorizedNonOidc))
		newTokenScopes = append(newTokenScopes, oidcScopes...)
		newTokenScopes = append(newTokenScopes, authorizedNonOidc...)
	}

	// Get user attributes from attribute cache.
	// cacheEntry is kept so its current TTLSeconds can be compared later.
	attrs := make(map[string]interface{})
	var cacheEntry *attributecache.AttributeCache
	var fetchErr *tidcommon.ServiceError
	if refreshTokenClaims.AttributeCacheID != "" {
		cacheEntry, fetchErr = h.attrCacheService.GetAttributeCache(ctx, refreshTokenClaims.AttributeCacheID)
		if fetchErr != nil {
			logger.Error(ctx, "Failed to get user attributes from attribute cache",
				log.String("error", fetchErr.ErrorDescription.DefaultValue))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		if cacheEntry == nil {
			logger.Error(ctx, "Attribute cache entry not found for cache ID",
				log.String("cache_id", refreshTokenClaims.AttributeCacheID))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		attrs = cacheEntry.Attributes
	}

	userSubConfig := oauthApp.UserAccessTokenConfig()
	accessTokenCtx := &tokenservice.AccessTokenBuildContext{
		Subject:           refreshTokenClaims.Sub,
		Audiences:         audiences,
		ClientID:          tokenRequest.ClientID,
		Scopes:            newTokenScopes,
		SubjectAttributes: tokenservice.FilterAttributesByAllowList(attrs, userSubConfig),
		AttributeCacheID:  refreshTokenClaims.AttributeCacheID,
		GrantType:         refreshTokenClaims.GrantType,
		OAuthApp:          oauthApp,
		ClaimsRequest:     refreshTokenClaims.ClaimsRequest,
		ClaimsLocales:     refreshTokenClaims.ClaimsLocales,
		ValidityPeriod:    userSubConfig.ValidityPeriodOrZero(),
		DPoPJkt:           dpop.GetJkt(ctx),
		TokenFamilyID:     refreshTokenClaims.TokenFamilyID,
	}
	// Replay the on-behalf-of decision frozen at issuance, sourced from the stored marker
	// rather than the client's current setting.
	if refreshTokenClaims.ActorSub != "" {
		accessTokenCtx.ActorClaims = &tokenservice.SubjectTokenClaims{Sub: refreshTokenClaims.ActorSub}
	}
	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, accessTokenCtx)
	if err != nil {
		logger.Error(ctx, "Failed to generate access token", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate access token",
		}
	}

	// Prepare the token response
	tokenResponse := &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}

	// Generate ID token if 'openid' scope is present
	if slices.Contains(newTokenScopes, constants.ScopeOpenID) {
		idToken, idErr := h.tokenBuilder.BuildIDToken(ctx, &tokenservice.IDTokenBuildContext{
			Subject:        refreshTokenClaims.Sub,
			Audience:       tokenRequest.ClientID,
			Scopes:         newTokenScopes,
			UserAttributes: attrs,
			OAuthApp:       oauthApp,
			ClaimsRequest:  refreshTokenClaims.ClaimsRequest,
		})
		if idErr != nil {
			logger.Error(ctx, "Failed to generate ID token", log.Error(idErr))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate token",
			}
		}
		tokenResponse.IDToken = *idToken
	}

	renewRefreshToken := h.cfg.OAuth.RefreshToken.RenewOnGrant

	// Issue a new refresh token if renew_on_grant is enabled; otherwise reuse the existing one.
	// The new refresh token carries the same single resource server audience.
	if renewRefreshToken {
		logger.Debug(ctx, "Renewing refresh token", log.String("client_id", tokenRequest.ClientID))
		errResp := h.IssueRefreshToken(ctx, tokenResponse, oauthApp,
			refreshTokenClaims.Sub, audiences,
			refreshTokenClaims.GrantType, newTokenScopes,
			refreshTokenClaims.ClaimsRequest, refreshTokenClaims.ClaimsLocales,
			refreshTokenClaims.AttributeCacheID, refreshTokenClaims.TokenFamilyID,
			refreshTokenClaims.Exp)
		if errResp != nil && errResp.Error != "" {
			logger.Error(ctx, "Failed to issue refresh token", log.String("error", errResp.Error))
			return nil, errResp
		}

		// Single-use: revoke the consumed refresh token so it cannot be replayed (RFC 9700 §4.14.2).
		// Fail closed — if the revocation cannot be recorded, the old token would remain usable, so the
		// rotation is rejected and the client retries with the still-valid old token.
		if h.refreshRevoker != nil && h.cfg.OAuth.RefreshToken.RevokePreviousOnRenewEnabled() {
			expiryTime := time.Unix(refreshTokenClaims.Exp, 0).UTC()
			if err := h.refreshRevoker.RevokeRefreshToken(
				ctx, refreshTokenClaims.JTI, expiryTime); err != nil {
				logger.Error(ctx, "Failed to revoke rotated refresh token", log.Error(err))
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorServerError,
					ErrorDescription: "Failed to rotate refresh token",
				}
			}
		}
	} else {
		tokenResponse.RefreshToken = model.TokenDTO{
			Token:    tokenRequest.RefreshToken,
			IssuedAt: refreshTokenClaims.Iat,
			Scopes:   refreshTokenClaims.Scopes,
			ClientID: tokenRequest.ClientID,
		}
	}

	if errResp := h.extendCacheTTL(ctx, cacheEntry, refreshTokenClaims.Exp,
		accessToken.ExpiresIn, refreshTokenClaims.AttributeCacheID,
		logger); errResp != nil {
		return nil, errResp
	}

	return tokenResponse, nil
}

// revokeTokenFamilyOnReplay revokes the token family of a replayed (already-revoked) refresh token, when
// enabled. It is best-effort: the refresh grant is rejected as invalid_grant regardless, and a failed
// family revoke is logged but does not change that outcome. The refresh token's signature was already
// verified upstream, so its tfid claim is trustworthy; the payload is decoded here only to read it.
func (h *refreshTokenGrantHandler) revokeTokenFamilyOnReplay(ctx context.Context, refreshToken string,
	logger *log.Logger) {
	if h.criteriaRevoker == nil || !h.cfg.OAuth.Revocation.TokenFamily.OnRefreshReplayEnabled() {
		return
	}
	claims, err := jwt.DecodeJWTPayload(refreshToken)
	if err != nil {
		logger.Debug(ctx, "Could not decode replayed refresh token to resolve its token family",
			log.Error(err))
		return
	}
	tokenFamilyID, _ := claims[constants.ClaimTokenFamilyID].(string)
	if tokenFamilyID == "" {
		return
	}
	if err := h.criteriaRevoker.RevokeTokenFamily(ctx, tokenFamilyID,
		revocation.RevocationReasonRefreshReplay); err != nil {
		logger.Error(ctx, "Failed to revoke token family on refresh token replay", log.Error(err))
	}
}

// IssueRefreshToken generates a new refresh token for the given OAuth application and scopes.
func (h *refreshTokenGrantHandler) IssueRefreshToken(
	ctx context.Context,
	tokenResponse *model.TokenResponseDTO,
	oauthApp *providers.OAuthClient,
	subject string, audiences []string, grantType string,
	scopes []string,
	claimsRequest *model.ClaimsRequest,
	claimsLocales string,
	attributeCacheID string,
	tokenFamilyID string,
	expiresAt int64,
) *model.ErrorResponse {
	tokenCtx := &tokenservice.RefreshTokenBuildContext{
		ExpiresAt:            expiresAt,
		ClientID:             oauthApp.ClientID,
		Scopes:               scopes,
		GrantType:            grantType,
		AccessTokenSubject:   subject,
		AccessTokenAudiences: audiences,
		AttributeCacheID:     attributeCacheID,
		OAuthApp:             oauthApp,
		ClaimsRequest:        claimsRequest,
		ClaimsLocales:        claimsLocales,
		DPoPJkt:              dpopJktForRefresh(ctx, oauthApp),
		TokenFamilyID:        tokenFamilyID,
	}
	if oauthApp.ShouldAppendActorClaim() {
		tokenCtx.ActorSub = oauthApp.ID
	}

	// Build refresh token using token builder
	refreshToken, err := h.tokenBuilder.BuildRefreshToken(ctx, tokenCtx)
	if err != nil {
		return &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate refresh token",
		}
	}

	if tokenResponse == nil {
		tokenResponse = &model.TokenResponseDTO{}
	}
	tokenResponse.RefreshToken = *refreshToken
	return nil
}

// dpopJktForRefresh returns the DPoP jkt to bind onto a newly issued refresh token.
// Confidential clients receive unbound refresh tokens.
func dpopJktForRefresh(ctx context.Context, oauthApp *providers.OAuthClient) string {
	if oauthApp == nil || !oauthApp.PublicClient {
		return ""
	}
	return dpop.GetJkt(ctx)
}

// extendCacheTTL extends the attribute cache TTL when the desired lifetime exceeds what is already
// stored. The desired TTL is the larger of:
//   - the refresh token's expiry, which a rotated token inherits, so it is the same either way
//   - the newly issued access token's expiry (now + ExpiresIn)
//
// This ensures the cache outlives whichever token lives longest without needlessly
// re-writing an already-sufficient entry.
func (h *refreshTokenGrantHandler) extendCacheTTL(
	ctx context.Context,
	cacheEntry *attributecache.AttributeCache,
	refreshExpiry, accessExpiresIn int64,
	cacheID string,
	logger *log.Logger,
) *model.ErrorResponse {
	if cacheEntry == nil {
		return nil
	}
	now := time.Now().Unix()
	accessExpiry := now + accessExpiresIn
	maxExpiry := refreshExpiry
	if accessExpiry > maxExpiry {
		maxExpiry = accessExpiry
	}
	desiredTTL := maxExpiry - now + constants.AttributeCacheTTLBufferSeconds
	extErr := h.attrCacheService.ExtendAttributeCacheTTL(ctx, cacheID, int(desiredTTL))
	if extErr != nil {
		logger.Error(ctx, "Failed to extend attribute cache TTL",
			log.String("cache_id", cacheID),
			log.String("error", extErr.Error.String()))
		return &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to extend attribute cache TTL",
		}
	}
	return nil
}

// verifyCredentialsUnchanged rejects a refresh token established at or before the user's password or
// the client's secret last changed. It returns the subject's entity for reauthorizeScopes to reuse.
func (h *refreshTokenGrantHandler) verifyCredentialsUnchanged(ctx context.Context,
	claims *tokenservice.RefreshTokenClaims, oauthApp *providers.OAuthClient,
	logger *log.Logger) (*providers.Entity, *model.ErrorResponse) {
	if h.actorProvider == nil {
		return nil, nil
	}

	subjectEntity, errResp := h.resolveSubjectEntity(ctx, claims.Sub, oauthApp, logger)
	if errResp != nil {
		return nil, errResp
	}
	if credentialChangedSince(ctx, subjectEntity, claims.Iat, logger) {
		logger.Debug(ctx, "Rejecting refresh token established before a user credential change")
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid refresh token",
		}
	}

	if oauthApp == nil || oauthApp.ID == "" || oauthApp.ID == claims.Sub {
		return subjectEntity, nil
	}
	clientEntity, svcErr := h.actorProvider.GetActor(oauthApp.ID)
	if svcErr != nil {
		logger.Error(ctx, "Failed to resolve the refresh token client",
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to verify credential state",
		}
	}
	if credentialChangedSince(ctx, clientEntity, claims.Iat, logger) {
		logger.Debug(ctx, "Rejecting refresh token established before a client secret rotation")
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid refresh token",
		}
	}
	return subjectEntity, nil
}

// resolveSubjectEntity resolves the token's subject to its entity, or nil when it does not name one.
//
// sub is not always an entity ID: a client can map it to a user attribute
// (InboundClient.SubjectAttribute). An unresolvable subject is therefore ambiguous, and the client's
// mapping settles it. A client that maps nothing can only have issued an entity ID, so the subject
// is gone.
func (h *refreshTokenGrantHandler) resolveSubjectEntity(ctx context.Context, subject string,
	oauthApp *providers.OAuthClient, logger *log.Logger) (*providers.Entity, *model.ErrorResponse) {
	if subject == "" {
		return nil, nil
	}

	entity, svcErr := h.actorProvider.GetActor(subject)
	if svcErr == nil {
		return entity, nil
	}
	if svcErr.Type != tidcommon.ClientErrorType {
		logger.Error(ctx, "Failed to resolve refresh token subject",
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to resolve the refresh token subject",
		}
	}

	mapsSubject, errResp := h.clientMapsSubject(ctx, oauthApp, logger)
	if errResp != nil {
		return nil, errResp
	}
	if mapsSubject {
		logger.Debug(ctx, "Refresh token subject is a mapped value; skipping subject-derived checks")
		return nil, nil
	}

	logger.Debug(ctx, "Refresh token subject no longer exists")
	return nil, &model.ErrorResponse{
		Error:            constants.ErrorInvalidGrant,
		ErrorDescription: "Invalid refresh token",
	}
}

// clientMapsSubject reports whether the client maps the token subject to a user attribute. An
// unidentifiable client reports true, since the caller rejects on false.
func (h *refreshTokenGrantHandler) clientMapsSubject(ctx context.Context,
	oauthApp *providers.OAuthClient, logger *log.Logger) (bool, *model.ErrorResponse) {
	if oauthApp == nil || oauthApp.ID == "" {
		return true, nil
	}

	client, svcErr := h.actorProvider.GetInboundClientByID(ctx, oauthApp.ID)
	if svcErr != nil {
		logger.Error(ctx, "Failed to resolve the client's subject mapping",
			log.String("error", svcErr.Error.DefaultValue))
		return false, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to resolve the refresh token subject",
		}
	}
	return client != nil && len(client.SubjectAttribute) > 0, nil
}

// credentialChangedSince reports whether the entity's credential changed at or after iat, which has
// second granularity, so a change in the same second counts. An absent marker reports false. So does
// an unreadable one, since locking the entity out is the worse failure, but that is a data fault and
// is logged.
func credentialChangedSince(ctx context.Context, entity *providers.Entity, iat int64,
	logger *log.Logger) bool {
	if entity == nil || len(entity.SystemAttributes) == 0 {
		return false
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(entity.SystemAttributes, &attrs); err != nil {
		logger.Error(ctx, "Failed to parse system attributes while reading the credential marker",
			log.MaskedString(log.LoggerKeyUserID, entity.ID), log.Error(err))
		return false
	}
	value, present := attrs[authnprovidercm.SystemAttrCredentialUpdatedAt]
	if !present {
		return false
	}
	raw, ok := value.(string)
	if !ok || raw == "" {
		logger.Error(ctx, "Credential marker is not a non-empty string",
			log.MaskedString(log.LoggerKeyUserID, entity.ID))
		return false
	}
	changedAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		logger.Error(ctx, "Credential marker is not a valid RFC 3339 timestamp",
			log.MaskedString(log.LoggerKeyUserID, entity.ID), log.Error(err))
		return false
	}
	return iat <= changedAt.UTC().Unix()
}

// reauthorizeScopes re-evaluates the subject's permission scopes against their current role and group
// assignments, dropping the ones they no longer hold.
func (h *refreshTokenGrantHandler) reauthorizeScopes(ctx context.Context, subjectEntity *providers.Entity,
	resourceServerID string, resourceServerIdentifier string, scopes []string,
	logger *log.Logger) ([]string, *model.ErrorResponse) {
	// An unresolved subject (a mapped sub) would authorize nothing and strip every scope.
	if len(scopes) == 0 || h.authzService == nil || subjectEntity == nil || subjectEntity.ID == "" {
		return scopes, nil
	}
	subject := subjectEntity.ID

	// Roles reach a subject directly and through their groups, so both are evaluated together.
	groups, groupErr := h.actorProvider.GetActorGroups(subject)
	if groupErr != nil {
		logger.Error(ctx, "Failed to resolve group memberships for refresh token subject",
			log.MaskedString(log.LoggerKeyUserID, subject),
			log.String("error", groupErr.Error.DefaultValue))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}
	var groupIDs []string
	for _, group := range groups {
		if group.ID != "" && !slices.Contains(groupIDs, group.ID) {
			groupIDs = append(groupIDs, group.ID)
		}
	}

	authzResp, svcErr := h.authzService.EvaluateAccessBatch(ctx,
		buildAccessEvaluationsRequest(subject, subjectEntity.Category, groupIDs, scopes,
			resourceServerID, resourceServerIdentifier))
	if svcErr != nil {
		logger.Error(ctx, "Failed to evaluate authorized permissions for refresh token subject",
			log.MaskedString(log.LoggerKeyUserID, subject),
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	authorizedScopes := filterAuthorizedScopes(scopes, authzResp.Evaluations)
	if len(authorizedScopes) != len(scopes) {
		logger.Debug(ctx, "Dropped permission scopes the subject is no longer authorized for",
			log.MaskedString(log.LoggerKeyUserID, subject),
			log.Int("grantedCount", len(scopes)),
			log.Int("authorizedCount", len(authorizedScopes)))
	}
	return authorizedScopes, nil
}

// validateAndApplyScopes validates and applies OAuth2 scope downscoping logic per RFC 6749 §6.
// If no scopes are requested, all refresh token scopes are granted.
// If scopes are requested, they must be a subset of the original grant; otherwise an invalid_scope error is returned.
func (h *refreshTokenGrantHandler) validateAndApplyScopes(ctx context.Context, requestedScopes string,
	refreshTokenScopes []string, logger *log.Logger) ([]string, *model.ErrorResponse) {
	trimmedRequestedScopes := tokenservice.ParseScopes(requestedScopes)

	if len(trimmedRequestedScopes) == 0 {
		logger.Debug(ctx, "No scopes requested. Granting all scopes from refresh token",
			log.Any("scopes", refreshTokenScopes))
		return refreshTokenScopes, nil
	}

	for _, requestedScope := range trimmedRequestedScopes {
		if !slices.Contains(refreshTokenScopes, requestedScope) {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidScope,
				ErrorDescription: "Requested scope exceeds the scope granted by the resource owner",
			}
		}
	}

	logger.Debug(ctx, "Applied scope downscoping", log.Any("grantedScopes", trimmedRequestedScopes))
	return trimmedRequestedScopes, nil
}
