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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

const (
	testJWTTokenString = "test.jwt.token"     //nolint:gosec // Test token, not a real credential
	invalidJWTFormat   = "invalid.jwt.format" //nolint:gosec // Test token, not a real credential
	testClientID       = "client123"
)

type TokenValidatorTestSuite struct {
	suite.Suite
	mockJWTService         *jwtmock.JWTServiceInterfaceMock
	mockEnforcementService *revocationmock.EnforcementServiceInterfaceMock
	validator              *tokenValidator
	oauthApp               *providers.OAuthClient
}

func TestTokenValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(TokenValidatorTestSuite))
}

func (suite *TokenValidatorTestSuite) SetupTest() {
	config.ResetServerRuntime()

	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://example.com",
			ValidityPeriod: 3600,
			Audience:       "application", // Default audience for tests
			Leeway:         30,            // 30 seconds leeway for clock skew
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockEnforcementService = revocationmock.NewEnforcementServiceInterfaceMock(suite.T())
	// Default: tokens are not revoked. Individual tests override this to exercise revocation.
	suite.mockEnforcementService.On("EnsureNotRevoked", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	suite.validator = &tokenValidator{
		cfg: oauthconfig.Config{
			JWT: engineconfig.JWTConfig{
				Issuer:         "https://example.com",
				ValidityPeriod: 3600,
				Audience:       "application",
				Leeway:         30,
			},
		},
		jwtService:         suite.mockJWTService,
		enforcementService: suite.mockEnforcementService,
	}

	suite.oauthApp = &providers.OAuthClient{
		ClientID: "test-client",
	}
}

// Helper function to create a test JWT token
func (suite *TokenValidatorTestSuite) createTestJWT(claims map[string]interface{}) string {
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	return fmt.Sprintf("%s.%s.signature", headerB64, claimsB64)
}

// getDefaultAudience returns the configured default audience from the validator cfg.
func (suite *TokenValidatorTestSuite) getDefaultAudience() string {
	defaultAudience := suite.validator.cfg.JWT.Audience
	if defaultAudience == "" {
		suite.T().Skip("Default audience not configured in validator cfg")
		return ""
	}
	return defaultAudience
}

const testThunderIssuer = "https://thunder.io"

func (suite *TokenValidatorTestSuite) TestIsSelfIssuer_WithValidDeploymentIssuer() {
	suite.validator.cfg.JWT.Issuer = testThunderIssuer
	result := suite.validator.isSelfIssuer(testThunderIssuer)

	assert.True(suite.T(), result)
}

func (suite *TokenValidatorTestSuite) TestIsSelfIssuer_WithInvalidIssuer() {
	suite.validator.cfg.JWT.Issuer = testThunderIssuer
	result := suite.validator.isSelfIssuer("https://evil.example.com")

	assert.False(suite.T(), result)
}

func (suite *TokenValidatorTestSuite) TestIsSelfIssuer_WithEmptyIssuer() {
	suite.validator.cfg.JWT.Issuer = testThunderIssuer
	result := suite.validator.isSelfIssuer("")

	assert.False(suite.T(), result)
}

// ============================================================================
// ValidateSubjectToken Tests - Success Cases
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Success_BasicToken() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":   "user123",
		"iss":   "https://example.com",
		"aud":   defaultAudience, // Use default audience for the issuer
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://example.com", result.Iss)
	assert.Equal(suite.T(), []string{"read", "write"}, result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Success_WithTokenConfig() {
	// App with token config should still validate using server-level issuer from config
	customOAuthApp := &providers.OAuthClient{
		ClientID: "test-client",
		Token:    &providers.OAuthTokenConfig{},
	}

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, customOAuthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "https://example.com", result.Iss)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Success_WithoutNbfClaim() {
	defaultAudience := suite.getDefaultAudience()

	// nbf is optional, should succeed without it
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience, // Use default audience for the issuer
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Success_WithEmptyScopes() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience, // Use default audience for the issuer
		"exp": float64(now + 3600),
		// No scope claim
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestExtractSubjectTokenClaims_MapsReservedSubClaimToAttributes() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":        "user123",
		"iss":        "https://example.com",
		"aud":        suite.getDefaultAudience(),
		"exp":        float64(now + 3600),
		"given_name": "Jane",
	}
	mappings := []providers.AttributeMapping{
		{ExternalAttribute: "sub", LocalAttribute: "username"},
		{ExternalAttribute: "sub", LocalAttribute: "email"},
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
	}

	result, err := suite.validator.extractSubjectTokenClaims(
		"", "https://example.com", claims, suite.oauthApp, mappings)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	// The sub value flows into every attribute it is mapped to.
	assert.Equal(suite.T(), "user123", result.UserAttributes["username"])
	assert.Equal(suite.T(), "user123", result.UserAttributes["email"])
	assert.Equal(suite.T(), "Jane", result.UserAttributes["firstName"])
	// Reserved claims are still filtered out of the attribute set.
	assert.NotContains(suite.T(), result.UserAttributes, "sub")
	assert.NotContains(suite.T(), result.UserAttributes, "given_name")
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_InvalidJWTFormat() {
	token := invalidJWTFormat

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to decode token")
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_MalformedJWT() {
	token := "not-a-jwt-at-all" //nolint:gosec // Test token, not a real credential

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to decode token")
}

// An ID-JAG is an authorization grant, not a subject token; ValidateSubjectToken must reject any token
// whose typ header marks it as an ID-JAG before any signature or claim processing (typ confusion).
func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_IDJAGTypRejected() {
	header := map[string]interface{}{"alg": "RS256", "typ": jwt.TokenTypeIDJAG}
	claims := map[string]interface{}{
		"iss": suite.validator.cfg.JWT.Issuer,
		"sub": "user123",
		"exp": float64(time.Now().Unix() + 3600),
	}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	token := fmt.Sprintf("%s.%s.signature",
		base64.RawURLEncoding.EncodeToString(headerJSON),
		base64.RawURLEncoding.EncodeToString(claimsJSON))

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "ID-JAG cannot be presented as a subject_token")
}

// ============================================================================
// ValidateSubjectToken Tests - Issuer Validation Errors
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_MissingIssuerClaim() {
	claims := map[string]interface{}{
		"sub": "user123",
		// Missing iss claim
	}
	token := suite.createTestJWT(claims)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing 'iss' claim")
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_InvalidIssuerType() {
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": 12345, // Wrong type
	}
	token := suite.createTestJWT(claims)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing 'iss' claim")
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_UntrustedIssuer() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://evil-issuer.com",
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to exchange token for issuer")
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_InvalidSignature() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "SIGNATURE_VERIFICATION_FAILED",
			Error: tidcommon.I18nMessage{
				Key: "error.test.signature_verification_failed", DefaultValue: "Signature verification failed",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key:          "error.test.the_jwt_signature_verification_failed",
				DefaultValue: "The JWT signature verification failed",
			},
		})

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid subject token signature")
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// ValidateSubjectToken Tests - Claims Validation Errors
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_MissingSubClaim() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"iss": "https://example.com",
		"exp": float64(now + 3600),
		// Missing sub claim
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_InvalidSubType() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": 12345, // Wrong type
		"iss": "https://example.com",
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_ExpiredToken() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"exp": float64(now - 3600), // Expired
		"nbf": float64(now - 7200),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "token has expired")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Error_NotYetValid() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"exp": float64(now + 3600),
		"nbf": float64(now + 1800), // Not yet valid
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "token not yet valid")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestVerifyTokenSignatureByIssuer_Success_ServerIssuer() {
	token := testJWTTokenString

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	err := suite.validator.verifyTokenSignatureByIssuer(context.Background(), token, "https://example.com")

	assert.NoError(suite.T(), err)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestVerifyTokenSignatureByIssuer_Success_WithTokenConfig() {
	// Server-level issuer is used for signature verification regardless of app token config
	token := testJWTTokenString

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	err := suite.validator.verifyTokenSignatureByIssuer(context.Background(), token, "https://example.com")

	assert.NoError(suite.T(), err)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestVerifyTokenSignatureByIssuer_Error_SignatureFailure() {
	token := testJWTTokenString

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "SIGNATURE_MISMATCH",
			Error: tidcommon.I18nMessage{
				Key: "error.test.signature_mismatch", DefaultValue: "Signature mismatch",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.the_jwt_signature_does_not_match", DefaultValue: "The JWT signature does not match",
			},
		})

	err := suite.validator.verifyTokenSignatureByIssuer(context.Background(), token, "https://example.com")

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to verify token signature")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestVerifyTokenSignatureByIssuer_Error_ExternalIssuerNotSupported() {
	// External issuer (not in trusted server issuers)
	token := testJWTTokenString

	err := suite.validator.verifyTokenSignatureByIssuer(context.Background(), token, "https://external-idp.com")

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no verification method configured for issuer")
	assert.Contains(suite.T(), err.Error(), "https://external-idp.com")
}

func (suite *TokenValidatorTestSuite) TestFederationScenario_DecodeBeforeVerify() {
	defaultAudience := suite.getDefaultAudience()

	// This test verifies the decode-first approach for federation
	// Token with valid issuer should pass issuer check before signature verification
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience, // Use default audience for the issuer
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	// Signature verification should be called AFTER issuer validation
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestFederationScenario_FailFastOnUntrustedIssuer() {
	// Token with untrusted issuer should fail before signature verification
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://untrusted-issuer.com",
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	// Should not call VerifyJWTSignature because issuer check fails first
	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to exchange token for issuer")
	// VerifyJWTSignature should NOT have been called
	suite.mockJWTService.AssertNotCalled(suite.T(), "VerifyJWTSignature")
}

func (suite *TokenValidatorTestSuite) TestFederationScenario_OnlyServerIssuerIsValid() {
	// Only the server-level issuer from config is accepted; app-level issuers are no longer supported
	appWithTokenConfig := &providers.OAuthClient{
		ClientID: "test-client",
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{},
		},
	}

	now := time.Now().Unix()

	// Test token from server issuer (matches config-level issuer)
	claimsValid := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"exp": float64(now + 3600),
	}
	tokenValid := suite.createTestJWT(claimsValid)
	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, tokenValid).Return(nil)

	resultValid, errValid := suite.validator.ValidateSubjectToken(
		context.Background(), tokenValid, appWithTokenConfig)
	assert.NoError(suite.T(), errValid)
	assert.NotNil(suite.T(), resultValid)

	// Test token from unknown issuer (not in valid issuers - should fail)
	claimsInvalid := map[string]interface{}{
		"sub": "user456",
		"iss": "https://unknown-issuer.com",
		"exp": float64(now + 3600),
	}
	tokenInvalid := suite.createTestJWT(claimsInvalid)

	resultInvalid, errInvalid := suite.validator.ValidateSubjectToken(
		context.Background(), tokenInvalid, appWithTokenConfig)
	assert.Error(suite.T(), errInvalid)
	assert.Nil(suite.T(), resultInvalid)
	assert.Contains(suite.T(), errInvalid.Error(), "failed to exchange token for issuer")
	// VerifyJWTSignature should NOT have been called for untrusted issuer
	suite.mockJWTService.AssertNotCalled(suite.T(), "VerifyJWTSignature", tokenInvalid)

	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestFederationScenario_FutureExternalIssuerSupport() {
	// This test documents the intended behavior for future external issuer support
	// When JWKS support is added, verifyTokenSignatureByIssuer should use JWKS endpoint

	token := testJWTTokenString
	externalIssuer := "https://external-idp.com"

	// Currently returns error because no JWKS support yet
	err := suite.validator.verifyTokenSignatureByIssuer(context.Background(), token, externalIssuer)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no verification method configured")

	// TODO: When JWKS support is added, this should:
	// 1. Fetch JWKS from external issuer's .well-known endpoint
	// 2. Verify signature using public key from JWKS
	// 3. Return nil on success
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Security_RejectsTokenWithoutExp() {
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		// Missing exp claim - security risk
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	// Should reject tokens without expiration
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_EdgeCase_VeryLongToken() {
	defaultAudience := suite.getDefaultAudience()
	now := time.Now().Unix()
	largeClaims := map[string]interface{}{
		"sub":   "user123",
		"iss":   "https://example.com",
		"aud":   defaultAudience, // Use default audience for the issuer
		"exp":   float64(now + 3600),
		"large": string(make([]byte, 10000)), // 10KB of data
	}
	token := suite.createTestJWT(largeClaims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// §1 — Auth-assertion multi-aud rejection test
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_AuthAssertion_RejectsMultiAud() {
	// Auth assertions with more than one audience element must be rejected (defense-in-depth;
	// auth assertions are a narrow control surface).
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://example.com",
		"aud":       []interface{}{"a", "b"},
		"exp":       float64(now + 3600),
		"assurance": "high", // marks this as an auth assertion
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "auth assertion must have a single audience")
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// §1 — Non-assertion multi-aud tests
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_NonAssertion_AcceptsMultiAud() {
	// Non-assertion subject tokens with a multi-value aud are accepted when at least one element
	// matches the requesting app's EntityID or the configured default audience.
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": []interface{}{"x", "y"},
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	oauthAppWithID := &providers.OAuthClient{
		ClientID: "test-client",
		ID:       "x", // Matches one element of the aud array.
	}

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, oauthAppWithID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"x", "y"}, result.Aud)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_NonAssertion_ToleratesMalformedAud() {
	// Non-assertion subject tokens with a malformed aud (wrong type) do NOT return an error;
	// Aud is set to nil/empty for legacy tolerance. Downstream code treats nil Aud as
	// "no declared audience".
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": 123, // numeric — wrong type, silently ignored
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.Aud)
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// ValidateIDJAGSubjectToken Tests (draft-ietf-oauth-identity-assertion-authz-grant)
// The ID-JAG issuance leg requires a genuine self-issued ID token as the subject_token.
// ============================================================================

// A genuine self-issued ID token (typ=JWT, no access_token_sub, sub set, aud contains the client)
// is accepted and its claims are returned.
func (suite *TokenValidatorTestSuite) TestValidateIDJAGSubjectToken_Success() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": testClientID,
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateIDJAGSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://example.com", result.Iss)
	assert.Equal(suite.T(), []string{testClientID}, result.Aud)
	suite.mockJWTService.AssertExpectations(suite.T())
}

// Core token-laundering regression: an access token (typ=at+jwt) whose sub and aud=[client_id]
// would satisfy the ID-JAG client binding under the old aud-only logic is rejected on its typ
// header, before any signature verification, because it is not a genuine ID token. This is the
// re-audiencing hop the RFC 8693 token-exchange path could otherwise produce.
func (suite *TokenValidatorTestSuite) TestValidateIDJAGSubjectToken_RejectsAccessTokenTyp() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": testClientID,
		"exp": float64(now + 3600),
	}
	token := suite.createTestAccessToken(claims)

	result, err := suite.validator.ValidateIDJAGSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "must be an ID token")
}

// A refresh token carries typ=JWT (shared with ID tokens) but a top-level access_token_sub claim;
// it is rejected so a refresh token cannot be laundered into an ID-JAG.
func (suite *TokenValidatorTestSuite) TestValidateIDJAGSubjectToken_RejectsRefreshTokenShape() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              testClientID,
		"iss":              "https://example.com",
		"aud":              testClientID,
		"access_token_sub": "user123",
		"exp":              float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	result, err := suite.validator.ValidateIDJAGSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "refresh token")
}

// A token whose typ header marks it as an ID-JAG (oauth-id-jag+jwt) is rejected; only typ=JWT is
// accepted on the ID-JAG subject-token path.
func (suite *TokenValidatorTestSuite) TestValidateIDJAGSubjectToken_RejectsIDJAGTyp() {
	header := map[string]interface{}{"alg": "RS256", "typ": jwt.TokenTypeIDJAG}
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": testClientID,
		"exp": float64(time.Now().Unix() + 3600),
	}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	token := fmt.Sprintf("%s.%s.signature",
		base64.RawURLEncoding.EncodeToString(headerJSON),
		base64.RawURLEncoding.EncodeToString(claimsJSON))

	result, err := suite.validator.ValidateIDJAGSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "must be an ID token")
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_Basic() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"scope":            "read write",
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
		"aci":              "test-cache-id",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), []string{testAppID}, result.Audiences)
	assert.Equal(suite.T(), "authorization_code", result.GrantType)
	assert.Equal(suite.T(), []string{"read", "write"}, result.Scopes)
	assert.Equal(suite.T(), "test-cache-id", result.AttributeCacheID)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_WithActorSub() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"scope":            "read write",
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
		"act_sub":          "act-entity-id",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "act-entity-id", result.ActorSub)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_WithoutUserAttributes() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"scope":            "read write",
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "", result.AttributeCacheID)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_EmptyScopes() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_InvalidSignature() {
	token := "invalid.token.signature"

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "SIGNATURE_VERIFICATION_FAILED",
			Error: tidcommon.I18nMessage{
				Key: "error.test.signature_verification_failed", DefaultValue: "Signature verification failed",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key:          "error.test.the_jwt_signature_verification_failed",
				DefaultValue: "The JWT signature verification failed",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid refresh token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_InvalidJWTFormat() {
	token := invalidJWTFormat

	// VerifyJWT is called first and should fail for invalid format
	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			Code: "INVALID_JWT_FORMAT",
			Error: tidcommon.I18nMessage{
				Key: "error.test.invalid_jwt_format", DefaultValue: "Invalid JWT format",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.the_jwt_format_is_invalid", DefaultValue: "The JWT format is invalid",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid refresh token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_DecodeFailure() {
	// Invalid base64 in payload
	//nolint:gosec // Test token, not a real credential
	token := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.invalid-base64.signature"

	// VerifyJWT is called first and should fail for invalid base64
	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "INVALID_JWT_SIGNATURE",
			Error: tidcommon.I18nMessage{
				Key: "error.test.invalid_jwt_signature", DefaultValue: "Invalid JWT signature",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.the_jwt_signature_is_invalid", DefaultValue: "The JWT signature is invalid",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid refresh token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_MissingIat() {
	// iat is optional per RFC 7519, so refresh token should work without it
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "test-client",
		"iss": "https://example.com",
		"aud": "test-client",
		"exp": float64(now + 3600),
		// Missing iat - should be allowed
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), int64(0), result.Iat) // iat should be 0 when missing
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_ExpiredToken() {
	// VerifyJWT validates exp claim, so it should return an error for expired tokens
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now - 3600), // Expired
		"nbf":              float64(now - 7200), // Required by VerifyJWT
		"iat":              float64(now - 7200),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	// VerifyJWT should catch expired tokens
	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").
		Return(&tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "TOKEN_EXPIRED",
			Error: tidcommon.I18nMessage{Key: "error.test.token_has_expired", DefaultValue: "Token has expired"},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.the_token_has_expired", DefaultValue: "The token has expired",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid refresh token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_NotYetValid() {
	// VerifyJWT validates nbf claim, so it should return an error for not yet valid tokens
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"nbf":              float64(now + 1800), // Not yet valid
		"iat":              float64(now),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	// VerifyJWT should catch not yet valid tokens
	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			Code: "TOKEN_NOT_VALID_YET",
			Error: tidcommon.I18nMessage{
				Key: "error.test.token_not_valid_yet", DefaultValue: "Token not valid yet",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.token_not_valid_yet_nbf", DefaultValue: "Token not valid yet (nbf)",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid refresh token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_MissingSub() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
		// Missing sub
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_WrongClientID() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "wrong-client",
		"iss":              "https://example.com",
		"aud":              "wrong-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "refresh token does not belong to the requesting client")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_MissingAccessTokenSub() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
		// Missing access_token_sub
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'access_token_sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_MissingAccessTokenAud() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"grant_type":       "authorization_code",
		// Missing access_token_aud
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'access_token_aud' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_MissingGrantType() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		// Missing grant_type
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'grant_type' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_WithClaimsLocales() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                         "test-client",
		"iss":                         "https://example.com",
		"aud":                         "test-client",
		"exp":                         float64(now + 3600),
		"iat":                         float64(now),
		"scope":                       "read write",
		"access_token_sub":            "user123",
		"access_token_aud":            testAppID,
		"grant_type":                  "authorization_code",
		"aci":                         "test-cache-id",
		"access_token_claims_locales": "en-US fr-CA ja",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), []string{testAppID}, result.Audiences)
	assert.Equal(suite.T(), "authorization_code", result.GrantType)
	assert.Equal(suite.T(), []string{"read", "write"}, result.Scopes)
	assert.Equal(suite.T(), "test-cache-id", result.AttributeCacheID)
	assert.Equal(suite.T(), "en-US fr-CA ja", result.ClaimsLocales)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_WithDPoPJkt() {
	const testJkt = "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I"
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"scope":            "read",
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
		"dpop_jkt":         testJkt,
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testJkt, result.DPoPJkt)
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_WithoutDPoPJkt() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://example.com",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"scope":            "read",
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(context.Background(), token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.DPoPJkt)
}

// ============================================================================
// ValidateAuthAssertion Tests - Success Cases
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Success_WithAppID() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                    "user123",
		"iss":                    "https://example.com",
		"aud":                    testAppID, // Matches client app_id
		"exp":                    float64(now + 3600),
		"nbf":                    float64(now - 60),
		"assurance":              map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
		"authorized_permissions": "read:documents write:documents",
		"userType":               "person",
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID
	suite.oauthApp.ClientID = testClientID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://example.com", result.Iss)
	assert.Equal(suite.T(), []string{testAppID}, result.Aud)
	assert.Equal(suite.T(), []string{"read:documents", "write:documents"}, result.Scopes)
	assert.Equal(suite.T(), "person", result.UserAttributes["userType"])
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Success_WithEmptyScopes() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience, // Use default audience
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
		// No authorized_permissions or scope
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), []string{}, result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Success_WithScopeClaim() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                    "user123",
		"iss":                    "https://example.com",
		"aud":                    defaultAudience, // Use default audience
		"exp":                    float64(now + 3600),
		"nbf":                    float64(now - 60),
		"scope":                  "openid profile", // Standard scope claim takes priority
		"authorized_permissions": "read:documents write:documents",
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"openid", "profile"}, result.Scopes) // scope claim takes priority
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Success_WithUserAttributes() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                    "user123",
		"iss":                    "https://example.com",
		"aud":                    defaultAudience, // Use default audience
		"exp":                    float64(now + 3600),
		"nbf":                    float64(now - 60),
		"authorized_permissions": "read write",
		"userType":               "person",
		"email":                  "user@example.com",
		"customAttr":             "customValue",
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "person", result.UserAttributes["userType"])
	assert.Equal(suite.T(), "user@example.com", result.UserAttributes["email"])
	assert.Equal(suite.T(), "customValue", result.UserAttributes["customAttr"])
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// ValidateAuthAssertion Tests - Error Cases
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_MissingAudience() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		// Missing aud claim
		"exp":       float64(now + 3600),
		"nbf":       float64(now - 60),
		"assurance": map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing 'aud' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_AudienceMismatch() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://example.com",
		"aud":       "different-app-id", // Doesn't match default audience or client app_id
		"exp":       float64(now + 3600),
		"nbf":       float64(now - 60),
		"assurance": map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID
	suite.oauthApp.ClientID = testClientID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), "auth assertion audience mismatch", err.Error())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Success_WithDefaultAudience() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience, // Matches configured default audience
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID // Different from audience
	suite.oauthApp.ClientID = testClientID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://example.com", result.Iss)
	assert.Equal(suite.T(), []string{defaultAudience}, result.Aud)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_InvalidJWTFormat() {
	token := invalidJWTFormat

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to decode token")
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_MissingIssuer() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		// Missing iss claim
		"aud": "app123",
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing 'iss' claim")
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_ExpiredToken() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": "app123",
		"exp": float64(now - 3600), // Expired
		"nbf": float64(now - 7200),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "token has expired")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_InvalidIssuer() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://invalid-issuer.com", // Not in valid issuers
		"aud": "app123",
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to exchange token for issuer")
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_InvalidSignature() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": "app123",
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(&tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "INVALID_SIGNATURE",
		Error: tidcommon.I18nMessage{Key: "error.test.invalid_signature", DefaultValue: "Invalid signature"},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.the_jwt_signature_is_invalid", DefaultValue: "The JWT signature is invalid",
		},
	})

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid subject token signature")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_InvalidSubType() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": 12345, // Invalid type - should be string
		"iss": "https://example.com",
		"aud": "app123",
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_TokenNotYetValid() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": "app123",
		"exp": float64(now + 3600),
		"nbf": float64(now + 60), // Not yet valid - nbf is in the future
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "token not yet valid")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_MissingExp() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": "app123",
		// Missing exp claim
		"nbf": float64(now - 60),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'exp' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Error_InvalidAudType() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://example.com",
		"aud":       12345, // Invalid type - should be string
		"exp":       float64(now + 3600),
		"nbf":       float64(now - 60),
		"assurance": map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "claim aud has unsupported type")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Success_WithEmptyAuthorizedPermissions() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                    "user123",
		"iss":                    "https://example.com",
		"aud":                    defaultAudience, // Use default audience
		"exp":                    float64(now + 3600),
		"nbf":                    float64(now - 60),
		"authorized_permissions": "", // Empty string
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{}, result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// Leeway Tests - Time-based claim validation with clock skew tolerance
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Leeway_ExpiredWithinLeeway_ShouldPass() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	// Token expired 10 seconds ago, but leeway is 30 seconds - should pass
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience,
		"exp": float64(now - 10), // Expired 10 seconds ago
		"nbf": float64(now - 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Leeway_ExpiredBeyondLeeway_ShouldFail() {
	now := time.Now().Unix()
	// Token expired 60 seconds ago, leeway is 30 seconds - should fail
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"exp": float64(now - 60), // Expired 60 seconds ago
		"nbf": float64(now - 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "token has expired")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Leeway_NbfInFutureWithinLeeway_ShouldPass() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	// Token nbf is 10 seconds in future, but leeway is 30 seconds - should pass
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience,
		"exp": float64(now + 3600),
		"nbf": float64(now + 10), // Not valid for another 10 seconds
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Leeway_NbfInFutureBeyondLeeway_ShouldFail() {
	now := time.Now().Unix()
	// Token nbf is 60 seconds in future, leeway is 30 seconds - should fail
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"exp": float64(now + 3600),
		"nbf": float64(now + 60), // Not valid for another 60 seconds
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "token not yet valid")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Leeway_ExpirationBoundary_ShouldFail() {
	testCases := []struct {
		name      string
		leeway    int64
		expOffset int64 // offset from now in seconds (negative = expired)
		desc      string
	}{
		{
			name:      "ZeroLeeway_ExpiredOneSecondAgo",
			leeway:    0,
			expOffset: -1,
			desc:      "Token expired 1 second ago with zero leeway should fail",
		},
		{
			name:      "ExactlyAtBoundary",
			leeway:    30,
			expOffset: -30,
			desc:      "Token exp exactly at leeway boundary (now >= exp + leeway) should fail",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.validator.cfg.JWT.Leeway = tc.leeway

			now := time.Now().Unix()
			claims := map[string]interface{}{
				"sub": "user123",
				"iss": "https://example.com",
				"exp": float64(now + tc.expOffset),
				"nbf": float64(now - 3600),
			}
			token := suite.createTestJWT(claims)

			suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil).Once()

			result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

			assert.Error(suite.T(), err, tc.desc)
			assert.Nil(suite.T(), result)
			assert.Contains(suite.T(), err.Error(), "token has expired")
		})
	}
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Leeway_ExpJustInsideBoundary_ShouldPass() {
	suite.validator.cfg.JWT.Leeway = 30

	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	// Token exp is just inside leeway boundary (now - 29 seconds)
	// Condition: now >= exp + leeway
	// = now >= (now - 29) + 30 = now >= now + 1 = FALSE (should pass)
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience,
		"exp": float64(now - 29), // Just inside boundary
		"nbf": float64(now - 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	suite.mockJWTService.AssertExpectations(suite.T())
}

// createTestAccessToken creates a test JWT token with the "at+jwt" typ header.
func (suite *TokenValidatorTestSuite) createTestAccessToken(
	claims map[string]interface{},
) string {
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "at+jwt",
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	return fmt.Sprintf("%s.%s.signature", headerB64, claimsB64)
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Success() {
	claims := map[string]interface{}{
		"sub":        "user123",
		"iss":        "https://example.com",
		"aud":        "test-app",
		"scope":      "openid profile",
		"client_id":  "test-client",
		"grant_type": "authorization_code",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://example.com", result.Iss)
	assert.Equal(suite.T(), []string{"test-app"}, result.Aud)
	assert.Equal(suite.T(), "test-client", result.ClientID)
	assert.Equal(suite.T(), "authorization_code", result.GrantType)
	assert.Equal(suite.T(), []string{"openid", "profile"}, result.Scopes)
	assert.NotNil(suite.T(), result.Claims)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Success_MinClaims() {
	// Token with only the mandatory claims and no optional ones.
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://example.com",
		"aud":       "test-app",
		"client_id": "test-client",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://example.com", result.Iss)
	assert.Equal(suite.T(), []string{"test-app"}, result.Aud)
	assert.Equal(suite.T(), "test-client", result.ClientID)
	assert.Empty(suite.T(), result.GrantType)
	assert.Empty(suite.T(), result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

// revocationEnforcementCase describes a deny-list enforcement outcome for table-driven tests.
type revocationEnforcementCase struct {
	name        string
	jti         string
	returnedErr error
}

// revocationEnforcementCases returns the revoked and enforcement-unavailable cases, using jtiPrefix
// to keep jti values distinct per token type.
func revocationEnforcementCases(jtiPrefix string) []revocationEnforcementCase {
	return []revocationEnforcementCase{
		{"revoked", jtiPrefix + "-jti-revoked", revocation.ErrTokenRevoked},
		{"enforcement unavailable", jtiPrefix + "-jti-unknown", revocation.ErrEnforcementUnavailable},
	}
}

// validatorWithEnforcement builds a tokenValidator whose enforcement service returns returnedErr for
// the given jti, reusing the suite's JWT service.
func (suite *TokenValidatorTestSuite) validatorWithEnforcement(
	jti string, returnedErr error,
) *tokenValidator {
	enforcement := revocationmock.NewEnforcementServiceInterfaceMock(suite.T())
	enforcement.On("EnsureNotRevoked", mock.Anything, jti, mock.Anything).Return(returnedErr)
	return &tokenValidator{
		cfg:                suite.validator.cfg,
		jwtService:         suite.mockJWTService,
		enforcementService: enforcement,
	}
}

// A revoked access token surfaces revocation.ErrTokenRevoked from the validator, since enforcement
// runs as the final step of validation.
func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Revoked() {
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://example.com",
		"aud":       "test-app",
		"client_id": "test-client",
		"jti":       "at-jti-revoked",
		"tfid":      "tfid-at-revoked",
	}
	token := suite.createTestAccessToken(claims)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	// The token's tfid claim must reach the enforcement service verbatim so family-scoped revocation works.
	enforcement := revocationmock.NewEnforcementServiceInterfaceMock(suite.T())
	enforcement.On("EnsureNotRevoked", mock.Anything, "at-jti-revoked", "tfid-at-revoked").
		Return(revocation.ErrTokenRevoked)
	validator := &tokenValidator{
		cfg:                suite.validator.cfg,
		jwtService:         suite.mockJWTService,
		enforcementService: enforcement,
	}

	result, err := validator.ValidateAccessToken(context.Background(), token)

	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, revocation.ErrTokenRevoked)
}

// When the deny list cannot be consulted, the validator surfaces revocation.ErrEnforcementUnavailable
// (fail-closed) rather than returning claims.
func (suite *TokenValidatorTestSuite) TestValidateAccessToken_EnforcementUnavailable() {
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://example.com",
		"aud":       "test-app",
		"client_id": "test-client",
		"jti":       "at-jti-unknown",
	}
	token := suite.createTestAccessToken(claims)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	enforcement := revocationmock.NewEnforcementServiceInterfaceMock(suite.T())
	enforcement.On("EnsureNotRevoked", mock.Anything, "at-jti-unknown", mock.Anything).
		Return(revocation.ErrEnforcementUnavailable)
	validator := &tokenValidator{
		cfg:                suite.validator.cfg,
		jwtService:         suite.mockJWTService,
		enforcementService: enforcement,
	}

	result, err := validator.ValidateAccessToken(context.Background(), token)

	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, revocation.ErrEnforcementUnavailable)
}

// When token revocation is disabled, enforcementService is nil. Validation must still succeed
// rather than dereferencing the nil service.
func (suite *TokenValidatorTestSuite) TestValidateAccessToken_NilEnforcementService_Succeeds() {
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://example.com",
		"aud":       "test-app",
		"client_id": "test-client",
		"jti":       "at-jti-no-enforcement",
	}
	token := suite.createTestAccessToken(claims)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	validator := &tokenValidator{
		cfg:        suite.validator.cfg,
		jwtService: suite.mockJWTService,
	}

	result, err := validator.ValidateAccessToken(context.Background(), token)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
}

// Refresh token validation enforces the deny list as its final step: a revoked token surfaces
// revocation.ErrTokenRevoked and an unavailable deny list fails closed with
// revocation.ErrEnforcementUnavailable rather than returning claims.
func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_RevocationEnforced() {
	for _, tc := range revocationEnforcementCases("rt") {
		suite.Run(tc.name, func() {
			now := time.Now().Unix()
			claims := map[string]interface{}{
				"sub":              "test-client",
				"iss":              "https://example.com",
				"aud":              "test-client",
				"exp":              float64(now + 3600),
				"iat":              float64(now),
				"scope":            "read write",
				"access_token_sub": "user123",
				"access_token_aud": testAppID,
				"grant_type":       "authorization_code",
				"jti":              tc.jti,
			}
			token := suite.createTestJWT(claims)
			suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

			validator := suite.validatorWithEnforcement(tc.jti, tc.returnedErr)
			result, err := validator.ValidateRefreshToken(context.Background(), token, "test-client")

			assert.Nil(suite.T(), result)
			assert.ErrorIs(suite.T(), err, tc.returnedErr)
		})
	}
}

// Self-issued subject token validation enforces the deny list after the signature and claim checks,
// surfacing revocation.ErrTokenRevoked for a revoked token and failing closed with
// revocation.ErrEnforcementUnavailable when the deny list is unavailable.
func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_SelfIssued_RevocationEnforced() {
	for _, tc := range revocationEnforcementCases("st") {
		suite.Run(tc.name, func() {
			defaultAudience := suite.getDefaultAudience()
			now := time.Now().Unix()
			claims := map[string]interface{}{
				"sub":   "user123",
				"iss":   "https://example.com",
				"aud":   defaultAudience,
				"exp":   float64(now + 3600),
				"nbf":   float64(now - 60),
				"scope": "read write",
				"jti":   tc.jti,
			}
			token := suite.createTestJWT(claims)
			suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

			validator := suite.validatorWithEnforcement(tc.jti, tc.returnedErr)
			result, err := validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

			assert.Nil(suite.T(), result)
			assert.ErrorIs(suite.T(), err, tc.returnedErr)
		})
	}
}

// ValidateToken (used by introspection) verifies the signature, enforces the deny list, and returns
// the raw claims for a valid, non-revoked token.
func (suite *TokenValidatorTestSuite) TestValidateToken_Success() {
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"jti": "vt-jti-active",
	}
	token := suite.createTestJWT(claims)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	result, err := suite.validator.ValidateToken(context.Background(), token)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "user123", result["sub"])
	assert.Equal(suite.T(), "vt-jti-active", result["jti"])
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ValidateToken enforces the deny list after signature verification: a revoked token surfaces
// revocation.ErrTokenRevoked (so introspection reports it inactive) and an unavailable deny list
// fails closed with revocation.ErrEnforcementUnavailable.
func (suite *TokenValidatorTestSuite) TestValidateToken_RevocationEnforced() {
	for _, tc := range revocationEnforcementCases("vt") {
		suite.Run(tc.name, func() {
			claims := map[string]interface{}{
				"sub": "user123",
				"iss": "https://example.com",
				"jti": tc.jti,
			}
			token := suite.createTestJWT(claims)
			suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

			validator := suite.validatorWithEnforcement(tc.jti, tc.returnedErr)
			result, err := validator.ValidateToken(context.Background(), token)

			assert.Nil(suite.T(), result)
			assert.ErrorIs(suite.T(), err, tc.returnedErr)
		})
	}
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_VerifyFails() {
	token := "invalid.token.signature"

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").
		Return(&tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "JWT-1004",
			Error: tidcommon.I18nMessage{
				Key: "error.test.invalid_token_signature", DefaultValue: "Invalid token signature",
			},
		})

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(),
		"access token verification failed")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_WrongTyp() {
	// Token with typ "JWT" instead of "at+jwt" should be rejected.
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
	}
	token := suite.createTestJWT(claims) // Uses typ: "JWT"

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid token type")
	assert.Contains(suite.T(), err.Error(), "at+jwt")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_MissingTyp() {
	// Token without a typ header should be rejected.
	header := map[string]interface{}{
		"alg": "RS256",
	}
	claims := map[string]interface{}{
		"sub": "user123",
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	token := fmt.Sprintf("%s.%s.signature", headerB64, claimsB64)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid token type")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_MissingSub() {
	claims := map[string]interface{}{
		"iss":       "https://example.com",
		"aud":       "test-app",
		"client_id": "test-client",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_MissingAud() {
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://example.com",
		"client_id": "test-client",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'aud' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_MissingIss() {
	claims := map[string]interface{}{
		"sub":       "user123",
		"aud":       "test-app",
		"client_id": "test-client",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'iss' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_MissingClientID() {
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": "test-app",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'client_id' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_EmptySub() {
	claims := map[string]interface{}{
		"sub":       "",
		"iss":       "https://example.com",
		"aud":       "test-app",
		"client_id": "test-client",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_EmptyClientID() {
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://example.com",
		"aud":       "test-app",
		"client_id": "",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, token, "", "https://example.com").Return(nil)

	result, err := suite.validator.ValidateAccessToken(context.Background(), token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'client_id' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// External IDP Token Exchange Tests — audience validates against server issuer
// ============================================================================

const (
	testExternalIssuer       = "https://external-idp.example.com"
	testExternalJWKS         = "https://external-idp.example.com/.well-known/jwks.json"
	testTrustedTokenAudience = "google-client-id.apps.googleusercontent.com"
)

type ExternalIDPValidatorTestSuite struct {
	suite.Suite
	mockJWTService         *jwtmock.JWTServiceInterfaceMock
	mockIDPService         *idpmock.IDPServiceInterfaceMock
	mockEnforcementService *revocationmock.EnforcementServiceInterfaceMock
	validator              *tokenValidator
	oauthApp               *providers.OAuthClient
}

func TestExternalIDPValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ExternalIDPValidatorTestSuite))
}

func (suite *ExternalIDPValidatorTestSuite) SetupTest() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://example.com",
			ValidityPeriod: 3600,
			Audience:       "application",
			Leeway:         30,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockEnforcementService = revocationmock.NewEnforcementServiceInterfaceMock(suite.T())
	suite.mockEnforcementService.On("EnsureNotRevoked", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	suite.validator = &tokenValidator{
		cfg: oauthconfig.Config{
			JWT: engineconfig.JWTConfig{
				Issuer:         "https://example.com",
				ValidityPeriod: 3600,
				Audience:       "application",
				Leeway:         30,
			},
		},
		jwtService:         suite.mockJWTService,
		idpService:         suite.mockIDPService,
		enforcementService: suite.mockEnforcementService,
	}
	suite.oauthApp = &providers.OAuthClient{
		ClientID: "test-client",
	}
}

// buildExternalIDPDTOs builds a minimal []providers.IDPDTO for the standard test external IDP.
func buildExternalIDPDTOs() []providers.IDPDTO {
	propTokenExchange, _ := cmodels.NewProperty(idp.PropTokenExchangeEnabled, "true", false)
	propJWKS, _ := cmodels.NewProperty(idp.PropJwksEndpoint, testExternalJWKS, false)
	propIssuer, _ := cmodels.NewProperty(idp.PropIssuer, testExternalIssuer, false)
	return []providers.IDPDTO{
		{Properties: []cmodels.Property{*propTokenExchange, *propJWKS, *propIssuer}},
	}
}

func buildExternalIDPDTOsWithAudience() []providers.IDPDTO {
	idpDTOs := buildExternalIDPDTOs()
	propAudience, _ := cmodels.NewProperty(idp.PropTrustedTokenAudience, testTrustedTokenAudience, false)
	idpDTOs[0].Properties = append(idpDTOs[0].Properties, *propAudience)
	return idpDTOs
}

func (suite *ExternalIDPValidatorTestSuite) validateConfiguredAudienceSubjectToken(
	aud interface{},
) *SubjectTokenClaims {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"aud": aud,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createExternalJWT(claims)
	idpDTOs := buildExternalIDPDTOsWithAudience()

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
	return result
}

func (suite *ExternalIDPValidatorTestSuite) validateRejectedExternalAudience(
	aud interface{},
	idpDTOs []providers.IDPDTO,
) {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"aud": aud,
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createExternalJWT(claims)

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(),
		"external token audience does not contain expected server issuer")
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

// createExternalJWT creates a signed-looking JWT for an external IDP test.
func (suite *ExternalIDPValidatorTestSuite) createExternalJWT(claims map[string]interface{}) string {
	header := map[string]interface{}{"alg": "RS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	return fmt.Sprintf("%s.%s.signature", headerB64, claimsB64)
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Success_AudIsServerIssuer() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"aud": "https://example.com",
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createExternalJWT(claims)
	idpDTOs := buildExternalIDPDTOs()

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "ext-user-123", result.Sub)
	assert.Equal(suite.T(), testExternalIssuer, result.Iss)
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Success_AudArrayHasServerIssuer() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"aud": []interface{}{"https://example.com", "other-audience"},
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createExternalJWT(claims)
	idpDTOs := buildExternalIDPDTOs()

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "ext-user-123", result.Sub)
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ID-JAGs may only be issued for self-issued subject tokens. A token that passes the generic
// ValidateSubjectToken checks (valid external-issuer signature and audience) is still rejected by
// ValidateIDJAGSubjectToken because its issuer is not this server's own configured issuer.
func (suite *ExternalIDPValidatorTestSuite) TestValidateIDJAGSubjectToken_ExternalIDP_Error_NotSelfIssuer() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"aud": "https://example.com",
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createExternalJWT(claims)
	idpDTOs := buildExternalIDPDTOs()

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateIDJAGSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "subject_token must be issued by this server")
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Success_ConfiguredAudience() {
	result := suite.validateConfiguredAudienceSubjectToken(testTrustedTokenAudience)
	assert.Equal(suite.T(), "ext-user-123", result.Sub)
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Success_ServerIssuerAud() {
	result := suite.validateConfiguredAudienceSubjectToken("https://example.com")
	assert.Equal(suite.T(), "ext-user-123", result.Sub)
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Success_ConfiguredAudienceList() {
	result := suite.validateConfiguredAudienceSubjectToken([]interface{}{testTrustedTokenAudience, "other-audience"})
	assert.Equal(suite.T(), "ext-user-123", result.Sub)
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Error_AudNotServerIssuer() {
	suite.validateRejectedExternalAudience("some-client-id", buildExternalIDPDTOs())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Error_AudNotConfiguredAudience() {
	suite.validateRejectedExternalAudience("unexpected-client-id", buildExternalIDPDTOsWithAudience())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Error_MissingAudClaim() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)
	idpDTOs := buildExternalIDPDTOs()

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to extract audience from external token")
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Error_InvalidSignature() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"aud": "https://example.com",
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)
	idpDTOs := buildExternalIDPDTOs()

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).
		Return(&tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "SIGNATURE_VERIFICATION_FAILED",
			Error: tidcommon.I18nMessage{Key: "error.test.sig_failed", DefaultValue: "Signature verification failed"},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.sig_failed_desc", DefaultValue: "JWT signature verification failed",
			},
		})

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid subject token signature")
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Error_TokenExchangeNotEnabled() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"aud": "https://example.com",
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)

	propTokenExchange, _ := cmodels.NewProperty(idp.PropTokenExchangeEnabled, "false", false)
	propJWKS, _ := cmodels.NewProperty(idp.PropJwksEndpoint, testExternalJWKS, false)
	propIssuer, _ := cmodels.NewProperty(idp.PropIssuer, testExternalIssuer, false)
	idpDTOs := []providers.IDPDTO{
		{Properties: []cmodels.Property{*propTokenExchange, *propJWKS, *propIssuer}},
	}

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "token exchange not enabled")
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Error_NoJWKSEndpoint() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"aud": "https://example.com",
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)

	propTokenExchange, _ := cmodels.NewProperty(idp.PropTokenExchangeEnabled, "true", false)
	propIssuer, _ := cmodels.NewProperty(idp.PropIssuer, testExternalIssuer, false)
	idpDTOs := []providers.IDPDTO{
		{Properties: []cmodels.Property{*propTokenExchange, *propIssuer}},
	}

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "no JWKS endpoint configured")
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Error_IDPNotFound() {
	unknownIssuer := "https://unknown-idp.example.com"

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": unknownIssuer,
		"aud": "https://example.com",
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, unknownIssuer).
		Return(nil, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "IDP_NOT_FOUND",
			Error: tidcommon.I18nMessage{Key: "error.test.idp_not_found", DefaultValue: "IDP not found"},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.idp_not_found_desc", DefaultValue: "No IDP found for the given issuer",
			},
		})

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to exchange token for issuer")
	suite.mockIDPService.AssertExpectations(suite.T())
}

// buildExternalIDPDTOsWithMappings builds an external IDP with an attribute mapping.
func buildExternalIDPDTOsWithMappings(mappings []providers.AttributeMapping) []providers.IDPDTO {
	dtos := buildExternalIDPDTOs()
	dtos[0].AttributeConfiguration = &providers.AttributeConfiguration{
		UserTypeResolution:        &providers.UserTypeResolution{Default: "person"},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{{UserType: "person", Attributes: mappings}},
	}
	return dtos
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Mappings_Renamed() {
	now := time.Now().Unix()
	externalAttribute := "https://claims.example.com/email"
	claims := map[string]interface{}{
		"sub":             "ext-user-123",
		"iss":             testExternalIssuer,
		"aud":             "https://example.com",
		"exp":             float64(now + 3600),
		externalAttribute: "user@example.com",
	}
	token := suite.createExternalJWT(claims)
	idpDTOs := buildExternalIDPDTOsWithMappings(
		[]providers.AttributeMapping{{ExternalAttribute: externalAttribute, LocalAttribute: "email"}})

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user@example.com", result.UserAttributes["email"])
	_, originalPresent := result.UserAttributes[externalAttribute]
	assert.False(suite.T(), originalPresent)
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Mappings_ResolvedByClaim() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":       "ext-user-123",
		"iss":       testExternalIssuer,
		"aud":       "https://example.com",
		"exp":       float64(now + 3600),
		"user_type": "staff",
		"emp_id":    "E-42",
	}
	token := suite.createExternalJWT(claims)
	idpDTOs := buildExternalIDPDTOs()
	idpDTOs[0].AttributeConfiguration = &providers.AttributeConfiguration{
		UserTypeResolution: &providers.UserTypeResolution{
			Default:           "person",
			ExternalAttribute: "user_type",
			ValueMapping:      map[string]string{"staff": "employee"},
		},
		UserTypeAttributeMappings: []providers.UserTypeAttributeMapping{
			{UserType: "person", Attributes: []providers.AttributeMapping{}},
			{UserType: "employee", Attributes: []providers.AttributeMapping{
				{ExternalAttribute: "emp_id", LocalAttribute: "employeeNumber"}}},
		},
	}

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	// The claim-resolved "employee" user type's mapping is applied, not the default's.
	assert.Equal(suite.T(), "E-42", result.UserAttributes["employeeNumber"])
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_NoMappings_Verbatim() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":   "ext-user-123",
		"iss":   testExternalIssuer,
		"aud":   "https://example.com",
		"exp":   float64(now + 3600),
		"email": "user@example.com",
	}
	token := suite.createExternalJWT(claims)
	idpDTOs := buildExternalIDPDTOs()

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user@example.com", result.UserAttributes["email"])
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_PopulatesCnfJkt() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience,
		"exp": float64(now + 3600),
		"cnf": map[string]interface{}{"jkt": "thumbprint-abc"},
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "thumbprint-abc", result.CnfJkt)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_NoCnf_EmptyJkt() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience,
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.CnfJkt)
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_MalformedCnf_Error() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://example.com",
		"aud": defaultAudience,
		"exp": float64(now + 3600),
		"cnf": "not-an-object",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", mock.Anything, token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
}

// ============================================================================
// ID-JAG Assertion Validation Tests (draft-ietf-oauth-identity-assertion-authz-grant)
// The server issuer configured for these tests is "https://example.com".
// ============================================================================

type IDJAGValidatorTestSuite struct {
	suite.Suite
	mockJWTService         *jwtmock.JWTServiceInterfaceMock
	mockIDPService         *idpmock.IDPServiceInterfaceMock
	mockEnforcementService *revocationmock.EnforcementServiceInterfaceMock
	validator              *tokenValidator
}

func TestIDJAGValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(IDJAGValidatorTestSuite))
}

func (suite *IDJAGValidatorTestSuite) SetupTest() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://example.com",
			ValidityPeriod: 3600,
			Audience:       "application",
			Leeway:         30,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockEnforcementService = revocationmock.NewEnforcementServiceInterfaceMock(suite.T())
	suite.mockEnforcementService.On("EnsureNotRevoked", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	suite.validator = &tokenValidator{
		cfg: oauthconfig.Config{
			JWT: engineconfig.JWTConfig{
				Issuer:         "https://example.com",
				ValidityPeriod: 3600,
				Audience:       "application",
				Leeway:         30,
			},
		},
		jwtService:         suite.mockJWTService,
		idpService:         suite.mockIDPService,
		enforcementService: suite.mockEnforcementService,
	}
}

// buildIDJAGIDPDTOs builds a []providers.IDPDTO for a trusted external IdP with ID-JAG enabled.
func buildIDJAGIDPDTOs() []providers.IDPDTO {
	propIDJag, _ := cmodels.NewProperty(idp.PropIDJagEnabled, "true", false)
	propJWKS, _ := cmodels.NewProperty(idp.PropJwksEndpoint, testExternalJWKS, false)
	propIssuer, _ := cmodels.NewProperty(idp.PropIssuer, testExternalIssuer, false)
	return []providers.IDPDTO{
		{Properties: []cmodels.Property{*propIDJag, *propJWKS, *propIssuer}},
	}
}

// createAssertion builds a JWT with the given typ header and claims for ID-JAG assertion tests.
func (suite *IDJAGValidatorTestSuite) createAssertion(typ string, claims map[string]interface{}) string {
	header := map[string]interface{}{"alg": "RS256", "typ": typ}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	return fmt.Sprintf("%s.%s.signature", headerB64, claimsB64)
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_Success() {
	claims := suite.idjagClaims()
	claims["scope"] = JoinScopes([]string{"read", "write"})
	assertion := suite.createAssertion(jwt.TokenTypeIDJAG, claims)

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(buildIDJAGIDPDTOs(), nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, assertion, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "ext-user-123", result.Sub)
	assert.Equal(suite.T(), testExternalIssuer, result.Iss)
	assert.Equal(suite.T(), []string{"read", "write"}, result.Scopes)
	assert.Equal(suite.T(), testIDJAGAssertionJTI, result.JTI)
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_SingleResourceClaim() {
	claims := suite.idjagClaims()
	claims["resource"] = "https://rs01.example.com"
	assertion := suite.createAssertion(jwt.TokenTypeIDJAG, claims)

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(buildIDJAGIDPDTOs(), nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, assertion, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"https://rs01.example.com"}, result.Resources)
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_MultipleResourcesClaim() {
	claims := suite.idjagClaims()
	claims["resource"] = []string{"https://rs01.example.com", "https://rs02.example.com"}
	assertion := suite.createAssertion(jwt.TokenTypeIDJAG, claims)

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(buildIDJAGIDPDTOs(), nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, assertion, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), []string{"https://rs01.example.com", "https://rs02.example.com"}, result.Resources)
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_NoResourceClaim() {
	assertion := suite.createAssertion(jwt.TokenTypeIDJAG, suite.idjagClaims())

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(buildIDJAGIDPDTOs(), nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, assertion, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.Resources)
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_WrongTyp() {
	assertion := suite.createAssertion(jwt.TokenTypeJWT, suite.idjagClaims())

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "unsupported assertion type")
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_UntrustedIssuer() {
	assertion := suite.createAssertion(jwt.TokenTypeIDJAG, suite.idjagClaims())

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return([]providers.IDPDTO{}, nil)

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "untrusted assertion issuer")
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_IDJagNotEnabled() {
	assertion := suite.createAssertion(jwt.TokenTypeIDJAG, suite.idjagClaims())

	// IdP has token exchange enabled but NOT ID-JAG.
	propTokenExchange, _ := cmodels.NewProperty(idp.PropTokenExchangeEnabled, "true", false)
	propJWKS, _ := cmodels.NewProperty(idp.PropJwksEndpoint, testExternalJWKS, false)
	propIssuer, _ := cmodels.NewProperty(idp.PropIssuer, testExternalIssuer, false)
	idpDTOs := []providers.IDPDTO{
		{Properties: []cmodels.Property{*propTokenExchange, *propJWKS, *propIssuer}},
	}
	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(idpDTOs, nil)

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "ID-JAG not enabled")
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_InvalidSignature() {
	assertion := suite.createAssertion(jwt.TokenTypeIDJAG, suite.idjagClaims())

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(buildIDJAGIDPDTOs(), nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, assertion, testExternalJWKS).
		Return(&tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "SIGNATURE_VERIFICATION_FAILED",
			Error: tidcommon.I18nMessage{Key: "error.test.sig_failed", DefaultValue: "Signature verification failed"},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.sig_failed_desc", DefaultValue: "JWT signature verification failed",
			},
		})

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid assertion signature")
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

// assertRejectsSignedAssertion configures a trusted, ID-JAG-enabled issuer whose signature verifies,
// then asserts that ValidateIDJAGAssertion rejects the given claims with an error containing errSub.
// It covers the post-signature validation checks (time, audience, client binding).
func (suite *IDJAGValidatorTestSuite) assertRejectsSignedAssertion(
	claims map[string]interface{}, errSub string) {
	assertion := suite.createAssertion(jwt.TokenTypeIDJAG, claims)
	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(buildIDJAGIDPDTOs(), nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, assertion, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), errSub)
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *IDJAGValidatorTestSuite) idjagClaims() map[string]interface{} {
	now := time.Now().Unix()
	return map[string]interface{}{
		"sub":       "ext-user-123",
		"iss":       testExternalIssuer,
		"aud":       "https://example.com",
		"client_id": testClientID,
		"jti":       testIDJAGAssertionJTI,
		"iat":       float64(now),
		"exp":       float64(now + 300),
	}
}

const testIDJAGAssertionJTI = "assertion-jti-1" //nolint:gosec // Test identifier, not a credential

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_MissingJTI() {
	claims := suite.idjagClaims()
	delete(claims, "jti")
	suite.assertRejectsSignedAssertion(claims, "missing 'jti' claim")
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_MissingIAT() {
	claims := suite.idjagClaims()
	delete(claims, "iat")
	suite.assertRejectsSignedAssertion(claims, "missing 'iat' claim")
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_Expired() {
	claims := suite.idjagClaims()
	claims["exp"] = float64(time.Now().Unix() - 3600)
	suite.assertRejectsSignedAssertion(claims, "token has expired")
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_AudMismatch() {
	claims := suite.idjagClaims()
	claims["aud"] = "https://not-this-server.example.com"
	suite.assertRejectsSignedAssertion(claims, "assertion audience does not match server issuer")
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_ClientIDMismatch() {
	claims := suite.idjagClaims()
	claims["client_id"] = "a-different-client"
	suite.assertRejectsSignedAssertion(claims, "does not match the authenticated client")
}

func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_MissingClientIDClaim() {
	claims := suite.idjagClaims()
	delete(claims, "client_id")
	suite.assertRejectsSignedAssertion(claims, "missing 'client_id' claim")
}

// The draft allows aud to be an array only if it contains exactly one element; a two-element aud is
// rejected even when one element matches the server issuer.
func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_MultiValuedAudRejected() {
	claims := suite.idjagClaims()
	claims["aud"] = []string{"https://example.com", "https://other.example.com"}
	suite.assertRejectsSignedAssertion(claims, "assertion must have exactly one audience")
}

// A single-element aud array is equivalent to a string audience and is accepted.
func (suite *IDJAGValidatorTestSuite) TestValidateIDJAGAssertion_SingleElementAudArrayAccepted() {
	claims := suite.idjagClaims()
	claims["aud"] = []string{"https://example.com"}
	assertion := suite.createAssertion(jwt.TokenTypeIDJAG, claims)

	suite.mockIDPService.On("GetIdentityProvidersByProperty", context.Background(),
		idp.PropIssuer, testExternalIssuer).Return(buildIDJAGIDPDTOs(), nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", mock.Anything, assertion, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateIDJAGAssertion(context.Background(), assertion, testClientID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "ext-user-123", result.Sub)
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}
