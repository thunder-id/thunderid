// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

func boolPtr(b bool) *bool { return &b }

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
				UseNumericOnly:        boolPtr(true),
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
		UseNumericOnly:        boolPtr(true),
		ValidityPeriodSeconds: 120,
		MaxGenerationAttempts: 3,
	}
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())

	cacheManager := cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")

	suite.service = &otpService{
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OTPService")),
		jwtService:      suite.mockJWTService,
		usedTokensCache: cache.GetCache[bool](cacheManager, "UsedOTPSessionTokensCache"),
	}
}

// --- GenerateOTP tests ---

func (suite *OTPServiceTestSuite) TestGenerateOTP_EmptyRecipient() {
	_, _, _, err := suite.service.GenerateOTP(context.Background(), "", "mobile_number", nil, "")
	suite.NotNil(err)
	suite.Equal(ErrorInvalidRecipient.Code, err.Code)
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_WhitespaceRecipient() {
	_, _, _, err := suite.service.GenerateOTP(context.Background(), "   ", "mobile_number", nil, "")
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
		context.Background(), "+15559876543", "mobile_number", nil, "")

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
		context.Background(), "+15559876543", "mobile_number", nil, prevToken)

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
		context.Background(), "+15559876543", "mobile_number", nil, prevToken)

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
		context.Background(), "+15559876543", "mobile_number", nil, "invalid-token")

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
		context.Background(), "+15559876543", "mobile_number", nil, "")

	suite.Empty(sessionToken)
	suite.Empty(otpValue)
	suite.NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// --- OTPConfig override tests ---

func (suite *OTPServiceTestSuite) TestGenerateOTP_WithLengthOverride() {
	length := 8
	cfg := &common.OTPConfig{Length: &length}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	sessionToken, otpValue, expirySeconds, err := suite.service.GenerateOTP(
		context.Background(), "+15559876543", "mobile_number", cfg, "")

	suite.Nil(err)
	suite.Equal("session-token-123", sessionToken)
	suite.Len(otpValue, 8)
	suite.Greater(expirySeconds, int64(0))
	for _, ch := range otpValue {
		suite.Contains("9245378016", string(ch))
	}
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_WithAlphanumericOverride() {
	numericOnly := false
	cfg := &common.OTPConfig{UseNumericOnly: &numericOnly}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	sessionToken, otpValue, expirySeconds, err := suite.service.GenerateOTP(
		context.Background(), "+15559876543", "mobile_number", cfg, "")

	suite.Nil(err)
	suite.Equal("session-token-123", sessionToken)
	suite.Greater(expirySeconds, int64(0))
	alphanumericCharset := "KIGXHOYSPRWCEFMVUQLZDNABJT9245378016"
	for _, ch := range otpValue {
		suite.Contains(alphanumericCharset, string(ch))
	}
}

func (suite *OTPServiceTestSuite) TestGenerateOTP_WithValidityOverride() {
	validity := 300
	cfg := &common.OTPConfig{ValidityPeriodSeconds: &validity}

	suite.mockJWTService.On("GenerateJWT",
		mock.Anything, otpSessionAudience, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return("session-token-123", int64(0), (*tidcommon.ServiceError)(nil)).Once()

	sessionToken, otpValue, expirySeconds, err := suite.service.GenerateOTP(
		context.Background(), "+15559876543", "mobile_number", cfg, "")

	suite.Nil(err)
	suite.Equal("session-token-123", sessionToken)
	suite.GreaterOrEqual(expirySeconds, int64(300))
	suite.Len(otpValue, 6)
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
	cacheManager := cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")
	svc := newOTPService(cacheManager, suite.mockJWTService)
	suite.NotNil(svc)
}

// --- resolveOTPConfig tests ---

func (suite *OTPServiceTestSuite) TestResolveOTPConfig_NilOverride() {
	cfg := suite.service.resolveOTPConfig(nil)

	suite.Equal(6, cfg.Length)
	suite.True(cfg.UsesNumericOnly())
	suite.Equal(120, cfg.ValidityPeriodSeconds)
}

func (suite *OTPServiceTestSuite) TestResolveOTPConfig_ValidLength() {
	length := 8
	cfg := suite.service.resolveOTPConfig(&common.OTPConfig{Length: &length})

	suite.Equal(8, cfg.Length)
}

func (suite *OTPServiceTestSuite) TestResolveOTPConfig_InvalidLengthBelowMin() {
	length := 3
	cfg := suite.service.resolveOTPConfig(&common.OTPConfig{Length: &length})

	suite.Equal(6, cfg.Length)
}

func (suite *OTPServiceTestSuite) TestResolveOTPConfig_InvalidLengthAboveMax() {
	length := 11
	cfg := suite.service.resolveOTPConfig(&common.OTPConfig{Length: &length})

	suite.Equal(6, cfg.Length)
}

func (suite *OTPServiceTestSuite) TestResolveOTPConfig_ValidValidity() {
	validity := 300
	cfg := suite.service.resolveOTPConfig(&common.OTPConfig{ValidityPeriodSeconds: &validity})

	suite.Equal(300, cfg.ValidityPeriodSeconds)
}

func (suite *OTPServiceTestSuite) TestResolveOTPConfig_InvalidValidityBelowMin() {
	validity := 29
	cfg := suite.service.resolveOTPConfig(&common.OTPConfig{ValidityPeriodSeconds: &validity})

	suite.Equal(120, cfg.ValidityPeriodSeconds)
}

func (suite *OTPServiceTestSuite) TestResolveOTPConfig_InvalidValidityAboveMax() {
	validity := 601
	cfg := suite.service.resolveOTPConfig(&common.OTPConfig{ValidityPeriodSeconds: &validity})

	suite.Equal(120, cfg.ValidityPeriodSeconds)
}

func (suite *OTPServiceTestSuite) TestResolveOTPConfig_UseNumericOnly() {
	numericOnly := false
	cfg := suite.service.resolveOTPConfig(&common.OTPConfig{UseNumericOnly: &numericOnly})

	suite.False(cfg.UsesNumericOnly())
}
