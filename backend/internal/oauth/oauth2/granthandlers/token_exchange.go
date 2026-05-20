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

package granthandlers

import (
	"context"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// tokenExchangeGrantHandler handles the token exchange grant type.
type tokenExchangeGrantHandler struct {
	tokenBuilder    tokenservice.TokenBuilderInterface
	tokenValidator  tokenservice.TokenValidatorInterface
	resourceService resource.ResourceServiceInterface
}

// newTokenExchangeGrantHandler creates a new instance of tokenExchangeGrantHandler.
func newTokenExchangeGrantHandler(
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	resourceService resource.ResourceServiceInterface,
) GrantHandlerInterface {
	return &tokenExchangeGrantHandler{
		tokenBuilder:    tokenBuilder,
		tokenValidator:  tokenValidator,
		resourceService: resourceService,
	}
}

// ValidateGrant validates the token exchange grant type request.
func (h *tokenExchangeGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *inboundmodel.OAuthClient) *model.ErrorResponse {
	if constants.GrantType(tokenRequest.GrantType) != constants.GrantTypeTokenExchange {
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
			requestedType != constants.TokenTypeIdentifierJWT {
			return &model.ErrorResponse{
				Error:            constants.ErrorInvalidRequest,
				ErrorDescription: "Unsupported requested_token_type. Only access tokens and JWT tokens are supported",
			}
		}
	}

	return nil
}

// HandleGrant handles the token exchange grant type.
func (h *tokenExchangeGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *inboundmodel.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenExchangeGrantHandler"))

	// Validate and extract subject token claims
	subjectClaims, err := h.tokenValidator.ValidateSubjectToken(ctx, tokenRequest.SubjectToken, oauthApp)
	if err != nil {
		logger.Debug("Failed to validate subject token", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Invalid subject_token",
		}
	}

	// Validate and extract actor token claims if present
	var actorClaims *tokenservice.SubjectTokenClaims
	if tokenRequest.ActorToken != "" {
		actorClaims, err = h.tokenValidator.ValidateSubjectToken(ctx, tokenRequest.ActorToken, oauthApp)
		if err != nil {
			logger.Debug("Failed to validate actor token", log.Error(err))
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
	accessToken, err := h.tokenBuilder.BuildAccessToken(&tokenservice.AccessTokenBuildContext{
		Context:        ctx,
		Subject:        subjectClaims.Sub,
		Audiences:      finalAudiences,
		ClientID:       tokenRequest.ClientID,
		Scopes:         finalScopes,
		UserAttributes: subjectClaims.UserAttributes,
		GrantType:      string(constants.GrantTypeTokenExchange),
		OAuthApp:       oauthApp,
		ActorClaims:    actorClaims,
	})
	if err != nil {
		logger.Error("Failed to generate token", log.Error(err))
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	return &model.TokenResponseDTO{
		AccessToken: *accessToken,
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
