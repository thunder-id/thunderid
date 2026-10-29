// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
		context.Background(), "", authnprovidercm.UserAttributeUserID, nil)
	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPWhitespaceRecipient() {
	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), "   ", authnprovidercm.UserAttributeUserID, nil)
	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPDefaultsRecipientAttr() {
	suite.mockNotifOTPSvc.On("GenerateOTP",
		mock.Anything, testRecipient, authnprovidercm.UserAttributeUserID, mock.Anything,
	).Return(testSessionToken, testOTPCode, int64(300), (*tidcommon.ServiceError)(nil)).Once()

	_, _, _, err := suite.service.GenerateOTP(context.Background(), testRecipient, "", nil)
	suite.Nil(err)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPSuccess() {
	suite.mockNotifOTPSvc.On("GenerateOTP",
		mock.Anything, testRecipient, authnprovidercm.UserAttributeUserID, mock.Anything,
	).Return(testSessionToken, testOTPCode, int64(300), (*tidcommon.ServiceError)(nil)).Once()

	sessionToken, otpValue, expirySeconds, err := suite.service.GenerateOTP(
		context.Background(), testRecipient, authnprovidercm.UserAttributeUserID, nil)

	suite.Nil(err)
	suite.Equal(testSessionToken, sessionToken)
	suite.Equal(testOTPCode, otpValue)
	suite.Equal(int64(300), expirySeconds)
}

func (suite *OTPAuthnServiceTestSuite) TestGenerateOTPDelegatesError() {
	suite.mockNotifOTPSvc.On("GenerateOTP",
		mock.Anything, testRecipient, authnprovidercm.UserAttributeUserID, mock.Anything,
	).Return("", "", int64(0), &tidcommon.InternalServerError).Once()

	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), testRecipient, authnprovidercm.UserAttributeUserID, nil)

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
			result, _, err := suite.service.Authenticate(context.Background(), tc.sessionToken, tc.otp)
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

	result, failure, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(err)
	suite.Nil(failure, "a verified code is not a failure")
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

	result, failure, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(err)
	suite.Nil(failure, "a verified code is not a failure")
	suite.NotNil(result)
	suite.Equal(testUserID, result.Token[authnprovidercm.UserAttributeUserID])
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateIncorrectOTP() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return(&notifcommon.VerifyOTPResultDTO{
		Status: notifcommon.OTPVerifyStatusInvalid,
	}, (*tidcommon.ServiceError)(nil)).Once()

	result, _, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorIncorrectOTP.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateInvalidSessionToken() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return((*notifcommon.VerifyOTPResultDTO)(nil), &notification.ErrorInvalidSessionToken).Once()

	result, _, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateInternalError() {
	suite.mockNotifOTPSvc.On("VerifyOTP",
		mock.Anything, notifcommon.VerifyOTPDTO{SessionToken: testSessionToken, OTPCode: testOTPCode},
	).Return((*notifcommon.VerifyOTPResultDTO)(nil), &tidcommon.InternalServerError).Once()

	result, _, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

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

	result, _, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

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

	result, failure, err := suite.service.Authenticate(context.Background(), testSessionToken, testOTPCode)

	suite.Nil(err)
	suite.Nil(failure, "a verified code is not a failure")
	suite.NotNil(result)
	suite.Equal(testRecipient, result.Token[userAttributeMobileNumber])
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateIncorrectOTPCarriesTheRecipient() {
	suite.mockNotifOTPSvc.On("VerifyOTP", mock.Anything, mock.Anything).
		Return(&notifcommon.VerifyOTPResultDTO{
			Status:        notifcommon.OTPVerifyStatusInvalid,
			Recipient:     "+1234567890",
			RecipientAttr: "mobile_number",
		}, nil).Once()

	result, failure, err := suite.service.Authenticate(
		context.Background(), testSessionToken, testOTPCode)

	suite.Nil(result)
	suite.Require().NotNil(err)
	suite.Equal(ErrorIncorrectOTP.Code, err.Code)
	suite.Require().NotNil(failure, "a wrong OTP knows whose code it was")
	suite.Equal("mobile_number", failure.RecipientAttr)
	suite.Equal("+1234567890", failure.Recipient)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateIncorrectOTPWithNoRecipientCarriesNothing() {
	suite.mockNotifOTPSvc.On("VerifyOTP", mock.Anything, mock.Anything).
		Return(&notifcommon.VerifyOTPResultDTO{
			Status: notifcommon.OTPVerifyStatusInvalid,
		}, nil).Once()

	_, failure, err := suite.service.Authenticate(
		context.Background(), testSessionToken, testOTPCode)

	suite.Require().NotNil(err)
	suite.Nil(failure)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateExpiredOTPCarriesNothing() {
	suite.mockNotifOTPSvc.On("VerifyOTP", mock.Anything, mock.Anything).
		Return(&notifcommon.VerifyOTPResultDTO{
			Status:        notifcommon.OTPVerifyStatusExpired,
			Recipient:     "+1234567890",
			RecipientAttr: "mobile_number",
		}, nil).Once()

	result, failure, err := suite.service.Authenticate(
		context.Background(), testSessionToken, testOTPCode)

	suite.Nil(result)
	suite.Require().NotNil(err)
	suite.Equal(ErrorIncorrectOTP.Code, err.Code,
		"an expired code must be indistinguishable from a wrong one to the caller")
	suite.Nil(failure, "an expired code compared nothing, so it attributes nothing")
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateUnrecognizedOTPStatusCarriesNothing() {
	suite.mockNotifOTPSvc.On("VerifyOTP", mock.Anything, mock.Anything).
		Return(&notifcommon.VerifyOTPResultDTO{
			Status:        notifcommon.OTPVerifyStatus("SOMETHING_NEW"),
			Recipient:     "+1234567890",
			RecipientAttr: "mobile_number",
		}, nil).Once()

	result, failure, err := suite.service.Authenticate(
		context.Background(), testSessionToken, testOTPCode)

	suite.Nil(result)
	suite.Require().NotNil(err)
	suite.Equal(ErrorIncorrectOTP.Code, err.Code)
	suite.Nil(failure, "only an INVALID code attributes a failure")
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateUnreadableSessionCarriesNothing() {
	suite.mockNotifOTPSvc.On("VerifyOTP", mock.Anything, mock.Anything).
		Return(nil, &notification.ErrorInvalidSessionToken).Once()

	_, failure, err := suite.service.Authenticate(
		context.Background(), testSessionToken, testOTPCode)

	suite.Require().NotNil(err)
	suite.Nil(failure)
}
