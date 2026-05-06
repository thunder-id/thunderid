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

package authn

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/asgardeo/thunder/internal/authn/assert"
	"github.com/asgardeo/thunder/internal/authn/common"
	"github.com/asgardeo/thunder/internal/authn/passkey"
	authnprovidercm "github.com/asgardeo/thunder/internal/authnprovider/common"
	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/idp"
	notifcommon "github.com/asgardeo/thunder/internal/notification/common"
	"github.com/asgardeo/thunder/internal/system/config"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	"github.com/asgardeo/thunder/internal/system/i18n/core"
	"github.com/asgardeo/thunder/internal/system/log"
	"github.com/asgardeo/thunder/tests/mocks/authn/assertmock"
	"github.com/asgardeo/thunder/tests/mocks/authn/githubmock"
	"github.com/asgardeo/thunder/tests/mocks/authn/googlemock"
	"github.com/asgardeo/thunder/tests/mocks/authn/oauthmock"
	"github.com/asgardeo/thunder/tests/mocks/authn/oidcmock"
	"github.com/asgardeo/thunder/tests/mocks/authn/otpmock"
	"github.com/asgardeo/thunder/tests/mocks/authn/passkeymock"
	"github.com/asgardeo/thunder/tests/mocks/authnprovider/managermock"
	"github.com/asgardeo/thunder/tests/mocks/idp/idpmock"
	"github.com/asgardeo/thunder/tests/mocks/jose/jwtmock"
)

const (
	testUserID           = "user123"
	testUserType         = "person"
	testIDPID            = "idp_123"
	testOrgUnit          = "org_unit_123"
	testAuthCode         = "auth_code_123"
	testToken            = "token_123"
	testSessionTkn       = "session_token_123"
	testJWTToken         = "jwt_token_123" // #nosec G101
	testRedirectURL      = "https://oauth.provider.com/authorize"
	invalidAssertion     = "invalid.jwt.token"
	testRelyingPartyID   = "example.com"
	testRelyingPartyName = "Example Inc"
	testCredentialID     = "credential-id-123" // #nosec G101
	testCredentialType   = "public-key"
)

type AuthenticationServiceTestSuite struct {
	suite.Suite
	mockIDPService      *idpmock.IDPServiceInterfaceMock
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockAssertGenerator *assertmock.AuthAssertGeneratorInterfaceMock
	mockAuthnProvider   *managermock.AuthnProviderManagerInterfaceMock
	mockOTPService      *otpmock.OTPAuthnServiceInterfaceMock
	mockOAuthService    *oauthmock.OAuthAuthnServiceInterfaceMock
	mockOIDCService     *oidcmock.OIDCAuthnServiceInterfaceMock
	mockGoogleService   *googlemock.GoogleOIDCAuthnServiceInterfaceMock
	mockGithubService   *githubmock.GithubOAuthAuthnServiceInterfaceMock
	mockPasskeyService  *passkeymock.WebAuthnAuthnServiceInterfaceMock
	service             *authenticationService
}

func TestAuthenticationServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthenticationServiceTestSuite))
}

func (suite *AuthenticationServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         mock.Anything,
			ValidityPeriod: 3600,
			Audience:       "application",
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *AuthenticationServiceTestSuite) SetupTest() {
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockAssertGenerator = &assertmock.AuthAssertGeneratorInterfaceMock{}
	suite.mockAuthnProvider = &managermock.AuthnProviderManagerInterfaceMock{}
	suite.mockOTPService = &otpmock.OTPAuthnServiceInterfaceMock{}
	suite.mockOAuthService = &oauthmock.OAuthAuthnServiceInterfaceMock{}
	suite.mockOIDCService = &oidcmock.OIDCAuthnServiceInterfaceMock{}
	suite.mockGoogleService = &googlemock.GoogleOIDCAuthnServiceInterfaceMock{}
	suite.mockGithubService = &githubmock.GithubOAuthAuthnServiceInterfaceMock{}
	suite.mockPasskeyService = passkeymock.NewWebAuthnAuthnServiceInterfaceMock(suite.T())

	suite.service = &authenticationService{
		idpService:             suite.mockIDPService,
		jwtService:             suite.mockJWTService,
		authAssertionGenerator: suite.mockAssertGenerator,
		authnProvider:          suite.mockAuthnProvider,
		otpService:             suite.mockOTPService,
		oauthService:           suite.mockOAuthService,
		oidcService:            suite.mockOIDCService,
		googleService:          suite.mockGoogleService,
		githubService:          suite.mockGithubService,
		passkeyService:         suite.mockPasskeyService,
	}
}

func (suite *AuthenticationServiceTestSuite) TestAuthenticateWithCredentials() {
	identifiers := map[string]interface{}{
		"username": "testuser",
	}
	authnCredentials := map[string]interface{}{
		"password": "testpass",
	}

	testCases := []struct {
		name              string
		skipAssertion     bool
		existingAssertion string
		expectAssertion   bool
		validateClaims    bool
		setupMocks        func()
		validateAssertion func(result *common.AuthenticationResponse)
	}{
		{
			name:            "Success without assertion",
			skipAssertion:   true,
			expectAssertion: false,
			setupMocks: func() {
				var mockAuthUser authnprovidermgr.AuthUser
				mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],` +
					`"userHistory":[{"userId":"user123","userType":"person","ouId":"org_unit_123",` +
					`"isValuesIncluded":true}],"userState":"exists"}`
				_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything,
					authnprovidercm.AuthnDataTypeCredentials,
					&authnprovidercm.CredentialsAuthnData{
						Identifiers: identifiers,
						Credentials: authnCredentials,
					}, mock.Anything, mock.Anything, mock.Anything).
					Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()
			},
			validateAssertion: func(result *common.AuthenticationResponse) {
				suite.Empty(result.Assertion)
			},
		},
		{
			name:            "Success with assertion generation",
			skipAssertion:   false,
			expectAssertion: true,
			validateClaims:  true,
			setupMocks: func() {
				var mockAuthUser authnprovidermgr.AuthUser
				mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":` +
					`"user123","userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":` +
					`"exists"}`
				_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything,
					authnprovidercm.AuthnDataTypeCredentials,
					&authnprovidercm.CredentialsAuthnData{
						Identifiers: identifiers,
						Credentials: authnCredentials,
					}, mock.Anything, mock.Anything, mock.Anything).
					Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()
				suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel1,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything, mock.Anything,
					mock.MatchedBy(func(claims map[string]interface{}) bool {
						// Verify that assurance claims are present
						_, hasAssurance := claims["assurance"]
						return hasAssurance
					}), mock.Anything, mock.Anything).Return(testJWTToken, int64(3600), nil).Once()
			},
			validateAssertion: func(result *common.AuthenticationResponse) {
				suite.Equal(testJWTToken, result.Assertion)
			},
		},
		{
			name:              "Success with existing assertion",
			skipAssertion:     false,
			existingAssertion: "", // Will be set in setupMocks
			expectAssertion:   true,
			validateClaims:    true,
			setupMocks: func() {
				var mockAuthUser authnprovidermgr.AuthUser
				mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":` +
					`"user123","userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],` +
					`"userState":"exists"}`
				_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything,
					authnprovidercm.AuthnDataTypeCredentials,
					&authnprovidercm.CredentialsAuthnData{
						Identifiers: identifiers,
						Credentials: authnCredentials,
					}, mock.Anything, mock.Anything, mock.Anything).
					Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()
				suite.mockJWTService.On("VerifyJWT", mock.Anything, "", mock.Anything).Return(nil).Once()
				suite.mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel2,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return(testJWTToken, int64(3600), nil).Once()
			},
			validateAssertion: func(result *common.AuthenticationResponse) {
				suite.Equal(testJWTToken, result.Assertion)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			// Create existing assertion if needed
			existingAssertion := tc.existingAssertion
			if tc.name == "Success with existing assertion" {
				existingAssertion = suite.createTestAssertion(testUserID)
			}

			result, err := suite.service.AuthenticateWithCredentials(
				context.Background(), identifiers, authnCredentials, tc.skipAssertion, existingAssertion)

			suite.Nil(err)
			suite.NotNil(result)
			suite.Equal(testUserID, result.ID)
			suite.Equal(testOrgUnit, result.OUID)
			tc.validateAssertion(result)
		})
	}
}

func (suite *AuthenticationServiceTestSuite) TestAuthenticateWithCredentialsServiceError() {
	identifiers := map[string]interface{}{
		"username": "testuser",
	}
	authnCredentials := map[string]interface{}{
		"password": "wrongpass",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything,
		authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers,
			Credentials: authnCredentials,
		}, mock.Anything, mock.Anything, mock.Anything).Return(
		authnprovidermgr.AuthUser{}, &authnprovidermgr.ErrorAuthenticationFailed)

	result, err := suite.service.AuthenticateWithCredentials(context.Background(), identifiers,
		authnCredentials, false, "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidCredentials.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestAuthenticateWithCredentialsJWTGenerationError() {
	identifiers := map[string]interface{}{
		"username": "testuser",
	}
	authnCredentials := map[string]interface{}{
		"password": "testpass",
	}

	var mockAuthUser authnprovidermgr.AuthUser
	mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers,
			Credentials: authnCredentials,
		}, mock.Anything, mock.Anything, mock.Anything).Return(
		mockAuthUser, (*serviceerror.ServiceError)(nil))
	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(
		&assert.AssertionResult{
			Context: &assert.AssuranceContext{
				AAL: assert.AALLevel1,
				IAL: assert.IALLevel1,
			},
		}, nil).Once()
	suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "JWT_GENERATION_FAILED",
			Error: core.I18nMessage{
				Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.failed_to_generate_jwt_token", DefaultValue: "Failed to generate JWT token",
			},
		})

	result, err := suite.service.AuthenticateWithCredentials(context.Background(), identifiers,
		authnCredentials, false, "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestAuthenticateWithCredentialsSubjectMismatch() {
	identifiers := map[string]interface{}{
		"username": "testuser",
	}
	authnCredentials := map[string]interface{}{
		"password": "testpass",
	}

	// Create assertion with different subject
	existingAssertion := suite.createTestAssertion("different_user_id")

	var mockAuthUser authnprovidermgr.AuthUser
	mockJSON1 := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(mockJSON1), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers,
			Credentials: authnCredentials,
		}, mock.Anything, mock.Anything, mock.Anything).Return(
		mockAuthUser, (*serviceerror.ServiceError)(nil))
	suite.mockJWTService.On("VerifyJWT", existingAssertion, "", mock.Anything).Return(nil)

	result, err := suite.service.AuthenticateWithCredentials(context.Background(), identifiers,
		authnCredentials, false, existingAssertion)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorAssertionSubjectMismatch.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestAuthenticateWithCredentialsInvalidExistingAssertion() {
	identifiers := map[string]interface{}{
		"username": "testuser",
	}
	authnCredentials := map[string]interface{}{
		"password": "testpass",
	}

	var mockAuthUser authnprovidermgr.AuthUser
	mockJSON2 := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(mockJSON2), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers,
			Credentials: authnCredentials,
		}, mock.Anything, mock.Anything, mock.Anything).Return(
		mockAuthUser, (*serviceerror.ServiceError)(nil))
	suite.mockJWTService.On("VerifyJWT", invalidAssertion, "", mock.Anything).Return(&serviceerror.ServiceError{
		Type:  serviceerror.ServerErrorType,
		Code:  "INVALID_JWT",
		Error: core.I18nMessage{Key: "error.test.invalid_jwt", DefaultValue: "Invalid JWT"},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.the_jwt_signature_is_invalid", DefaultValue: "The JWT signature is invalid",
		},
	})

	result, err := suite.service.AuthenticateWithCredentials(context.Background(), identifiers,
		authnCredentials, false, invalidAssertion)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestAuthenticateWithCredentialsExistingAssertionWithoutAssurance() {
	identifiers := map[string]interface{}{
		"username": "testuser",
	}
	authnCredentials := map[string]interface{}{
		"password": "testpass",
	}

	// Create assertion without assurance claim
	existingAssertion := suite.createTestAssertionWithoutAssurance(testUserID)

	var mockAuthUser authnprovidermgr.AuthUser
	mockJSON3 := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(mockJSON3), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, authnprovidercm.AuthnDataTypeCredentials,
		&authnprovidercm.CredentialsAuthnData{
			Identifiers: identifiers,
			Credentials: authnCredentials,
		}, mock.Anything, mock.Anything, mock.Anything).Return(
		mockAuthUser, (*serviceerror.ServiceError)(nil))
	suite.mockJWTService.On("VerifyJWT", existingAssertion, "", mock.Anything).Return(nil)

	result, err := suite.service.AuthenticateWithCredentials(context.Background(), identifiers,
		authnCredentials, false, existingAssertion)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestSendOTPSuccess() {
	senderID := "sender_123"
	recipient := "+1234567890"
	sessionToken := testSessionTkn

	suite.mockOTPService.On("SendOTP", mock.Anything, senderID, notifcommon.ChannelTypeSMS, recipient).
		Return(sessionToken, nil)

	result, err := suite.service.SendOTP(context.Background(), senderID, notifcommon.ChannelTypeSMS, recipient)

	suite.Nil(err)
	suite.Equal(sessionToken, result)
}

func (suite *AuthenticationServiceTestSuite) TestSendOTPServiceError() {
	senderID := "sender_123"
	recipient := "+1234567890"
	svcErr := &serviceerror.ServiceError{
		Type:             serviceerror.ClientErrorType,
		Code:             "OTP_ERROR",
		Error:            core.I18nMessage{Key: "error.test.otp_error", DefaultValue: "OTP error"},
		ErrorDescription: core.I18nMessage{Key: "error.test.failed_to_send_otp", DefaultValue: "Failed to send OTP"},
	}

	suite.mockOTPService.On("SendOTP", mock.Anything, senderID, notifcommon.ChannelTypeSMS, recipient).
		Return("", svcErr)

	result, err := suite.service.SendOTP(context.Background(), senderID, notifcommon.ChannelTypeSMS, recipient)

	suite.Empty(result)
	suite.NotNil(err)
	suite.Equal(svcErr.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyOTP() {
	sessionToken := testSessionTkn
	otpCode := "123456"

	testCases := []struct {
		name              string
		skipAssertion     bool
		existingAssertion string
		expectAssertion   bool
		setupMocks        func()
		validateAssertion func(result *common.AuthenticationResponse)
	}{
		{
			name:              "Success without assertion",
			skipAssertion:     true,
			existingAssertion: "",
			expectAssertion:   false,
			setupMocks: func() {
				var mockAuthUser authnprovidermgr.AuthUser
				mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":` +
					`"user123","userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":` +
					`"exists"}`
				_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
				suite.mockAuthnProvider.On("AuthenticateUser",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()
			},
			validateAssertion: func(result *common.AuthenticationResponse) {
				suite.Empty(result.Assertion)
			},
		},
		{
			name:              "Success with assertion generation",
			skipAssertion:     false,
			existingAssertion: "",
			expectAssertion:   true,
			setupMocks: func() {
				var mockAuthUser authnprovidermgr.AuthUser
				mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":` +
					`"user123","userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":` +
					`"exists"}`
				_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
				suite.mockAuthnProvider.On("AuthenticateUser",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()
				suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel1,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything, mock.Anything,
					mock.MatchedBy(func(claims map[string]interface{}) bool {
						// Verify that assurance claims are present
						_, hasAssurance := claims["assurance"]
						return hasAssurance
					}), mock.Anything, mock.Anything).Return(testJWTToken, int64(3600), nil).Once()
			},
			validateAssertion: func(result *common.AuthenticationResponse) {
				suite.Equal(testJWTToken, result.Assertion)
			},
		},
		{
			name:              "Success with existing assertion (MFA)",
			skipAssertion:     false,
			existingAssertion: suite.createTestAssertion(testUserID),
			expectAssertion:   true,
			setupMocks: func() {
				existingAssertion := suite.createTestAssertion(testUserID)
				var mockAuthUser authnprovidermgr.AuthUser
				mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":` +
					`"user123","userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":` +
					`"exists"}`
				_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
				suite.mockAuthnProvider.On("AuthenticateUser",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()
				suite.mockJWTService.On("VerifyJWT", existingAssertion, "", mock.Anything).Return(nil).Once()
				suite.mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel2,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything, mock.Anything,
					mock.MatchedBy(func(claims map[string]interface{}) bool {
						// Verify that assurance claims are present for MFA
						_, hasAssurance := claims["assurance"]
						return hasAssurance
					}), mock.Anything, mock.Anything).Return("new_jwt_token_with_mfa", int64(3600), nil).Once()
			},
			validateAssertion: func(result *common.AuthenticationResponse) {
				suite.NotEmpty(result.Assertion)
				suite.Equal("new_jwt_token_with_mfa", result.Assertion)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			result, err := suite.service.VerifyOTP(context.Background(), sessionToken, tc.skipAssertion,
				tc.existingAssertion, otpCode)

			suite.Nil(err)
			suite.NotNil(result)
			suite.Equal(testUserID, result.ID)
			tc.validateAssertion(result)
		})
	}
}

func (suite *AuthenticationServiceTestSuite) TestVerifyOTPServiceError() {
	sessionToken := testSessionTkn
	otpCode := "wrong_otp"

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.ErrorAuthenticationFailed)

	result, err := suite.service.VerifyOTP(context.Background(), sessionToken, false, "", otpCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorOTPAuthenticationFailed.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationOAuthSuccess() {
	idpID := testIDPID
	redirectURL := testRedirectURL
	identityProvider := &idp.IDPDTO{
		ID:   idpID,
		Type: idp.IDPTypeOAuth,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, idpID).Return(redirectURL, nil)
	suite.mockJWTService.On("GenerateJWT", "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testSessionTkn, int64(600), nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeOAuth, idpID)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(redirectURL, result.RedirectURL)
	suite.Equal(testSessionTkn, result.SessionToken)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationOIDCSuccess() {
	idpID := testIDPID
	redirectURL := "https://oidc.provider.com/authorize"
	identityProvider := &idp.IDPDTO{
		ID:   idpID,
		Type: idp.IDPTypeOIDC,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockOIDCService.On("BuildAuthorizeURL", mock.Anything, idpID).Return(redirectURL, nil)
	suite.mockJWTService.On("GenerateJWT", "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testSessionTkn, int64(600), nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeOIDC, idpID)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(redirectURL, result.RedirectURL)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationGoogleSuccess() {
	idpID := testIDPID
	redirectURL := "https://accounts.google.com/o/oauth2/v2/auth"
	identityProvider := &idp.IDPDTO{
		ID:   idpID,
		Type: idp.IDPTypeGoogle,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockGoogleService.On("BuildAuthorizeURL", mock.Anything, idpID).Return(redirectURL, nil)
	suite.mockJWTService.On("GenerateJWT", "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testSessionTkn, int64(600), nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeGoogle, idpID)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(redirectURL, result.RedirectURL)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationGitHubSuccess() {
	idpID := testIDPID
	redirectURL := "https://github.com/login/oauth/authorize"
	identityProvider := &idp.IDPDTO{
		ID:   idpID,
		Type: idp.IDPTypeGitHub,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockGithubService.On("BuildAuthorizeURL", mock.Anything, idpID).Return(redirectURL, nil)
	suite.mockJWTService.On("GenerateJWT", "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testSessionTkn, int64(600), nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeGitHub, idpID)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(redirectURL, result.RedirectURL)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationEmptyIDPID() {
	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeOAuth, "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidIDPID.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationIDPNotFound() {
	idpID := "nonexistent_idp"
	svcErr := &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "IDP_NOT_FOUND",
		Error: core.I18nMessage{Key: "error.test.idp_not_found", DefaultValue: "IDP not found"},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.the_identity_provider_was_not_found", DefaultValue: "The identity provider was not found",
		},
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(nil, svcErr)

	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeOAuth, idpID)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Contains(err.ErrorDescription.DefaultValue, idpID)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationInvalidIDPType() {
	idpID := testIDPID
	identityProvider := &idp.IDPDTO{
		ID:   idpID,
		Type: idp.IDPTypeGoogle,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeGitHub, idpID)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidIDPType.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationCrossTypeAllowed() {
	idpID := testIDPID
	redirectURL := testRedirectURL
	identityProvider := &idp.IDPDTO{
		ID:   idpID,
		Type: idp.IDPTypeOAuth,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, idpID).Return(redirectURL, nil)
	suite.mockJWTService.On("GenerateJWT", "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testSessionTkn, int64(600), nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeOIDC, idpID)

	suite.Nil(err)
	suite.NotNil(result)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationJWTGenerationError() {
	idpID := testIDPID
	redirectURL := testRedirectURL
	identityProvider := &idp.IDPDTO{
		ID:   idpID,
		Type: idp.IDPTypeOAuth,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, idpID).Return(redirectURL, nil)
	suite.mockJWTService.On("GenerateJWT", "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "JWT_GENERATION_FAILED",
			Error: core.I18nMessage{
				Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.failed_to_generate_session_token", DefaultValue: "Failed to generate session token",
			},
		})

	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeOAuth, idpID)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) mockFederatedAuthnSuccess(idpType idp.IDPType) string {
	sessionToken := suite.createSessionToken(idpType)
	suite.mockJWTService.On("VerifyJWT", sessionToken, "auth-svc", mock.Anything).Return(nil)
	var mockAuthUser authnprovidermgr.AuthUser
	fedJSON := `{"authHistory":[{"authType":"OIDC","isVerified":true,"runtimeAttributes":{"sub":"EXT_SUB"}}],` +
		`"userHistory":[{"userId":"user123","userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],` +
		`"userState":"exists"}`
	_ = json.Unmarshal([]byte(fedJSON), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(data *authnprovidercm.FederatedAuthnData) bool {
			return data != nil
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()
	return sessionToken
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationOAuthSuccess() {
	sessionToken := suite.mockFederatedAuthnSuccess(idp.IDPTypeOAuth)
	result, err := suite.service.FinishIDPAuthentication(context.Background(), idp.IDPTypeOAuth, sessionToken, true, "",
		testAuthCode)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
	suite.Empty(result.Assertion)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationOIDCSuccess() {
	sessionToken := suite.mockFederatedAuthnSuccess(idp.IDPTypeOIDC)
	result, err := suite.service.FinishIDPAuthentication(context.Background(), idp.IDPTypeOIDC, sessionToken, true, "",
		testAuthCode)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationGoogleSuccess() {
	sessionToken := suite.mockFederatedAuthnSuccess(idp.IDPTypeGoogle)
	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), idp.IDPTypeGoogle, sessionToken, true, "", testAuthCode)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationGitHubSuccess() {
	sessionToken := suite.mockFederatedAuthnSuccess(idp.IDPTypeGitHub)
	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), idp.IDPTypeGitHub, sessionToken, true, "", testAuthCode)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationWithAssertion() {
	testCases := []struct {
		name              string
		skipAssertion     bool
		existingAssertion string
		setupMocks        func()
		validateAssertion func(result *common.AuthenticationResponse)
	}{
		{
			name:              "Success with assertion generation",
			skipAssertion:     false,
			existingAssertion: "",
			setupMocks: func() {
				sessionToken := suite.createSessionToken(idp.IDPTypeOAuth)
				suite.mockJWTService.On("VerifyJWT", sessionToken, "auth-svc", mock.Anything).Return(nil).Once()
				var mockAuthUser authnprovidermgr.AuthUser
				fedJSON := `{"authHistory":[{"authType":"OIDC","isVerified":true,"runtimeAttributes":` +
					`{"sub":"EXT_SUB"}}],"userHistory":[{"userId":"user123","userType":"person","ouId":` +
					`"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
				_ = json.Unmarshal([]byte(fedJSON), &mockAuthUser)
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
					mock.MatchedBy(func(data *authnprovidercm.FederatedAuthnData) bool {
						return data != nil
					}), mock.Anything, mock.Anything, mock.Anything).
					Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()
				suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel1,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT",
					testUserID, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
				).Return(testJWTToken, int64(3600), nil).Once()
			},
			validateAssertion: func(result *common.AuthenticationResponse) {
				suite.Equal(testJWTToken, result.Assertion)
			},
		},
		{
			name:              "Success with existing assertion (MFA)",
			skipAssertion:     false,
			existingAssertion: suite.createTestAssertion(testUserID),
			setupMocks: func() {
				sessionToken := suite.createSessionToken(idp.IDPTypeOAuth)
				existingAssertion := suite.createTestAssertion(testUserID)
				suite.mockJWTService.On("VerifyJWT", sessionToken, "auth-svc", mock.Anything).Return(nil).Once()
				suite.mockJWTService.On("VerifyJWT", existingAssertion, "", mock.Anything).Return(nil).Once()
				var mockAuthUser authnprovidermgr.AuthUser
				fedJSON2 := `{"authHistory":[{"authType":"OIDC","isVerified":true,"runtimeAttributes":` +
					`{"sub":"EXT_SUB"}}],"userHistory":[{"userId":"user123","userType":"person","ouId":` +
					`"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
				_ = json.Unmarshal([]byte(fedJSON2), &mockAuthUser)
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
					mock.MatchedBy(func(data *authnprovidercm.FederatedAuthnData) bool {
						return data != nil
					}), mock.Anything, mock.Anything, mock.Anything).
					Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()
				suite.mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel2,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything, mock.Anything,
					mock.MatchedBy(func(claims map[string]interface{}) bool {
						_, hasAssurance := claims["assurance"]
						return hasAssurance
					}), mock.Anything, mock.Anything).Return("new_jwt_token_with_mfa", int64(3600), nil).Once()
			},
			validateAssertion: func(result *common.AuthenticationResponse) {
				suite.NotEmpty(result.Assertion)
				suite.Equal("new_jwt_token_with_mfa", result.Assertion)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			sessionToken := suite.createSessionToken(idp.IDPTypeOAuth)
			result, err := suite.service.FinishIDPAuthentication(context.Background(), idp.IDPTypeOAuth, sessionToken,
				tc.skipAssertion, tc.existingAssertion, testAuthCode)

			suite.Nil(err)
			suite.NotNil(result)
			suite.Equal(testUserID, result.ID)
			tc.validateAssertion(result)
		})
	}
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationEmptySessionToken() {
	result, err := suite.service.FinishIDPAuthentication(context.Background(), idp.IDPTypeOAuth, "", false, "",
		testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorEmptySessionToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationEmptyAuthCode() {
	sessionToken := suite.createSessionToken(idp.IDPTypeOAuth)

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), idp.IDPTypeOAuth, sessionToken, false, "",
		"")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorEmptyAuthCode.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationInvalidSessionToken() {
	suite.mockJWTService.On("VerifyJWT", "invalid_token", "auth-svc", mock.Anything).
		Return(&serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Code:  "INVALID_TOKEN",
			Error: core.I18nMessage{Key: "error.test.invalid_token", DefaultValue: "Invalid token"},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.the_session_token_is_invalid", DefaultValue: "The session token is invalid",
			},
		})

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), idp.IDPTypeOAuth, "invalid_token", false, "", testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationTypeMismatch() {
	sessionToken := suite.createSessionToken(idp.IDPTypeGoogle)
	suite.mockJWTService.On("VerifyJWT", sessionToken, "auth-svc", mock.Anything).Return(nil)

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), idp.IDPTypeGitHub, sessionToken, false, "",
		testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidIDPType.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationUserNotFound() {
	sessionToken := suite.createSessionToken(idp.IDPTypeOAuth)
	suite.mockJWTService.On("VerifyJWT", sessionToken, "auth-svc", mock.Anything).Return(nil)
	var mockAuthUser authnprovidermgr.AuthUser
	notExistsJSON := `{"authHistory":[{"authType":"OIDC","isVerified":false,"runtimeAttributes":{"sub":"EXT_SUB"}}],` +
		`"userHistory":[],"userState":"not_exists"}`
	_ = json.Unmarshal([]byte(notExistsJSON), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(data *authnprovidercm.FederatedAuthnData) bool {
			return data != nil
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), idp.IDPTypeOAuth, sessionToken, false, "",
		testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorUserNotFound.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationProviderAuthFailure() {
	sessionToken := suite.createSessionToken(idp.IDPTypeOAuth)
	suite.mockJWTService.On("VerifyJWT", sessionToken, "auth-svc", mock.Anything).Return(nil)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(data *authnprovidercm.FederatedAuthnData) bool {
			return data != nil
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.ErrorAuthenticationFailed).Once()

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), idp.IDPTypeOAuth, sessionToken, false, "",
		testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorFederatedAuthenticationFailed.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestValidateIDPTypeExactMatch() {
	err := suite.service.validateIDPType(idp.IDPTypeOAuth, idp.IDPTypeOAuth, nil)
	suite.Nil(err)
}

func (suite *AuthenticationServiceTestSuite) TestValidateIDPTypeEmptyRequested() {
	err := suite.service.validateIDPType("", idp.IDPTypeOAuth, nil)
	suite.Nil(err)
}

func (suite *AuthenticationServiceTestSuite) TestValidateIDPTypeCrossAllowed() {
	err := suite.service.validateIDPType(idp.IDPTypeOAuth, idp.IDPTypeOIDC, nil)
	suite.Nil(err)

	err = suite.service.validateIDPType(idp.IDPTypeOIDC, idp.IDPTypeOAuth, nil)
	suite.Nil(err)
}

func (suite *AuthenticationServiceTestSuite) TestValidateIDPTypeMismatch() {
	logger := log.GetLogger()
	err := suite.service.validateIDPType(idp.IDPTypeGoogle, idp.IDPTypeGitHub, logger)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidIDPType.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestHandleIDPServiceErrorServerError() {
	idpID := "test_idp"
	svcErr := &serviceerror.ServiceError{
		Type:  serviceerror.ServerErrorType,
		Code:  "INTERNAL_ERROR",
		Error: core.I18nMessage{Key: "error.test.internal_error", DefaultValue: "Internal error"},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.internal_error_description", DefaultValue: "Database connection failed",
		},
	}
	logger := log.GetLogger()

	result := suite.service.handleIDPServiceError(idpID, svcErr, logger)

	suite.NotNil(result)
	suite.Equal(serviceerror.InternalServerError.Code, result.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyAndDecodeSessionTokenMalformedPayload() {
	logger := log.GetLogger()
	badToken := "header.invalid-base64.signature"

	suite.mockJWTService.On("VerifyJWT", badToken, "auth-svc", mock.Anything).Return(nil)

	result, err := suite.service.verifyAndDecodeSessionToken(badToken, logger)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyAndDecodeSessionTokenMissingAuthData() {
	logger := log.GetLogger()
	payload := map[string]interface{}{
		"sub": "test",
	}
	payloadBytes, _ := json.Marshal(payload)
	encoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	tokenWithoutAuthData := "header." + encoded + ".signature"

	suite.mockJWTService.On("VerifyJWT", tokenWithoutAuthData, "auth-svc", mock.Anything).Return(nil)

	result, err := suite.service.verifyAndDecodeSessionToken(tokenWithoutAuthData, logger)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationBuildURLError() {
	idpID := testIDPID
	identityProvider := &idp.IDPDTO{
		ID:   idpID,
		Type: idp.IDPTypeOAuth,
	}
	svcErr := &serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "INVALID_CONFIG",
		Error: core.I18nMessage{
			Key: "error.test.invalid_configuration", DefaultValue: "Invalid configuration",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.missing_redirect_uri", DefaultValue: "Missing redirect URI",
		},
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, idpID).Return("", svcErr)

	result, err := suite.service.StartIDPAuthentication(context.Background(), idp.IDPTypeOAuth, idpID)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(svcErr.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationProviderServerError() {
	sessionToken := suite.createSessionToken(idp.IDPTypeOIDC)
	suite.mockJWTService.On("VerifyJWT", sessionToken, "auth-svc", mock.Anything).Return(nil)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(data *authnprovidercm.FederatedAuthnData) bool {
			return data != nil
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &serviceerror.InternalServerError).Once()

	result, err := suite.service.FinishIDPAuthentication(context.Background(), idp.IDPTypeOIDC, sessionToken, true, "",
		testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) createSessionToken(idpType idp.IDPType) string {
	sessionData := AuthSessionData{
		IDPID:   testIDPID,
		IDPType: idpType,
	}
	payload := map[string]interface{}{
		"auth_data": sessionData,
	}
	payloadBytes, _ := json.Marshal(payload)
	encoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return "header." + encoded + ".signature"
}

func (suite *AuthenticationServiceTestSuite) TestValidateAndAppendAuthAssertionExtractClaimsError() {
	var mockAuthUser authnprovidermgr.AuthUser
	mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	logger := log.GetLogger()

	// Create assertion without sub claim
	payload := map[string]interface{}{
		"assurance": map[string]interface{}{
			"aal": "aal1",
			"ial": "ial1",
			"authenticators": []map[string]interface{}{
				{
					"authenticator": "CredentialsAuthenticator",
					"step":          1,
					"timestamp":     int64(1735689600),
				},
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	invalidAssertion := "header." + encodedPayload + ".signature"

	suite.mockJWTService.On("VerifyJWT", invalidAssertion, "", mock.Anything).Return(nil).Once()

	svcErr := suite.service.validateAndAppendAuthAssertion(
		authResponse, &mockAuthUser, invalidAssertion, logger)

	suite.NotNil(svcErr)
	suite.Equal(common.ErrorInvalidAssertion.Code, svcErr.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationAssertionGenerationError() {
	sessionToken := suite.createSessionToken(idp.IDPTypeOAuth)
	suite.mockJWTService.On("VerifyJWT", sessionToken, "auth-svc", mock.Anything).Return(nil).Once()
	var mockAuthUser authnprovidermgr.AuthUser
	assertErrFedJSON := `{"authHistory":[{"authType":"OIDC","isVerified":true,"runtimeAttributes":` +
		`{"sub":"EXT_SUB"}}],"userHistory":[{"userId":"user123","userType":"person","ouId":"org_unit_123",` +
		`"isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(assertErrFedJSON), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(data *authnprovidercm.FederatedAuthnData) bool {
			return data != nil
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()

	// Create invalid existing assertion that will fail JWT verification
	suite.mockJWTService.On("VerifyJWT", invalidAssertion, "", mock.Anything).
		Return(&serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Code:  "INVALID_SIGNATURE",
			Error: core.I18nMessage{Key: "error.test.invalid_signature", DefaultValue: "Invalid signature"},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.the_jwt_signature_is_invalid", DefaultValue: "The JWT signature is invalid",
			},
		}).Once()

	result, err := suite.service.FinishIDPAuthentication(context.Background(), idp.IDPTypeOAuth, sessionToken, false,
		invalidAssertion, testAuthCode)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestValidateAndAppendAuthAssertionStepOne() {
	var mockAuthUser authnprovidermgr.AuthUser
	mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	logger := log.GetLogger()

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(
		&assert.AssertionResult{
			Context: &assert.AssuranceContext{
				AAL: assert.AALLevel1,
				IAL: assert.IALLevel1,
			},
		}, nil).Once()
	suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testJWTToken, int64(3600), nil).Once()

	// Test with empty existingAssertion
	svcErr := suite.service.validateAndAppendAuthAssertion(
		authResponse, &mockAuthUser, "", logger)
	suite.Nil(svcErr)
	suite.Equal(testJWTToken, authResponse.Assertion)
}

func (suite *AuthenticationServiceTestSuite) TestValidateAndAppendAuthAssertionSubjectMismatch() {
	var mockAuthUser authnprovidermgr.AuthUser
	mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}

	// Create assertion with different subject
	existingAssertion := suite.createTestAssertion("different_user_id")

	suite.mockJWTService.On("VerifyJWT", existingAssertion, "", mock.Anything).Return(nil)

	svcErr := suite.service.validateAndAppendAuthAssertion(
		authResponse, &mockAuthUser, existingAssertion, log.GetLogger())

	suite.NotNil(svcErr)
	suite.Equal(common.ErrorAssertionSubjectMismatch.Code, svcErr.Code)
}

func (suite *AuthenticationServiceTestSuite) TestExtractClaimsFromAssertionMissingAssurance() {
	// Create assertion without assurance claim
	assertionWithoutAssurance := suite.createTestAssertionWithoutAssurance(testUserID)

	suite.mockJWTService.On("VerifyJWT", assertionWithoutAssurance, "", mock.Anything).Return(nil)

	_, _, svcErr := suite.service.extractClaimsFromAssertion(
		assertionWithoutAssurance, log.GetLogger())

	suite.NotNil(svcErr)
	suite.Equal(common.ErrorInvalidAssertion.Code, svcErr.Code)
}

func (suite *AuthenticationServiceTestSuite) TestExtractClaimsFromAssertionErrorCases() {
	logger := log.GetLogger()

	testCases := []struct {
		name      string
		payload   map[string]interface{}
		setupMock func(assertion string)
	}{
		{
			name: "MissingSubClaim",
			payload: map[string]interface{}{
				"assurance": map[string]interface{}{
					"aal": "aal1",
					"ial": "ial1",
					"authenticators": []map[string]interface{}{
						{
							"authenticator": "CredentialsAuthenticator",
							"step":          1,
							"timestamp":     int64(1735689600),
						},
					},
				},
			},
			setupMock: func(assertion string) {
				suite.mockJWTService.On("VerifyJWT", assertion, "", mock.Anything).Return(nil).Once()
			},
		},
		{
			name: "InvalidSubClaimType",
			payload: map[string]interface{}{
				"sub": 12345, // Invalid: should be string
				"assurance": map[string]interface{}{
					"aal": "aal1",
					"ial": "ial1",
					"authenticators": []map[string]interface{}{
						{
							"authenticator": "CredentialsAuthenticator",
							"step":          1,
							"timestamp":     int64(1735689600),
						},
					},
				},
			},
			setupMock: func(assertion string) {
				suite.mockJWTService.On("VerifyJWT", assertion, "", mock.Anything).Return(nil).Once()
			},
		},
		{
			name: "EmptySubClaim",
			payload: map[string]interface{}{
				"sub": "", // Empty string
				"assurance": map[string]interface{}{
					"aal": "aal1",
					"ial": "ial1",
					"authenticators": []map[string]interface{}{
						{
							"authenticator": "CredentialsAuthenticator",
							"step":          1,
							"timestamp":     int64(1735689600),
						},
					},
				},
			},
			setupMock: func(assertion string) {
				suite.mockJWTService.On("VerifyJWT", assertion, "", mock.Anything).Return(nil).Once()
			},
		},
		{
			name: "MissingAssuranceClaim",
			payload: map[string]interface{}{
				"sub": testUserID,
			},
			setupMock: func(assertion string) {
				suite.mockJWTService.On("VerifyJWT", assertion, "", mock.Anything).Return(nil).Once()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			payloadBytes, _ := json.Marshal(tc.payload)
			encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
			testAssertion := "header." + encodedPayload + ".signature"

			tc.setupMock(testAssertion)

			assuranceCtx, sub, err := suite.service.extractClaimsFromAssertion(testAssertion, logger)

			suite.Nil(assuranceCtx)
			suite.Empty(sub, "sub should be empty for test case: %s", tc.name)
			suite.NotNil(err, "error should not be nil for test case: %s", tc.name)
			suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
		})
	}
}

func (suite *AuthenticationServiceTestSuite) TestExtractClaimsFromAssertionDecodeError() {
	logger := log.GetLogger()

	// Create a malformed JWT that will fail payload decoding
	malformedAssertion := "header.not-valid-base64!!.signature"
	suite.mockJWTService.On("VerifyJWT", malformedAssertion, "", mock.Anything).Return(nil).Once()

	assuranceCtx, sub, err := suite.service.extractClaimsFromAssertion(malformedAssertion, logger)
	suite.Nil(assuranceCtx)
	suite.Empty(sub)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestExtractClaimsFromAssertionUnmarshalError() {
	logger := log.GetLogger()

	// Create assertion with assurance as a value that will fail to unmarshal into AssuranceContext
	validPayload := map[string]interface{}{
		"sub":       testUserID,
		"assurance": []int{1, 2, 3},
	}
	payloadBytes, _ := json.Marshal(validPayload)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	testAssertion := "header." + encodedPayload + ".signature"
	suite.mockJWTService.On("VerifyJWT", testAssertion, "", mock.Anything).Return(nil).Once()

	assuranceCtx, sub, err := suite.service.extractClaimsFromAssertion(testAssertion, logger)
	suite.Nil(assuranceCtx)
	suite.Empty(sub)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyOTPJWTGenerationError() {
	sessionToken := testSessionTkn
	otpCode := "123456"
	var mockAuthUser authnprovidermgr.AuthUser
	otpErrJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(otpErrJSON), &mockAuthUser)

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))
	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(
		&assert.AssertionResult{
			Context: &assert.AssuranceContext{
				AAL: assert.AALLevel1,
				IAL: assert.IALLevel1,
			},
		}, nil).Once()
	suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "JWT_GENERATION_FAILED",
			Error: core.I18nMessage{
				Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.failed_to_generate_jwt_token", DefaultValue: "Failed to generate JWT token",
			},
		})

	result, err := suite.service.VerifyOTP(context.Background(), sessionToken, false, "", otpCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestExtractClaimsFromAssertionInvalidJWTSignature() {
	logger := log.GetLogger()

	suite.mockJWTService.On("VerifyJWT", invalidAssertion, "", mock.Anything).
		Return(&serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Code:  "INVALID_SIGNATURE",
			Error: core.I18nMessage{Key: "error.test.invalid_signature", DefaultValue: "Invalid signature"},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.the_jwt_signature_is_invalid", DefaultValue: "The JWT signature is invalid",
			},
		})

	assuranceCtx, sub, err := suite.service.extractClaimsFromAssertion(invalidAssertion, logger)

	suite.Nil(assuranceCtx)
	suite.Empty(sub)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestExtractClaimsFromAssertionMalformedAssurance() {
	logger := log.GetLogger()

	// Create assertion with invalid assurance structure
	payload := map[string]interface{}{
		"sub":       testUserID,
		"assurance": "invalid_string_instead_of_object",
	}
	payloadBytes, _ := json.Marshal(payload)
	encoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	malformedAssertion := "header." + encoded + ".signature"

	suite.mockJWTService.On("VerifyJWT", malformedAssertion, "", mock.Anything).Return(nil)

	assuranceCtx, sub, err := suite.service.extractClaimsFromAssertion(malformedAssertion, logger)

	suite.Nil(assuranceCtx)
	suite.Empty(sub)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestValidateAndAppendAuthAssertionGenerationError() {
	var mockAuthUser authnprovidermgr.AuthUser
	mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	logger := log.GetLogger()

	// Create a service with a mock assertion generator that returns an error
	mockAssertGenerator := assertmock.NewAuthAssertGeneratorInterfaceMock(suite.T())
	mockAssertGenerator.On("GenerateAssertion", mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "ASSERTION_ERROR",
			Error: core.I18nMessage{
				Key: "error.test.assertion_generation_failed", DefaultValue: "Assertion generation failed",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.failed_to_generate_assertion", DefaultValue: "Failed to generate assertion",
			},
		})

	service := &authenticationService{
		authAssertionGenerator: mockAssertGenerator,
		jwtService:             suite.mockJWTService,
	}

	err := service.validateAndAppendAuthAssertion(authResponse, &mockAuthUser, "", logger)

	suite.NotNil(err)
	suite.Equal("ASSERTION_ERROR", err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestValidateAndAppendAuthAssertionUpdateError() {
	var mockAuthUser authnprovidermgr.AuthUser
	mockJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[{"userId":"user123",` +
		`"userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(mockJSON), &mockAuthUser)
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	logger := log.GetLogger()
	existingAssertion := suite.createTestAssertion(testUserID)

	suite.mockJWTService.On("VerifyJWT", existingAssertion, "", mock.Anything).Return(nil)

	// Create a service with a mock assertion generator that returns an error on update
	mockAssertGenerator := assertmock.NewAuthAssertGeneratorInterfaceMock(suite.T())
	mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "UPDATE_ERROR",
			Error: core.I18nMessage{
				Key: "error.test.assertion_update_failed", DefaultValue: "Assertion update failed",
			},
			ErrorDescription: core.I18nMessage{
				Key: "error.test.failed_to_update_assertion", DefaultValue: "Failed to update assertion",
			},
		})

	service := &authenticationService{
		authAssertionGenerator: mockAssertGenerator,
		jwtService:             suite.mockJWTService,
	}

	err := service.validateAndAppendAuthAssertion(authResponse, &mockAuthUser, existingAssertion, logger)

	suite.NotNil(err)
	suite.Equal("UPDATE_ERROR", err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyRegistration_Success() {
	attestation := "direct"

	authSelection := &PasskeyAuthenticatorSelectionDTO{
		AuthenticatorAttachment: "platform",
		RequireResidentKey:      true,
		ResidentKey:             "required",
		UserVerification:        "required",
	}

	expectedResponse := &passkey.PasskeyRegistrationStartData{
		SessionToken: testSessionTkn,
	}

	suite.mockPasskeyService.On("StartRegistration", mock.Anything, mock.Anything).Return(expectedResponse, nil).Once()

	result, err := suite.service.StartPasskeyRegistration(
		context.Background(), testUserID, testRelyingPartyID, testRelyingPartyName, authSelection, attestation)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(expectedResponse, result)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyRegistration_WithoutAuthSelection() {
	attestation := ""

	expectedResponse := &passkey.PasskeyRegistrationStartData{
		SessionToken: testSessionTkn,
	}

	suite.mockPasskeyService.On("StartRegistration", mock.Anything, mock.Anything).Return(expectedResponse, nil).Once()

	result, err := suite.service.StartPasskeyRegistration(
		context.Background(), testUserID, testRelyingPartyID, testRelyingPartyName, nil, attestation)

	suite.Nil(err)
	suite.NotNil(result)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyRegistration_ServiceError() {
	serviceError := &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "PASS_ERROR",
		Error: core.I18nMessage{Key: "error.test.passkey_error", DefaultValue: "Passkey error"},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.failed_to_start_registration", DefaultValue: "Failed to start registration",
		},
	}

	suite.mockPasskeyService.On("StartRegistration", mock.Anything, mock.Anything).
		Return(nil, serviceError).Once()

	result, err := suite.service.StartPasskeyRegistration(
		context.Background(), testUserID, testRelyingPartyID, testRelyingPartyName, nil, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(serviceError, err)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyRegistration_Success() {
	credential := PasskeyPublicKeyCredentialDTO{
		ID:   "credential-id-123",
		Type: "public-key",
		Response: PasskeyCredentialResponseDTO{
			ClientDataJSON:    "base64-client-data",
			AttestationObject: "base64-attestation",
		},
	}
	sessionToken := testSessionTkn
	credentialName := "My Passkey"

	expectedResponse := &passkey.PasskeyRegistrationFinishData{
		CredentialID:   "credential-id-123",
		CredentialName: "My Passkey",
	}

	suite.mockPasskeyService.On("FinishRegistration", mock.Anything, mock.Anything).Return(expectedResponse, nil).Once()

	result, err := suite.service.FinishPasskeyRegistration(
		context.Background(), credential, sessionToken, credentialName,
	)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(expectedResponse, result)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyRegistration_WithoutCredentialName() {
	credential := PasskeyPublicKeyCredentialDTO{
		ID:   "credential-id-123",
		Type: "public-key",
		Response: PasskeyCredentialResponseDTO{
			ClientDataJSON:    "base64-client-data",
			AttestationObject: "base64-attestation",
		},
	}
	sessionToken := testSessionTkn

	expectedResponse := &passkey.PasskeyRegistrationFinishData{
		CredentialID: "credential-id-123",
	}

	suite.mockPasskeyService.On("FinishRegistration", mock.Anything, mock.Anything).
		Return(expectedResponse, nil).Once()

	result, err := suite.service.FinishPasskeyRegistration(context.Background(), credential, sessionToken, "")

	suite.Nil(err)
	suite.NotNil(result)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyRegistration_ServiceError() {
	credential := PasskeyPublicKeyCredentialDTO{
		ID:   "credential-id-123",
		Type: "public-key",
		Response: PasskeyCredentialResponseDTO{
			ClientDataJSON:    "base64-client-data",
			AttestationObject: "base64-attestation",
		},
	}

	serviceError := &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "INVALID_ATTESTATION",
		Error: core.I18nMessage{Key: "error.test.invalid_attestation", DefaultValue: "Invalid attestation"},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.failed_to_verify_attestation", DefaultValue: "Failed to verify attestation",
		},
	}

	suite.mockPasskeyService.On("FinishRegistration", mock.Anything, mock.Anything).
		Return(nil, serviceError).Once()

	result, err := suite.service.FinishPasskeyRegistration(context.Background(), credential, testSessionTkn, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(serviceError, err)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyAuthentication_Success() {
	expectedResponse := &passkey.PasskeyAuthenticationStartData{
		SessionToken: testSessionTkn,
	}

	suite.mockPasskeyService.On(
		"StartAuthentication", mock.Anything, mock.MatchedBy(func(req *passkey.PasskeyAuthenticationStartRequest) bool {
			return req != nil && req.UserID == testUserID && req.RelyingPartyID == testRelyingPartyID
		})).Return(expectedResponse, nil).Once()

	result, err := suite.service.StartPasskeyAuthentication(context.Background(), testUserID, testRelyingPartyID)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(expectedResponse, result)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyAuthentication_ServiceError() {
	serviceError := &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "USER_NOT_FOUND",
		Error: core.I18nMessage{Key: "error.test.user_not_found", DefaultValue: "User not found"},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.no_user_found_with_the_given_id", DefaultValue: "No user found with the given ID",
		},
	}

	suite.mockPasskeyService.On(
		"StartAuthentication", mock.Anything, mock.MatchedBy(func(req *passkey.PasskeyAuthenticationStartRequest) bool {
			return req != nil && req.UserID == testUserID && req.RelyingPartyID == testRelyingPartyID
		})).Return(nil, serviceError).Once()

	result, err := suite.service.StartPasskeyAuthentication(context.Background(), testUserID, testRelyingPartyID)

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(serviceError, err)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_Success() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
		UserHandle:        "base64-user-handle",
	}
	sessionToken := testSessionTkn

	var mockAuthUser authnprovidermgr.AuthUser
	passkeyJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":` + // #nosec G101
		`[{"userId":"user123","userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(passkeyJSON), &mockAuthUser)

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()

	// Mock assertion generation
	mockAssertionResult := &assert.AssertionResult{
		Context: &assert.AssuranceContext{
			Authenticators: []authnprovidermgr.AuthenticatorReference{
				{Authenticator: "Passkey", Step: 1},
			},
		},
	}
	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(mockAssertionResult, nil).Once()

	suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["userType"] == "person" && claims["ouId"] == testOrgUnit
		}), mock.Anything, mock.Anything).Return(testJWTToken, int64(3600), nil).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, sessionToken, false, "")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
	suite.Equal("person", result.Type)
	suite.Equal(testOrgUnit, result.OUID)
	suite.Equal(testJWTToken, result.Assertion)
	suite.mockPasskeyService.AssertExpectations(suite.T())
	suite.mockAssertGenerator.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_WithSkipAssertion() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
		UserHandle:        "", // Empty for this test
	}
	sessionToken := testSessionTkn

	var mockAuthUser authnprovidermgr.AuthUser
	passkeyJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":` + // #nosec G101
		`[{"userId":"user123","userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],"userState":"exists"}`
	_ = json.Unmarshal([]byte(passkeyJSON), &mockAuthUser)

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, sessionToken, true, "")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
	suite.Empty(result.Assertion)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_WithExistingAssertion() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
	}
	sessionToken := testSessionTkn
	existingAssertion := suite.createTestAssertion(testUserID)

	var mockAuthUser authnprovidermgr.AuthUser
	passkeyJSON := `{"authHistory":[{"authType":"LOCAL","isVerified":true}],` + // #nosec G101
		`"userHistory":[{"userId":"user123","userType":"person","ouId":"org_unit_123","isValuesIncluded":true}],` +
		`"userState":"exists"}`
	_ = json.Unmarshal([]byte(passkeyJSON), &mockAuthUser)

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(mockAuthUser, (*serviceerror.ServiceError)(nil)).Once()

	// Mock JWT verification for existing assertion
	suite.mockJWTService.On("VerifyJWT", existingAssertion, "", mock.Anything).Return(nil).Once()

	// Mock assertion update
	mockUpdatedResult := &assert.AssertionResult{
		Context: &assert.AssuranceContext{
			Authenticators: []authnprovidermgr.AuthenticatorReference{
				{Authenticator: "CredentialsAuthenticator", Step: 1},
				{Authenticator: "Passkey", Step: 2},
			},
		},
	}
	suite.mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything).
		Return(mockUpdatedResult, nil).Once()

	suite.mockJWTService.On("GenerateJWT", testUserID, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("updated.jwt.token", int64(3600), nil).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, sessionToken, false, existingAssertion)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("updated.jwt.token", result.Assertion)
	suite.mockPasskeyService.AssertExpectations(suite.T())
	suite.mockAssertGenerator.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_ServiceError() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
	}

	serviceError := &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "INVALID_SIGNATURE",
		Error: core.I18nMessage{Key: "error.test.invalid_signature", DefaultValue: "Invalid signature"},
		ErrorDescription: core.I18nMessage{
			Key: "error.test.failed_to_verify_signature", DefaultValue: "Failed to verify signature",
		},
	}

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, serviceError).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, testSessionTkn, false, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(serviceError, err)
	suite.mockPasskeyService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) createTestAssertion(subject string) string {
	assuranceCtx := map[string]interface{}{
		"aal": "aal1",
		"ial": "ial1",
		"authenticators": []map[string]interface{}{
			{
				"authenticator": "CredentialsAuthenticator",
				"step":          1,
				"timestamp":     int64(1735689600), // 2025-01-01T00:00:00Z in Unix epoch
			},
		},
	}

	payload := map[string]interface{}{
		"sub":       subject,
		"assurance": assuranceCtx,
	}

	payloadBytes, _ := json.Marshal(payload)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	return fmt.Sprintf("header.%s.signature", encodedPayload)
}

func (suite *AuthenticationServiceTestSuite) createTestAssertionWithoutAssurance(subject string) string {
	payload := map[string]interface{}{
		"sub": subject,
	}

	payloadBytes, _ := json.Marshal(payload)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	return fmt.Sprintf("header.%s.signature", encodedPayload)
}
