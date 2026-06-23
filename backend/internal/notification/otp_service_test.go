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
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnOTP "github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/tests/mocks/authn/otpmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/clientmock"
	"github.com/thunder-id/thunderid/tests/mocks/templatemock"
)

type OTPServiceTestSuite struct {
	suite.Suite
	mockOTPAuthnSvc     *otpmock.OTPAuthnServiceInterfaceMock
	mockSenderService   *NotificationSenderMgtSvcInterfaceMock
	mockTemplateService *templatemock.TemplateServiceInterfaceMock
	service             *otpService
}

func TestOTPServiceTestSuite(t *testing.T) {
	suite.Run(t, new(OTPServiceTestSuite))
}

func (suite *OTPServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
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
	suite.mockOTPAuthnSvc = otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	suite.mockSenderService = NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	suite.mockTemplateService = templatemock.NewTemplateServiceInterfaceMock(suite.T())

	suite.service = &otpService{
		otpAuthnSvc:      suite.mockOTPAuthnSvc,
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
		Return(nil, &serviceerror.InternalServerError).Once()

	result, err := suite.service.SendOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
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
	suite.mockOTPAuthnSvc.On("GenerateOTP", mock.Anything, "+15559876543", "mobileNumber").
		Return("", "", int64(0), &serviceerror.InternalServerError).Once()

	res, err := suite.service.SendOTP(context.Background(), req)

	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_Success() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	suite.mockOTPAuthnSvc.On("GenerateOTP", mock.Anything, "+15559876543", "mobileNumber").
		Return("session-token-123", "123456", int64(300), nil).Once()

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
	suite.mockOTPAuthnSvc.On("GenerateOTP", mock.Anything, "+15559876543", "mobileNumber").
		Return("session-token-123", "123456", int64(300), nil).Once()

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
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_ClientProviderError() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	suite.mockOTPAuthnSvc.On("GenerateOTP", mock.Anything, "+15559876543", "mobileNumber").
		Return("session-token-123", "123456", int64(300), nil).Once()

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	cp := clientmock.NewClientFactoryInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything, mock.Anything).Return(nil, &serviceerror.InternalServerError).Once()
	suite.service.clientFactory = cp

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_ClientChannelNotSupported() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()
	suite.mockOTPAuthnSvc.On("GenerateOTP", mock.Anything, "+15559876543", "mobileNumber").
		Return("session-token-123", "123456", int64(300), nil).Once()

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
	suite.mockOTPAuthnSvc.On("GenerateOTP", mock.Anything, "+15559876543", "mobileNumber").
		Return("token", "654321", int64(300), nil).Once()

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
	suite.mockOTPAuthnSvc.On("GenerateOTP", mock.Anything, "+15559876543", "mobileNumber").
		Return("session-token-123", "123456", int64(300), nil).Once()

	renderErr := &serviceerror.ServiceError{Code: "TPL-5000"}
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(nil, renderErr).Once()

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

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
	req := common.VerifyOTPDTO{SessionToken: "session-token-123", OTPCode: "123456"}

	suite.mockOTPAuthnSvc.On("Authenticate", mock.Anything, "session-token-123", "123456").
		Return(nil, nil).Once()

	res, err := suite.service.VerifyOTP(context.Background(), req)
	suite.Nil(err)
	suite.NotNil(res)
	suite.Equal(common.OTPVerifyStatusVerified, res.Status)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_IncorrectOTP() {
	req := common.VerifyOTPDTO{SessionToken: "session-token-123", OTPCode: "000000"}

	suite.mockOTPAuthnSvc.On("Authenticate", mock.Anything, "session-token-123", "000000").
		Return(nil, &authnOTP.ErrorIncorrectOTP).Once()

	res, err := suite.service.VerifyOTP(context.Background(), req)
	suite.Nil(err)
	suite.NotNil(res)
	suite.Equal(common.OTPVerifyStatusInvalid, res.Status)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_InvalidSessionToken() {
	req := common.VerifyOTPDTO{SessionToken: "invalid-token", OTPCode: "123456"}

	suite.mockOTPAuthnSvc.On("Authenticate", mock.Anything, "invalid-token", "123456").
		Return(nil, &authnOTP.ErrorInvalidSessionToken).Once()

	res, err := suite.service.VerifyOTP(context.Background(), req)
	suite.NotNil(res)
	suite.Nil(err)
	suite.Equal(common.OTPVerifyStatusInvalid, res.Status)
}

func (suite *OTPServiceTestSuite) TestNewOTPService_Constructors() {
	svc := newOTPService(suite.mockOTPAuthnSvc, suite.mockSenderService,
		suite.mockTemplateService, clientmock.NewClientFactoryInterfaceMock(suite.T()))
	suite.NotNil(svc)
}
