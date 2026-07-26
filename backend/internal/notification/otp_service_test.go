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

package notification

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

// buildTestJWT builds a minimal JWT whose payload encodes the given otpSessionData.
// The header and signature are synthetic; VerifyJWT is mocked so no real crypto is needed.
func buildTestJWT(sessionData otpSessionData) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payloadJSON, _ := json.Marshal(map[string]interface{}{
		"otp_data": sessionData,
	})
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	return fmt.Sprintf("%s.%s.sig", header, payload)
}

type OTPServiceTestSuite struct {
	suite.Suite
	mockJWTService *jwtmock.JWTServiceInterfaceMock
	service        *otpService
}

func TestOTPServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OTPServiceTestSuite))
}

func (suite *OTPServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
		Notification: config.NotificationConfig{
			OTP: config.OTPConfig{
				Length:                6,
				UseNumericOnly:        true,
				ValidityPeriodSeconds: 120,
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *OTPServiceTestSuite) SetupTest() {
	config.GetServerRuntime().Config.Notification.OTP = config.OTPConfig{
		Length:                6,
		UseNumericOnly:        true,
		ValidityPeriodSeconds: 120,
		MaxGenerationAttempts: 3,
	}
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	suite.service = &otpService{
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OTPService")),
		jwtService: suite.mockJWTService,
	}
}

// --- GenerateOTP tests ---

func (suite *OTPServiceTestSuite) TestGenerateOTP_EmptyRecipient() {
	_, _, _, err := suite.service.GenerateOTP(context.Background(), "", "mobile_number", "")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_WhitespaceRecipient() {
	_, _, _, err := suite.service.GenerateOTP(context.Background(), "   ", "mobile_number", "")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_Success() {
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			data, ok := claims["otp_data"].(otpSessionData)
			return ok && data.AttemptCount == 1
		}), mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	sessionToken, otpValue, expirySeconds, err := suite.service.GenerateOTP(
		context.Background(), "+15559876543", "mobile_number", "")

	suite.Nil(err)
	suite.Equal("session-token-123", sessionToken)
	suite.Len(otpValue, 6)
	suite.Greater(expirySeconds, int64(0))
	for _, ch := range otpValue {
		suite.Contains("9245378016", string(ch))
	}
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_WithPreviousSessionToken_IncrementsAttemptCount() {
	prevSessionData := otpSessionData{
		Recipient:     "+15559876543",
		RecipientAttr: "mobile_number",
		OTPValue:      cryptolib.GenerateThumbprintFromString("123456"),
		ExpiryTime:    9999999999999,
		AttemptCount:  1,
	}
	prevToken := buildTestJWT(prevSessionData)

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, prevToken, otpSessionAudience, mock.Anything,
	).Return((*tidcommon.ServiceError)(nil)).Once()

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			data, ok := claims["otp_data"].(otpSessionData)
			return ok && data.AttemptCount == 2
		}), mock.Anything, mock.Anything,
	).Return("session-token-456", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), "+15559876543", "mobile_number", prevToken)

	suite.Nil(err)
	suite.Equal("session-token-456", sessionToken)
	suite.Len(otpValue, 6)
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_ExceedsMaxGenerationAttempts() {
	prevSessionData := otpSessionData{
		Recipient:     "+15559876543",
		RecipientAttr: "mobile_number",
		OTPValue:      cryptolib.GenerateThumbprintFromString("123456"),
		ExpiryTime:    9999999999999,
		AttemptCount:  3, // max is 3
	}
	prevToken := buildTestJWT(prevSessionData)

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, prevToken, otpSessionAudience, mock.Anything,
	).Return((*tidcommon.ServiceError)(nil)).Once()

	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), "+15559876543", "mobile_number", prevToken)

	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(ErrorMaxOTPAttemptsExceeded.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_InvalidPreviousSessionToken() {
	jwtErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "JWT-0002",
		Error: tidcommon.I18nMessage{DefaultValue: "Invalid JWT"},
	}
	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, "invalid-token", otpSessionAudience, mock.Anything,
	).Return(jwtErr).Once()

	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), "+15559876543", "mobile_number", "invalid-token")

	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_JWTError() {
	jwtErr := &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "JWT-0001",
		Error: tidcommon.I18nMessage{DefaultValue: "JWT generation failed"},
	}
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("", int64(0), jwtErr).Once()

	sessionToken, otpValue, _, err := suite.service.GenerateOTP(
		context.Background(), "+15559876543", "mobile_number", "")

	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// --- VerifyOTP tests ---

func (suite *OTPServiceTestSuite) TestVerifyOTP_EmptySessionToken() {
	request := common.VerifyOTPDTO{
		SessionToken: "",
		OTPCode:      "123456",
	}

	result, err := suite.service.VerifyOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_EmptyOTPCode() {
	request := common.VerifyOTPDTO{
		SessionToken: "session-token-123",
		OTPCode:      "",
	}

	result, err := suite.service.VerifyOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidOTP.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_Success() {
	sessionData := otpSessionData{
		Recipient:     "+15559876543",
		RecipientAttr: "mobile_number",
		OTPValue:      cryptolib.GenerateThumbprintFromString("123456"),
		ExpiryTime:    9999999999999,
	}
	testToken := buildTestJWT(sessionData)

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, testToken, otpSessionAudience, mock.Anything,
	).Return((*tidcommon.ServiceError)(nil)).Once()

	req := common.VerifyOTPDTO{SessionToken: testToken, OTPCode: "123456"}
	res, err := suite.service.VerifyOTP(context.Background(), req)

	suite.Nil(err)
	suite.NotNil(res)
	suite.Equal(common.OTPVerifyStatusVerified, res.Status)
	suite.Equal("+15559876543", res.Recipient)
	suite.Equal("mobile_number", res.RecipientAttr)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_IncorrectOTP() {
	sessionData := otpSessionData{
		Recipient:     "+15559876543",
		RecipientAttr: "mobile_number",
		OTPValue:      cryptolib.GenerateThumbprintFromString("123456"),
		ExpiryTime:    9999999999999,
	}
	testToken := buildTestJWT(sessionData)

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, testToken, otpSessionAudience, mock.Anything,
	).Return((*tidcommon.ServiceError)(nil)).Once()

	req := common.VerifyOTPDTO{SessionToken: testToken, OTPCode: "000000"}
	res, err := suite.service.VerifyOTP(context.Background(), req)

	suite.Nil(err)
	suite.NotNil(res)
	suite.Equal(common.OTPVerifyStatusInvalid, res.Status)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_ExpiredOTP() {
	sessionData := otpSessionData{
		Recipient:     "+15559876543",
		RecipientAttr: "mobile_number",
		OTPValue:      cryptolib.GenerateThumbprintFromString("123456"),
		ExpiryTime:    1, // expired
	}
	testToken := buildTestJWT(sessionData)

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, testToken, otpSessionAudience, mock.Anything,
	).Return((*tidcommon.ServiceError)(nil)).Once()

	req := common.VerifyOTPDTO{SessionToken: testToken, OTPCode: "123456"}
	res, err := suite.service.VerifyOTP(context.Background(), req)

	suite.Nil(err)
	suite.NotNil(res)
	suite.Equal(common.OTPVerifyStatusInvalid, res.Status)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_InvalidSessionToken() {
	jwtErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "JWT-0002",
		Error: tidcommon.I18nMessage{DefaultValue: "Invalid JWT"},
	}
	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, "invalid-token", otpSessionAudience, mock.Anything,
	).Return(jwtErr).Once()

	req := common.VerifyOTPDTO{SessionToken: "invalid-token", OTPCode: "123456"}
	res, err := suite.service.VerifyOTP(context.Background(), req)

	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_MalformedJWTPayload() {
	malformedToken := "header.!!!invalid_base64!!!.sig"

	suite.mockJWTService.On("VerifyJWT",
		mock.Anything, malformedToken, otpSessionAudience, mock.Anything,
	).Return((*tidcommon.ServiceError)(nil)).Once()

	req := common.VerifyOTPDTO{SessionToken: malformedToken, OTPCode: "123456"}
	res, err := suite.service.VerifyOTP(context.Background(), req)

	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestNewOTPService_Constructor() {
	svc := newOTPService(suite.mockJWTService)
	suite.NotNil(svc)
}
