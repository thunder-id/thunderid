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

package otp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testSessionToken = "token123" // nolint:gosec // G101: test data, not a real secret
	testOTPCode      = "123456"
	testRecipient    = "+1234567890"
	testUserID       = "user-abc-123"
)

// buildTestJWT builds a minimal JWT string whose payload encodes the given otpSessionData.
// The header and signature are synthetic; VerifyJWT is mocked so no real crypto is needed.
func buildTestJWT(sessionData otpSessionData) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payloadJSON, _ := json.Marshal(map[string]interface{}{
		"otp_data": sessionData,
	})
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	return fmt.Sprintf("%s.%s.sig", header, payload)
}

type OTPAuthnServiceTestSuite struct {
	suite.Suite
	mockJWTService *jwtmock.JWTServiceInterfaceMock
	service        OTPAuthnServiceInterface
}

func TestOTPAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OTPAuthnServiceTestSuite))
}

func (suite *OTPAuthnServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Notification: config.NotificationConfig{
			OTP: config.OTPConfig{
				Length:                6,
				UseNumericOnly:        true,
				ValidityPeriodSeconds: 300,
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	suite.Require().NoError(err)
}

func (suite *OTPAuthnServiceTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.service = newOTPAuthnService(suite.mockJWTService)
}

// --- GenerateOTP tests ---

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPSuccess() {
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(testSessionToken, int64(0), nil)

	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), testRecipient, authnprovidercm.UserAttributeUserID)

	suite.Nil(err)
	suite.Equal(testSessionToken, sessionToken)
	suite.Len(otpValue, 6)
	for _, ch := range otpValue {
		suite.Contains("9245378016", string(ch))
	}
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPDefaultsRecipientAttr() {
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(testSessionToken, int64(0), nil)

	_, _, _, err := suite.service.GenerateOTP(context.Background(), testRecipient, "")
	suite.Nil(err)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPEmptyRecipient() {
	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), "", authnprovidercm.UserAttributeUserID)
	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPWhitespaceRecipient() {
	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), "   ", authnprovidercm.UserAttributeUserID)
	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPJWTError() {
	jwtErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "JWT-0001",
		ErrorDescription: core.I18nMessage{
			Key:          "error.test.jwt_failed",
			DefaultValue: "JWT generation failed",
		},
		Error: core.I18nMessage{
			Key:          "error.test.jwt_failed",
			DefaultValue: "JWT generation failed",
		},
	}
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("", int64(0), jwtErr)

	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), testRecipient, authnprovidercm.UserAttributeUserID)
	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

// --- Authenticate tests ---

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateSuccess() {
	sessionData := otpSessionData{
		Recipient:     testRecipient,
		RecipientAttr: userAttributeMobileNumber,
		OTPValue:      cryptolib.GenerateThumbprintFromString(testOTPCode),
		ExpiryTime:    9999999999999,
	}
	testToken := buildTestJWT(sessionData)

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, testToken, otpSessionAudience, mock.Anything,
	).Return(nil)

	result, err := suite.service.Authenticate(context.Background(), testToken, testOTPCode)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testRecipient, result.Token[userAttributeMobileNumber])
	suite.Equal(testRecipient, result.AuthenticatedClaims[userAttributeMobileNumber])
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateSuccessWithUserIDRecipientAttr() {
	sessionData := otpSessionData{
		Recipient:     testUserID,
		RecipientAttr: authnprovidercm.UserAttributeUserID,
		OTPValue:      cryptolib.GenerateThumbprintFromString(testOTPCode),
		ExpiryTime:    9999999999999,
	}
	testToken := buildTestJWT(sessionData)

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, testToken, otpSessionAudience, mock.Anything,
	).Return(nil)

	result, err := suite.service.Authenticate(context.Background(), testToken, testOTPCode)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.Token[authnprovidercm.UserAttributeUserID])
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateExpiredOTP() {
	sessionData := otpSessionData{
		Recipient:     testRecipient,
		RecipientAttr: userAttributeMobileNumber,
		OTPValue:      cryptolib.GenerateThumbprintFromString(testOTPCode),
		ExpiryTime:    1, // expired long ago
	}
	testToken := buildTestJWT(sessionData)

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, testToken, otpSessionAudience, mock.Anything,
	).Return(nil)

	result, err := suite.service.Authenticate(context.Background(), testToken, testOTPCode)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorIncorrectOTP.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateIncorrectOTP() {
	sessionData := otpSessionData{
		Recipient:     testRecipient,
		RecipientAttr: userAttributeMobileNumber,
		OTPValue:      cryptolib.GenerateThumbprintFromString(testOTPCode),
		ExpiryTime:    9999999999999,
	}
	testToken := buildTestJWT(sessionData)

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, testToken, otpSessionAudience, mock.Anything,
	).Return(nil)

	result, err := suite.service.Authenticate(context.Background(), testToken, "999999")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorIncorrectOTP.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateInvalidSessionToken() {
	jwtErr := &serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "JWT-0002",
		ErrorDescription: core.I18nMessage{
			Key:          "error.test.jwt_invalid",
			DefaultValue: "Invalid JWT",
		},
		Error: core.I18nMessage{
			Key:          "error.test.jwt_invalid",
			DefaultValue: "Invalid JWT",
		},
	}
	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, "bad-token", otpSessionAudience, mock.Anything,
	).Return(jwtErr)

	result, err := suite.service.Authenticate(context.Background(), "bad-token", testOTPCode)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateWithInvalidInputs() {
	tests := []struct {
		name         string
		sessionToken string
		otp          string
		expectedCode string
	}{
		{"EmptySessionToken", "", testOTPCode, ErrorInvalidSessionToken.Code},
		{"EmptyOTP", testSessionToken, "", ErrorInvalidOTP.Code},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			result, err := suite.service.Authenticate(context.Background(), tc.sessionToken, tc.otp)
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedCode, err.Code)
		})
	}
}
