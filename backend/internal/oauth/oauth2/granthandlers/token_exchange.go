// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/attributecache"
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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// tokenExchangeGrantHandler handles the token exchange grant type.
type tokenExchangeGrantHandler struct {
	tokenBuilder         tokenservice.TokenBuilderInterface
	tokenValidator       tokenservice.TokenValidatorInterface
	resourceService      providers.ResourceServerProvider
	attributeCache       attributecache.AttributeCacheServiceInterface
	deviceSecretProvider providers.DeviceSecretProvider
	actorProvider        providers.ActorProvider
	cfg                  oauthconfig.Config
}

// newTokenExchangeGrantHandler creates a new instance of tokenExchangeGrantHandler.
func newTokenExchangeGrantHandler(
	tokenBuilder tokenservice.TokenBuilderInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	resourceService providers.ResourceServerProvider,
	attributeCache attributecache.AttributeCacheServiceInterface,
	deviceSecretProvider providers.DeviceSecretProvider,
	actorProvider providers.ActorProvider,
	cfg oauthconfig.Config,
) GrantHandlerInterface {
	return &tokenExchangeGrantHandler{
		tokenBuilder:         tokenBuilder,
		tokenValidator:       tokenValidator,
		resourceService:      resourceService,
		attributeCache:       attributeCache,
		deviceSecretProvider: deviceSecretProvider,
		actorProvider:        actorProvider,
		cfg:                  cfg,
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
		// Native SSO: the device secret always accompanies the ID token it was minted with.
		if tokenRequest.ActorTokenType == string(constants.TokenTypeIdentifierDeviceSecret) &&
			tokenRequest.SubjectTokenType != string(constants.TokenTypeIdentifierIDToken) {
			return &model.ErrorResponse{
				Error: constants.ErrorInvalidRequest,
				ErrorDescription: "A device secret actor_token must be presented with a subject_token of type " +
					string(constants.TokenTypeIdentifierIDToken),
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

	// Reject any token endpoint call with token_exchange grant that includes device_sso scope (Task 10.b)
	requestedScopes := tokenservice.ParseScopes(tokenRequest.Scope)
	if slices.Contains(requestedScopes, constants.ScopeDeviceSSO) {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidScope,
			ErrorDescription: "device_sso scope is valid only on the token endpoint with authorization_code grant",
		}
	}

	if tokenRequest.ActorTokenType == string(constants.TokenTypeIdentifierDeviceSecret) {
		if errResp := h.validateNativeSSOAudience(tokenRequest); errResp != nil {
			return errResp
		}
	}

	// Native SSO issues tokens for an OpenID Connect sign-in: a child app that names its scopes must
	// ask for openid among them.
	if tokenRequest.ActorTokenType == string(constants.TokenTypeIdentifierDeviceSecret) &&
		tokenRequest.Scope != "" && !slices.Contains(requestedScopes, constants.ScopeOpenID) {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidScope,
			ErrorDescription: "A Native SSO token exchange that requests scopes must include the openid scope",
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
	var subjectClaims *tokenservice.SubjectTokenClaims
	var err error
	if tokenRequest.ActorTokenType == string(constants.TokenTypeIdentifierDeviceSecret) {
		// Native SSO: the ID token may have expired; the device secret bounds the exchange.
		subjectClaims, err = h.tokenValidator.ValidateNativeSSOSubjectToken(ctx, tokenRequest.SubjectToken, oauthApp)
	} else {
		subjectClaims, err = h.tokenValidator.ValidateSubjectToken(ctx, tokenRequest.SubjectToken, oauthApp)
	}
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
	var deviceSecretClaims *providers.DeviceSecretClaims
	if tokenRequest.ActorToken != "" {
		if tokenRequest.ActorTokenType == string(constants.TokenTypeIdentifierDeviceSecret) {
			var dsErr *model.ErrorResponse
			deviceSecretClaims, dsErr = h.validateDeviceSecretActor(ctx, tokenRequest, oauthApp, subjectClaims)
			if dsErr != nil {
				return nil, dsErr
			}
			// The originating application is the actor on whose behalf the child app acts. The device
			// secret names it by client_id; act.sub carries the application's resource ID, which is what
			// the token builder resolves actors by.
			originApp, svcErr := h.actorProvider.GetOAuthClientByClientID(ctx, deviceSecretClaims.ClientID)
			if svcErr != nil || originApp == nil {
				logger.Debug(ctx, "Originating application of the device secret could not be resolved")
				return nil, &model.ErrorResponse{
					Error:            constants.ErrorInvalidGrant,
					ErrorDescription: "The device secret was not issued to a registered client",
				}
			}
			actorClaims = &tokenservice.SubjectTokenClaims{
				Sub:    originApp.ID,
				CnfJkt: deviceSecretClaims.CnfJKT,
			}
		} else {
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
	}

	// Determine final scopes. On a Native SSO exchange the device secret records what the
	// originating app was granted, and the child app may not exceed it.
	grantedScopes := subjectClaims.Scopes
	if deviceSecretClaims != nil && deviceSecretClaims.Scope != "" {
		grantedScopes = tokenservice.ParseScopes(deviceSecretClaims.Scope)
	}
	finalScopes, errResp := h.getScopes(tokenRequest, grantedScopes)
	if errResp != nil {
		return nil, errResp
	}
	// device_sso is never passed on to the child app (Task 10.b): an exchange that omits scope
	// inherits the device secret's scopes, which include it.
	if deviceSecretClaims != nil {
		finalScopes = slices.DeleteFunc(slices.Clone(finalScopes), func(s string) bool {
			return s == constants.ScopeDeviceSSO
		})
		// The child application only ever receives scopes it is registered for: an explicit request
		// for anything else is refused, and an omitted scope inherits the registered subset only.
		if len(oauthApp.Scopes) > 0 {
			if tokenRequest.Scope == "" {
				finalScopes = slices.DeleteFunc(finalScopes, func(s string) bool {
					return !slices.Contains(oauthApp.Scopes, s)
				})
			} else {
				for _, s := range finalScopes {
					if !slices.Contains(oauthApp.Scopes, s) {
						return nil, &model.ErrorResponse{
							Error:            constants.ErrorInvalidScope,
							ErrorDescription: fmt.Sprintf("Scope '%s' is not registered for this client", s),
						}
					}
				}
			}
		}
	}

	// Native SSO never captures consent in the child app: the exchange succeeds only when the
	// consent the user gave the originating app covers what the child app asked for.
	if deviceSecretClaims != nil {
		if errResp := h.resolveDerivedConsent(ctx, tokenRequest, subjectClaims, deviceSecretClaims,
			grantedScopes, finalScopes); errResp != nil {
			return nil, errResp
		}
	}

	// Retain OIDC scopes (governed by the app's scope-to-claims mapping); only permission scopes
	// are downscoped to the target resource server.
	oidcScopes, permissionScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
		tokenservice.JoinScopes(finalScopes), oauthApp.ScopeClaims)

	// Bind the token to a single target resource server (RFC 8707 resource or configured default).
	// The RFC 8693 audience parameter does not select the token's audience (a Native SSO exchange
	// checks it names this issuer, see validateNativeSSOAudience). A request that resolves no permission scopes
	// and carries no resource is not bound to a resource server: its audience is the app's configured
	// default audiences, falling back to the client_id.
	targetRS, resErr := resourceindicators.ResolveAudienceBinding(
		ctx, h.resourceService, tokenRequest.Resources, permissionScopes)
	if resErr != nil {
		return nil, resErr
	}

	var finalAudiences []string
	if targetRS == nil {
		finalAudiences = []string{oauthApp.ResolveDefaultAudience(tokenRequest.ClientID)}
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
	// A Native SSO exchange carries the user's attributes in the device secret rather than in an
	// attribute cache the subject token points at; materialise them so userinfo can serve the child.
	var attributeCacheID string
	if deviceSecretClaims != nil {
		attributeCacheID, errResp = h.cacheDeviceSecretAttributes(ctx, oauthApp, deviceSecretClaims, finalScopes)
		if errResp != nil {
			return nil, errResp
		}
	}

	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, &tokenservice.AccessTokenBuildContext{
		Subject:           subjectClaims.Sub,
		Audiences:         finalAudiences,
		ClientID:          tokenRequest.ClientID,
		Scopes:            finalScopes,
		SubjectAttributes: tokenservice.FilterAttributesByAllowList(subjectClaims.UserAttributes, userSubConfig),
		AttributeCacheID:  attributeCacheID,
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

// validateNativeSSOAudience enforces OpenID Connect Native SSO for Mobile Apps 1.0 §4.1: the audience
// parameter of a device-secret token exchange is required and must be this authorization server's
// issuer URL. The parameter names the server the exchange is addressed to; the issued access token's
// audience is still resolved from the resource parameter or the client's defaults.
func (h *tokenExchangeGrantHandler) validateNativeSSOAudience(tokenRequest *model.TokenRequest) *model.ErrorResponse {
	if len(tokenRequest.Audiences) == 0 {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "The audience parameter is required for a Native SSO token exchange",
		}
	}
	issuer := h.cfg.JWT.Issuer
	if len(tokenRequest.Audiences) != 1 || issuer == "" || tokenRequest.Audiences[0] != issuer {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidTarget,
			ErrorDescription: "The audience of a Native SSO token exchange must be the issuer URL",
		}
	}
	return nil
}

// validateDeviceSecretActor validates a Native SSO device secret presented as the actor_token and
// its binding to the presented ID token: the client must be enabled for device_sso, the ID token
// must carry ds_hash = BASE64URL(SHA-256(device_secret)) and the same sid as the device secret,
// the device secret's cnf.jkt must match the DPoP proof key, and both tokens must name one subject.
func (h *tokenExchangeGrantHandler) validateDeviceSecretActor(ctx context.Context,
	tokenRequest *model.TokenRequest, oauthApp *providers.OAuthClient,
	subjectClaims *tokenservice.SubjectTokenClaims) (*providers.DeviceSecretClaims, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenExchangeGrantHandler"))

	if h.deviceSecretProvider == nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Device secret provider is not configured",
		}
	}
	// A client redeems a device secret only when it is itself registered for Native SSO.
	if !slices.Contains(oauthApp.Scopes, constants.ScopeDeviceSSO) {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorUnauthorizedClient,
			ErrorDescription: "The device_sso scope is not enabled for this client",
		}
	}

	dsClaims, svcErr := h.deviceSecretProvider.ValidateDeviceSecret(ctx, tokenRequest.ClientID, tokenRequest.ActorToken)
	if svcErr != nil {
		desc := svcErr.ErrorDescription.String()
		if desc == "" {
			desc = svcErr.Error.String()
		}
		logger.Debug(ctx, "Failed to validate device secret", log.Error(errors.New(desc)))
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to validate device secret",
			}
		}
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: desc,
		}
	}
	if dsClaims == nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to validate device secret",
		}
	}

	// ds_hash binds the ID token to exactly this device secret.
	if subjectClaims.DSHash == "" {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "subject_token does not carry a ds_hash binding to a device secret",
		}
	}
	if subjectClaims.DSHash != deviceSecretHash(tokenRequest.ActorToken) {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "ds_hash in subject_token does not match actor_token",
		}
	}

	// Both tokens descend from the same authentication session.
	if subjectClaims.SID == "" || dsClaims.SessionID == "" || subjectClaims.SID != dsClaims.SessionID {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "sid in subject_token does not match the device secret",
		}
	}

	// The child app must prove possession of the key the device secret was bound to: the DPoP proof
	// on this request is signed by the originating app's key, whose thumbprint is cnf.jkt. A device
	// secret without that binding would be a bearer credential and is not accepted.
	if dsClaims.CnfJKT == "" {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "The device secret is not bound to a DPoP key (cnf.jkt)",
		}
	}
	currentDPoPJkt := dpop.GetJkt(ctx)
	if currentDPoPJkt == "" {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidDPoPProof,
			ErrorDescription: "A DPoP proof signed by the key bound to the device secret is required",
		}
	}
	if dsClaims.CnfJKT != currentDPoPJkt {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "DPoP key thumbprint does not match cnf.jkt in device secret",
		}
	}

	if dsClaims.Sub == "" || subjectClaims.Sub == "" || dsClaims.Sub != subjectClaims.Sub {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Subject in device secret does not match subject_token",
		}
	}

	return dsClaims, nil
}

// resolveDerivedConsent asks the DeviceSecretProvider, when it implements
// DeviceSSOConsentProvider, whether the consent captured by the originating app covers the child
// app's exchange. A refusal is answered with interaction_required so the child app can fall back
// to an interactive authorization and capture consent of its own.
func (h *tokenExchangeGrantHandler) resolveDerivedConsent(ctx context.Context,
	tokenRequest *model.TokenRequest, subjectClaims *tokenservice.SubjectTokenClaims,
	dsClaims *providers.DeviceSecretClaims, grantedScopes, requestedScopes []string) *model.ErrorResponse {
	consentProvider, ok := h.deviceSecretProvider.(providers.DeviceSSOConsentProvider)
	if !ok {
		return nil
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenExchangeGrantHandler"))

	result, svcErr := consentProvider.ResolveDerivedConsent(ctx, &providers.DerivedConsentRequest{
		Sub:             subjectClaims.Sub,
		ClientID:        tokenRequest.ClientID,
		OriginClientID:  dsClaims.ClientID,
		RequestedScopes: requestedScopes,
		GrantedScopes:   grantedScopes,
		AuthID:          dsClaims.AuthID,
		SessionID:       dsClaims.SessionID,
	})
	if svcErr != nil {
		desc := svcErr.ErrorDescription.String()
		if desc == "" {
			desc = svcErr.Error.String()
		}
		logger.Error(ctx, "Failed to resolve derived consent", log.Error(errors.New(desc)))
		return &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to resolve consent for the token exchange",
		}
	}
	// Consent is reused only on an affirmative decision: a provider that returns no result has not
	// decided, and the silent path must not widen what the user agreed to.
	if result == nil {
		return &model.ErrorResponse{
			Error:            constants.ErrorInteractionRequired,
			ErrorDescription: "The consent provider did not return an allowed decision",
		}
	}
	if result.Allowed {
		return nil
	}

	desc := "The consent granted to the originating application does not cover this request"
	if len(result.MissingScopes) > 0 {
		desc += "; scopes not consented: " + strings.Join(result.MissingScopes, " ")
	}
	if len(result.MissingClaims) > 0 {
		desc += "; claims not consented: " + strings.Join(result.MissingClaims, " ")
	}
	return &model.ErrorResponse{
		Error:            constants.ErrorInteractionRequired,
		ErrorDescription: desc,
	}
}

// cacheDeviceSecretAttributes stores the claim snapshot the device secret carries as an attribute
// cache entry for the exchanged access token, keeping only the claims the final scopes release to
// the child app. The entry lives as long as the access token (plus the usual buffer).
func (h *tokenExchangeGrantHandler) cacheDeviceSecretAttributes(ctx context.Context,
	oauthApp *providers.OAuthClient, dsClaims *providers.DeviceSecretClaims, scopes []string,
) (string, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "TokenExchangeGrantHandler"))
	if h.attributeCache == nil || len(dsClaims.Claims) == 0 {
		logger.Debug(ctx, "No attributes to cache for the exchanged token",
			log.Bool("cacheConfigured", h.attributeCache != nil), log.Int("claimCount", len(dsClaims.Claims)))
		return "", nil
	}
	allowed := make(map[string]bool)
	for _, scope := range scopes {
		for _, claim := range oauth2utils.ResolveScopeClaims(scope, oauthApp.ScopeClaims) {
			allowed[claim] = true
		}
	}
	attrs := make(map[string]interface{}, len(allowed))
	for name, value := range dsClaims.Claims {
		if allowed[name] {
			attrs[name] = value
		}
	}
	if len(attrs) == 0 {
		logger.Debug(ctx, "None of the device secret claims are released by the exchanged scopes",
			log.Int("claimCount", len(dsClaims.Claims)), log.Int("scopeCount", len(scopes)))
		return "", nil
	}
	accessValidity := tokenservice.ResolveTokenConfig(h.cfg, oauthApp, tokenservice.TokenTypeAccess,
		oauthApp.UserAccessTokenConfig().ValidityPeriodOrZero()).ValidityPeriod
	ttl := accessValidity + constants.AttributeCacheTTLBufferSeconds
	entry, svcErr := h.attributeCache.CreateAttributeCache(ctx, &attributecache.AttributeCache{
		Attributes: attrs,
		TTLSeconds: ttl,
	})
	if svcErr != nil {
		logger.Error(ctx, "Failed to cache device secret attributes",
			log.Error(errors.New(svcErr.ErrorDescription.DefaultValue)))
		return "", &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to prepare user attributes for the exchanged token",
		}
	}
	return entry.ID, nil
}

// deviceSecretHash computes ds_hash = BASE64URL(SHA-256(device_secret)).
func deviceSecretHash(deviceSecret string) string {
	sum := sha256.Sum256([]byte(deviceSecret))
	return base64.RawURLEncoding.EncodeToString(sum[:])
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
		if !subjectScopeSet[requestedScope] {
			// Task 10.a: the scope requested by the second app must be a subset of the scope originally
			// granted.
			if tokenRequest.ActorTokenType == string(constants.TokenTypeIdentifierDeviceSecret) {
				return nil, &model.ErrorResponse{
					Error: constants.ErrorInvalidScope,
					ErrorDescription: fmt.Sprintf(
						"Requested scope '%s' exceeds scopes originally granted in subject token", requestedScope),
				}
			}
			continue
		}
		validRequestedScopes = append(validRequestedScopes, requestedScope)
	}

	return validRequestedScopes, nil
}
