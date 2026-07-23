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

package tokenservice

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/internal/idp"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// TokenValidatorInterface defines the interface for validating tokens. Every method verifies the
// token and then enforces the revocation deny list as a final step, so a caller cannot obtain claims
// for a revoked token. A revoked token yields revocation.ErrTokenRevoked and an unavailable deny list
// yields revocation.ErrEnforcementUnavailable (fail-closed); callers discriminate via errors.Is.
type TokenValidatorInterface interface {
	ValidateAccessToken(ctx context.Context, token string) (*AccessTokenClaims, error)
	ValidateRefreshToken(ctx context.Context, token string, clientID string) (*RefreshTokenClaims, error)
	ValidateSubjectToken(ctx context.Context, token string, oauthApp *providers.OAuthClient) (
		*SubjectTokenClaims, error)
	// ValidateIDJAGSubjectToken validates a subject token for the ID-JAG issuance leg of token
	// exchange (draft-ietf-oauth-identity-assertion-authz-grant). It performs the same validation as
	// ValidateSubjectToken and additionally requires the token to be a genuine ID token: its typ
	// header must be "JWT" (rejecting at+jwt access tokens) and it must not carry an access_token_sub
	// claim (rejecting refresh tokens). This blocks laundering a re-audienced access token or a
	// refresh token into an ID-JAG.
	ValidateIDJAGSubjectToken(ctx context.Context, token string, oauthApp *providers.OAuthClient) (
		*SubjectTokenClaims, error)
	// ValidateToken verifies a self-issued token's signature and enforces revocation without pinning
	// its type, returning the raw claims. Used by token introspection, which is token-type agnostic.
	ValidateToken(ctx context.Context, token string) (map[string]interface{}, error)
	// ValidateIDJAGAssertion validates an ID-JAG assertion presented on the jwt-bearer grant,
	// binding it to the authenticated client via its client_id claim.
	ValidateIDJAGAssertion(ctx context.Context, assertion, clientID string) (*IDJAGAssertionClaims, error)
}

// TokenValidator implements TokenValidatorInterface.
type tokenValidator struct {
	cfg                oauthconfig.Config
	jwtService         jwt.JWTServiceInterface
	idpService         providers.IDPProvider
	enforcementService revocation.EnforcementServiceInterface
}

// NewTokenValidator creates a new TokenValidator instance.
func newTokenValidator(
	cfg oauthconfig.Config,
	jwtService jwt.JWTServiceInterface,
	idpService providers.IDPProvider,
	enforcementService revocation.EnforcementServiceInterface,
) TokenValidatorInterface {
	return &tokenValidator{
		cfg:                cfg,
		jwtService:         jwtService,
		idpService:         idpService,
		enforcementService: enforcementService,
	}
}

// ValidateAccessToken validates an access token and extracts the claims.
func (tv *tokenValidator) ValidateAccessToken(ctx context.Context, token string) (*AccessTokenClaims, error) {
	// Verify signature and standard claims.
	expectedIss := tv.cfg.JWT.Issuer
	if err := tv.jwtService.VerifyJWT(ctx, token, "", expectedIss); err != nil {
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

	jti, _ := extractStringClaim(claims, constants.ClaimJTI)
	tokenFamilyID, _ := extractStringClaim(claims, constants.ClaimTokenFamilyID)
	if err := tv.ensureNotRevoked(ctx, jti, tokenFamilyID); err != nil {
		return nil, err
	}

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
func (tv *tokenValidator) ValidateRefreshToken(
	ctx context.Context, token string, clientID string,
) (*RefreshTokenClaims, error) {
	if err := tv.jwtService.VerifyJWT(ctx, token, "", ""); err != nil {
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
	exp, _ := extractInt64Claim(claims, "exp")
	scopes := extractScopesFromClaims(claims, false)
	attributeCacheID, _ := extractStringClaim(claims, "aci")
	actorSub, _ := extractStringClaim(claims, "act_sub")
	jti, _ := extractStringClaim(claims, "jti")
	tokenFamilyID, _ := extractStringClaim(claims, constants.ClaimTokenFamilyID)

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

	var dpopJkt string
	if _, exists := claims["dpop_jkt"]; exists {
		s, err := extractStringClaim(claims, "dpop_jkt")
		if err != nil {
			return nil, fmt.Errorf("invalid 'dpop_jkt' claim in refresh token: %w", err)
		}
		dpopJkt = s
	}

	if err := tv.ensureNotRevoked(ctx, jti, tokenFamilyID); err != nil {
		return nil, err
	}

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
		DPoPJkt:          dpopJkt,
		ActorSub:         actorSub,
		JTI:              jti,
		Exp:              exp,
		TokenFamilyID:    tokenFamilyID,
	}, nil
}

// ValidateSubjectToken validates a subject token for token exchange.
func (tv *tokenValidator) ValidateSubjectToken(
	ctx context.Context,
	token string,
	oauthApp *providers.OAuthClient,
) (*SubjectTokenClaims, error) {
	// An ID-JAG is an authorization grant, not a subject token, and must never be redeemable on token
	// exchange. Reject it up front based on its typ header before any other processing.
	header, err := jwt.DecodeJWTHeader(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token header: %w", err)
	}
	if typ, _ := header["typ"].(string); typ == jwt.TokenTypeIDJAG {
		return nil, fmt.Errorf("an ID-JAG cannot be presented as a subject_token")
	}

	claims, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	iss, err := extractStringClaim(claims, "iss")
	if err != nil {
		return nil, fmt.Errorf("subject token is missing 'iss' claim: %w", err)
	}

	// Try the server's own issuer first.
	if tv.isSelfIssuer(iss) {
		if err := tv.verifyTokenSignatureByIssuer(ctx, token, iss); err != nil {
			return nil, fmt.Errorf("invalid subject token signature: %w", err)
		}
		selfClaims, err := tv.extractSubjectTokenClaims(token, iss, claims, oauthApp, nil)
		if err != nil {
			return nil, err
		}
		selfTokenFamilyID, _ := extractStringClaim(claims, constants.ClaimTokenFamilyID)
		if err := tv.ensureNotRevoked(ctx, selfClaims.JTI, selfTokenFamilyID); err != nil {
			return nil, err
		}
		return selfClaims, nil
	}

	// Not a server-issued token — try external IDP issuers.
	issuerInfo, resolveErr := tv.resolveExternalIssuer(ctx, iss, claims)
	if resolveErr != nil {
		return nil, fmt.Errorf("failed to exchange token for issuer %q: %w", iss, resolveErr)
	}

	svcErr := tv.jwtService.VerifyJWTSignatureWithJWKS(ctx, token, issuerInfo.JWKSURL)
	if svcErr != nil {
		return nil, fmt.Errorf("invalid subject token signature: %v", svcErr.Error)
	}

	auds, audErr := extractAudiences(claims)
	if audErr != nil {
		return nil, fmt.Errorf("failed to extract audience from external token: %w", audErr)
	}
	if err := tv.validateExternalTokenAudience(auds, issuerInfo); err != nil {
		return nil, err
	}

	return tv.extractSubjectTokenClaims(token, iss, claims, oauthApp, issuerInfo.AttributeMappings)
}

// ValidateIDJAGSubjectToken validates a subject token for the ID-JAG issuance leg of token exchange
// (draft-ietf-oauth-identity-assertion-authz-grant). Beyond the standard subject-token validation
// performed by ValidateSubjectToken (signature, revocation deny list, time claims, and claim
// extraction), it enforces that the token is a genuine ID token. The typ header must be "JWT", which
// rejects access tokens (typ "at+jwt") and blocks laundering a re-audienced token-exchange access
// token into an ID-JAG. A top-level access_token_sub claim marks a refresh token (which shares
// typ "JWT" with ID tokens) and is likewise rejected. It also requires the token's issuer to be this
// server's own configured issuer, since ID-JAGs may only be issued for self-issued subject tokens.
// The generic ValidateSubjectToken path used by RFC 8693 token exchange and actor-token validation is
// intentionally left unchanged.
func (tv *tokenValidator) ValidateIDJAGSubjectToken(
	ctx context.Context,
	token string,
	oauthApp *providers.OAuthClient,
) (*SubjectTokenClaims, error) {
	header, err := jwt.DecodeJWTHeader(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode subject token header: %w", err)
	}
	if typ, _ := header["typ"].(string); typ != jwt.TokenTypeJWT {
		return nil, fmt.Errorf("subject_token must be an ID token, but has typ %q", typ)
	}

	claims, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode subject token payload: %w", err)
	}
	if _, isRefreshToken := claims["access_token_sub"]; isRefreshToken {
		return nil, fmt.Errorf("subject_token must be an ID token, not a refresh token")
	}

	subjectClaims, err := tv.ValidateSubjectToken(ctx, token, oauthApp)
	if err != nil {
		return nil, err
	}
	if subjectClaims.Iss != tv.cfg.JWT.Issuer {
		return nil, fmt.Errorf("subject_token must be issued by this server, got issuer %q", subjectClaims.Iss)
	}

	return subjectClaims, nil
}

// ValidateToken verifies a self-issued token's signature (type-agnostic) and enforces the revocation
// deny list, returning the raw claims. Token introspection uses this because it accepts both access
// and refresh tokens and must not pin a token type.
func (tv *tokenValidator) ValidateToken(ctx context.Context, token string) (map[string]interface{}, error) {
	if err := tv.jwtService.VerifyJWT(ctx, token, "", ""); err != nil {
		return nil, fmt.Errorf("token verification failed: %v", err.Error)
	}

	claims, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token payload: %w", err)
	}

	jti, _ := extractStringClaim(claims, constants.ClaimJTI)
	tokenFamilyID, _ := extractStringClaim(claims, constants.ClaimTokenFamilyID)
	if err := tv.ensureNotRevoked(ctx, jti, tokenFamilyID); err != nil {
		return nil, err
	}

	return claims, nil
}

// ValidateIDJAGAssertion validates an ID-JAG assertion presented on the jwt-bearer grant
// (draft-ietf-oauth-identity-assertion-authz-grant). It requires the oauth-id-jag+jwt typ header,
// resolves the assertion's issuer to a trusted external IdP with ID-JAG enabled, verifies the
// signature against that IdP's JWKS, validates the time claims, requires the audience to equal this
// server's issuer, and binds the assertion to the authenticated client via the client_id claim.
func (tv *tokenValidator) ValidateIDJAGAssertion(
	ctx context.Context,
	assertion string,
	clientID string,
) (*IDJAGAssertionClaims, error) {
	header, err := jwt.DecodeJWTHeader(assertion)
	if err != nil {
		return nil, fmt.Errorf("failed to decode assertion header: %w", err)
	}
	if typ, _ := header["typ"].(string); typ != jwt.TokenTypeIDJAG {
		return nil, fmt.Errorf("unsupported assertion type: expected %q", jwt.TokenTypeIDJAG)
	}

	claims, err := jwt.DecodeJWTPayload(assertion)
	if err != nil {
		return nil, fmt.Errorf("failed to decode assertion payload: %w", err)
	}

	iss, err := extractStringClaim(claims, "iss")
	if err != nil {
		return nil, fmt.Errorf("assertion is missing 'iss' claim: %w", err)
	}

	issuerInfo, resolveErr := tv.resolveIDJAGIssuer(ctx, iss)
	if resolveErr != nil {
		return nil, fmt.Errorf("untrusted assertion issuer %q: %w", iss, resolveErr)
	}

	if svcErr := tv.jwtService.VerifyJWTSignatureWithJWKS(ctx, assertion, issuerInfo.JWKSURL); svcErr != nil {
		return nil, fmt.Errorf("invalid assertion signature: %v", svcErr.Error)
	}

	if err := tv.validateTimeClaims(claims); err != nil {
		return nil, err
	}

	// The draft lists jti, iat, and exp as REQUIRED; exp is enforced by validateTimeClaims above.
	// Require a non-empty jti and an iat here. One-time-use (replay) caching keyed on jti is
	// intentionally deferred to a future version, so this remains a presence check for forward
	// compatibility.
	if _, iatErr := extractInt64Claim(claims, "iat"); iatErr != nil {
		return nil, fmt.Errorf("assertion is missing 'iat' claim: %w", iatErr)
	}
	jti, jtiErr := extractStringClaim(claims, "jti")
	if jtiErr != nil {
		return nil, fmt.Errorf("assertion is missing 'jti' claim: %w", jtiErr)
	}

	serverIssuer := tv.cfg.JWT.Issuer
	auds, audErr := extractAudiences(claims)
	if audErr != nil {
		return nil, fmt.Errorf("assertion is missing 'aud' claim: %w", audErr)
	}
	// The draft permits aud to be a string or an array, but an array MUST contain exactly one element.
	if len(auds) != 1 {
		return nil, fmt.Errorf("assertion must have exactly one audience")
	}
	if auds[0] != serverIssuer {
		return nil, fmt.Errorf("assertion audience does not match server issuer %q", serverIssuer)
	}

	assertionClientID, err := extractStringClaim(claims, "client_id")
	if err != nil {
		return nil, fmt.Errorf("assertion is missing 'client_id' claim: %w", err)
	}
	if assertionClientID != clientID {
		return nil, fmt.Errorf("assertion 'client_id' does not match the authenticated client")
	}

	sub, err := extractStringClaim(claims, "sub")
	if err != nil {
		return nil, fmt.Errorf("assertion is missing 'sub' claim: %w", err)
	}

	return &IDJAGAssertionClaims{
		Sub:       sub,
		Iss:       iss,
		Scopes:    extractScopesFromClaims(claims, false),
		Resources: extractStringSliceClaim(claims, "resource"),
		JTI:       jti,
	}, nil
}

// resolveIDJAGIssuer looks up an external IDP whose issuer property matches the given issuer and
// requires ID-JAG to be enabled. It mirrors resolveExternalIssuer but is a separate path so token
// exchange trust resolution is unaffected.
func (tv *tokenValidator) resolveIDJAGIssuer(ctx context.Context, issuer string) (
	*tokenExchangeIssuerInfo, error) {
	if tv.idpService == nil {
		return nil, fmt.Errorf("no external issuers configured")
	}

	idpDTOs, svcErr := tv.idpService.GetIdentityProvidersByProperty(ctx, idp.PropIssuer, issuer)
	if svcErr != nil || len(idpDTOs) == 0 {
		return nil, fmt.Errorf("no trusted issuer configured for '%s'", issuer)
	}

	idpDTO := idpDTOs[0]
	if idp.GetPropertyValue(idpDTO.Properties, idp.PropIDJagEnabled) != "true" {
		return nil, fmt.Errorf("ID-JAG not enabled for issuer '%s'", issuer)
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

// tokenExchangeIssuerInfo holds the resolved properties needed to validate an external token.
type tokenExchangeIssuerInfo struct {
	Issuer               string
	JWKSURL              string
	TrustedTokenAudience string
	AttributeMappings    []providers.AttributeMapping
}

// resolveExternalIssuer looks up an external IDP whose issuer property matches the given issuer.
// The subject token claims are used to resolve the user type for attribute mapping.
func (tv *tokenValidator) resolveExternalIssuer(
	ctx context.Context,
	issuer string,
	claims map[string]interface{},
) (*tokenExchangeIssuerInfo, error) {
	if tv.idpService == nil {
		return nil, fmt.Errorf("no external issuers configured")
	}

	idpDTOs, svcErr := tv.idpService.GetIdentityProvidersByProperty(ctx, idp.PropIssuer, issuer)
	if svcErr != nil || len(idpDTOs) == 0 {
		return nil, fmt.Errorf("no external issuer configured for '%s'", issuer)
	}

	idpDTO := idpDTOs[0]
	if idp.GetPropertyValue(idpDTO.Properties, idp.PropTokenExchangeEnabled) != "true" {
		return nil, fmt.Errorf("token exchange not enabled for issuer '%s'", issuer)
	}

	jwksURL := idp.GetPropertyValue(idpDTO.Properties, idp.PropJwksEndpoint)
	if jwksURL == "" {
		return nil, fmt.Errorf("no JWKS endpoint configured for issuer '%s'", issuer)
	}

	return &tokenExchangeIssuerInfo{
		Issuer:               issuer,
		JWKSURL:              jwksURL,
		TrustedTokenAudience: idp.GetPropertyValue(idpDTO.Properties, idp.PropTrustedTokenAudience),
		AttributeMappings:    idp.GetAttributeMappings(&idpDTO, claims),
	}, nil
}

func (tv *tokenValidator) validateExternalTokenAudience(auds []string, issuerInfo *tokenExchangeIssuerInfo) error {
	serverIssuer := tv.cfg.JWT.Issuer
	if slices.Contains(auds, serverIssuer) {
		return nil
	}

	if issuerInfo.TrustedTokenAudience != "" && slices.Contains(auds, issuerInfo.TrustedTokenAudience) {
		return nil
	}

	if issuerInfo.TrustedTokenAudience != "" {
		return fmt.Errorf("external token audience does not contain expected server issuer %q or configured "+
			"trusted token audience", serverIssuer)
	}

	return fmt.Errorf("external token audience does not contain expected server issuer %q", serverIssuer)
}

// extractSubjectTokenClaims extracts and validates claims from a decoded subject token.
func (tv *tokenValidator) extractSubjectTokenClaims(
	_ string,
	iss string,
	claims map[string]interface{},
	oauthApp *providers.OAuthClient,
	attributeMappings []providers.AttributeMapping,
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

		defaultAudience := tv.cfg.JWT.Audience
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

	// Apply attribute mappings against the full claim set first so reserved claims (e.g. sub) remain
	// available as mapping sources, then drop the reserved claims that were not mapped to attributes.
	userAttributes := ExtractUserAttributes(idp.ApplyAttributeMappings(claims, attributeMappings))

	// Extract nested act claim if present
	var nestedAct map[string]interface{}
	if actClaim, ok := claims["act"].(map[string]interface{}); ok {
		nestedAct = actClaim
	}

	cnfJkt, err := dpop.ExtractCnfJkt(claims)
	if err != nil {
		return nil, err
	}

	// Only self-issued tokens participate in deny-list (revocation) enforcement; an external
	// issuer's jti and token family id have no meaning in this server's deny list.
	var jti, tokenFamilyID string
	if tv.isSelfIssuer(iss) {
		jti, _ = extractStringClaim(claims, "jti")
		tokenFamilyID, _ = extractStringClaim(claims, constants.ClaimTokenFamilyID)
	}

	return &SubjectTokenClaims{
		Sub:            sub,
		Iss:            iss,
		Aud:            auds,
		Scopes:         scopes,
		UserAttributes: userAttributes,
		NestedAct:      nestedAct,
		CnfJkt:         cnfJkt,
		JTI:            jti,
		TokenFamilyID:  tokenFamilyID,
	}, nil
}

// verifyTokenSignatureByIssuer verifies JWT signature using issuer-specific verification method.
func (tv *tokenValidator) verifyTokenSignatureByIssuer(
	ctx context.Context,
	token string,
	issuer string,
) error {
	if !tv.isSelfIssuer(issuer) {
		return fmt.Errorf("no verification method configured for issuer: %s", issuer)
	}
	svcErr := tv.jwtService.VerifyJWTSignature(ctx, token)
	if svcErr != nil {
		return fmt.Errorf("failed to verify token signature: %v", svcErr.Error)
	}
	return nil
}

// isSelfIssuer reports whether the given issuer is the server's own configured issuer.
func (tv *tokenValidator) isSelfIssuer(issuer string) bool {
	return issuer == tv.cfg.JWT.Issuer
}

// validateTimeClaims validates time-based claims (exp, nbf).
func (tv *tokenValidator) validateTimeClaims(claims map[string]interface{}) error {
	// Get leeway from config to account for clock skew
	leeway := tv.cfg.JWT.Leeway
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

func (tv *tokenValidator) ensureNotRevoked(ctx context.Context, jti, tokenFamilyID string) error {
	if tv.enforcementService != nil {
		return tv.enforcementService.EnsureNotRevoked(ctx, jti, tokenFamilyID)
	}
	return nil
}
