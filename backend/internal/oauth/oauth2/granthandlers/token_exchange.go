/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

package granthandlers

import (
	"context"
	"errors"
	"slices"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// tokenExchangeGrantHandler handles the token exchange grant type.
type tokenExchangeGrantHandler struct {
	tokenBuilder    tokenservice.TokenBuilderInterface
	tokenValidator  tokenservice.TokenValidatorInterface
	resourceService providers.ResourceServerProvider
}

// newTokenExchangeGrantHandler creates a new instance of tokenExchangeGrantHandler.
func newTokenExchangeGrantHandler(
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	resourceService providers.ResourceServerProvider,
) GrantHandlerInterface {
	return &tokenExchangeGrantHandler{
		tokenBuilder:    tokenBuilder,
		tokenValidator:  tokenValidator,
		resourceService: resourceService,
	}
}

// ValidateGrant validates the token exchange grant type request.
func (h *tokenExchangeGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) *model.ErrorResponse {
	if providers.GrantType(tokenRequest.GrantType) != providers.GrantTypeTokenExchange {
		return &model.ErrorResponse{
			Error:            constants.ErrorUnsupportedGrantType,
			ErrorDescription: "Unsupported grant type",
		}
	}

	if tokenRequest.SubjectToken == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Missing required parameter: subject_token",
		}
	}

	if tokenRequest.SubjectTokenType == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Missing required parameter: subject_token_type",
		}
	}

	if !constants.TokenTypeIdentifier(tokenRequest.SubjectTokenType).IsValid() {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Unsupported subject_token_type",
		}
	}

	if errResp := resourceindicators.ValidateResourceURIs(tokenRequest.Resources); errResp != nil {
		return errResp
	}

	if tokenRequest.ActorToken != "" && tokenRequest.ActorTokenType == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "actor_token_type is required when actor_token is provided",
		}
	}

	if tokenRequest.ActorTokenType != "" {
		if tokenRequest.ActorToken == "" {
			return &model.ErrorResponse{
				Error:            constants.ErrorInvalidRequest,
				ErrorDescription: "actor_token_type must not be provided without actor_token",
			}
		}
		if !constants.TokenTypeIdentifier(tokenRequest.ActorTokenType).IsValid() {
			return &model.ErrorResponse{
				Error:            constants.ErrorInvalidRequest,
				ErrorDescription: "Unsupported actor_token_type",
			}
		}
	}

	if tokenRequest.RequestedTokenType != "" {
		requestedType := constants.TokenTypeIdentifier(tokenRequest.RequestedTokenType)

		if !requestedType.IsValid() {
			return &model.ErrorResponse{
				Error:            constants.ErrorInvalidRequest,
				ErrorDescription: "Unsupported requested_token_type",
			}
		}
		// TODO: Add support for other token types if needed
		if requestedType != constants.TokenTypeIdentifierAccessToken &&
			requestedType != constants.TokenTypeIdentifierJWT &&
			requestedType != constants.TokenTypeIdentifierIDJAG {
			return &model.ErrorResponse{
				Error: constants.ErrorInvalidRequest,
				ErrorDescription: "Unsupported requested_token_type. Only access tokens, JWT tokens, " +
					"and ID-JAGs are supported",
			}
		}

		// The draft restricts the subject token to an identity assertion; we support only ID tokens in v1.
		if requestedType == constants.TokenTypeIdentifierIDJAG &&
			tokenRequest.SubjectTokenType != string(constants.TokenTypeIdentifierIDToken) {
			return &model.ErrorResponse{
				Error: constants.ErrorInvalidRequest,
				ErrorDescription: "ID-JAG requests require subject_token_type " +
					"urn:ietf:params:oauth:token-type:id_token",
			}
		}
	}

	return nil
}

// HandleGrant handles the token exchange grant type.
func (h *tokenExchangeGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenExchangeGrantHandler"))

	// An ID-JAG request (draft-ietf-oauth-identity-assertion-authz-grant) follows a distinct path:
	// it exchanges a self-issued ID token for a JWT authorization grant targeted at an external
	// resource authorization server, rather than issuing an access token.
	if constants.TokenTypeIdentifier(tokenRequest.RequestedTokenType) == constants.TokenTypeIdentifierIDJAG {
		return h.handleIDJAGGrant(ctx, tokenRequest, oauthApp)
	}

	// Validate and extract subject token claims. ValidateSubjectToken enforces the RFC 7009 deny list
	// for self-issued tokens; a revoked token is rejected as invalid_request like any other invalid
	// subject_token, while an unavailable deny list fails closed with server_error.
	subjectClaims, err := h.tokenValidator.ValidateSubjectToken(ctx, tokenRequest.SubjectToken, oauthApp)
	if err != nil {
		logger.Debug(ctx, "Failed to validate subject token", log.Error(err))
		if errors.Is(err, revocation.ErrEnforcementUnavailable) {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Token revocation status could not be verified",
			}
		}
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Invalid subject_token",
		}
	}

	// Enforce subject_token DPoP binding. The proof's jkt is verified earlier in the
	// token service and propagated via context.
	if errResp := dpop.VerifyProofBinding(ctx, subjectClaims.CnfJkt, "subject_token"); errResp != nil {
		return nil, errResp
	}

	// Validate and extract actor token claims if present
	var actorClaims *tokenservice.SubjectTokenClaims
	if tokenRequest.ActorToken != "" {
		actorClaims, err = h.tokenValidator.ValidateSubjectToken(ctx, tokenRequest.ActorToken, oauthApp)
		if err != nil {
			logger.Debug(ctx, "Failed to validate actor token", log.Error(err))
			if errors.Is(err, revocation.ErrEnforcementUnavailable) {
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorServerError,
					ErrorDescription: "Token revocation status could not be verified",
				}
			}
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidRequest,
				ErrorDescription: "Invalid actor_token",
			}
		}
	}

	// Determine final scopes
	finalScopes, errResp := h.getScopes(tokenRequest, subjectClaims.Scopes)
	if errResp != nil {
		return nil, errResp
	}

	// Determine final audiences per RFC 8693 §2.1: audience and resource parameters may be
	// combined. audience values are opaque logical names passed verbatim; resource values are
	// RFC 8707 URIs that resolve to registered Resource Servers and participate in scope
	// downscoping.
	resolvedRSes, resErr := resourceindicators.ResolveResourceServers(ctx, h.resourceService, tokenRequest.Resources)
	if resErr != nil {
		return nil, resErr
	}
	// Narrow to RS-defined permissions when resource params present (RFC 8707 §2.2), matching client_credentials.
	if len(resolvedRSes) > 0 {
		rsValidScopes, rsErr := resourceindicators.ComputeRSValidScopes(
			ctx, h.resourceService, resolvedRSes, finalScopes)
		if rsErr != nil {
			return nil, rsErr
		}
		finalScopes = resourceindicators.UnionScopes(rsValidScopes)
	}
	rsAudiences, audErr := resourceindicators.ComposeAudiences(ctx, h.resourceService, tokenRequest.ClientID,
		resolvedRSes, finalScopes)
	if audErr != nil {
		return nil, audErr
	}
	finalAudiences := mergeAudiences(tokenRequest.Audiences, rsAudiences, tokenRequest.ClientID)

	// Build access token using token builder
	userSubConfig := oauthApp.UserAccessTokenConfig()
	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, &tokenservice.AccessTokenBuildContext{
		Subject:           subjectClaims.Sub,
		Audiences:         finalAudiences,
		ClientID:          tokenRequest.ClientID,
		Scopes:            finalScopes,
		SubjectAttributes: tokenservice.FilterAttributesByAllowList(subjectClaims.UserAttributes, userSubConfig),
		GrantType:         string(providers.GrantTypeTokenExchange),
		OAuthApp:          oauthApp,
		ActorClaims:       actorClaims,
		ValidityPeriod:    userSubConfig.ValidityPeriodOrZero(),
		DPoPJkt:           dpop.GetJkt(ctx),
	})
	if err != nil {
		logger.Error(ctx, "Failed to generate token", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	return &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}, nil
}

// handleIDJAGGrant issues an ID-JAG (Identity Assertion Authorization Grant) in response to a
// token-exchange request with requested_token_type=urn:ietf:params:oauth:token-type:id-jag. The
// subject_token must be a self-issued token, and the audience parameter must match one of the
// application's configured allowed audiences. DPoP binding is out of scope for ID-JAGs.
func (h *tokenExchangeGrantHandler) handleIDJAGGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) (*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenExchangeGrantHandler"))

	// The resource parameter (RFC 8707) is optional for ID-JAG requests; when present it is validated
	// up front and later embedded in the ID-JAG's `resource` claim for the resource AS to process.
	if errResp := resourceindicators.ValidateResourceURIs(tokenRequest.Resources); errResp != nil {
		return nil, errResp
	}

	// The application must have ID-JAG enabled and configured with at least one allowed audience.
	if oauthApp == nil || oauthApp.Token == nil || oauthApp.Token.IDJAG == nil ||
		!oauthApp.Token.IDJAG.Enabled || len(oauthApp.Token.IDJAG.AllowedAudiences) == 0 {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "The client is not permitted to request ID-JAGs",
		}
	}

	// Only confidential clients may request ID-JAGs; a public client cannot safely hold the grant.
	if oauthApp.TokenEndpointAuthMethod == providers.TokenEndpointAuthMethodNone {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidClient,
			ErrorDescription: "ID-JAG requests require a confidential client",
		}
	}

	// The audience parameter is required and must exactly match a configured allowed audience. Exactly
	// one audience is permitted so the issued ID-JAG is unambiguously targeted at a single RS.
	if len(tokenRequest.Audiences) == 0 {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "The audience parameter is required for ID-JAG requests",
		}
	}
	if len(tokenRequest.Audiences) > 1 {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "Exactly one audience is required for ID-JAG requests",
		}
	}
	audience := tokenRequest.Audiences[0]
	if !slices.Contains(oauthApp.Token.IDJAG.AllowedAudiences, audience) {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "The requested audience is not permitted for this client",
		}
	}

	// The subject token must be a genuine self-issued ID token. ValidateIDJAGSubjectToken rejects
	// access tokens (typ at+jwt), refresh tokens (access_token_sub claim), and external-issuer subject
	// tokens, so a re-audienced token-exchange access token cannot be laundered into an ID-JAG.
	subjectClaims, err := h.tokenValidator.ValidateIDJAGSubjectToken(ctx, tokenRequest.SubjectToken, oauthApp)
	if err != nil {
		logger.Debug(ctx, "Failed to validate subject token for ID-JAG request", log.Error(err))
		if errors.Is(err, revocation.ErrEnforcementUnavailable) {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Token revocation status could not be verified",
			}
		}
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "subject_token must be an ID token issued to this client",
		}
	}

	// Bind the subject token to the authenticated client: the draft requires the assertion's audience
	// to match the client_id of the client authentication.
	if len(subjectClaims.Aud) == 0 || !slices.Contains(subjectClaims.Aud, tokenRequest.ClientID) {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "subject_token audience does not match the authenticated client",
		}
	}

	grantedScopes := tokenservice.ParseScopes(tokenRequest.Scope)

	idjag, err := h.tokenBuilder.BuildIDJAG(ctx, &tokenservice.IDJAGBuildContext{
		Subject:   subjectClaims.Sub,
		Audience:  audience,
		ClientID:  tokenRequest.ClientID,
		Scopes:    grantedScopes,
		Resources: tokenRequest.Resources,
		OAuthApp:  oauthApp,
	})
	if err != nil {
		logger.Error(ctx, "Failed to generate ID-JAG", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	return &model.TokenResponseDTO{
		AccessToken: *idjag,
	}, nil
}

// getScopes validates and determines the scopes for the new token.
func (h *tokenExchangeGrantHandler) getScopes(
	tokenRequest *model.TokenRequest,
	subjectScopes []string,
) ([]string, *model.ErrorResponse) {
	// If no scopes requested, return subject scopes
	if tokenRequest.Scope == "" {
		return subjectScopes, nil
	}

	requestedScopes := tokenservice.ParseScopes(tokenRequest.Scope)

	if len(requestedScopes) == 0 {
		return []string{}, nil
	}

	// If subject token has no scopes, reject requests asking for scopes
	if len(subjectScopes) == 0 {
		return nil, &model.ErrorResponse{
			Error: constants.ErrorInvalidScope,
			ErrorDescription: "Cannot request scopes when the subject token has no scopes. " +
				"Requested scopes must be a subset of the subject token's scopes.",
		}
	}

	// Filter requested scopes to only include those present in subject token
	subjectScopeSet := make(map[string]bool)
	for _, s := range subjectScopes {
		subjectScopeSet[s] = true
	}

	validRequestedScopes := []string{}
	for _, requestedScope := range requestedScopes {
		if subjectScopeSet[requestedScope] {
			validRequestedScopes = append(validRequestedScopes, requestedScope)
		}
	}

	return validRequestedScopes, nil
}

// mergeAudiences combines opaque audience values with RS-resolved audiences per RFC 8693 §2.1.
// Rules:
//   - Start with explicitAudiences verbatim (preserving order, deduped within itself).
//   - Append rsAudiences items not already present.
//   - If rsAudiences is the clientID-only fallback (len==1 and value==clientID) and
//     explicitAudiences is non-empty, the clientID fallback is dropped — the explicit audience
//     request is sufficient.
//   - If the merged set is empty, fall back to []string{clientID} (or empty if clientID is empty).
func mergeAudiences(explicitAudiences []string, rsAudiences []string, clientID string) []string {
	seen := make(map[string]struct{}, len(explicitAudiences)+len(rsAudiences))
	merged := make([]string, 0, len(explicitAudiences)+len(rsAudiences))

	for _, a := range explicitAudiences {
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		merged = append(merged, a)
	}

	// Drop the clientID-only fallback from rsAudiences when explicit audiences were provided.
	isFallback := len(rsAudiences) == 1 && rsAudiences[0] == clientID
	if isFallback && len(explicitAudiences) > 0 {
		rsAudiences = nil
	}

	for _, a := range rsAudiences {
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		merged = append(merged, a)
	}

	if len(merged) == 0 {
		if clientID != "" {
			return []string{clientID}
		}
		return []string{}
	}
	return merged
}
