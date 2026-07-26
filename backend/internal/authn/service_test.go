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

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/assert"
	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/tests/mocks/authn/assertmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/githubmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/googlemock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/otpmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
	"github.com/thunder-id/thunderid/tests/mocks/templatemock"
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
	testSenderID         = "sender_123"
)

type AuthenticationServiceTestSuite struct {
	suite.Suite
	mockIDPService      *idpmock.IDPServiceInterfaceMock
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockAssertGenerator *assertmock.AuthAssertGeneratorInterfaceMock
	mockAuthnProvider   *managermock.AuthnProviderManagerMock
	mockOTPService      *otpmock.OTPAuthnServiceInterfaceMock
	mockNotifSenderSvc  *notificationmock.NotificationSenderServiceInterfaceMock
	mockTemplateService *templatemock.TemplateServiceInterfaceMock
	mockOAuthService    *oauthmock.OAuthAuthnServiceInterfaceMock
	mockOIDCService     *oidcmock.OIDCAuthnServiceInterfaceMock
	mockGoogleService   *googlemock.GoogleOIDCAuthnServiceInterfaceMock
	mockGithubService   *githubmock.GithubOAuthAuthnServiceInterfaceMock
	service             *authenticationService
}

func TestAuthenticationServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthenticationServiceTestSuite))
}

func (suite *AuthenticationServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         mock.Anything,
			ValidityPeriod: 3600,
			Audience:       "application",
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}

	// Register authenticators for IDP types
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          "OAuthAuthenticator",
		AssociatedIDP: providers.IDPTypeOAuth,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          "OIDCAuthenticator",
		AssociatedIDP: providers.IDPTypeOIDC,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          "GoogleAuthenticator",
		AssociatedIDP: providers.IDPTypeGoogle,
	})
	common.RegisterAuthenticator(common.AuthenticatorMeta{
		Name:          "GitHubAuthenticator",
		AssociatedIDP: providers.IDPTypeGitHub,
	})
}

func (suite *AuthenticationServiceTestSuite) SetupTest() {
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockAssertGenerator = &assertmock.AuthAssertGeneratorInterfaceMock{}
	suite.mockAuthnProvider = &managermock.AuthnProviderManagerMock{}
	suite.mockOTPService = &otpmock.OTPAuthnServiceInterfaceMock{}
	suite.mockNotifSenderSvc = notificationmock.NewNotificationSenderServiceInterfaceMock(suite.T())
	suite.mockTemplateService = templatemock.NewTemplateServiceInterfaceMock(suite.T())
	suite.mockOAuthService = &oauthmock.OAuthAuthnServiceInterfaceMock{}
	suite.mockOIDCService = &oidcmock.OIDCAuthnServiceInterfaceMock{}
	suite.mockGoogleService = &googlemock.GoogleOIDCAuthnServiceInterfaceMock{}
	suite.mockGithubService = &githubmock.GithubOAuthAuthnServiceInterfaceMock{}

	suite.service = &authenticationService{
		idpService:             suite.mockIDPService,
		jwtService:             suite.mockJWTService,
		authAssertionGenerator: suite.mockAssertGenerator,
		authnProvider:          suite.mockAuthnProvider,
		otpService:             suite.mockOTPService,
		notifSenderSvc:         suite.mockNotifSenderSvc,
		templateService:        suite.mockTemplateService,
		oauthService:           suite.mockOAuthService,
		oidcService:            suite.mockOIDCService,
		googleService:          suite.mockGoogleService,
		githubService:          suite.mockGithubService,
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
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, identifiers,
					authnCredentials, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
				suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{
						EntityID:   testUserID,
						EntityType: testUserType,
						OUID:       testOrgUnit,
					}, nil).Once()
				suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.AttributesResponse{}, nil).Once()
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
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, identifiers,
					authnCredentials, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
				suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{
						EntityID:   testUserID,
						EntityType: testUserType,
						OUID:       testOrgUnit,
					}, nil).Once()
				suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.AttributesResponse{}, nil).Once()
				suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything, mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel1,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything, mock.Anything,
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
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, identifiers,
					authnCredentials, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
				suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{
						EntityID:   testUserID,
						EntityType: testUserType,
						OUID:       testOrgUnit,
					}, nil).Once()
				suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.AttributesResponse{}, nil).Once()
				suite.mockJWTService.On("VerifyJWT", mock.Anything, mock.Anything, "", mock.Anything).Return(nil).Once()
				suite.mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything, mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel2,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything, mock.Anything,
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

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, identifiers,
		authnCredentials, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, (providers.AuthenticatedClaims)(nil),
		&authnprovidermgr.ErrorAuthenticationFailed)

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

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, identifiers, authnCredentials, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, nil)
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: testUserType,
			OUID:       testOrgUnit,
		}, nil)
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.AttributesResponse{}, nil)
	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything, mock.Anything).Return(
		&assert.AssertionResult{
			Context: &assert.AssuranceContext{
				AAL: assert.AALLevel1,
				IAL: assert.IALLevel1,
			},
		}, nil).Once()
	suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "JWT_GENERATION_FAILED",
			Error: tidcommon.I18nMessage{
				Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.failed_to_generate_jwt_token", DefaultValue: "Failed to generate JWT token",
			},
		})

	result, err := suite.service.AuthenticateWithCredentials(context.Background(), identifiers,
		authnCredentials, false, "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
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

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, identifiers, authnCredentials, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, nil)
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: testUserType,
			OUID:       testOrgUnit,
		}, nil)
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.AttributesResponse{}, nil)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, existingAssertion, "", mock.Anything).Return(nil)

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

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, identifiers, authnCredentials, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, nil)
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: testUserType,
			OUID:       testOrgUnit,
		}, nil)
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, &providers.AttributesResponse{}, nil)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, invalidAssertion, "", mock.Anything).
		Return(&tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "INVALID_JWT",
			Error: tidcommon.I18nMessage{Key: "error.test.invalid_jwt", DefaultValue: "Invalid JWT"},
			ErrorDescription: tidcommon.I18nMessage{
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

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, identifiers, authnCredentials, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, nil)
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: testUserType,
			OUID:       testOrgUnit,
		}, nil)
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, &providers.AttributesResponse{}, nil)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, existingAssertion, "", mock.Anything).Return(nil)

	result, err := suite.service.AuthenticateWithCredentials(context.Background(), identifiers,
		authnCredentials, false, existingAssertion)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestSendOTPSuccess() {
	senderID := testSenderID
	recipient := "+1234567890"

	suite.mockOTPService.On("GenerateOTP", mock.Anything, recipient, "mobile_number", mock.Anything).
		Return(testSessionTkn, "123456", int64(300), nil)
	suite.mockTemplateService.On("Render",
		mock.Anything, template.ScenarioOTP, template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your OTP is 123456"}, nil)
	suite.mockNotifSenderSvc.On("Send",
		mock.Anything, notifcommon.ChannelTypeSMS, senderID, mock.Anything).
		Return(nil)

	result, err := suite.service.SendOTP(context.Background(), senderID, notifcommon.ChannelTypeSMS, recipient, "")

	suite.Nil(err)
	suite.Equal(testSessionTkn, result)
}

func (suite *AuthenticationServiceTestSuite) TestSendOTPGenerateError() {
	senderID := testSenderID
	recipient := "+1234567890"
	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "OTP_ERROR",
		Error: tidcommon.I18nMessage{Key: "error.test.otp_error", DefaultValue: "OTP error"},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.failed_to_generate_otp", DefaultValue: "Failed to generate OTP"},
	}

	suite.mockOTPService.On("GenerateOTP", mock.Anything, recipient, "mobile_number", mock.Anything).
		Return("", "", int64(0), svcErr)

	result, err := suite.service.SendOTP(context.Background(), senderID, notifcommon.ChannelTypeSMS, recipient, "")

	suite.Empty(result)
	suite.NotNil(err)
	suite.Equal(svcErr.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestSendOTPSendError() {
	senderID := testSenderID
	recipient := "+1234567890"
	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "OTP_ERROR",
		Error: tidcommon.I18nMessage{Key: "error.test.otp_error", DefaultValue: "OTP error"},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.failed_to_send_otp", DefaultValue: "Failed to send OTP",
		},
	}

	suite.mockOTPService.On("GenerateOTP", mock.Anything, recipient, "mobile_number", mock.Anything).
		Return(testSessionTkn, "123456", int64(300), nil)
	suite.mockTemplateService.On("Render",
		mock.Anything, template.ScenarioOTP, template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your OTP is 123456"}, nil)
	suite.mockNotifSenderSvc.On("Send",
		mock.Anything, notifcommon.ChannelTypeSMS, senderID, mock.Anything).
		Return(svcErr)

	result, err := suite.service.SendOTP(context.Background(), senderID, notifcommon.ChannelTypeSMS, recipient, "")

	suite.Empty(result)
	suite.NotNil(err)
	suite.Equal(svcErr.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyOTP() {
	sessionToken := testSessionTkn
	otpCode := "123456"
	testEntityRef := &providers.EntityReference{
		EntityID:   testUserID,
		EntityType: "person",
		OUID:       testOrgUnit,
	}

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
				suite.mockAuthnProvider.On("AuthenticateUser",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
				suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, testEntityRef, nil).Once()
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
				suite.mockAuthnProvider.On("AuthenticateUser",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
				suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, testEntityRef, nil).Once()
				suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything, mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel1,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything, mock.Anything,
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
				suite.mockAuthnProvider.On("AuthenticateUser",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
				suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, testEntityRef, nil).Once()
				suite.mockJWTService.
					On("VerifyJWT", mock.Anything, existingAssertion, "", mock.Anything).Return(nil).Once()
				suite.mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything, mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel2,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything, mock.Anything,
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
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil),
			&authnprovidermgr.ErrorAuthenticationFailed)

	result, err := suite.service.VerifyOTP(context.Background(), sessionToken, false, "", otpCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorOTPAuthenticationFailed.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationOAuthSuccess() {
	idpID := testIDPID
	redirectURL := testRedirectURL
	identityProvider := &providers.IDPDTO{
		ID:   idpID,
		Type: providers.IDPTypeOAuth,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, idpID).
		Return(redirectURL, map[string]string{oauth2const.RequestParamState: "test-state"}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testSessionTkn, int64(600), nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), providers.IDPTypeOAuth, idpID)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(redirectURL, result.RedirectURL)
	suite.Equal(testSessionTkn, result.SessionToken)
}

func (suite *AuthenticationServiceTestSuite) assertStartIDPAuthSuccess(
	idpType providers.IDPType, redirectURL string, setupBuildURL func(string),
) {
	idpID := testIDPID

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).
		Return(&providers.IDPDTO{ID: idpID, Type: idpType}, nil)
	setupBuildURL(idpID)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testSessionTkn, int64(600), nil)

	result, err := suite.service.StartIDPAuthentication(
		context.Background(), idpType, idpID)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(redirectURL, result.RedirectURL)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationOIDCSuccess() {
	metadata := map[string]string{
		oauth2const.RequestParamState: "test-state",
		oauth2const.RequestParamNonce: "test-nonce",
	}
	suite.assertStartIDPAuthSuccess(
		providers.IDPTypeOIDC, "https://oidc.provider.com/authorize",
		func(idpID string) {
			suite.mockOIDCService.On("BuildAuthorizeURL", mock.Anything, idpID).
				Return("https://oidc.provider.com/authorize", metadata, nil)
		},
	)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationGoogleSuccess() {
	metadata := map[string]string{
		oauth2const.RequestParamState: "test-state",
		oauth2const.RequestParamNonce: "test-nonce",
	}
	suite.assertStartIDPAuthSuccess(
		providers.IDPTypeGoogle, "https://accounts.google.com/o/oauth2/v2/auth",
		func(idpID string) {
			suite.mockGoogleService.On("BuildAuthorizeURL", mock.Anything, idpID).
				Return("https://accounts.google.com/o/oauth2/v2/auth", metadata, nil)
		},
	)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationGitHubSuccess() {
	idpID := testIDPID
	redirectURL := "https://github.com/login/oauth/authorize"
	identityProvider := &providers.IDPDTO{
		ID:   idpID,
		Type: providers.IDPTypeGitHub,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockGithubService.On("BuildAuthorizeURL", mock.Anything, idpID).
		Return(redirectURL, map[string]string{oauth2const.RequestParamState: "test-state"}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testSessionTkn, int64(600), nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), providers.IDPTypeGitHub, idpID)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(redirectURL, result.RedirectURL)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationEmptyIDPID() {
	result, err := suite.service.StartIDPAuthentication(context.Background(), providers.IDPTypeOAuth, "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidIDPID.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationIDPNotFound() {
	idpID := "nonexistent_idp"
	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "IDP_NOT_FOUND",
		Error: tidcommon.I18nMessage{Key: "error.test.idp_not_found", DefaultValue: "IDP not found"},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.the_identity_provider_was_not_found", DefaultValue: "The identity provider was not found",
		},
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(nil, svcErr)

	result, err := suite.service.StartIDPAuthentication(context.Background(), providers.IDPTypeOAuth, idpID)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Contains(err.ErrorDescription.DefaultValue, idpID)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationInvalidIDPType() {
	idpID := testIDPID
	identityProvider := &providers.IDPDTO{
		ID:   idpID,
		Type: providers.IDPTypeGoogle,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), providers.IDPTypeGitHub, idpID)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidIDPType.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationCrossTypeAllowed() {
	idpID := testIDPID
	redirectURL := testRedirectURL
	identityProvider := &providers.IDPDTO{
		ID:   idpID,
		Type: providers.IDPTypeOAuth,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, idpID).
		Return(redirectURL, map[string]string{oauth2const.RequestParamState: "test-state"}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testSessionTkn, int64(600), nil)

	result, err := suite.service.StartIDPAuthentication(context.Background(), providers.IDPTypeOIDC, idpID)

	suite.Nil(err)
	suite.NotNil(result)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationJWTGenerationError() {
	idpID := testIDPID
	redirectURL := testRedirectURL
	identityProvider := &providers.IDPDTO{
		ID:   idpID,
		Type: providers.IDPTypeOAuth,
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, idpID).
		Return(redirectURL, map[string]string{oauth2const.RequestParamState: "test-state"}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "auth-svc",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "JWT_GENERATION_FAILED",
			Error: tidcommon.I18nMessage{
				Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.failed_to_generate_session_token", DefaultValue: "Failed to generate session token",
			},
		})

	result, err := suite.service.StartIDPAuthentication(context.Background(), providers.IDPTypeOAuth, idpID)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) mockFederatedAuthnSuccess(idpType providers.IDPType) string {
	sessionToken := suite.createSessionToken(idpType)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, sessionToken, "auth-svc", mock.Anything).Return(nil)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(creds map[string]interface{}) bool {
			_, ok := creds["federated"]
			return ok
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: "person",
			OUID:       testOrgUnit,
		}, nil).Once()
	return sessionToken
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationOAuthSuccess() {
	sessionToken := suite.mockFederatedAuthnSuccess(providers.IDPTypeOAuth)
	result, err := suite.service.FinishIDPAuthentication(
		context.Background(),
		providers.IDPTypeOAuth,
		sessionToken,
		true,
		"",
		testAuthCode,
	)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
	suite.Empty(result.Assertion)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationOIDCSuccess() {
	sessionToken := suite.mockFederatedAuthnSuccess(providers.IDPTypeOIDC)
	result, err := suite.service.FinishIDPAuthentication(
		context.Background(),
		providers.IDPTypeOIDC,
		sessionToken,
		true,
		"",
		testAuthCode,
	)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationGoogleSuccess() {
	sessionToken := suite.mockFederatedAuthnSuccess(providers.IDPTypeGoogle)
	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), providers.IDPTypeGoogle, sessionToken, true, "", testAuthCode)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationGitHubSuccess() {
	sessionToken := suite.mockFederatedAuthnSuccess(providers.IDPTypeGitHub)
	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), providers.IDPTypeGitHub, sessionToken, true, "", testAuthCode)
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
				sessionToken := suite.createSessionToken(providers.IDPTypeOAuth)
				suite.mockJWTService.
					On("VerifyJWT", mock.Anything, sessionToken, "auth-svc", mock.Anything).Return(nil).Once()
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
					mock.MatchedBy(func(creds map[string]interface{}) bool {
						_, ok := creds["federated"]
						return ok
					}), mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
				suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{
						EntityID:   testUserID,
						EntityType: "person",
						OUID:       testOrgUnit,
					}, nil).Once()
				suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything, mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel1,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", mock.Anything,
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
				sessionToken := suite.createSessionToken(providers.IDPTypeOAuth)
				existingAssertion := suite.createTestAssertion(testUserID)
				suite.mockJWTService.
					On("VerifyJWT", mock.Anything, sessionToken, "auth-svc", mock.Anything).Return(nil).Once()
				suite.mockJWTService.
					On("VerifyJWT", mock.Anything, existingAssertion, "", mock.Anything).Return(nil).Once()
				suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
					mock.MatchedBy(func(creds map[string]interface{}) bool {
						_, ok := creds["federated"]
						return ok
					}), mock.Anything, mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
				suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
					Return(providers.AuthUser{}, &providers.EntityReference{
						EntityID:   testUserID,
						EntityType: "person",
						OUID:       testOrgUnit,
					}, nil).Once()
				suite.mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything, mock.Anything).Return(
					&assert.AssertionResult{
						Context: &assert.AssuranceContext{
							AAL: assert.AALLevel2,
							IAL: assert.IALLevel1,
						},
					}, nil).Once()
				suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything, mock.Anything,
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

			sessionToken := suite.createSessionToken(providers.IDPTypeOAuth)
			result, err := suite.service.FinishIDPAuthentication(
				context.Background(),
				providers.IDPTypeOAuth,
				sessionToken,
				tc.skipAssertion,
				tc.existingAssertion,
				testAuthCode,
			)

			suite.Nil(err)
			suite.NotNil(result)
			suite.Equal(testUserID, result.ID)
			tc.validateAssertion(result)
		})
	}
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationEmptySessionToken() {
	result, err := suite.service.FinishIDPAuthentication(context.Background(), providers.IDPTypeOAuth, "", false, "",
		testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorEmptySessionToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationEmptyAuthCode() {
	sessionToken := suite.createSessionToken(providers.IDPTypeOAuth)

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), providers.IDPTypeOAuth, sessionToken, false, "",
		"")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorEmptyAuthCode.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationInvalidSessionToken() {
	suite.mockJWTService.On("VerifyJWT", mock.Anything, "invalid_token", "auth-svc", mock.Anything).
		Return(&tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "INVALID_TOKEN",
			Error: tidcommon.I18nMessage{Key: "error.test.invalid_token", DefaultValue: "Invalid token"},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.the_session_token_is_invalid", DefaultValue: "The session token is invalid",
			},
		})

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), providers.IDPTypeOAuth, "invalid_token", false, "", testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationTypeMismatch() {
	sessionToken := suite.createSessionToken(providers.IDPTypeGoogle)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, sessionToken, "auth-svc", mock.Anything).Return(nil)

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), providers.IDPTypeGitHub, sessionToken, false, "",
		testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidIDPType.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationUserNotFound() { //nolint:dupl
	sessionToken := suite.createSessionToken(providers.IDPTypeOAuth)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, sessionToken, "auth-svc", mock.Anything).Return(nil)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(creds map[string]interface{}) bool {
			_, ok := creds["federated"]
			return ok
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil),
			&authnprovidermgr.ErrorUserNotFound).Once()

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), providers.IDPTypeOAuth, sessionToken, false, "",
		testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorFederatedAuthenticationFailed.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationProviderAuthFailure() { //nolint:dupl
	sessionToken := suite.createSessionToken(providers.IDPTypeOAuth)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, sessionToken, "auth-svc", mock.Anything).Return(nil)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(creds map[string]interface{}) bool {
			_, ok := creds["federated"]
			return ok
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{},
			(providers.AuthenticatedClaims)(nil), &authnprovidermgr.ErrorAuthenticationFailed).Once()

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), providers.IDPTypeOAuth, sessionToken, false, "",
		testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorFederatedAuthenticationFailed.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestValidateIDPTypeExactMatch() {
	err := suite.service.validateIDPType(context.Background(), providers.IDPTypeOAuth, providers.IDPTypeOAuth, nil)
	suite.Nil(err)
}

func (suite *AuthenticationServiceTestSuite) TestValidateIDPTypeEmptyRequested() {
	err := suite.service.validateIDPType(context.Background(), "", providers.IDPTypeOAuth, nil)
	suite.Nil(err)
}

func (suite *AuthenticationServiceTestSuite) TestValidateIDPTypeCrossAllowed() {
	err := suite.service.validateIDPType(context.Background(), providers.IDPTypeOAuth, providers.IDPTypeOIDC, nil)
	suite.Nil(err)

	err = suite.service.validateIDPType(context.Background(), providers.IDPTypeOIDC, providers.IDPTypeOAuth, nil)
	suite.Nil(err)
}

func (suite *AuthenticationServiceTestSuite) TestValidateIDPTypeMismatch() {
	logger := log.GetLogger()
	err := suite.service.validateIDPType(context.Background(), providers.IDPTypeGoogle, providers.IDPTypeGitHub, logger)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidIDPType.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestHandleIDPServiceErrorServerError() {
	idpID := "test_idp"
	svcErr := &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "INTERNAL_ERROR",
		Error: tidcommon.I18nMessage{Key: "error.test.internal_error", DefaultValue: "Internal error"},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.internal_error_description", DefaultValue: "Database connection failed",
		},
	}
	logger := log.GetLogger()

	result := suite.service.handleIDPServiceError(context.Background(), idpID, svcErr, logger)

	suite.NotNil(result)
	suite.Equal(tidcommon.InternalServerError.Code, result.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyAndDecodeSessionTokenMalformedPayload() {
	logger := log.GetLogger()
	badToken := "header.invalid-base64.signature"

	suite.mockJWTService.On("VerifyJWT", mock.Anything, badToken, "auth-svc", mock.Anything).Return(nil)

	result, err := suite.service.verifyAndDecodeSessionToken(context.Background(), badToken, logger)

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

	suite.mockJWTService.On("VerifyJWT", mock.Anything, tokenWithoutAuthData, "auth-svc", mock.Anything).Return(nil)

	result, err := suite.service.verifyAndDecodeSessionToken(context.Background(), tokenWithoutAuthData, logger)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestStartIDPAuthenticationBuildURLError() {
	idpID := testIDPID
	identityProvider := &providers.IDPDTO{
		ID:   idpID,
		Type: providers.IDPTypeOAuth,
	}
	svcErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "INVALID_CONFIG",
		Error: tidcommon.I18nMessage{
			Key: "error.test.invalid_configuration", DefaultValue: "Invalid configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.missing_redirect_uri", DefaultValue: "Missing redirect URI",
		},
	}

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, idpID).Return(identityProvider, nil)
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, idpID).
		Return("", (map[string]string)(nil), svcErr)

	result, err := suite.service.StartIDPAuthentication(context.Background(), providers.IDPTypeOAuth, idpID)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(svcErr.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationProviderServerError() {
	sessionToken := suite.createSessionToken(providers.IDPTypeOIDC)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, sessionToken, "auth-svc", mock.Anything).Return(nil)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(creds map[string]interface{}) bool {
			_, ok := creds["federated"]
			return ok
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil),
			&tidcommon.InternalServerError).Once()

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(),
		providers.IDPTypeOIDC,
		sessionToken,
		true,
		"",
		testAuthCode,
	)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) createSessionToken(idpType providers.IDPType) string {
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
	testUser := &providers.Entity{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: testUser.Type,
		OUID: testUser.OUID,
	}
	logger := log.GetLogger()

	// Create assertion without sub claim
	payload := map[string]interface{}{
		"assurance": map[string]interface{}{
			"aal": "aal1",
			"ial": "ial1",
			"authenticators": []map[string]interface{}{
				{
					"authenticator": common.AuthenticatorCredentials,
					"step":          1,
					"timestamp":     int64(1735689600),
				},
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	invalidAssertion := "header." + encodedPayload + ".signature"

	suite.mockJWTService.On("VerifyJWT", mock.Anything, invalidAssertion, "", mock.Anything).Return(nil).Once()

	svcErr := suite.service.validateAndAppendAuthAssertion(context.Background(),
		authResponse, testUser, common.AuthenticatorSMSOTP, invalidAssertion, logger)

	suite.NotNil(svcErr)
	suite.Equal(common.ErrorInvalidAssertion.Code, svcErr.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationAssertionGenerationError() {
	sessionToken := suite.createSessionToken(providers.IDPTypeOAuth)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, sessionToken, "auth-svc", mock.Anything).Return(nil).Once()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(creds map[string]interface{}) bool {
			_, ok := creds["federated"]
			return ok
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: "person",
			OUID:       testOrgUnit,
		}, nil).Once()

	// Create invalid existing assertion that will fail JWT verification
	suite.mockJWTService.On("VerifyJWT", mock.Anything, invalidAssertion, "", mock.Anything).
		Return(&tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "INVALID_SIGNATURE",
			Error: tidcommon.I18nMessage{Key: "error.test.invalid_signature", DefaultValue: "Invalid signature"},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.the_jwt_signature_is_invalid", DefaultValue: "The JWT signature is invalid",
			},
		}).Once()

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(),
		providers.IDPTypeOAuth,
		sessionToken,
		false,
		invalidAssertion,
		testAuthCode,
	)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestValidateAndAppendAuthAssertionStepOne() {
	testUser := &providers.Entity{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: testUser.Type,
		OUID: testUser.OUID,
	}
	logger := log.GetLogger()

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything, mock.Anything).Return(
		&assert.AssertionResult{
			Context: &assert.AssuranceContext{
				AAL: assert.AALLevel1,
				IAL: assert.IALLevel1,
			},
		}, nil).Once()
	suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testJWTToken, int64(3600), nil).Once()

	// Test with empty existingAssertion
	svcErr := suite.service.validateAndAppendAuthAssertion(context.Background(),
		authResponse, testUser, common.AuthenticatorCredentials, "", logger)
	suite.Nil(svcErr)
	suite.Equal(testJWTToken, authResponse.Assertion)
}

func (suite *AuthenticationServiceTestSuite) TestValidateAndAppendAuthAssertionSubjectMismatch() {
	testUser := &providers.Entity{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: testUser.Type,
		OUID: testUser.OUID,
	}

	// Create assertion with different subject
	existingAssertion := suite.createTestAssertion("different_user_id")

	suite.mockJWTService.On("VerifyJWT", mock.Anything, existingAssertion, "", mock.Anything).Return(nil)

	svcErr := suite.service.validateAndAppendAuthAssertion(context.Background(),
		authResponse, testUser, common.AuthenticatorSMSOTP, existingAssertion, log.GetLogger())

	suite.NotNil(svcErr)
	suite.Equal(common.ErrorAssertionSubjectMismatch.Code, svcErr.Code)
}

func (suite *AuthenticationServiceTestSuite) TestExtractClaimsFromAssertionMissingAssurance() {
	// Create assertion without assurance claim
	assertionWithoutAssurance := suite.createTestAssertionWithoutAssurance(testUserID)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, assertionWithoutAssurance, "", mock.Anything).Return(nil)

	_, _, svcErr := suite.service.extractClaimsFromAssertion(
		context.Background(), assertionWithoutAssurance, log.GetLogger())

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
							"authenticator": common.AuthenticatorCredentials,
							"step":          1,
							"timestamp":     int64(1735689600),
						},
					},
				},
			},
			setupMock: func(assertion string) {
				suite.mockJWTService.On("VerifyJWT", mock.Anything, assertion, "", mock.Anything).Return(nil).Once()
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
							"authenticator": common.AuthenticatorCredentials,
							"step":          1,
							"timestamp":     int64(1735689600),
						},
					},
				},
			},
			setupMock: func(assertion string) {
				suite.mockJWTService.On("VerifyJWT", mock.Anything, assertion, "", mock.Anything).Return(nil).Once()
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
							"authenticator": common.AuthenticatorCredentials,
							"step":          1,
							"timestamp":     int64(1735689600),
						},
					},
				},
			},
			setupMock: func(assertion string) {
				suite.mockJWTService.On("VerifyJWT", mock.Anything, assertion, "", mock.Anything).Return(nil).Once()
			},
		},
		{
			name: "MissingAssuranceClaim",
			payload: map[string]interface{}{
				"sub": testUserID,
			},
			setupMock: func(assertion string) {
				suite.mockJWTService.On("VerifyJWT", mock.Anything, assertion, "", mock.Anything).Return(nil).Once()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			payloadBytes, _ := json.Marshal(tc.payload)
			encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
			testAssertion := "header." + encodedPayload + ".signature"

			tc.setupMock(testAssertion)

			assuranceCtx, sub, err := suite.service.
				extractClaimsFromAssertion(context.Background(), testAssertion, logger)

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
	suite.mockJWTService.On("VerifyJWT", mock.Anything, malformedAssertion, "", mock.Anything).Return(nil).Once()

	assuranceCtx, sub, err := suite.service.extractClaimsFromAssertion(context.Background(), malformedAssertion, logger)
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
	suite.mockJWTService.On("VerifyJWT", mock.Anything, testAssertion, "", mock.Anything).Return(nil).Once()

	assuranceCtx, sub, err := suite.service.extractClaimsFromAssertion(context.Background(), testAssertion, logger)
	suite.Nil(assuranceCtx)
	suite.Empty(sub)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyOTPJWTGenerationError() {
	sessionToken := testSessionTkn
	otpCode := "123456"

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil)
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: "person",
			OUID:       testOrgUnit,
		}, nil)
	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything, mock.Anything).Return(
		&assert.AssertionResult{
			Context: &assert.AssuranceContext{
				AAL: assert.AALLevel1,
				IAL: assert.IALLevel1,
			},
		}, nil).Once()
	suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "JWT_GENERATION_FAILED",
			Error: tidcommon.I18nMessage{
				Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.failed_to_generate_jwt_token", DefaultValue: "Failed to generate JWT token",
			},
		})

	result, err := suite.service.VerifyOTP(context.Background(), sessionToken, false, "", otpCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestExtractClaimsFromAssertionInvalidJWTSignature() {
	logger := log.GetLogger()

	suite.mockJWTService.On("VerifyJWT", mock.Anything, invalidAssertion, "", mock.Anything).
		Return(&tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "INVALID_SIGNATURE",
			Error: tidcommon.I18nMessage{Key: "error.test.invalid_signature", DefaultValue: "Invalid signature"},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.the_jwt_signature_is_invalid", DefaultValue: "The JWT signature is invalid",
			},
		})

	assuranceCtx, sub, err := suite.service.extractClaimsFromAssertion(context.Background(), invalidAssertion, logger)

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

	suite.mockJWTService.On("VerifyJWT", mock.Anything, malformedAssertion, "", mock.Anything).Return(nil)

	assuranceCtx, sub, err := suite.service.extractClaimsFromAssertion(context.Background(), malformedAssertion, logger)

	suite.Nil(assuranceCtx)
	suite.Empty(sub)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidAssertion.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestValidateAndAppendAuthAssertionGenerationError() {
	testUser := &providers.Entity{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	logger := log.GetLogger()

	// Create a service with a mock assertion generator that returns an error
	mockAssertGenerator := assertmock.NewAuthAssertGeneratorInterfaceMock(suite.T())
	mockAssertGenerator.On("GenerateAssertion", mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "ASSERTION_ERROR",
			Error: tidcommon.I18nMessage{
				Key: "error.test.assertion_generation_failed", DefaultValue: "Assertion generation failed",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.failed_to_generate_assertion", DefaultValue: "Failed to generate assertion",
			},
		})

	service := &authenticationService{
		authAssertionGenerator: mockAssertGenerator,
		jwtService:             suite.mockJWTService,
	}

	err := service.validateAndAppendAuthAssertion(
		context.Background(), authResponse, testUser, common.AuthenticatorCredentials, "", logger)

	suite.NotNil(err)
	suite.Equal("ASSERTION_ERROR", err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestValidateAndAppendAuthAssertionUpdateError() {
	testUser := &providers.Entity{
		ID:   testUserID,
		Type: "person",
		OUID: testOrgUnit,
	}
	authResponse := &common.AuthenticationResponse{
		ID:   testUserID,
		Type: testUser.Type,
		OUID: testUser.OUID,
	}
	logger := log.GetLogger()
	existingAssertion := suite.createTestAssertion(testUserID)

	suite.mockJWTService.On("VerifyJWT", mock.Anything, existingAssertion, "", mock.Anything).Return(nil)

	// Create a service with a mock assertion generator that returns an error on update
	mockAssertGenerator := assertmock.NewAuthAssertGeneratorInterfaceMock(suite.T())
	mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "UPDATE_ERROR",
			Error: tidcommon.I18nMessage{
				Key: "error.test.assertion_update_failed", DefaultValue: "Assertion update failed",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.failed_to_update_assertion", DefaultValue: "Failed to update assertion",
			},
		})

	service := &authenticationService{
		authAssertionGenerator: mockAssertGenerator,
		jwtService:             suite.mockJWTService,
	}

	err := service.validateAndAppendAuthAssertion(
		context.Background(), authResponse, testUser, common.AuthenticatorSMSOTP, existingAssertion, logger)

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

	suite.mockAuthnProvider.On("InitiateEnrollment", mock.Anything, passkey.CredentialType, mock.Anything,
		mock.Anything).Return(expectedResponse, nil).Once()

	result, err := suite.service.StartPasskeyRegistration(
		context.Background(), testUserID, testRelyingPartyID, testRelyingPartyName, authSelection, attestation)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(expectedResponse, result)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyRegistration_WithoutAuthSelection() {
	attestation := ""

	expectedResponse := &passkey.PasskeyRegistrationStartData{
		SessionToken: testSessionTkn,
	}

	suite.mockAuthnProvider.On("InitiateEnrollment", mock.Anything, passkey.CredentialType, mock.Anything,
		mock.Anything).Return(expectedResponse, nil).Once()

	result, err := suite.service.StartPasskeyRegistration(
		context.Background(), testUserID, testRelyingPartyID, testRelyingPartyName, nil, attestation)

	suite.Nil(err)
	suite.NotNil(result)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyRegistration_ServiceError() {
	serviceError := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "PASS_ERROR",
		Error: tidcommon.I18nMessage{Key: "error.test.passkey_error", DefaultValue: "Passkey error"},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.failed_to_start_registration", DefaultValue: "Failed to start registration",
		},
	}

	suite.mockAuthnProvider.On("InitiateEnrollment", mock.Anything, passkey.CredentialType, mock.Anything,
		mock.Anything).Return(nil, serviceError).Once()

	result, err := suite.service.StartPasskeyRegistration(
		context.Background(), testUserID, testRelyingPartyID, testRelyingPartyName, nil, "")

	suite.NotNil(err)
	suite.Nil(result)
	// Provider-layer client errors are mapped to a clean passkey enrollment error.
	suite.Equal(ErrorPasskeyEnrollmentFailed.Code, err.Code)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyRegistration_ServerError() {
	suite.mockAuthnProvider.On("InitiateEnrollment", mock.Anything, passkey.CredentialType, mock.Anything,
		mock.Anything).Return(nil, &tidcommon.InternalServerError).Once()

	result, err := suite.service.StartPasskeyRegistration(
		context.Background(), testUserID, testRelyingPartyID, testRelyingPartyName, nil, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
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

	suite.mockAuthnProvider.On("Enroll", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: "person",
			OUID:       testOrgUnit,
		}, nil).Once()

	// Completing enrollment authenticates the user, so an assertion is generated.
	mockAssertionResult := &assert.AssertionResult{
		Context: &assert.AssuranceContext{
			Authenticators: []common.AuthenticatorReference{
				{Authenticator: common.AuthenticatorPasskey, Step: 1},
			},
		},
	}
	suite.mockAssertGenerator.On(
		"GenerateAssertion",
		mock.Anything,
		mock.MatchedBy(func(refs []common.AuthenticatorReference) bool {
			return len(refs) == 1 && refs[0].Authenticator == common.AuthenticatorPasskey
		})).Return(mockAssertionResult, nil).Once()
	suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["userType"] == "person" && claims["ouId"] == testOrgUnit
		}), mock.Anything, mock.Anything).Return(testJWTToken, int64(3600), nil).Once()

	result, err := suite.service.FinishPasskeyRegistration(
		context.Background(), credential, sessionToken, false, "",
	)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
	suite.Equal("person", result.Type)
	suite.Equal(testOrgUnit, result.OUID)
	suite.Equal(testJWTToken, result.Assertion)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockAssertGenerator.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyRegistration_WithSkipAssertion() {
	credential := PasskeyPublicKeyCredentialDTO{
		ID:   "credential-id-123",
		Type: "public-key",
		Response: PasskeyCredentialResponseDTO{
			ClientDataJSON:    "base64-client-data",
			AttestationObject: "base64-attestation",
		},
	}

	suite.mockAuthnProvider.On("Enroll", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: "person",
			OUID:       testOrgUnit,
		}, nil).Once()

	result, err := suite.service.FinishPasskeyRegistration(context.Background(), credential, testSessionTkn, true, "")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
	suite.Empty(result.Assertion)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyRegistration_EnrollmentFailed() {
	credential := PasskeyPublicKeyCredentialDTO{
		ID:   "credential-id-123",
		Type: "public-key",
		Response: PasskeyCredentialResponseDTO{
			ClientDataJSON:    "base64-client-data",
			AttestationObject: "base64-attestation",
		},
	}

	enrollErr := authnprovidermgr.ErrorEnrollmentFailed
	suite.mockAuthnProvider.On("Enroll", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(providers.AuthUser{}, providers.AuthenticatedClaims(nil), &enrollErr).Once()

	result, err := suite.service.FinishPasskeyRegistration(context.Background(), credential, testSessionTkn, false, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(ErrorPasskeyEnrollmentFailed.Code, err.Code)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyRegistration_ClientErrorMappedToEnrollmentFailed() {
	credential := PasskeyPublicKeyCredentialDTO{
		ID:   "credential-id-123",
		Type: "public-key",
		Response: PasskeyCredentialResponseDTO{
			ClientDataJSON:    "base64-client-data",
			AttestationObject: "base64-attestation",
		},
	}

	clientErrs := []tidcommon.ServiceError{
		authnprovidermgr.ErrorUserNotFound,
		authnprovidermgr.ErrorInvalidRequest,
	}
	for _, clientErr := range clientErrs {
		enrollErr := clientErr
		suite.mockAuthnProvider.On("Enroll", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything).Return(providers.AuthUser{}, providers.AuthenticatedClaims(nil), &enrollErr).Once()

		result, err := suite.service.FinishPasskeyRegistration(context.Background(), credential, testSessionTkn, false,
			"")

		suite.NotNil(err)
		suite.Nil(result)
		suite.Equal(ErrorPasskeyEnrollmentFailed.Code, err.Code)
	}
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyRegistration_ServerError() {
	credential := PasskeyPublicKeyCredentialDTO{
		ID:   "credential-id-123",
		Type: "public-key",
		Response: PasskeyCredentialResponseDTO{
			ClientDataJSON:    "base64-client-data",
			AttestationObject: "base64-attestation",
		},
	}

	serverErr := tidcommon.InternalServerError
	suite.mockAuthnProvider.On("Enroll", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(providers.AuthUser{}, providers.AuthenticatedClaims(nil), &serverErr).Once()

	result, err := suite.service.FinishPasskeyRegistration(context.Background(), credential, testSessionTkn, false, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyAuthentication_Success() {
	expectedResponse := &passkey.PasskeyAuthenticationStartData{
		SessionToken: testSessionTkn,
	}

	suite.mockAuthnProvider.On(
		"InitiateAuthentication", mock.Anything, passkey.CredentialType,
		mock.MatchedBy(func(req *passkey.PasskeyAuthenticationStartRequest) bool {
			return req != nil && req.UserID == testUserID && req.RelyingPartyID == testRelyingPartyID
		}), mock.Anything).Return(expectedResponse, nil).Once()

	result, err := suite.service.StartPasskeyAuthentication(context.Background(), testUserID, testRelyingPartyID)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(expectedResponse, result)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyAuthentication_ServiceError() {
	serviceError := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "USER_NOT_FOUND",
		Error: tidcommon.I18nMessage{Key: "error.test.user_not_found", DefaultValue: "User not found"},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.no_user_found_with_the_given_id", DefaultValue: "No user found with the given ID",
		},
	}

	suite.mockAuthnProvider.On(
		"InitiateAuthentication", mock.Anything, passkey.CredentialType,
		mock.MatchedBy(func(req *passkey.PasskeyAuthenticationStartRequest) bool {
			return req != nil && req.UserID == testUserID && req.RelyingPartyID == testRelyingPartyID
		}), mock.Anything).Return(nil, serviceError).Once()

	result, err := suite.service.StartPasskeyAuthentication(context.Background(), testUserID, testRelyingPartyID)

	suite.NotNil(err)
	suite.Nil(result)
	// Provider-layer client errors are mapped to a clean passkey authentication error.
	suite.Equal(ErrorPasskeyAuthenticationFailed.Code, err.Code)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestStartPasskeyAuthentication_ServerError() {
	suite.mockAuthnProvider.On("InitiateAuthentication", mock.Anything, passkey.CredentialType, mock.Anything,
		mock.Anything).Return(nil, &tidcommon.InternalServerError).Once()

	result, err := suite.service.StartPasskeyAuthentication(context.Background(), testUserID, testRelyingPartyID)

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_Success() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
		UserHandle:        "base64-user-handle",
	}
	sessionToken := testSessionTkn

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: "person",
			OUID:       testOrgUnit,
		}, nil).Once()

	// Mock assertion generation
	mockAssertionResult := &assert.AssertionResult{
		Context: &assert.AssuranceContext{
			Authenticators: []common.AuthenticatorReference{
				{Authenticator: common.AuthenticatorPasskey, Step: 1},
			},
		},
	}
	suite.mockAssertGenerator.On(
		"GenerateAssertion",
		mock.Anything,
		mock.MatchedBy(func(refs []common.AuthenticatorReference) bool {
			return len(refs) == 1 && refs[0].Authenticator == common.AuthenticatorPasskey
		})).Return(mockAssertionResult, nil).Once()

	suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything, mock.Anything,
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

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: "person",
			OUID:       testOrgUnit,
		}, nil).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, sessionToken, true, "")

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
	suite.Empty(result.Assertion)
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_WithExistingAssertion() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
	}
	sessionToken := testSessionTkn
	existingAssertion := suite.createTestAssertion(testUserID)

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: "person",
			OUID:       testOrgUnit,
		}, nil).Once()

	// Mock JWT verification for existing assertion
	suite.mockJWTService.On("VerifyJWT", mock.Anything, existingAssertion, "", mock.Anything).Return(nil).Once()

	// Mock assertion update
	mockUpdatedResult := &assert.AssertionResult{
		Context: &assert.AssuranceContext{
			Authenticators: []common.AuthenticatorReference{
				{Authenticator: common.AuthenticatorCredentials, Step: 1},
				{Authenticator: common.AuthenticatorPasskey, Step: 2},
			},
		},
	}
	suite.mockAssertGenerator.On("UpdateAssertion", mock.Anything, mock.Anything, mock.Anything).
		Return(mockUpdatedResult, nil).Once()

	suite.mockJWTService.On("GenerateJWT", mock.Anything, testUserID, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("updated.jwt.token", int64(3600), nil).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, sessionToken, false, existingAssertion)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal("updated.jwt.token", result.Assertion)
	suite.mockAssertGenerator.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_ServiceError() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
	}

	serviceError := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "INVALID_SIGNATURE",
		Error: tidcommon.I18nMessage{Key: "error.test.invalid_signature", DefaultValue: "Invalid signature"},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.failed_to_verify_signature", DefaultValue: "Failed to verify signature",
		},
	}

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil), serviceError).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, testSessionTkn, false, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(ErrorPasskeyAuthenticationFailed.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestAuthenticateWithCredentialsEmptyInputs() {
	result, err := suite.service.AuthenticateWithCredentials(
		context.Background(), map[string]interface{}{}, map[string]interface{}{"password": "pass"}, false, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorEmptyAttributesOrCredentials.Code, err.Code)

	result, err = suite.service.AuthenticateWithCredentials(
		context.Background(), map[string]interface{}{"user": "u"}, map[string]interface{}{}, false, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorEmptyAttributesOrCredentials.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestAuthenticateWithCredentialsGetEntityReferenceError() {
	identifiers := map[string]interface{}{"username": "testuser"}
	authnCredentials := map[string]interface{}{"password": "testpass"}

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, identifiers, authnCredentials, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, nil)
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (*providers.EntityReference)(nil),
			&authnprovidermgr.ErrorGetEntityReferenceClientError)

	result, err := suite.service.AuthenticateWithCredentials(context.Background(), identifiers,
		authnCredentials, false, "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestAuthenticateWithCredentialsGetUserAttributesError() {
	identifiers := map[string]interface{}{"username": "testuser"}
	authnCredentials := map[string]interface{}{"password": "testpass"}

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, identifiers, authnCredentials, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, nil)
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, &providers.EntityReference{
			EntityID:   testUserID,
			EntityType: testUserType,
			OUID:       testOrgUnit,
		}, nil)
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (*providers.AttributesResponse)(nil),
			&authnprovidermgr.ErrorGetAttributesClientError)

	result, err := suite.service.AuthenticateWithCredentials(context.Background(), identifiers,
		authnCredentials, false, "")

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestMapCredentialsAuthnErrorCases() {
	logger := log.GetLogger()

	err := suite.service.mapCredentialsAuthnError(context.Background(), &authnprovidermgr.ErrorUserNotFound, logger)
	suite.Equal(common.ErrorUserNotFound.Code, err.Code)

	err = suite.service.mapCredentialsAuthnError(context.Background(), &authnprovidermgr.ErrorInvalidRequest, logger)
	suite.Equal(ErrorEmptyAttributesOrCredentials.Code, err.Code)

	err = suite.service.mapCredentialsAuthnError(context.Background(), &tidcommon.InternalServerError, logger)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestMapCredentialsGetAttributesErrorCases() {
	logger := log.GetLogger()

	err := suite.service.mapCredentialsGetAttributesError(context.Background(),
		&authnprovidermgr.ErrorGetAttributesClientError, logger)
	suite.Equal(ErrorInvalidToken.Code, err.Code)

	err = suite.service.mapCredentialsGetAttributesError(context.Background(),
		&authnprovidermgr.ErrorGetEntityReferenceClientError, logger)
	suite.Equal(ErrorInvalidToken.Code, err.Code)

	err = suite.service.mapCredentialsGetAttributesError(context.Background(),
		&tidcommon.InternalServerError, logger)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestMapFederatedAuthnErrorInvalidRequest() {
	logger := log.GetLogger()

	err := suite.service.mapFederatedAuthnError(context.Background(),
		&authnprovidermgr.ErrorInvalidRequest, logger)
	suite.Equal(ErrorFederatedAuthenticationFailed.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyOTPServerError() {
	sessionToken := testSessionTkn
	otpCode := "123456"

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil),
			&tidcommon.InternalServerError)

	result, err := suite.service.VerifyOTP(context.Background(), sessionToken, false, "", otpCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyOTPPassThroughError() {
	sessionToken := testSessionTkn
	otpCode := "123456"
	customErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "CUSTOM_ERROR",
		Error: tidcommon.I18nMessage{Key: "error.test.custom", DefaultValue: "Custom error"},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.custom_desc", DefaultValue: "Custom error description",
		},
	}

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil), customErr)

	result, err := suite.service.VerifyOTP(context.Background(), sessionToken, false, "", otpCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal("CUSTOM_ERROR", err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyOTPGetEntityReferenceError() {
	sessionToken := testSessionTkn
	otpCode := "123456"

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil)
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (*providers.EntityReference)(nil),
			&authnprovidermgr.ErrorGetEntityReferenceClientError)

	result, err := suite.service.VerifyOTP(context.Background(), sessionToken, false, "", otpCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_ServerError() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
	}

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil),
			&tidcommon.InternalServerError).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, testSessionTkn, false, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_AuthnFailed() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
	}

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil),
			&authnprovidermgr.ErrorAuthenticationFailed).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, testSessionTkn, false, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(ErrorPasskeyAuthenticationFailed.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_ClientErrorMappedToAuthnFailed() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
	}

	clientErrs := []tidcommon.ServiceError{
		authnprovidermgr.ErrorUserNotFound,
		authnprovidermgr.ErrorInvalidRequest,
	}
	for _, clientErr := range clientErrs {
		authnErr := clientErr
		suite.mockAuthnProvider.On(
			"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything).
			Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil), &authnErr).Once()

		result, err := suite.service.FinishPasskeyAuthentication(
			context.Background(), testCredentialID, testCredentialType, response, testSessionTkn, false, "")

		suite.NotNil(err)
		suite.Nil(result)
		suite.Equal(ErrorPasskeyAuthenticationFailed.Code, err.Code)
	}
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthenticationServiceTestSuite) TestFinishPasskeyAuthentication_GetEntityReferenceError() {
	response := PasskeyCredentialResponseDTO{
		ClientDataJSON:    "base64-client-data",
		AuthenticatorData: "base64-auth-data",
		Signature:         "base64-signature",
	}

	suite.mockAuthnProvider.On(
		"AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (*providers.EntityReference)(nil),
			&authnprovidermgr.ErrorGetEntityReferenceClientError).Once()

	result, err := suite.service.FinishPasskeyAuthentication(
		context.Background(), testCredentialID, testCredentialType, response, testSessionTkn, false, "")

	suite.NotNil(err)
	suite.Nil(result)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestFinishIDPAuthenticationGetEntityReferenceError() {
	sessionToken := suite.createSessionToken(providers.IDPTypeOAuth)
	suite.mockJWTService.On("VerifyJWT", mock.Anything, sessionToken, "auth-svc", mock.Anything).Return(nil)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything,
		mock.MatchedBy(func(creds map[string]interface{}) bool {
			_, ok := creds["federated"]
			return ok
		}), mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, nil).Once()
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (*providers.EntityReference)(nil),
			&authnprovidermgr.ErrorGetEntityReferenceClientError).Once()

	result, err := suite.service.FinishIDPAuthentication(
		context.Background(), providers.IDPTypeOAuth, sessionToken, false, "", testAuthCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestVerifyAndDecodeSessionTokenInvalidAuthData() {
	logger := log.GetLogger()
	payload := map[string]interface{}{
		"auth_data": "invalid_string_not_object",
	}
	payloadBytes, _ := json.Marshal(payload)
	encoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	badToken := "header." + encoded + ".signature"

	suite.mockJWTService.On("VerifyJWT", mock.Anything, badToken, "auth-svc", mock.Anything).Return(nil)

	result, err := suite.service.verifyAndDecodeSessionToken(context.Background(), badToken, logger)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *AuthenticationServiceTestSuite) TestNewAuthenticationService() {
	svc := newAuthenticationService(
		suite.mockIDPService,
		suite.mockJWTService,
		suite.mockAssertGenerator,
		suite.mockAuthnProvider,
		suite.mockOTPService,
		suite.mockNotifSenderSvc,
		suite.mockTemplateService,
		nil,
		suite.mockOAuthService,
		suite.mockOIDCService,
		suite.mockGoogleService,
		suite.mockGithubService,
	)
	suite.NotNil(svc)
}

func (suite *AuthenticationServiceTestSuite) createTestAssertion(subject string) string {
	assuranceCtx := map[string]interface{}{
		"aal": "aal1",
		"ial": "ial1",
		"authenticators": []map[string]interface{}{
			{
				"authenticator": common.AuthenticatorCredentials,
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
