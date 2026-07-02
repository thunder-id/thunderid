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
	"errors"
	"fmt"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/clientmock"
	"github.com/thunder-id/thunderid/tests/mocks/templatemock"
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
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockSenderService   *NotificationSenderMgtSvcInterfaceMock
	mockTemplateService *templatemock.TemplateServiceInterfaceMock
	service             *otpService
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
	}
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockSenderService = NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	suite.mockTemplateService = templatemock.NewTemplateServiceInterfaceMock(suite.T())

	suite.service = &otpService{
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OTPService")),
		jwtService:       suite.mockJWTService,
		senderMgtService: suite.mockSenderService,
		clientFactory:    clientmock.NewClientFactoryInterfaceMock(suite.T()),
		templateService:  suite.mockTemplateService,
	}
}

func (suite *OTPServiceTestSuite) getValidSender() *common.NotificationSenderDTO {
	return &common.NotificationSenderDTO{
		ID:       "sender-123",
		Name:     "Test SMS Sender",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createTestProperty("auth_token", "test-token", true),
			createTestProperty("sender_id", "+15551234567", false),
		},
	}
}

// --- GenerateOTP tests ---

func (suite *OTPServiceTestSuite) TestGenerateOTP_EmptyRecipient() {
	_, _, _, err := suite.service.GenerateOTP(context.Background(), "", "mobile_number")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_WhitespaceRecipient() {
	_, _, _, err := suite.service.GenerateOTP(context.Background(), "   ", "mobile_number")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_Success() {
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	sessionToken, otpValue, expirySeconds, err := suite.service.GenerateOTP(
		context.Background(), "+15559876543", "mobile_number")

	suite.Nil(err)
	suite.Equal("session-token-123", sessionToken)
	suite.Len(otpValue, 6)
	suite.Greater(expirySeconds, int64(0))
	for _, ch := range otpValue {
		suite.Contains("9245378016", string(ch))
	}
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
		context.Background(), "+15559876543", "mobile_number")

	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// --- SendOTP tests ---

func (suite *OTPServiceTestSuite) TestSendOTP_EmptyRecipient() {
	request := common.SendOTPDTO{
		Recipient: "",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	result, err := suite.service.SendOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_EmptySenderID() {
	request := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "",
		Channel:   "sms",
	}

	result, err := suite.service.SendOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSenderID.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_EmptyChannel() {
	request := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "",
	}

	// Empty channel is defaulted to SMS.
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(nil, nil).Once()

	result, err := suite.service.SendOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorSenderNotFound.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_UnsupportedChannel() {
	request := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "email",
	}

	result, err := suite.service.SendOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorUnsupportedChannel.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_SenderNotFound() {
	request := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(nil, nil).Once()

	result, err := suite.service.SendOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorSenderNotFound.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_SenderServiceError() {
	request := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").
		Return(nil, &tidcommon.InternalServerError).Once()

	result, err := suite.service.SendOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_SenderServiceError_NotFound() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(nil, &ErrorSenderNotFound).Once()

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorSenderNotFound.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_GenerateOTPError() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	jwtErr := &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "JWT-0001",
		Error: tidcommon.I18nMessage{DefaultValue: "JWT generation failed"},
	}
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("", int64(0), jwtErr).Once()

	res, err := suite.service.SendOTP(context.Background(), req)

	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_Success() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	mm := clientmock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(mock.Anything, common.ChannelTypeSMS, mock.Anything).Return(nil).Once()
	cp := clientmock.NewClientFactoryInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything, mock.Anything).Return(mm, nil).Once()
	suite.service.clientFactory = cp

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(err)
	suite.NotNil(res)
	suite.Equal("session-token-123", res.SessionToken)
}

func (suite *OTPServiceTestSuite) TestSendOTP_SendSMSError() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}
	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	mm := clientmock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(mock.Anything, common.ChannelTypeSMS, mock.Anything).Return(errors.New("send failed")).Once()
	cp := clientmock.NewClientFactoryInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything, mock.Anything).Return(mm, nil).Once()
	suite.service.clientFactory = cp

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_ClientProviderError() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	cp := clientmock.NewClientFactoryInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything, mock.Anything).Return(nil, &tidcommon.InternalServerError).Once()
	suite.service.clientFactory = cp

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_ClientChannelNotSupported() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	mm := clientmock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(false).Once()
	cp := clientmock.NewClientFactoryInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything, mock.Anything).Return(mm, nil).Once()
	suite.service.clientFactory = cp

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorUnsupportedChannel.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_TemplateRenderSuccess_UsesRenderedBody() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("token", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	renderedBody := "Your verification code is: 654321. This code will expire in 2 minutes."
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: renderedBody}, nil).Once()

	mm := clientmock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(mock.Anything, common.ChannelTypeSMS,
		common.NotificationData{Recipient: "+15559876543", Body: renderedBody}).
		Return(nil).Once()
	cp := clientmock.NewClientFactoryInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything, mock.Anything).Return(mm, nil).Once()
	suite.service.clientFactory = cp

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(err)
	suite.NotNil(res)
}

func (suite *OTPServiceTestSuite) TestSendOTP_TemplateRenderFailure_ReturnsInternalError() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	renderErr := &tidcommon.ServiceError{Code: "TPL-5000"}
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(nil, renderErr).Once()

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
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
	svc := newOTPService(suite.mockSenderService, suite.mockJWTService,
		suite.mockTemplateService, clientmock.NewClientFactoryInterfaceMock(suite.T()))
	suite.NotNil(svc)
}
