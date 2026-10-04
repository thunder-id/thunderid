// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"errors"
	"slices"
	"strings"

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
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// tokenExchangeGrantHandler handles the token exchange grant type.
type tokenExchangeGrantHandler struct {
	tokenBuilder    tokenservice.TokenBuilderInterface
	tokenValidator  tokenservice.TokenValidatorInterface
	resourceService providers.ResourceServerProvider
	cfg             oauthconfig.Config
}

// newTokenExchangeGrantHandler creates a new instance of tokenExchangeGrantHandler.
func newTokenExchangeGrantHandler(
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	resourceService providers.ResourceServerProvider,
	cfg oauthconfig.Config,
) GrantHandlerInterface {
	return &tokenExchangeGrantHandler{
		tokenBuilder:    tokenBuilder,
		tokenValidator:  tokenValidator,
		resourceService: resourceService,
		cfg:             cfg,
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
		switch {
		case errors.Is(err, revocation.ErrEnforcementUnavailable):
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Token revocation status could not be verified",
			}
		case errors.Is(err, tokenservice.ErrTokenExpired):
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidRequest,
				ErrorDescription: "The subject_token has expired",
			}
		case errors.Is(err, tokenservice.ErrIssuerNotTrusted):
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidRequest,
				ErrorDescription: "The subject_token issuer is not registered as a trusted token exchange issuer",
			}
		case errors.Is(err, tokenservice.ErrAudienceNotAccepted):
			return nil, &model.ErrorResponse{
				Error: constants.ErrorInvalidRequest,
				ErrorDescription: "The subject_token audience does not contain this server's issuer or the " +
					"trusted token audience configured for its issuer",
			}
		default:
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidRequest,
				ErrorDescription: "Invalid subject_token",
			}
		}
	}

	// Enforce RFC 9068: a token presented as subject_token_type=access_token must carry the at+jwt typ
	// header.
	if errResp := h.validateAccessTokenType(tokenRequest.SubjectToken,
		tokenRequest.SubjectTokenType, "subject_token"); errResp != nil {
		return nil, errResp
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
			// Attribute the actor_token rejection the same way as the subject_token above.
			switch {
			case errors.Is(err, revocation.ErrEnforcementUnavailable):
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorServerError,
					ErrorDescription: "Token revocation status could not be verified",
				}
			case errors.Is(err, tokenservice.ErrTokenExpired):
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorInvalidRequest,
					ErrorDescription: "The actor_token has expired",
				}
			case errors.Is(err, tokenservice.ErrIssuerNotTrusted):
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorInvalidRequest,
					ErrorDescription: "The actor_token issuer is not registered as a trusted token exchange issuer",
				}
			case errors.Is(err, tokenservice.ErrAudienceNotAccepted):
				return nil, &model.ErrorResponse{
					Error: constants.ErrorInvalidRequest,
					ErrorDescription: "The actor_token audience does not contain this server's issuer or the " +
						"trusted token audience configured for its issuer",
				}
			default:
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorInvalidRequest,
					ErrorDescription: "Invalid actor_token",
				}
			}
		}

		// Apply the same RFC 9068 access-token typ enforcement to the actor_token.
		if errResp := h.validateAccessTokenType(tokenRequest.ActorToken,
			tokenRequest.ActorTokenType, "actor_token"); errResp != nil {
			return nil, errResp
		}
	}

	// Determine final scopes
	finalScopes, errResp := h.getScopes(tokenRequest, subjectClaims.Scopes)
	if errResp != nil {
		return nil, errResp
	}

	// Retain OIDC scopes (governed by the app's scope-to-claims mapping); only permission scopes
	// are downscoped to the target resource server.
	oidcScopes, permissionScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
		tokenservice.JoinScopes(finalScopes), oauthApp.ScopeClaims)

	// Bind the token to a single target resource server (RFC 8707 resource or configured default).
	// The RFC 8693 audience parameter is not honored. A request that resolves no permission scopes
	// and carries no resource is not bound to a resource server: its audience is the app's configured
	// default audiences, falling back to the server issuer ID.
	targetRS, resErr := resourceindicators.ResolveAudienceBinding(
		ctx, h.resourceService, tokenRequest.Resources, permissionScopes)
	if resErr != nil {
		return nil, resErr
	}

	var finalAudiences []string
	if targetRS == nil {
		finalAudiences = []string{oauthApp.ResolveDefaultAudience(h.cfg.JWT.Issuer)}
		finalScopes = oidcScopes
	} else {
		permissionScopes, resErr = resourceindicators.DownscopeToResourceServer(
			ctx, h.resourceService, targetRS.ID, permissionScopes)
		if resErr != nil {
			return nil, resErr
		}

		finalScopes = make([]string, 0, len(oidcScopes)+len(permissionScopes))
		finalScopes = append(finalScopes, oidcScopes...)
		finalScopes = append(finalScopes, permissionScopes...)

		finalAudiences = []string{targetRS.Identifier}
	}

	// Resolve how the exchanged token relates to the subject token's revocation family: inherit joins
	// the subject's family (both revoked together); none (the default, and the empty value) issues an
	// independent token with no tfid. An unrecognized value is treated as none and surfaced.
	var exchangedTokenFamilyID string
	switch h.cfg.OAuth.TokenExchange.TokenFamily {
	case constants.TokenExchangeTokenFamilyInherit:
		exchangedTokenFamilyID = subjectClaims.TokenFamilyID
	case constants.TokenExchangeTokenFamilyNone, "":
	default:
		logger.Warn(ctx, "Unrecognized oauth.token_exchange.token_family mode; issuing an independent token",
			log.String("mode", h.cfg.OAuth.TokenExchange.TokenFamily))
	}

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
		TokenFamilyID:     exchangedTokenFamilyID,
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

// validateAccessTokenType enforces the RFC 9068 typ header for a token presented as an access token.
// When the declared token type is urn:ietf:params:oauth:token-type:access_token, the token's typ
// header must be at+jwt.
func (h *tokenExchangeGrantHandler) validateAccessTokenType(
	token string,
	tokenType string,
	tokenLabel string,
) *model.ErrorResponse {
	if constants.TokenTypeIdentifier(tokenType) != constants.TokenTypeIdentifierAccessToken {
		return nil
	}

	header, err := jwt.DecodeJWTHeader(token)
	if err != nil {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Invalid " + tokenLabel,
		}
	}

	if typ, _ := header["typ"].(string); !strings.EqualFold(typ, jwt.TokenTypeAccessToken) &&
		!strings.EqualFold(typ, jwt.TokenTypeAccessTokenWithPrefix) {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "The " + tokenLabel + " is not an access token (missing at+jwt typ header)",
		}
	}

	return nil
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
