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
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/messagemock"
	"github.com/thunder-id/thunderid/tests/mocks/templatemock"
)

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
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *OTPServiceTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockSenderService = NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	suite.mockTemplateService = templatemock.NewTemplateServiceInterfaceMock(suite.T())
	suite.service = &otpService{
		jwtService:       suite.mockJWTService,
		senderMgtService: suite.mockSenderService,
		clientProvider:   newNotificationClientProvider(),
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

	result, err := suite.service.SendOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidChannel.Code, err.Code)
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

func (suite *OTPServiceTestSuite) TestVerifyOTP_InvalidSessionToken() {
	request := common.VerifyOTPDTO{
		SessionToken: "invalid-token",
		OTPCode:      "123456",
	}

	// Expect VerifyJWT to be called; issuer can vary in tests so use Any
	suite.mockJWTService.EXPECT().VerifyJWT("invalid-token", "otp-svc", mock.Anything).
		Return(&ErrorInvalidSessionToken).Once()

	result, err := suite.service.VerifyOTP(context.Background(), request)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestGenerateOTP() {
	otp, err := suite.service.generateOTP()

	suite.NoError(err)
	suite.NotEmpty(otp.Value)
	suite.Len(otp.Value, 6) // Default OTP length
	suite.Greater(otp.ExpiryTimeInMillis, int64(0))
	suite.Greater(otp.ValidityPeriodInMillis, int64(0))
}

func (suite *OTPServiceTestSuite) TestGetOTPCharset() {
	charset := suite.service.getOTPCharset()

	suite.NotEmpty(charset)
	suite.Equal("9245378016", charset)
}

func (suite *OTPServiceTestSuite) TestGetOTPLength() {
	length := suite.service.getOTPLength()

	suite.Equal(6, length)
}

func (suite *OTPServiceTestSuite) TestUseOnlyNumericChars() {
	useNumeric := suite.service.useOnlyNumericChars()

	suite.True(useNumeric)
}

func (suite *OTPServiceTestSuite) TestGetOTPValidityPeriodInMillis() {
	validity := suite.service.getOTPValidityPeriodInMillis()

	suite.Equal(int64(120000), validity) // 2 minutes
}

func (suite *OTPServiceTestSuite) TestGetOTPCharset_NonNumeric() {
	// toggle package variable to force non-numeric branch
	prev := otpUseOnlyNumericChars
	otpUseOnlyNumericChars = false
	defer func() { otpUseOnlyNumericChars = prev }()

	charset := suite.service.getOTPCharset()
	suite.NotEmpty(charset)
	suite.NotEqual("9245378016", charset)
	suite.Equal("KIGXHOYSPRWCEFMVUQLZDNABJT9245378016", charset)
}

// SendOTP when OTP generation fails (force rand.Reader to error)
type badReader struct{}

func (b *badReader) Read(p []byte) (n int, err error) { return 0, errors.New("read error") }

func (suite *OTPServiceTestSuite) TestSendOTP_GenerateOTPError() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()

	// replace crypto/rand.Reader to force generateOTP to return an error
	orig := cryptorand.Reader
	cryptorand.Reader = &badReader{}
	defer func() { cryptorand.Reader = orig }()

	// ensure clientProvider is a no-op (won't be reached if generateOTP fails early)
	// mm := messagemock.NewMessageClientInterfaceMock(suite.T())
	suite.service.clientProvider = newNotificationClientProviderInterfaceMock(suite.T())

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

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	mm := messagemock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(common.ChannelTypeSMS, mock.Anything).Return(nil).Once()
	cp := newNotificationClientProviderInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything).Return(mm, nil).Once()
	suite.service.clientProvider = cp

	suite.mockJWTService.EXPECT().GenerateJWT(mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("session-token-123", int64(0), nil).Once()

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

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	mm := messagemock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(common.ChannelTypeSMS, mock.Anything).Return(errors.New("send failed")).Once()
	cp := newNotificationClientProviderInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything).Return(mm, nil).Once()
	suite.service.clientProvider = cp

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestSendOTP_GenerateJWTError() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}
	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	mm := messagemock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(common.ChannelTypeSMS, mock.Anything).Return(nil).Once()
	cp := newNotificationClientProviderInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything).Return(mm, nil).Once()
	suite.service.clientProvider = cp

	suite.mockJWTService.EXPECT().GenerateJWT(
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return("", int64(0), &serviceerror.InternalServerError).Once()

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_Success() {
	otpValue := "123456"
	otpHash := hash.GenerateThumbprintFromString(otpValue)
	expiry := time.Now().Add(1 * time.Minute).UnixMilli()

	payloadMap := map[string]interface{}{
		"otp_data": map[string]interface{}{
			"recipient":   "+15559876543",
			"channel":     "sms",
			"sender_id":   "sender-123",
			"otp_value":   otpHash,
			"expiry_time": expiry,
		},
	}

	payloadBytes, _ := json.Marshal(payloadMap)
	headerBytes, _ := json.Marshal(map[string]interface{}{"alg": "none"})

	headerEnc := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	token := fmt.Sprintf("%s.%s.", headerEnc, payloadEnc)

	suite.mockJWTService.EXPECT().VerifyJWT(token, mock.Anything, mock.Anything).Return(nil).Once()

	req := common.VerifyOTPDTO{SessionToken: token, OTPCode: otpValue}
	res, err := suite.service.VerifyOTP(context.Background(), req)
	suite.Nil(err)
	suite.NotNil(res)
	suite.Equal(common.OTPVerifyStatusVerified, res.Status)
	suite.Equal("+15559876543", res.Recipient)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_Expired() {
	otpValue := "123456"
	otpHash := hash.GenerateThumbprintFromString(otpValue)
	expiry := time.Now().Add(-1 * time.Minute).UnixMilli() // already expired

	payloadMap := map[string]interface{}{
		"otp_data": map[string]interface{}{
			"recipient":   "+15559876543",
			"channel":     "sms",
			"sender_id":   "sender-123",
			"otp_value":   otpHash,
			"expiry_time": expiry,
		},
	}

	payloadBytes, _ := json.Marshal(payloadMap)
	headerBytes, _ := json.Marshal(map[string]interface{}{"alg": "none"})

	headerEnc := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	token := fmt.Sprintf("%s.%s.", headerEnc, payloadEnc)

	suite.mockJWTService.EXPECT().VerifyJWT(token, mock.Anything, mock.Anything).Return(nil).Once()

	req := common.VerifyOTPDTO{SessionToken: token, OTPCode: otpValue}
	res, err := suite.service.VerifyOTP(context.Background(), req)
	suite.Nil(err)
	suite.NotNil(res)
	suite.Equal(common.OTPVerifyStatusInvalid, res.Status)
}

func (suite *OTPServiceTestSuite) TestSendOTP_ClientProviderError() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	// client provider returns a service error
	cp := newNotificationClientProviderInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything).Return(nil, &serviceerror.InternalServerError).Once()
	suite.service.clientProvider = cp

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

	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: "Your code is: 123456. Expires in 2 minutes."}, nil).Once()

	mm := messagemock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(false).Once()
	cp := newNotificationClientProviderInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything).Return(mm, nil).Once()
	suite.service.clientProvider = cp

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorUnsupportedChannel.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_MissingOTPData() {
	// build token with payload that lacks otp_data
	payloadMap := map[string]interface{}{"some": "value"}
	payloadBytes, _ := json.Marshal(payloadMap)
	headerBytes, _ := json.Marshal(map[string]interface{}{"alg": "none"})
	headerEnc := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	token := fmt.Sprintf("%s.%s.", headerEnc, payloadEnc)

	suite.mockJWTService.EXPECT().VerifyJWT(token, mock.Anything, mock.Anything).Return(nil).Once()

	req := common.VerifyOTPDTO{SessionToken: token, OTPCode: "123456"}
	res, err := suite.service.VerifyOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_BadPayloadDecode() {
	// craft token with invalid base64 payload part
	token := "hdr.invalid@@@.sig" // #nosec G101
	suite.mockJWTService.EXPECT().VerifyJWT(token, mock.Anything, mock.Anything).Return(nil).Once()

	req := common.VerifyOTPDTO{SessionToken: token, OTPCode: "123456"}
	res, err := suite.service.VerifyOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestVerifyOTP_Mismatch() {
	// prepare session data with a different OTP hash
	otpValue := "123456"
	wrongOTP := "000000"
	otpHash := hash.GenerateThumbprintFromString(wrongOTP) // stored hash is for wrongOTP
	expiry := time.Now().Add(1 * time.Minute).UnixMilli()

	payloadMap := map[string]interface{}{
		"otp_data": map[string]interface{}{
			"recipient":   "+15559876543",
			"channel":     "sms",
			"sender_id":   "sender-123",
			"otp_value":   otpHash,
			"expiry_time": expiry,
		},
	}

	payloadBytes, _ := json.Marshal(payloadMap)
	headerBytes, _ := json.Marshal(map[string]interface{}{"alg": "none"})

	headerEnc := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	token := fmt.Sprintf("%s.%s.", headerEnc, payloadEnc)

	suite.mockJWTService.EXPECT().VerifyJWT(token, mock.Anything, mock.Anything).Return(nil).Once()

	req := common.VerifyOTPDTO{SessionToken: token, OTPCode: otpValue}
	res, err := suite.service.VerifyOTP(context.Background(), req)
	suite.Nil(err)
	suite.NotNil(res)
	suite.Equal(common.OTPVerifyStatusInvalid, res.Status)
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

func (suite *OTPServiceTestSuite) TestVerifyOTP_UnmarshalError() {
	// create payload where otp_value is an array (will cause unmarshal into struct to fail)
	payloadMap := map[string]interface{}{
		"otp_data": map[string]interface{}{
			"recipient":   "+15559876543",
			"channel":     "sms",
			"sender_id":   "sender-123",
			"otp_value":   []int{1, 2, 3},
			"expiry_time": time.Now().Add(1 * time.Minute).UnixMilli(),
		},
	}

	payloadBytes, _ := json.Marshal(payloadMap)
	headerBytes, _ := json.Marshal(map[string]interface{}{"alg": "none"})

	headerEnc := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	token := fmt.Sprintf("%s.%s.", headerEnc, payloadEnc)

	suite.mockJWTService.EXPECT().VerifyJWT(token, mock.Anything, mock.Anything).Return(nil).Once()

	req := common.VerifyOTPDTO{SessionToken: token, OTPCode: "123456"}
	res, err := suite.service.VerifyOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidSessionToken.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestNewOTPService_Constructors() {
	svc := newOTPService(suite.mockSenderService, suite.mockJWTService, suite.mockTemplateService)
	suite.NotNil(svc)
}

func (suite *OTPServiceTestSuite) TestSendOTP_TemplateRenderSuccess_UsesRenderedBody() {
	req := common.SendOTPDTO{
		Recipient: "+15559876543",
		SenderID:  "sender-123",
		Channel:   "sms",
	}

	sender := suite.getValidSender()
	suite.mockSenderService.On("GetSender", mock.Anything, "sender-123").Return(sender, nil).Once()

	renderedBody := "Your verification code is: 654321. This code will expire in 2 minutes."
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: renderedBody}, nil).Once()

	mm := messagemock.NewNotificationClientInterfaceMock(suite.T())
	mm.EXPECT().IsChannelSupported(common.ChannelTypeSMS).Return(true).Once()
	mm.EXPECT().Send(common.ChannelTypeSMS, common.NotificationData{Recipient: "+15559876543", Body: renderedBody}).
		Return(nil).Once()
	cp := newNotificationClientProviderInterfaceMock(suite.T())
	cp.EXPECT().GetClient(mock.Anything).Return(mm, nil).Once()
	suite.service.clientProvider = cp

	suite.mockJWTService.EXPECT().GenerateJWT(mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("token", int64(0), nil).Once()

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

	renderErr := &serviceerror.ServiceError{Code: "TPL-5000"}
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioOTP,
		template.TemplateTypeSMS, mock.Anything).
		Return(nil, renderErr).Once()

	res, err := suite.service.SendOTP(context.Background(), req)
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestVerifyAndDecode_Success() {
	otpValue := "123456"
	otpHash := hash.GenerateThumbprintFromString(otpValue)
	expiry := time.Now().Add(1 * time.Minute).UnixMilli()

	payloadMap := map[string]interface{}{
		"otp_data": map[string]interface{}{
			"recipient":   "+15559876543",
			"channel":     "sms",
			"sender_id":   "sender-123",
			"otp_value":   otpHash,
			"expiry_time": expiry,
		},
	}

	payloadBytes, _ := json.Marshal(payloadMap)
	headerBytes, _ := json.Marshal(map[string]interface{}{"alg": "none"})

	headerEnc := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	token := fmt.Sprintf("%s.%s.", headerEnc, payloadEnc)

	suite.mockJWTService.EXPECT().VerifyJWT(token, mock.Anything, mock.Anything).Return(nil).Once()

	sessionData, svcErr := suite.service.verifyAndDecodeSessionToken(token, log.GetLogger())
	suite.Nil(svcErr)
	suite.NotNil(sessionData)
	suite.Equal("+15559876543", sessionData.Recipient)
}
