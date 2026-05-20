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

// Package google implements an authentication service for authenticating via Google OIDC.
package google

import (
	"context"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	authnoidc "github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	loggerComponentName = "GoogleOIDCAuthnService"
)

// GoogleOIDCAuthnServiceInterface defines the contract for Google OIDC based authenticator services.
type GoogleOIDCAuthnServiceInterface interface {
	authnoidc.OIDCAuthnCoreServiceInterface
}

// googleOIDCAuthnService is the default implementation of GoogleOIDCAuthnServiceInterface.
type googleOIDCAuthnService struct {
	internal   authnoidc.OIDCAuthnServiceInterface
	jwtService jwt.JWTServiceInterface
	logger     *log.Logger
}

// newGoogleOIDCAuthnService creates a new instance of Google OIDC authenticator service.
func newGoogleOIDCAuthnService(internal authnoidc.OIDCAuthnServiceInterface,
	jwtSvc jwt.JWTServiceInterface) GoogleOIDCAuthnServiceInterface {
	return &googleOIDCAuthnService{
		internal:   internal,
		jwtService: jwtSvc,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// BuildAuthorizeURL constructs the authorization request URL for Google OIDC authentication.
func (g *googleOIDCAuthnService) BuildAuthorizeURL(
	ctx context.Context, idpID string) (string, *serviceerror.ServiceError) {
	return g.internal.BuildAuthorizeURL(ctx, idpID)
}

// ExchangeCodeForToken exchanges the authorization code for a token with Google
// and validates the token response if validateResponse is true.
func (g *googleOIDCAuthnService) ExchangeCodeForToken(ctx context.Context, idpID, code string, validateResponse bool) (
	*authnoauth.TokenResponse, *serviceerror.ServiceError) {
	tokenResp, svcErr := g.internal.ExchangeCodeForToken(ctx, idpID, code, false)
	if svcErr != nil {
		return nil, svcErr
	}

	if validateResponse {
		svcErr = g.ValidateTokenResponse(ctx, idpID, tokenResp)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	return tokenResp, nil
}

// ValidateTokenResponse validates the token response returned from Google.
// ExchangeCodeForToken method calls this method to validate the token response if validateResponse is set
// to true. Hence generally you may not need to call this method explicitly.
func (g *googleOIDCAuthnService) ValidateTokenResponse(ctx context.Context, idpID string,
	tokenResp *authnoauth.TokenResponse) *serviceerror.ServiceError {
	svcErr := g.internal.ValidateTokenResponse(ctx, idpID, tokenResp, false)
	if svcErr != nil {
		return svcErr
	}

	return g.ValidateIDToken(ctx, idpID, tokenResp.IDToken)
}

// ValidateIDToken validates the ID token from Google with additional Google-specific validations.
// ValidateTokenResponse method calls this method to validate the token response if validateIDToken is set
// to true. Hence generally you may not need to call this method explicitly if ExchangeCodeForToken method
// is called with validateResponse set to true.
func (g *googleOIDCAuthnService) ValidateIDToken(
	ctx context.Context, idpID, idToken string) *serviceerror.ServiceError {
	logger := g.logger.With(log.String("idpId", idpID))
	logger.Debug("Validating ID token")

	if strings.TrimSpace(idToken) == "" {
		logger.Debug("ID token is empty")
		return &authnoidc.ErrorInvalidIDToken
	}

	// Get the OAuth client config for token validations
	oAuthClientConfig, svcErr := g.internal.GetOAuthClientConfig(ctx, idpID)
	if svcErr != nil {
		return svcErr
	}

	// Validate ID token signature using JWKS endpoint if available
	if oAuthClientConfig.OAuthEndpoints.JwksEndpoint != "" {
		err := g.jwtService.VerifyJWTSignatureWithJWKS(idToken, oAuthClientConfig.OAuthEndpoints.JwksEndpoint)
		if err != nil {
			logger.Debug("ID token signature validation failed", log.String("error", err.Error.DefaultValue))
			return &authnoidc.ErrorInvalidIDTokenSignature
		}
	} else {
		logger.Debug("Skipping ID token signature validation as JWKS endpoint is not configured")
	}

	logger.Debug("Validating Google specific ID token claims")

	// Extract ID token claims for Google-specific validation
	claims, err := jwt.DecodeJWTPayload(idToken)
	if err != nil {
		return &authnoidc.ErrorInvalidIDToken
	}

	// Validate issuer
	iss, ok := claims["iss"].(string)
	if !ok || (iss != Issuer1 && iss != Issuer2) {
		logger.Debug("Invalid ID token issuer", log.String("issuer", iss))
		return serviceerror.CustomServiceError(authnoidc.ErrorInvalidIDToken, core.I18nMessage{
			Key:          "error.authnservice.google.invalid_id_token_issuer_description",
			DefaultValue: "The issuer of the ID token is not a valid Google issuer",
		})
	}

	// Validate audience
	aud, ok := claims["aud"].(string)
	if !ok || aud != oAuthClientConfig.ClientID {
		logger.Debug("Invalid ID token audience", log.String("audience", aud),
			log.MaskedString("clientId", oAuthClientConfig.ClientID))
		return serviceerror.CustomServiceError(authnoidc.ErrorInvalidIDToken, core.I18nMessage{
			Key:          "error.authnservice.google.invalid_id_token_audience_description",
			DefaultValue: "The ID token audience does not match the expected client ID",
		})
	}

	// Get leeway from config to account for clock skew
	leeway := config.GetServerRuntime().Config.JWT.Leeway

	// Validate expiration time
	exp, ok := claims["exp"].(float64)
	if !ok {
		logger.Debug("Invalid ID token expiration claim", log.Any("exp", claims["exp"]))
		return serviceerror.CustomServiceError(authnoidc.ErrorInvalidIDToken, core.I18nMessage{
			Key:          "error.authnservice.google.invalid_id_token_exp_claim_description",
			DefaultValue: "The ID token expiration claim is missing or invalid",
		})
	}
	if time.Now().Unix() >= int64(exp)+leeway {
		logger.Debug("ID token has expired", log.Int("exp", int(exp)))
		return serviceerror.CustomServiceError(authnoidc.ErrorInvalidIDToken, core.I18nMessage{
			Key:          "error.authnservice.google.invalid_id_token_expired_description",
			DefaultValue: "The ID token has expired",
		})
	}

	// Check if token was issued in the future (with leeway for clock skew)
	iat, ok := claims["iat"].(float64)
	if !ok {
		logger.Debug("Invalid ID token issued-at claim", log.Any("iat", claims["iat"]))
		return serviceerror.CustomServiceError(authnoidc.ErrorInvalidIDToken, core.I18nMessage{
			Key:          "error.authnservice.google.invalid_id_token_iat_claim_description",
			DefaultValue: "The ID token issued-at (iat) claim is missing or invalid",
		})
	}
	if time.Now().Unix() < int64(iat)-leeway {
		logger.Debug("ID token was issued in the future", log.Int("iat", int(iat)))
		return serviceerror.CustomServiceError(authnoidc.ErrorInvalidIDToken, core.I18nMessage{
			Key:          "error.authnservice.google.invalid_id_token_future_iat_description",
			DefaultValue: "The ID token was issued in the future",
		})
	}

	// Check for specific domain if configured in additional params
	if hd, found := claims["hd"]; found {
		logger.Debug("hd claim found in ID token")
		if domain, exists := oAuthClientConfig.AdditionalParams["hd"]; exists && domain != "" {
			logger.Debug("Validating hosted domain (hd) claim")
			if hdStr, ok := hd.(string); !ok || hdStr != domain {
				logger.Debug("Invalid hosted domain (hd) claim", log.String("hd", hdStr),
					log.String("expectedDomain", domain))
				return serviceerror.CustomServiceError(authnoidc.ErrorInvalidIDToken, core.I18nMessage{
					Key:          "error.authnservice.google.invalid_id_token_hosted_domain_description",
					DefaultValue: "The ID token is not from the expected hosted domain: " + domain,
				})
			}
		}
	}

	return nil
}

// GetIDTokenClaims extracts and returns the claims from the Google ID token.
func (g *googleOIDCAuthnService) GetIDTokenClaims(idToken string) (
	map[string]interface{}, *serviceerror.ServiceError) {
	return g.internal.GetIDTokenClaims(idToken)
}

// FetchUserInfo retrieves user information from Google, ensuring email resolution if necessary.
func (g *googleOIDCAuthnService) FetchUserInfo(ctx context.Context, idpID, accessToken string) (
	map[string]interface{}, *serviceerror.ServiceError) {
	return g.internal.FetchUserInfo(ctx, idpID, accessToken)
}

// GetInternalUser retrieves the internal user based on the external subject identifier.
func (g *googleOIDCAuthnService) GetInternalUser(sub string) (*entityprovider.Entity, *serviceerror.ServiceError) {
	return g.internal.GetInternalUser(sub)
}

// GetOAuthClientConfig retrieves and validates the OAuth client configuration for the given identity provider ID.
func (g *googleOIDCAuthnService) GetOAuthClientConfig(ctx context.Context, idpID string) (
	*authnoauth.OAuthClientConfig, *serviceerror.ServiceError) {
	return g.internal.GetOAuthClientConfig(ctx, idpID)
}

// Authenticate performs the full Google OIDC authentication flow: exchanges the code for a token,
// extracts ID token claims, and resolves the internal user.
// A missing internal user is NOT an error — the caller decides how to handle it.
func (g *googleOIDCAuthnService) Authenticate(ctx context.Context, idpID, code string) (
	*common.FederatedAuthResult, *serviceerror.ServiceError) {
	logger := g.logger.With(log.String("idpId", idpID))
	logger.Debug("Performing federated Google OIDC authentication")

	tokenResp, svcErr := g.ExchangeCodeForToken(ctx, idpID, code, true)
	if svcErr != nil {
		return nil, svcErr
	}

	claims, svcErr := g.GetIDTokenClaims(tokenResp.IDToken)
	if svcErr != nil {
		return nil, svcErr
	}

	sub := ""
	if subVal, ok := claims["sub"]; ok && subVal != nil {
		if subStr, ok := subVal.(string); ok && subStr != "" {
			sub = subStr
		}
	}
	if sub == "" {
		logger.Debug("sub claim not found in ID token")
		return nil, &common.ErrorSubClaimNotFound
	}

	result := &common.FederatedAuthResult{
		Sub:    sub,
		Claims: claims,
	}
	user, svcErr := g.GetInternalUser(sub)
	if svcErr != nil {
		if svcErr.Code == common.ErrorUserNotFound.Code {
			return result, nil
		}
		if svcErr.Code == common.ErrorAmbiguousUser.Code {
			result.IsAmbiguousUser = true
			return result, nil
		}
		return nil, svcErr
	}
	result.InternalEntity = user
	return result, nil
}
