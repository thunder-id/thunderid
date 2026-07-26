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

package otp

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/notification"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

const (
	testSessionToken = "token123" // nolint:gosec // G101: test data, not a real secret
	testOTPCode      = "123456"
	testRecipient    = "+1234567890"
	testUserID       = "user-abc-123"
)

type OTPAuthnServiceTestSuite struct {
	suite.Suite
	mockNotifOTPSvc *notificationmock.OTPServiceInterfaceMock
	service         OTPAuthnServiceInterface
}

func TestOTPAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OTPAuthnServiceTestSuite))
}

func (suite *OTPAuthnServiceTestSuite) SetupTest() {
	suite.mockNotifOTPSvc = notificationmock.NewOTPServiceInterfaceMock(suite.T())
	suite.service = newOTPAuthnService(suite.mockNotifOTPSvc)
}

// --- GenerateOTP tests ---

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPEmptyRecipient() {
	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), "", authnprovidercm.UserAttributeUserID, "")
	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPWhitespaceRecipient() {
	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), "   ", authnprovidercm.UserAttributeUserID, "")
	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPDefaultsRecipientAttr() {
	suite.mockNotifOTPSvc.On("GenerateOTP",
		mock.Anything, testRecipient, authnprovidercm.UserAttributeUserID, "",
	).Return(testSessionToken, testOTPCode, int64(300), (*tidcommon.ServiceError)(nil)).Once()

	_, _, _, err := suite.service.GenerateOTP(context.Background(), testRecipient, "", "")
	suite.Nil(err)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPSuccess() {
	suite.mockNotifOTPSvc.On("GenerateOTP",
		mock.Anything, testRecipient, authnprovidercm.UserAttributeUserID, "prev-token",
	).Return(testSessionToken, testOTPCode, int64(300), (*tidcommon.ServiceError)(nil)).Once()

	sessionToken, otpValue, expirySeconds, err := suite.service.GenerateOTP(
		context.Background(), testRecipient, authnprovidercm.UserAttributeUserID, "prev-token")

	suite.Nil(err)
	suite.Equal(testSessionToken, sessionToken)
	suite.Equal(testOTPCode, otpValue)
	suite.Equal(int64(300), expirySeconds)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPMaxAttemptsExceeded() {
	suite.mockNotifOTPSvc.On("GenerateOTP",
		mock.Anything, testRecipient, authnprovidercm.UserAttributeUserID, "prev-token",
	).Return("", "", int64(0), &notification.ErrorMaxOTPAttemptsExceeded).Once()

	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), testRecipient, authnprovidercm.UserAttributeUserID, "prev-token")

	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(ErrorMaxOTPAttemptsExceeded.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPDelegatesError() {
	suite.mockNotifOTPSvc.On("GenerateOTP",
		mock.Anything, testRecipient, authnprovidercm.UserAttributeUserID, "",
	).Return("", "", int64(0), &tidcommon.InternalServerError).Once()

	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), testRecipient, authnprovidercm.UserAttributeUserID, "")

	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// --- Authenticate tests ---

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

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateSuccess() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return(&notifcommon.VerifyOTPResultDTO{
		Status:        notifcommon.OTPVerifyStatusVerified,
		Recipient:     testRecipient,
		RecipientAttr: userAttributeMobileNumber,
	}, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testRecipient, result.Token[userAttributeMobileNumber])
	suite.Equal(testRecipient, result.AuthenticatedClaims[userAttributeMobileNumber])
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateSuccessWithUserIDRecipientAttr() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return(&notifcommon.VerifyOTPResultDTO{
		Status:        notifcommon.OTPVerifyStatusVerified,
		Recipient:     testUserID,
		RecipientAttr: authnprovidercm.UserAttributeUserID,
	}, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.Token[authnprovidercm.UserAttributeUserID])
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateIncorrectOTP() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return(&notifcommon.VerifyOTPResultDTO{
		Status: notifcommon.OTPVerifyStatusInvalid,
	}, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorIncorrectOTP.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateInvalidSessionToken() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return((*notifcommon.VerifyOTPResultDTO)(nil), &notification.ErrorInvalidSessionToken).Once()

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateInternalError() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return((*notifcommon.VerifyOTPResultDTO)(nil), &tidcommon.InternalServerError).Once()

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateEmptyRecipientInResult() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return(&notifcommon.VerifyOTPResultDTO{
		Status:    notifcommon.OTPVerifyStatusVerified,
		Recipient: "",
	}, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateEmptyRecipientAttrDefaultsToMobileNumber() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return(&notifcommon.VerifyOTPResultDTO{
		Status:        notifcommon.OTPVerifyStatusVerified,
		Recipient:     testRecipient,
		RecipientAttr: "",
	}, (*tidcommon.ServiceError)(nil)).Once()

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testRecipient, result.Token[userAttributeMobileNumber])
}
