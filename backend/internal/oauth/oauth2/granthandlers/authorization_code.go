// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/pkce"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/resourceindicators"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// defaultDeviceSecretValidity bounds how long a Native SSO device secret may be exchanged. The
// DeviceSecretProvider may shorten it; the value only sets the expiry the engine asks for.
const defaultDeviceSecretValidity = 30 * 24 * time.Hour

// authorizationCodeGrantHandler handles the authorization code grant type.
type authorizationCodeGrantHandler struct {
	authzService         authz.AuthorizeServiceInterface
	tokenBuilder         tokenservice.TokenBuilderInterface
	attributeCache       attributecache.AttributeCacheServiceInterface
	resourceService      providers.ResourceServerProvider
	deviceSecretProvider providers.DeviceSecretProvider
}

// newAuthorizationCodeGrantHandler creates a new instance of AuthorizationCodeGrantHandler.
func newAuthorizationCodeGrantHandler(
	authzService authz.AuthorizeServiceInterface,
	tokenBuilder tokenservice.TokenBuilderInterface,
	attributeCache attributecache.AttributeCacheServiceInterface,
	resourceService providers.ResourceServerProvider,
	deviceSecretProvider providers.DeviceSecretProvider,
) GrantHandlerInterface {
	return &authorizationCodeGrantHandler{
		authzService:         authzService,
		tokenBuilder:         tokenBuilder,
		attributeCache:       attributeCache,
		resourceService:      resourceService,
		deviceSecretProvider: deviceSecretProvider,
	}
}

// ValidateGrant validates the authorization code grant request.
func (h *authorizationCodeGrantHandler) ValidateGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) *model.ErrorResponse {
	if tokenRequest.GrantType == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Missing grant_type parameter",
		}
	}
	if providers.GrantType(tokenRequest.GrantType) != providers.GrantTypeAuthorizationCode {
		return &model.ErrorResponse{
			Error:            constants.ErrorUnsupportedGrantType,
			ErrorDescription: "Unsupported grant type",
		}
	}
	if tokenRequest.Code == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "Authorization code is required",
		}
	}
	if tokenRequest.ClientID == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidClient,
			ErrorDescription: "client_id is required",
		}
	}

	if errResp := resourceindicators.ValidateResourceURIs(tokenRequest.Resources); errResp != nil {
		return errResp
	}

	return nil
}

// HandleGrant processes the authorization code grant request and generates a token response.
func (h *authorizationCodeGrantHandler) HandleGrant(ctx context.Context, tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient) (
	*model.TokenResponseDTO, *model.ErrorResponse) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizationCodeGrantHandler"))

	// Retrieve and validate authorization code
	authCode, errResponse := h.retrieveAndValidateAuthCode(ctx, tokenRequest, oauthApp, logger)
	if errResponse != nil {
		return nil, errResponse
	}

	if errResp := dpop.VerifyProofBinding(ctx, authCode.DPoPJkt, "authorization code"); errResp != nil {
		return nil, errResp
	}

	// Parse authorized scopes
	authorizedScopes := tokenservice.ParseScopes(authCode.Scopes)

	// Get user attributes from attribute cache
	attrs := make(map[string]interface{})
	if authCode.AttributeCacheID != "" {
		userAttributes, err := h.attributeCache.GetAttributeCache(ctx, authCode.AttributeCacheID)
		if err != nil {
			logger.Error(ctx,
				"Failed to get user attributes from attribute cache. "+err.ErrorDescription.DefaultValue)
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to get user attributes from attribute cache",
			}
		}
		attrs = userAttributes.Attributes
	}

	// Bind the token to a single target resource server. Prefer the token request's resource
	// (validated as a subset of the code's resources); otherwise use the resource recorded on the
	// authorization code; otherwise fall back to the configured default resource server.
	effectiveResources := tokenRequest.Resources
	if len(effectiveResources) == 0 {
		effectiveResources = authCode.Resources
	}

	// Retain OIDC scopes unchanged; downscope non-OIDC scopes to permissions defined on the target RS.
	oidcScopes, nonOidcScopes := oauth2utils.SeparateOIDCAndNonOIDCScopes(
		strings.Join(authorizedScopes, " "), oauthApp.ScopeClaims)
	targetRS, errResp := resourceindicators.ResolveAudienceBinding(
		ctx, h.resourceService, effectiveResources, nonOidcScopes)
	if errResp != nil {
		return nil, errResp
	}

	var accessTokenAudiences, accessTokenScopes []string
	if targetRS == nil {
		// OIDC-only (or scopeless) request with no resource: the token is not bound to a resource
		// server, so its audience is the app's configured default audiences (falling back to the
		// client_id) and it carries only the OIDC scopes.
		accessTokenAudiences = []string{oauthApp.ResolveDefaultAudience(tokenRequest.ClientID)}
		accessTokenScopes = oidcScopes
	} else {
		downscopedNonOidc, dErr := resourceindicators.DownscopeToResourceServer(
			ctx, h.resourceService, targetRS.ID, nonOidcScopes)
		if dErr != nil {
			return nil, dErr
		}
		accessTokenAudiences = []string{targetRS.Identifier}
		accessTokenScopes = make([]string, 0, len(oidcScopes)+len(downscopedNonOidc))
		accessTokenScopes = append(accessTokenScopes, oidcScopes...)
		accessTokenScopes = append(accessTokenScopes, downscopedNonOidc...)
	}

	// Native SSO preconditions are checked before any token is minted.
	deviceSSO := slices.Contains(accessTokenScopes, constants.ScopeDeviceSSO)
	if deviceSSO {
		if errResp := h.validateDeviceSSOGrant(ctx, oauthApp, accessTokenScopes); errResp != nil {
			return nil, errResp
		}
	}

	// Generate access token using tokenBuilder (attributes will be filtered in BuildAccessToken)
	userSubConfig := oauthApp.UserAccessTokenConfig()
	accessTokenCtx := &tokenservice.AccessTokenBuildContext{
		Subject:           authCode.AuthorizedUserID,
		Audiences:         accessTokenAudiences,
		ClientID:          tokenRequest.ClientID,
		Scopes:            accessTokenScopes,
		SubjectAttributes: tokenservice.FilterAttributesByAllowList(attrs, userSubConfig),
		AttributeCacheID:  authCode.AttributeCacheID,
		GrantType:         string(providers.GrantTypeAuthorizationCode),
		OAuthApp:          oauthApp,
		ClaimsRequest:     authCode.ClaimsRequest,
		ClaimsLocales:     authCode.ClaimsLocales,
		ValidityPeriod:    userSubConfig.ValidityPeriodOrZero(),
		DPoPJkt:           dpop.GetJkt(ctx),
		TokenFamilyID:     authCode.TokenFamilyID,
	}
	if oauthApp.ShouldAppendActorClaim() {
		accessTokenCtx.ActorClaims = &tokenservice.SubjectTokenClaims{Sub: oauthApp.ID}
	}
	accessToken, err := h.tokenBuilder.BuildAccessToken(ctx, accessTokenCtx)
	if err != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		}
	}

	// The refresh token preserves this single audience for continuity.
	accessToken.OriginalAudiences = accessTokenAudiences

	// Build token response
	tokenResponse := &model.TokenResponseDTO{
		AccessToken: *accessToken,
	}

	// Native SSO (OpenID Connect Native SSO for Mobile Apps 1.0): a grant that carries the
	// device_sso scope also mints a device secret bound to the ID token through ds_hash and sid.
	var dsHash string
	// This engine build has no SSO session store, so the authorization id serves as the sid.
	sessionID := ""
	if deviceSSO {
		// The device secret and the ID token share a session id. Deployments without an SSO session
		// store (such as an embedded engine) have no sid of their own, so the authorization id
		// identifies the authentication instead.
		if sessionID == "" {
			sessionID = authCode.CodeID
		}
		dsRes, dsErr := h.deviceSecretProvider.GenerateDeviceSecret(ctx, &providers.DeviceSecretParams{
			Sub:            authCode.AuthorizedUserID,
			AuthID:         authCode.CodeID,
			ClientID:       tokenRequest.ClientID,
			Scope:          strings.Join(accessTokenScopes, " "),
			ACR:            authCode.CompletedACR,
			Claims:         attrs,
			CnfJKT:         dpop.GetJkt(ctx),
			ExpiryDuration: defaultDeviceSecretValidity,
			SessionID:      sessionID,
		})
		if dsErr != nil {
			logger.Error(ctx, "Failed to generate device secret",
				log.Error(errors.New(dsErr.ErrorDescription.String())))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate device secret",
			}
		}
		if dsRes == nil || dsRes.DeviceSecret == "" || dsRes.DSHash == "" {
			logger.Error(ctx, "Device secret provider returned an empty device secret")
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate device secret",
			}
		}
		// The token exchange verifies ds_hash as BASE64URL(SHA-256(device_secret)) (Native SSO 1.0
		// leaves the binding to the deployment, so the engine owns it on both sides). A provider that
		// returns a different value would issue ID tokens no exchange could ever redeem.
		dsHash = deviceSecretHash(dsRes.DeviceSecret)
		if dsRes.DSHash != "" && dsRes.DSHash != dsHash {
			logger.Error(ctx, "Device secret provider returned a ds_hash that does not match the engine's derivation")
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate device secret",
			}
		}
		tokenResponse.DeviceSecret = dsRes.DeviceSecret
	}

	// Generate ID token if 'openid' scope is present
	if slices.Contains(accessTokenScopes, constants.ScopeOpenID) {
		idToken, err := h.tokenBuilder.BuildIDToken(ctx, &tokenservice.IDTokenBuildContext{
			Subject:        authCode.AuthorizedUserID,
			Audience:       tokenRequest.ClientID,
			Scopes:         accessTokenScopes,
			UserAttributes: attrs,
			AuthTime:       authCode.TimeCreated.Unix(),
			OAuthApp:       oauthApp,
			ClaimsRequest:  authCode.ClaimsRequest,
			Nonce:          authCode.Nonce,
			CompletedACR:   authCode.CompletedACR,
			SessionID:      sessionID,
			DSHash:         dsHash,
		})
		if err != nil {
			logger.Error(ctx, "Failed to generate ID token", log.Error(err))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorServerError,
				ErrorDescription: "Failed to generate token",
			}
		}
		tokenResponse.IDToken = *idToken
	}

	return tokenResponse, nil
}

// validateDeviceSSOGrant checks that a grant carrying the device_sso scope may mint a device secret:
// the deployment supports Native SSO, the client is registered for it, the grant also issues an ID
// token (the device secret and the ID token are issued together or not at all), and the token
// request is DPoP-bound so the device secret can be bound to the requesting app's key (cnf.jkt).
func (h *authorizationCodeGrantHandler) validateDeviceSSOGrant(ctx context.Context,
	oauthApp *providers.OAuthClient, scopes []string) *model.ErrorResponse {
	if h.deviceSecretProvider == nil {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidScope,
			ErrorDescription: "The device_sso scope is not supported by this deployment",
		}
	}
	// The token endpoint re-checks that the registered client is enabled for Native SSO, so a
	// client cannot obtain a device secret merely by asking for the scope at authorization.
	if !slices.Contains(oauthApp.Scopes, constants.ScopeDeviceSSO) {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidScope,
			ErrorDescription: "The device_sso scope is not enabled for this client",
		}
	}
	// Native SSO for Mobile Apps 1.0 §3.4: the device secret is returned together with an ID token,
	// which carries the ds_hash and sid that bind the two.
	if !slices.Contains(scopes, constants.ScopeOpenID) {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidScope,
			ErrorDescription: "The device_sso scope requires the openid scope",
		}
	}
	// The device secret is bound to the requesting app's DPoP key (cnf.jkt); without a proof it
	// would be a long-lived bearer credential.
	if dpop.GetJkt(ctx) == "" {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidRequest,
			ErrorDescription: "device_sso requires a DPoP-bound token request",
		}
	}
	return nil
}

func (h *authorizationCodeGrantHandler) retrieveAndValidateAuthCode(
	ctx context.Context,
	tokenRequest *model.TokenRequest,
	oauthApp *providers.OAuthClient,
	logger *log.Logger,
) (*authz.AuthorizationCode, *model.ErrorResponse) {
	authCode, codeErr := h.authzService.GetAuthorizationCodeDetails(ctx, tokenRequest.ClientID, tokenRequest.Code)
	if codeErr != nil {
		return nil, &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid authorization code",
		}
	}

	// Validate the retrieved authorization code
	errResponse := validateAuthorizationCode(tokenRequest, *authCode)
	if errResponse != nil && errResponse.Error != "" {
		return nil, errResponse
	}

	// Validate PKCE if required or if code challenge was provided during authorization
	if oauthApp.RequiresPKCE() || authCode.CodeChallenge != "" {
		if tokenRequest.CodeVerifier == "" {
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidGrant,
				ErrorDescription: "code_verifier is required",
			}
		}

		// Validate PKCE
		if err := pkce.ValidatePKCE(authCode.CodeChallenge, authCode.CodeChallengeMethod,
			tokenRequest.CodeVerifier); err != nil {
			logger.Debug(ctx, "PKCE validation failed", log.Error(err))
			return nil, &model.ErrorResponse{
				Error:            constants.ErrorInvalidGrant,
				ErrorDescription: "Invalid code verifier",
			}
		}
	}
	return authCode, nil
}

// validateAuthorizationCode validates the authorization code against the token request.
func validateAuthorizationCode(tokenRequest *model.TokenRequest,
	code authz.AuthorizationCode) *model.ErrorResponse {
	if tokenRequest.ClientID != code.ClientID {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid authorization code",
		}
	}

	// RFC 6749 §4.1.3: required only if included in the authorize request.
	if code.RedirectURIProvided && tokenRequest.RedirectURI != code.RedirectURI {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Invalid redirect URI",
		}
	}

	// If the authorization request included resources, the token request may either omit
	// resources entirely or supply a subset of those previously authorized.
	if len(code.Resources) > 0 && len(tokenRequest.Resources) > 0 {
		for _, r := range tokenRequest.Resources {
			if !slices.Contains(code.Resources, r) {
				return &model.ErrorResponse{
					Error:            constants.ErrorInvalidTarget,
					ErrorDescription: "Resource parameter mismatch",
				}
			}
		}
	}

	if code.ExpiryTime.Before(time.Now()) {
		return &model.ErrorResponse{
			Error:            constants.ErrorInvalidGrant,
			ErrorDescription: "Expired authorization code",
		}
	}

	return nil
}
