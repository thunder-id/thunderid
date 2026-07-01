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

	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

const (
	testSenderID     = "sender123"
	testSessionToken = "token123"
)

type OTPAuthnServiceTestSuite struct {
	suite.Suite
	mockOTPService *notificationmock.OTPServiceInterfaceMock
	service        OTPAuthnServiceInterface
}

func TestOTPAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OTPAuthnServiceTestSuite))
}

func (suite *OTPAuthnServiceTestSuite) SetupTest() {
	suite.mockOTPService = notificationmock.NewOTPServiceInterfaceMock(suite.T())
	suite.service = newOTPAuthnService(suite.mockOTPService)
}

func (suite *OTPAuthnServiceTestSuite) TestSendOTPSuccess() {
	channel := notifcommon.ChannelTypeSMS
	recipient := "+1234567890"

	result := &notifcommon.SendOTPResultDTO{
		SessionToken: testSessionToken,
	}

	suite.mockOTPService.On("SendOTP", mock.Anything, mock.MatchedBy(func(dto notifcommon.SendOTPDTO) bool {
		return dto.SenderID == testSenderID && dto.Channel == string(channel) && dto.Recipient == recipient
	})).Return(result, nil)

	token, err := suite.service.SendOTP(context.Background(), testSenderID, channel, recipient)
	suite.Nil(err)
	suite.Equal(testSessionToken, token)
}

func (suite *OTPAuthnServiceTestSuite) TestSendOTPInvalidInputs() {
	tests := []struct {
		name         string
		senderID     string
		channel      notifcommon.ChannelType
		recipient    string
		expectedCode string
	}{
		{
			"EmptySenderID",
			"",
			notifcommon.ChannelTypeSMS,
			"+1234567890",
			ErrorInvalidSenderID.Code,
		},
		{
			"EmptyRecipient",
			testSenderID,
			notifcommon.ChannelTypeSMS,
			"",
			ErrorInvalidRecipient.Code,
		},
		{
			"UnsupportedChannel",
			testSenderID,
			notifcommon.ChannelType("email"),
			"test@example.com",
			ErrorUnsupportedChannel.Code,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			token, err := suite.service.SendOTP(context.Background(), tc.senderID, tc.channel, tc.recipient)
			suite.Empty(token)
			suite.NotNil(err)
			suite.Equal(tc.expectedCode, err.Code)
		})
	}
}

func (suite *OTPAuthnServiceTestSuite) TestSendOTPWithServiceError() {
	tests := []struct {
		name               string
		mockReturnErr      *tidcommon.ServiceError
		expectedErrCode    string
		expectedDescSubstr string
	}{
		{
			name: "ServiceError",
			mockReturnErr: &tidcommon.ServiceError{
				Type: tidcommon.ServerErrorType,
				Code: "INTERNAL_ERROR",
				ErrorDescription: tidcommon.I18nMessage{
					Key: "error.test.service_unavailable", DefaultValue: "Service unavailable",
				},
			},
			expectedErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "ClientError",
			mockReturnErr: &tidcommon.ServiceError{
				Type: tidcommon.ClientErrorType,
				Code: "INVALID_FORMAT",
				ErrorDescription: tidcommon.I18nMessage{
					Key: "error.test.invalid_phone_number_format", DefaultValue: "Invalid phone number format",
				},
			},
			expectedErrCode:    ErrorClientErrorFromOTPService.Code,
			expectedDescSubstr: "Invalid phone number format",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			freshOTP := notificationmock.NewOTPServiceInterfaceMock(suite.T())
			suite.service = newOTPAuthnService(freshOTP)
			freshOTP.On("SendOTP", mock.Anything, mock.Anything).Return(nil, tc.mockReturnErr)

			token, err := suite.service.SendOTP(context.Background(), testSenderID,
				notifcommon.ChannelTypeSMS, "+1234567890")
			suite.Empty(token)
			suite.NotNil(err)
			suite.Equal(tc.expectedErrCode, err.Code)

			if tc.expectedDescSubstr != "" {
				suite.Contains(err.ErrorDescription.DefaultValue, tc.expectedDescSubstr)
			}
		})
	}
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateSuccess() {
	otp := "123456"
	recipient := "+1234567890"

	verifyResult := &notifcommon.VerifyOTPResultDTO{
		Status:    notifcommon.OTPVerifyStatusVerified,
		Recipient: recipient,
	}

	suite.mockOTPService.On("VerifyOTP", mock.Anything, mock.MatchedBy(func(dto notifcommon.VerifyOTPDTO) bool {
		return dto.SessionToken == testSessionToken && dto.OTPCode == otp
	})).Return(verifyResult, nil)

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, otp)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(recipient, result.Token["mobile_number"])
	suite.Equal(recipient, result.AuthenticatedClaims["mobile_number"])
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateWithInvalidInputs() {
	tests := []struct {
		name         string
		sessionToken string
		otp          string
		expectedCode string
	}{
		{
			"EmptySessionToken",
			"",
			"123456",
			ErrorInvalidSessionToken.Code,
		},
		{
			"EmptyOTP",
			testSessionToken,
			"",
			ErrorInvalidOTP.Code,
		},
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

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateWithIncorrectOTP() {
	verifyResult := &notifcommon.VerifyOTPResultDTO{
		Status:    notifcommon.OTPVerifyStatusInvalid,
		Recipient: "+1234567890",
	}

	suite.mockOTPService.On("VerifyOTP", mock.Anything, mock.Anything).Return(verifyResult, nil)

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, "123456")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorIncorrectOTP.Code, err.Code)
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateWithOTPServiceError() {
	tests := []struct {
		name               string
		mockReturnErr      *tidcommon.ServiceError
		expectedErrCode    string
		expectedDescSubstr string
	}{
		{
			name: "ServiceError",
			mockReturnErr: &tidcommon.ServiceError{
				Type: tidcommon.ServerErrorType,
				Code: "INTERNAL_ERROR",
				ErrorDescription: tidcommon.I18nMessage{
					Key: "error.test.service_unavailable", DefaultValue: "Service unavailable",
				},
			},
			expectedErrCode: tidcommon.InternalServerError.Code,
		},
		{
			name: "ClientError",
			mockReturnErr: &tidcommon.ServiceError{
				Type: tidcommon.ClientErrorType,
				Code: "OTP_EXPIRED",
				ErrorDescription: tidcommon.I18nMessage{
					Key: "error.test.otp_has_expired", DefaultValue: "OTP has expired",
				},
			},
			expectedErrCode:    ErrorClientErrorFromOTPService.Code,
			expectedDescSubstr: "OTP has expired",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			freshOTP := notificationmock.NewOTPServiceInterfaceMock(suite.T())
			suite.service = newOTPAuthnService(freshOTP)
			freshOTP.On("VerifyOTP", mock.Anything, mock.Anything).Return(nil, tc.mockReturnErr)

			result, err := suite.service.Authenticate(context.Background(), testSessionToken, "123456")
			suite.Nil(result)
			suite.NotNil(err)
			suite.Equal(tc.expectedErrCode, err.Code)

			if tc.expectedDescSubstr != "" {
				suite.Contains(err.ErrorDescription.DefaultValue, tc.expectedDescSubstr)
			}
		})
	}
}

func (suite *OTPAuthnServiceTestSuite) TestAuthenticateWithEmptyRecipient() {
	verifyResult := &notifcommon.VerifyOTPResultDTO{
		Status:    notifcommon.OTPVerifyStatusVerified,
		Recipient: "",
	}
	suite.mockOTPService.On("VerifyOTP", mock.Anything, mock.Anything).Return(verifyResult, nil)

	result, err := suite.service.Authenticate(context.Background(), testSessionToken, "123456")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}
