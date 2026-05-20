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
	"slices"
	"time"

	"github.com/thunder-id/thunderid/internal/idp"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// TokenValidatorInterface defines the interface for validating tokens.
type TokenValidatorInterface interface {
	ValidateAccessToken(token string) (*AccessTokenClaims, error)
	ValidateRefreshToken(token string, clientID string) (*RefreshTokenClaims, error)
	ValidateSubjectToken(ctx context.Context, token string, oauthApp *inboundmodel.OAuthClient) (
		*SubjectTokenClaims, error)
}

// TokenValidator implements TokenValidatorInterface.
type tokenValidator struct {
	jwtService jwt.JWTServiceInterface
	idpService idp.IDPServiceInterface
}

// NewTokenValidator creates a new TokenValidator instance.
func newTokenValidator(jwtService jwt.JWTServiceInterface, idpService idp.IDPServiceInterface) TokenValidatorInterface {
	return &tokenValidator{
		jwtService: jwtService,
		idpService: idpService,
	}
}

// ValidateAccessToken validates an access token and extracts the claims.
func (tv *tokenValidator) ValidateAccessToken(token string) (*AccessTokenClaims, error) {
	// Verify signature and standard claims.
	expectedIss := config.GetServerRuntime().Config.JWT.Issuer
	if err := tv.jwtService.VerifyJWT(token, "", expectedIss); err != nil {
		return nil, fmt.Errorf("access token verification failed: %v", err.Error)
	}

	// Validate the typ header.
	header, err := jwt.DecodeJWTHeader(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode access token header: %w", err)
	}

	typ, _ := header["typ"].(string)
	if typ != jwt.TokenTypeAccessToken {
		return nil, fmt.Errorf(
			"invalid token type: expected %q, got %q", jwt.TokenTypeAccessToken, typ)
	}

	// Decode payload claims.
	claims, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode access token payload: %w", err)
	}

	// Extract and validate claims.
	sub, subErr := extractStringClaim(claims, "sub")
	if subErr != nil {
		return nil, fmt.Errorf("missing required 'sub' claim in access token")
	}
	iss, issErr := extractStringClaim(claims, "iss")
	if issErr != nil {
		return nil, fmt.Errorf("missing required 'iss' claim in access token")
	}
	auds, audErr := extractAudiences(claims)
	if audErr != nil {
		return nil, fmt.Errorf("missing required 'aud' claim in access token")
	}
	clientID, cidErr := extractStringClaim(claims, "client_id")
	if cidErr != nil {
		return nil, fmt.Errorf("missing required 'client_id' claim in access token")
	}

	grantType, _ := extractStringClaim(claims, "grant_type")
	scopes := extractScopesFromClaims(claims, false)

	return &AccessTokenClaims{
		Sub:       sub,
		Iss:       iss,
		Aud:       auds,
		GrantType: grantType,
		Scopes:    scopes,
		ClientID:  clientID,
		Claims:    claims,
	}, nil
}

// ValidateRefreshToken validates a refresh token and extracts the claims.
func (tv *tokenValidator) ValidateRefreshToken(token string, clientID string) (*RefreshTokenClaims, error) {
	if err := tv.jwtService.VerifyJWT(token, "", ""); err != nil {
		return nil, fmt.Errorf("invalid refresh token: %v", err.Error)
	}

	claims, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode refresh token: %w", err)
	}

	if err := tv.validateOAuth2RefreshClaims(claims, clientID); err != nil {
		return nil, err
	}

	// Extract claims
	sub, _ := extractStringClaim(claims, "access_token_sub")
	audiences := extractStringSliceClaim(claims, "access_token_aud")
	grantType, _ := extractStringClaim(claims, "grant_type")
	iat, _ := extractInt64Claim(claims, "iat")
	scopes := extractScopesFromClaims(claims, false)
	attributeCacheID, _ := extractStringClaim(claims, "aci")

	// Extract claims request if present
	var claimsRequest *oauth2model.ClaimsRequest
	if claimsJSON, ok := claims["access_token_claims_request"].(string); ok && claimsJSON != "" {
		parsed, err := utils.ParseClaimsRequest(claimsJSON)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse claims_request from refresh token: %w", err)
		}
		claimsRequest = parsed
	}

	// Extract claims_locales if present
	claimsLocales, _ := extractStringClaim(claims, "access_token_claims_locales")

	// Extract user type and organizational unit details if present
	return &RefreshTokenClaims{
		Sub:              sub,
		Audiences:        audiences,
		GrantType:        grantType,
		Scopes:           scopes,
		AttributeCacheID: attributeCacheID,
		Iat:              iat,
		ClaimsRequest:    claimsRequest,
		ClaimsLocales:    claimsLocales,
	}, nil
}

// ValidateSubjectToken validates a subject token for token exchange.
func (tv *tokenValidator) ValidateSubjectToken(
	ctx context.Context,
	token string,
	oauthApp *inboundmodel.OAuthClient,
) (*SubjectTokenClaims, error) {
	claims, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	iss, err := extractStringClaim(claims, "iss")
	if err != nil {
		return nil, fmt.Errorf("subject token is missing 'iss' claim: %w", err)
	}

	// Try the server's own issuer first.
	if isSelfIssuer(iss) {
		if err := tv.verifyTokenSignatureByIssuer(token, iss); err != nil {
			return nil, fmt.Errorf("invalid subject token signature: %w", err)
		}
		return tv.extractSubjectTokenClaims(token, iss, claims, oauthApp)
	}

	// Not a server-issued token — try external IDP issuers.
	issuerInfo, resolveErr := tv.resolveExternalIssuer(ctx, iss)
	if resolveErr != nil {
		return nil, fmt.Errorf("failed to exchange token for issuer %q: %w", iss, resolveErr)
	}

	svcErr := tv.jwtService.VerifyJWTSignatureWithJWKS(token, issuerInfo.JWKSURL)
	if svcErr != nil {
		return nil, fmt.Errorf("invalid subject token signature: %v", svcErr.Error)
	}

	// Validate that the external token's audience contains this server's issuer.
	serverIssuer := config.GetServerRuntime().Config.JWT.Issuer
	auds, audErr := extractAudiences(claims)
	if audErr != nil {
		return nil, fmt.Errorf("failed to extract audience from external token: %w", audErr)
	}
	if !slices.Contains(auds, serverIssuer) {
		return nil, fmt.Errorf(
			"external token audience does not contain expected server issuer %q", serverIssuer)
	}

	return tv.extractSubjectTokenClaims(token, iss, claims, oauthApp)
}

// tokenExchangeIssuerInfo holds the resolved properties needed to validate an external token.
type tokenExchangeIssuerInfo struct {
	Issuer  string
	JWKSURL string
}

// resolveExternalIssuer looks up an external IDP whose issuer property matches the given issuer.
func (tv *tokenValidator) resolveExternalIssuer(ctx context.Context, issuer string) (
	*tokenExchangeIssuerInfo, error) {
	if tv.idpService == nil {
		return nil, fmt.Errorf("no external issuers configured")
	}

	idpDTO, svcErr := tv.idpService.GetIdentityProviderByIssuer(ctx, issuer)
	if svcErr != nil {
		return nil, fmt.Errorf("no external issuer configured for '%s'", issuer)
	}

	if idp.GetPropertyValue(idpDTO.Properties, idp.PropTokenExchangeEnabled) != "true" {
		return nil, fmt.Errorf("token exchange not enabled for issuer '%s'", issuer)
	}

	jwksURL := idp.GetPropertyValue(idpDTO.Properties, idp.PropJwksEndpoint)
	if jwksURL == "" {
		return nil, fmt.Errorf("no JWKS endpoint configured for issuer '%s'", issuer)
	}

	return &tokenExchangeIssuerInfo{
		Issuer:  issuer,
		JWKSURL: jwksURL,
	}, nil
}

// extractSubjectTokenClaims extracts and validates claims from a decoded subject token.
func (tv *tokenValidator) extractSubjectTokenClaims(
	_ string,
	iss string,
	claims map[string]interface{},
	oauthApp *inboundmodel.OAuthClient,
) (*SubjectTokenClaims, error) {
	sub, err := extractStringClaim(claims, "sub")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid 'sub' claim: %w", err)
	}

	// Validate time-based claims
	if err := tv.validateTimeClaims(claims); err != nil {
		return nil, err
	}

	isAuthAssertion := tv.isAuthAssertion(claims)

	// Extract and validate audience claim
	var auds []string
	if isAuthAssertion {
		// For auth assertions, audience is required, must be a single value, and must match
		// config default or requesting client's app_id. Multi-aud auth assertions are rejected
		// as a defense-in-depth measure (auth assertions are a narrow control surface).
		auds, err = extractAudiences(claims)
		if err != nil {
			return nil, fmt.Errorf("auth assertion is missing 'aud' claim: %w", err)
		}
		if len(auds) > 1 {
			return nil, fmt.Errorf("auth assertion must have a single audience")
		}

		defaultAudience := config.GetServerRuntime().Config.JWT.Audience
		clientAppID := oauthApp.ID

		if !slices.Contains([]string{defaultAudience, clientAppID}, auds[0]) {
			return nil, fmt.Errorf("auth assertion audience mismatch")
		}
	} else {
		// Non-assertion subject tokens tolerate missing/malformed aud; downstream code treats
		// nil/empty Aud as "no declared audience". Auth assertions remain strict (above).
		auds, _ = extractAudiences(claims)
	}

	// Extract scopes
	scopes := extractScopesFromClaims(claims, isAuthAssertion)

	// Extract user attributes
	userAttributes := ExtractUserAttributes(claims)

	// Extract nested act claim if present
	var nestedAct map[string]interface{}
	if actClaim, ok := claims["act"].(map[string]interface{}); ok {
		nestedAct = actClaim
	}

	return &SubjectTokenClaims{
		Sub:            sub,
		Iss:            iss,
		Aud:            auds,
		Scopes:         scopes,
		UserAttributes: userAttributes,
		NestedAct:      nestedAct,
	}, nil
}

// verifyTokenSignatureByIssuer verifies JWT signature using issuer-specific verification method.
func (tv *tokenValidator) verifyTokenSignatureByIssuer(
	token string,
	issuer string,
) error {
	if !isSelfIssuer(issuer) {
		return fmt.Errorf("no verification method configured for issuer: %s", issuer)
	}
	svcErr := tv.jwtService.VerifyJWTSignature(token)
	if svcErr != nil {
		return fmt.Errorf("failed to verify token signature: %v", svcErr.Error)
	}
	return nil
}

// validateTimeClaims validates time-based claims (exp, nbf).
func (tv *tokenValidator) validateTimeClaims(claims map[string]interface{}) error {
	// Get leeway from config to account for clock skew
	leeway := config.GetServerRuntime().Config.JWT.Leeway
	now := time.Now().Unix()

	exp, err := extractInt64Claim(claims, "exp")
	if err != nil {
		return fmt.Errorf("missing or invalid 'exp' claim: %w", err)
	}
	if now >= exp+leeway {
		return fmt.Errorf("token has expired")
	}

	nbf, err := extractInt64Claim(claims, "nbf")
	if err == nil {
		if now < nbf-leeway {
			return fmt.Errorf("token not yet valid")
		}
	}

	return nil
}

// validateOAuth2RefreshClaims validates OAuth2-specific refresh token claims.
func (tv *tokenValidator) validateOAuth2RefreshClaims(claims map[string]interface{}, clientID string) error {
	sub, err := extractStringClaim(claims, "sub")
	if err != nil {
		return fmt.Errorf("missing or invalid 'sub' claim: %w", err)
	}

	if sub != clientID {
		return fmt.Errorf("refresh token does not belong to the requesting client")
	}

	// Validate required refresh token claims
	if _, err := extractStringClaim(claims, "access_token_sub"); err != nil {
		return fmt.Errorf("missing or invalid 'access_token_sub' claim: %w", err)
	}

	if auds := extractStringSliceClaim(claims, "access_token_aud"); len(auds) == 0 {
		return fmt.Errorf("missing or invalid 'access_token_aud' claim")
	}

	if _, err := extractStringClaim(claims, "grant_type"); err != nil {
		return fmt.Errorf("missing or invalid 'grant_type' claim: %w", err)
	}

	return nil
}

// isAuthAssertion determines if a JWT token is an auth assertion.
func (tv *tokenValidator) isAuthAssertion(
	claims map[string]interface{},
) bool {
	// TODO: Revisit this once we have a proper way to determine if a token is an auth assertion.
	if _, hasAssurance := claims["assurance"]; hasAssurance {
		return true
	}

	return false
}
