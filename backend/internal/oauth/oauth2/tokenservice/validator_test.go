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
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/idp"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testJWTTokenString = "test.jwt.token"     //nolint:gosec // Test token, not a real credential
	invalidJWTFormat   = "invalid.jwt.format" //nolint:gosec // Test token, not a real credential
	testClientID       = "client123"
)

type TokenValidatorTestSuite struct {
	suite.Suite
	mockJWTService *jwtmock.JWTServiceInterfaceMock
	validator      *tokenValidator
	oauthApp       *inboundmodel.OAuthClient
}

func TestTokenValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(TokenValidatorTestSuite))
}

func (suite *TokenValidatorTestSuite) SetupTest() {
	config.ResetServerRuntime()

	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
			Audience:       "application", // Default audience for tests
			Leeway:         30,            // 30 seconds leeway for clock skew
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.validator = &tokenValidator{
		jwtService: suite.mockJWTService,
	}

	suite.oauthApp = &inboundmodel.OAuthClient{
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

// getDefaultAudience is a helper function to get the configured default audience from runtime.
// It skips the test if the runtime is not initialized or the audience is not configured.
func (suite *TokenValidatorTestSuite) getDefaultAudience() string {
	runtime := config.GetServerRuntime()
	if runtime == nil {
		suite.T().Skip("Server runtime not initialized")
		return ""
	}
	defaultAudience := runtime.Config.JWT.Audience
	if defaultAudience == "" {
		suite.T().Skip("Default audience not configured in runtime")
		return ""
	}
	return defaultAudience
}

// ============================================================================
// ValidateSubjectToken Tests - Success Cases
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Success_BasicToken() {
	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":   "user123",
		"iss":   "https://thunder.io",
		"aud":   defaultAudience, // Use default audience for the issuer
		"exp":   float64(now + 3600),
		"nbf":   float64(now - 60),
		"scope": "read write",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://thunder.io", result.Iss)
	assert.Equal(suite.T(), []string{"read", "write"}, result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Success_WithTokenConfig() {
	// App with token config should still validate using server-level issuer from config
	customOAuthApp := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token:    &inboundmodel.OAuthTokenConfig{},
	}

	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://thunder.io",
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, customOAuthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "https://thunder.io", result.Iss)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Success_WithoutNbfClaim() {
	defaultAudience := suite.getDefaultAudience()

	// nbf is optional, should succeed without it
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://thunder.io",
		"aud": defaultAudience, // Use default audience for the issuer
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"aud": defaultAudience, // Use default audience for the issuer
		"exp": float64(now + 3600),
		// No scope claim
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
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
		"iss": "https://thunder.io",
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).
		Return(&serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "SIGNATURE_VERIFICATION_FAILED",
			Error: core.I18nMessage{
				Key: "error.test.signature_verification_failed", DefaultValue: "Signature verification failed",
			},
			ErrorDescription: core.I18nMessage{
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
		"iss": "https://thunder.io",
		"exp": float64(now + 3600),
		// Missing sub claim
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"exp": float64(now - 3600), // Expired
		"nbf": float64(now - 7200),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"exp": float64(now + 3600),
		"nbf": float64(now + 1800), // Not yet valid
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "token not yet valid")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestVerifyTokenSignatureByIssuer_Success_ServerIssuer() {
	token := testJWTTokenString

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	err := suite.validator.verifyTokenSignatureByIssuer(token, "https://thunder.io")

	assert.NoError(suite.T(), err)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestVerifyTokenSignatureByIssuer_Success_WithTokenConfig() {
	// Server-level issuer is used for signature verification regardless of app token config
	token := testJWTTokenString

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	err := suite.validator.verifyTokenSignatureByIssuer(token, "https://thunder.io")

	assert.NoError(suite.T(), err)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestVerifyTokenSignatureByIssuer_Error_SignatureFailure() {
	token := testJWTTokenString

	suite.mockJWTService.On("VerifyJWTSignature", token).
		Return(&serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "SIGNATURE_MISMATCH",
			Error: core.I18nMessage{
				Key: "error.test.signature_mismatch", DefaultValue: "Signature mismatch",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.the_jwt_signature_does_not_match", DefaultValue: "The JWT signature does not match",
			},
		})

	err := suite.validator.verifyTokenSignatureByIssuer(token, "https://thunder.io")

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to verify token signature")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestVerifyTokenSignatureByIssuer_Error_ExternalIssuerNotSupported() {
	// External issuer (not in trusted server issuers)
	token := testJWTTokenString

	err := suite.validator.verifyTokenSignatureByIssuer(token, "https://external-idp.com")

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
		"iss": "https://thunder.io",
		"aud": defaultAudience, // Use default audience for the issuer
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
	appWithTokenConfig := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{},
		},
	}

	now := time.Now().Unix()

	// Test token from server issuer (matches config-level issuer)
	claimsValid := map[string]interface{}{
		"sub": "user123",
		"iss": "https://thunder.io",
		"exp": float64(now + 3600),
	}
	tokenValid := suite.createTestJWT(claimsValid)
	suite.mockJWTService.On("VerifyJWTSignature", tokenValid).Return(nil)

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
	err := suite.validator.verifyTokenSignatureByIssuer(token, externalIssuer)

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
		"iss": "https://thunder.io",
		// Missing exp claim - security risk
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	// Should reject tokens without expiration
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_EdgeCase_VeryLongToken() {
	// Get the configured default audience from runtime
	runtime := config.GetServerRuntime()
	if runtime == nil {
		suite.T().Skip("Server runtime not initialized")
		return
	}
	defaultAudience := runtime.Config.JWT.Audience
	if defaultAudience == "" {
		suite.T().Skip("Default audience not configured in runtime")
		return
	}

	// Test with token containing large claims
	now := time.Now().Unix()
	largeClaims := map[string]interface{}{
		"sub":   "user123",
		"iss":   "https://thunder.io",
		"aud":   defaultAudience, // Use default audience for the issuer
		"exp":   float64(now + 3600),
		"large": string(make([]byte, 10000)), // 10KB of data
	}
	token := suite.createTestJWT(largeClaims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss":       "https://thunder.io",
		"aud":       []interface{}{"a", "b"},
		"exp":       float64(now + 3600),
		"assurance": "high", // marks this as an auth assertion
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"aud": []interface{}{"x", "y"},
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	oauthAppWithID := &inboundmodel.OAuthClient{
		ClientID: "test-client",
		ID:       "x", // Matches one element of the aud array.
	}

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"aud": 123, // numeric — wrong type, silently ignored
		"exp": float64(now + 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.Aud)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_Basic() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://thunder.io",
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

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), []string{testAppID}, result.Audiences)
	assert.Equal(suite.T(), "authorization_code", result.GrantType)
	assert.Equal(suite.T(), []string{"read", "write"}, result.Scopes)
	assert.Equal(suite.T(), "test-cache-id", result.AttributeCacheID)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_WithoutUserAttributes() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://thunder.io",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"scope":            "read write",
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "", result.AttributeCacheID)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_EmptyScopes() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://thunder.io",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Empty(suite.T(), result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_InvalidSignature() {
	token := "invalid.token.signature"

	suite.mockJWTService.On("VerifyJWT", token, "", "").
		Return(&serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "SIGNATURE_VERIFICATION_FAILED",
			Error: core.I18nMessage{
				Key: "error.test.signature_verification_failed", DefaultValue: "Signature verification failed",
			},
			ErrorDescription: core.I18nMessage{
				Key:          "error.test.the_jwt_signature_verification_failed",
				DefaultValue: "The JWT signature verification failed",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid refresh token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_InvalidJWTFormat() {
	token := invalidJWTFormat

	// VerifyJWT is called first and should fail for invalid format
	suite.mockJWTService.On("VerifyJWT", token, "", "").
		Return(&serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType,
			Code: "INVALID_JWT_FORMAT",
			Error: core.I18nMessage{
				Key: "error.test.invalid_jwt_format", DefaultValue: "Invalid JWT format",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.the_jwt_format_is_invalid", DefaultValue: "The JWT format is invalid",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

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
	suite.mockJWTService.On("VerifyJWT", token, "", "").
		Return(&serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "INVALID_JWT_SIGNATURE",
			Error: core.I18nMessage{
				Key: "error.test.invalid_jwt_signature", DefaultValue: "Invalid JWT signature",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.the_jwt_signature_is_invalid", DefaultValue: "The JWT signature is invalid",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

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
		"iss": "https://thunder.io",
		"aud": "test-client",
		"exp": float64(now + 3600),
		// Missing iat - should be allowed
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

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
		"iss":              "https://thunder.io",
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
	suite.mockJWTService.On("VerifyJWT", token, "", "").
		Return(&serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  "TOKEN_EXPIRED",
			Error: core.I18nMessage{Key: "error.test.token_has_expired", DefaultValue: "Token has expired"},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.the_token_has_expired", DefaultValue: "The token has expired",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

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
		"iss":              "https://thunder.io",
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
	suite.mockJWTService.On("VerifyJWT", token, "", "").
		Return(&serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType,
			Code: "TOKEN_NOT_VALID_YET",
			Error: core.I18nMessage{
				Key: "error.test.token_not_valid_yet", DefaultValue: "Token not valid yet",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.token_not_valid_yet_nbf", DefaultValue: "Token not valid yet (nbf)",
			},
		})

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid refresh token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_MissingSub() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"iss":              "https://thunder.io",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
		// Missing sub
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_WrongClientID() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "wrong-client",
		"iss":              "https://thunder.io",
		"aud":              "wrong-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "refresh token does not belong to the requesting client")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_MissingAccessTokenSub() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://thunder.io",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_aud": testAppID,
		"grant_type":       "authorization_code",
		// Missing access_token_sub
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'access_token_sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_MissingAccessTokenAud() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://thunder.io",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"grant_type":       "authorization_code",
		// Missing access_token_aud
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'access_token_aud' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Error_MissingGrantType() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":              "test-client",
		"iss":              "https://thunder.io",
		"aud":              "test-client",
		"exp":              float64(now + 3600),
		"iat":              float64(now),
		"access_token_sub": "user123",
		"access_token_aud": testAppID,
		// Missing grant_type
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing or invalid 'grant_type' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateRefreshToken_Success_WithClaimsLocales() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                         "test-client",
		"iss":                         "https://thunder.io",
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

	suite.mockJWTService.On("VerifyJWT", token, "", "").Return(nil)

	result, err := suite.validator.ValidateRefreshToken(token, "test-client")

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

// ============================================================================
// ValidateAuthAssertion Tests - Success Cases
// ============================================================================

func (suite *TokenValidatorTestSuite) TestValidateAuthAssertion_Success_WithAppID() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub":                    "user123",
		"iss":                    "https://thunder.io",
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

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://thunder.io", result.Iss)
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
		"iss": "https://thunder.io",
		"aud": defaultAudience, // Use default audience
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
		// No authorized_permissions or scope
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss":                    "https://thunder.io",
		"aud":                    defaultAudience, // Use default audience
		"exp":                    float64(now + 3600),
		"nbf":                    float64(now - 60),
		"scope":                  "openid profile", // Standard scope claim takes priority
		"authorized_permissions": "read:documents write:documents",
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss":                    "https://thunder.io",
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

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		// Missing aud claim
		"exp":       float64(now + 3600),
		"nbf":       float64(now - 60),
		"assurance": map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss":       "https://thunder.io",
		"aud":       "different-app-id", // Doesn't match default audience or client app_id
		"exp":       float64(now + 3600),
		"nbf":       float64(now - 60),
		"assurance": map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID
	suite.oauthApp.ClientID = testClientID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"aud": defaultAudience, // Matches configured default audience
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID // Different from audience
	suite.oauthApp.ClientID = testClientID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://thunder.io", result.Iss)
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
		"iss": "https://thunder.io",
		"aud": "app123",
		"exp": float64(now - 3600), // Expired
		"nbf": float64(now - 7200),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"aud": "app123",
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(&serviceerror.ServiceError{
		Type:  serviceerror.ServerErrorType,
		Code:  "INVALID_SIGNATURE",
		Error: core.I18nMessage{Key: "error.test.invalid_signature", DefaultValue: "Invalid signature"},
		ErrorDescription: core.I18nMessage{
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
		"iss": "https://thunder.io",
		"aud": "app123",
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"aud": "app123",
		"exp": float64(now + 3600),
		"nbf": float64(now + 60), // Not yet valid - nbf is in the future
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"aud": "app123",
		// Missing exp claim
		"nbf": float64(now - 60),
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss":       "https://thunder.io",
		"aud":       12345, // Invalid type - should be string
		"exp":       float64(now + 3600),
		"nbf":       float64(now - 60),
		"assurance": map[string]interface{}{"aal": "AAL1", "ial": "IAL1"}, // Make it an auth assertion
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss":                    "https://thunder.io",
		"aud":                    defaultAudience, // Use default audience
		"exp":                    float64(now + 3600),
		"nbf":                    float64(now - 60),
		"authorized_permissions": "", // Empty string
	}
	token := suite.createTestJWT(claims)

	suite.oauthApp.ID = testAppID

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"aud": defaultAudience,
		"exp": float64(now - 10), // Expired 10 seconds ago
		"nbf": float64(now - 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"exp": float64(now - 60), // Expired 60 seconds ago
		"nbf": float64(now - 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"aud": defaultAudience,
		"exp": float64(now + 3600),
		"nbf": float64(now + 10), // Not valid for another 10 seconds
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss": "https://thunder.io",
		"exp": float64(now + 3600),
		"nbf": float64(now + 60), // Not valid for another 60 seconds
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
			config.ResetServerRuntime()
			testConfig := &config.Config{
				JWT: config.JWTConfig{
					Issuer:         "https://thunder.io",
					ValidityPeriod: 3600,
					Audience:       "application",
					Leeway:         tc.leeway,
				},
			}
			_ = config.InitializeServerRuntime("test", testConfig)

			now := time.Now().Unix()
			claims := map[string]interface{}{
				"sub": "user123",
				"iss": "https://thunder.io",
				"exp": float64(now + tc.expOffset),
				"nbf": float64(now - 3600),
			}
			token := suite.createTestJWT(claims)

			suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil).Once()

			result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

			assert.Error(suite.T(), err, tc.desc)
			assert.Nil(suite.T(), result)
			assert.Contains(suite.T(), err.Error(), "token has expired")
		})
	}
}

func (suite *TokenValidatorTestSuite) TestValidateSubjectToken_Leeway_ExpJustInsideBoundary_ShouldPass() {
	// Reset and test with 30 second leeway
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
			Audience:       "application",
			Leeway:         30, // 30 seconds leeway
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	defaultAudience := suite.getDefaultAudience()

	now := time.Now().Unix()
	// Token exp is just inside leeway boundary (now - 29 seconds)
	// Condition: now >= exp + leeway
	// = now >= (now - 29) + 30 = now >= now + 1 = FALSE (should pass)
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://thunder.io",
		"aud": defaultAudience,
		"exp": float64(now - 29), // Just inside boundary
		"nbf": float64(now - 3600),
	}
	token := suite.createTestJWT(claims)

	suite.mockJWTService.On("VerifyJWTSignature", token).Return(nil)

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
		"iss":        "https://thunder.io",
		"aud":        "test-app",
		"scope":      "openid profile",
		"client_id":  "test-client",
		"grant_type": "authorization_code",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://thunder.io", result.Iss)
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
		"iss":       "https://thunder.io",
		"aud":       "test-app",
		"client_id": "test-client",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user123", result.Sub)
	assert.Equal(suite.T(), "https://thunder.io", result.Iss)
	assert.Equal(suite.T(), []string{"test-app"}, result.Aud)
	assert.Equal(suite.T(), "test-client", result.ClientID)
	assert.Empty(suite.T(), result.GrantType)
	assert.Empty(suite.T(), result.Scopes)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_VerifyFails() {
	token := "invalid.token.signature"

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").
		Return(&serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Code:  "JWT-1004",
			Error: core.I18nMessage{Key: "error.test.invalid_token_signature", DefaultValue: "Invalid token signature"},
		})

	result, err := suite.validator.ValidateAccessToken(token)

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
		"iss": "https://thunder.io",
	}
	token := suite.createTestJWT(claims) // Uses typ: "JWT"

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

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

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid token type")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_MissingSub() {
	claims := map[string]interface{}{
		"iss":       "https://thunder.io",
		"aud":       "test-app",
		"client_id": "test-client",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_MissingAud() {
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://thunder.io",
		"client_id": "test-client",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

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

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'iss' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_MissingClientID() {
	claims := map[string]interface{}{
		"sub": "user123",
		"iss": "https://thunder.io",
		"aud": "test-app",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'client_id' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_EmptySub() {
	claims := map[string]interface{}{
		"sub":       "",
		"iss":       "https://thunder.io",
		"aud":       "test-app",
		"client_id": "test-client",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'sub' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenValidatorTestSuite) TestValidateAccessToken_Error_EmptyClientID() {
	claims := map[string]interface{}{
		"sub":       "user123",
		"iss":       "https://thunder.io",
		"aud":       "test-app",
		"client_id": "",
	}
	token := suite.createTestAccessToken(claims)

	suite.mockJWTService.On("VerifyJWT", token, "", "https://thunder.io").Return(nil)

	result, err := suite.validator.ValidateAccessToken(token)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "missing required 'client_id' claim")
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ============================================================================
// External IDP Token Exchange Tests — audience validates against server issuer
// ============================================================================

const (
	testExternalIssuer = "https://external-idp.example.com"
	testExternalJWKS   = "https://external-idp.example.com/.well-known/jwks.json"
)

type ExternalIDPValidatorTestSuite struct {
	suite.Suite
	mockJWTService *jwtmock.JWTServiceInterfaceMock
	mockIDPService *idpmock.IDPServiceInterfaceMock
	validator      *tokenValidator
	oauthApp       *inboundmodel.OAuthClient
}

func TestExternalIDPValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ExternalIDPValidatorTestSuite))
}

func (suite *ExternalIDPValidatorTestSuite) SetupTest() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://thunder.io",
			ValidityPeriod: 3600,
			Audience:       "application",
			Leeway:         30,
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.validator = &tokenValidator{
		jwtService: suite.mockJWTService,
		idpService: suite.mockIDPService,
	}
	suite.oauthApp = &inboundmodel.OAuthClient{
		ClientID: "test-client",
	}
}

// buildExternalIDPDTO builds a minimal idp.IDPDTO for the standard test external IDP.
func buildExternalIDPDTO() *idp.IDPDTO {
	propTokenExchange, _ := cmodels.NewProperty(idp.PropTokenExchangeEnabled, "true", false)
	propJWKS, _ := cmodels.NewProperty(idp.PropJwksEndpoint, testExternalJWKS, false)
	propIssuer, _ := cmodels.NewProperty(idp.PropIssuer, testExternalIssuer, false)
	return &idp.IDPDTO{
		Properties: []cmodels.Property{*propTokenExchange, *propJWKS, *propIssuer},
	}
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
		"aud": "https://thunder.io", // audience is this server's own issuer
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createExternalJWT(claims)
	idpDTO := buildExternalIDPDTO()

	suite.mockIDPService.On("GetIdentityProviderByIssuer", context.Background(), testExternalIssuer).
		Return(idpDTO, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", token, testExternalJWKS).Return(nil)

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
		"aud": []interface{}{"https://thunder.io", "other-audience"}, // array including server issuer
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createExternalJWT(claims)
	idpDTO := buildExternalIDPDTO()

	suite.mockIDPService.On("GetIdentityProviderByIssuer", context.Background(), testExternalIssuer).
		Return(idpDTO, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "ext-user-123", result.Sub)
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Error_AudNotServerIssuer() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		"aud": "some-client-id", // audience is a client_id, not the server issuer
		"exp": float64(now + 3600),
		"nbf": float64(now - 60),
	}
	token := suite.createExternalJWT(claims)
	idpDTO := buildExternalIDPDTO()

	suite.mockIDPService.On("GetIdentityProviderByIssuer", context.Background(), testExternalIssuer).
		Return(idpDTO, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", token, testExternalJWKS).Return(nil)

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "external token audience does not contain expected server issuer")
	suite.mockIDPService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *ExternalIDPValidatorTestSuite) TestValidateSubjectToken_ExternalIDP_Error_MissingAudClaim() {
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"sub": "ext-user-123",
		"iss": testExternalIssuer,
		// no aud claim
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)
	idpDTO := buildExternalIDPDTO()

	suite.mockIDPService.On("GetIdentityProviderByIssuer", context.Background(), testExternalIssuer).
		Return(idpDTO, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", token, testExternalJWKS).Return(nil)

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
		"aud": "https://thunder.io",
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)
	idpDTO := buildExternalIDPDTO()

	suite.mockIDPService.On("GetIdentityProviderByIssuer", context.Background(), testExternalIssuer).
		Return(idpDTO, nil)
	suite.mockJWTService.On("VerifyJWTSignatureWithJWKS", token, testExternalJWKS).
		Return(&serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Code:  "SIGNATURE_VERIFICATION_FAILED",
			Error: core.I18nMessage{Key: "error.test.sig_failed", DefaultValue: "Signature verification failed"},
			ErrorDescription: core.I18nMessage{
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
		"aud": "https://thunder.io",
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)

	propTokenExchange, _ := cmodels.NewProperty(idp.PropTokenExchangeEnabled, "false", false)
	propJWKS, _ := cmodels.NewProperty(idp.PropJwksEndpoint, testExternalJWKS, false)
	propIssuer, _ := cmodels.NewProperty(idp.PropIssuer, testExternalIssuer, false)
	idpDTO := &idp.IDPDTO{
		Properties: []cmodels.Property{*propTokenExchange, *propJWKS, *propIssuer},
	}

	suite.mockIDPService.On("GetIdentityProviderByIssuer", context.Background(), testExternalIssuer).
		Return(idpDTO, nil)

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
		"aud": "https://thunder.io",
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)

	propTokenExchange, _ := cmodels.NewProperty(idp.PropTokenExchangeEnabled, "true", false)
	propIssuer, _ := cmodels.NewProperty(idp.PropIssuer, testExternalIssuer, false)
	idpDTO := &idp.IDPDTO{
		Properties: []cmodels.Property{*propTokenExchange, *propIssuer},
	}

	suite.mockIDPService.On("GetIdentityProviderByIssuer", context.Background(), testExternalIssuer).
		Return(idpDTO, nil)

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
		"aud": "https://thunder.io",
		"exp": float64(now + 3600),
	}
	token := suite.createExternalJWT(claims)

	suite.mockIDPService.On("GetIdentityProviderByIssuer", context.Background(), unknownIssuer).
		Return(nil, &serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  "IDP_NOT_FOUND",
			Error: core.I18nMessage{Key: "error.test.idp_not_found", DefaultValue: "IDP not found"},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.idp_not_found_desc", DefaultValue: "No IDP found for the given issuer",
			},
		})

	result, err := suite.validator.ValidateSubjectToken(context.Background(), token, suite.oauthApp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to exchange token for issuer")
	suite.mockIDPService.AssertExpectations(suite.T())
}
