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
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
)

// otpUseOnlyNumericChars indicates whether to use only numeric characters for OTP generation.
var otpUseOnlyNumericChars = true

// OTPServiceInterface defines the interface for OTP operations.
type OTPServiceInterface interface {
	SendOTP(ctx context.Context, request common.SendOTPDTO) (*common.SendOTPResultDTO, *serviceerror.ServiceError)
	VerifyOTP(ctx context.Context, request common.VerifyOTPDTO) (
		*common.VerifyOTPResultDTO, *serviceerror.ServiceError)
}

// otpService implements the OTPServiceInterface.
type otpService struct {
	jwtService       jwt.JWTServiceInterface
	senderMgtService NotificationSenderMgtSvcInterface
	clientProvider   notificationClientProviderInterface
	templateService  template.TemplateServiceInterface
}

// newOTPService returns a new instance of OTPServiceInterface.
func newOTPService(notifSenderSvc NotificationSenderMgtSvcInterface,
	jwtSvc jwt.JWTServiceInterface, templateSvc template.TemplateServiceInterface) OTPServiceInterface {
	return &otpService{
		jwtService:       jwtSvc,
		senderMgtService: notifSenderSvc,
		clientProvider:   newNotificationClientProvider(),
		templateService:  templateSvc,
	}
}

// SendOTP sends an OTP to the specified recipient using the provided sender.
func (s *otpService) SendOTP(
	ctx context.Context, otpDTO common.SendOTPDTO) (*common.SendOTPResultDTO, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OTPService"))
	logger.Debug("Sending OTP", log.MaskedString("recipient", otpDTO.Recipient),
		log.String("channel", otpDTO.Channel), log.String("senderId", otpDTO.SenderID))

	if err := s.validateOTPSendRequest(otpDTO); err != nil {
		return nil, err
	}

	sender, svcErr := s.senderMgtService.GetSender(ctx, otpDTO.SenderID)
	if svcErr != nil {
		if svcErr.Code == ErrorSenderNotFound.Code {
			return nil, &ErrorSenderNotFound
		}
		return nil, &serviceerror.InternalServerError
	}
	if sender == nil {
		return nil, &ErrorSenderNotFound
	}

	// TODO: Validate whether the sender supports the requested channel when necessary
	//  improvements are implemented.

	otp, err := s.generateOTP()
	if err != nil {
		logger.Error("Failed to generate OTP", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Send OTP based on channel
	switch common.ChannelType(otpDTO.Channel) {
	case common.ChannelTypeSMS:
		if svcErr := s.sendSMSOTP(ctx, otpDTO.Recipient, otp.Value, *sender, logger); svcErr != nil {
			return nil, svcErr
		}
	default:
		return nil, &ErrorUnsupportedChannel
	}

	// Create session token
	sessionData := common.OTPSessionData{
		Recipient:  otpDTO.Recipient,
		Channel:    otpDTO.Channel,
		SenderID:   otpDTO.SenderID,
		OTPValue:   hash.GenerateThumbprintFromString(otp.Value),
		ExpiryTime: otp.ExpiryTimeInMillis,
	}

	sessionToken, err := s.createSessionToken(ctx, sessionData)
	if err != nil {
		logger.Error("Failed to create session token", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	logger.Debug("OTP sent successfully", log.MaskedString("recipient", otpDTO.Recipient))

	return &common.SendOTPResultDTO{
		SessionToken: sessionToken,
	}, nil
}

// VerifyOTP verifies the provided OTP against the session token.
func (s *otpService) VerifyOTP(
	ctx context.Context, otpDTO common.VerifyOTPDTO) (*common.VerifyOTPResultDTO, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OTPService"))
	logger.Debug("Verifying OTP")

	if err := s.validateOTPVerifyRequest(otpDTO); err != nil {
		return nil, err
	}

	sessionData, svcErr := s.verifyAndDecodeSessionToken(otpDTO.SessionToken, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	// Check if OTP has expired
	currentTime := time.Now().UnixMilli()
	if currentTime > sessionData.ExpiryTime {
		logger.Debug("OTP has expired")
		return &common.VerifyOTPResultDTO{
			Status:    common.OTPVerifyStatusInvalid,
			Recipient: sessionData.Recipient,
		}, nil
	}

	// Verify OTP value by comparing hashes
	providedOTPHash := hash.GenerateThumbprintFromString(otpDTO.OTPCode)
	if providedOTPHash != sessionData.OTPValue {
		logger.Debug("Invalid OTP provided")
		return &common.VerifyOTPResultDTO{
			Status:    common.OTPVerifyStatusInvalid,
			Recipient: sessionData.Recipient,
		}, nil
	}

	return &common.VerifyOTPResultDTO{
		Status:    common.OTPVerifyStatusVerified,
		Recipient: sessionData.Recipient,
	}, nil
}

// validateOTPSendRequest validates the OTP send request.
func (s *otpService) validateOTPSendRequest(request common.SendOTPDTO) *serviceerror.ServiceError {
	if request.Recipient == "" {
		return &ErrorInvalidRecipient
	}
	if request.SenderID == "" {
		return &ErrorInvalidSenderID
	}
	if request.Channel == "" {
		return &ErrorInvalidChannel
	}
	if request.Channel != string(common.ChannelTypeSMS) {
		return &ErrorUnsupportedChannel
	}
	return nil
}

// validateOTPVerifyRequest validates the OTP verify request.
func (s *otpService) validateOTPVerifyRequest(request common.VerifyOTPDTO) *serviceerror.ServiceError {
	if request.SessionToken == "" {
		return &ErrorInvalidSessionToken
	}
	if request.OTPCode == "" {
		return &ErrorInvalidOTP
	}
	return nil
}

// generateOTP generates a random OTP based on the configurations.
func (s *otpService) generateOTP() (common.OTP, error) {
	charSet := s.getOTPCharset()
	otpLength := s.getOTPLength()

	chars := []rune(charSet)
	result := make([]rune, otpLength)

	for i := 0; i < otpLength; i++ {
		max := big.NewInt(int64(len(chars)))
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return common.OTP{}, fmt.Errorf("failed to generate random number: %w", err)
		}
		result[i] = chars[n.Int64()]
	}

	token := string(result)
	currentTime := time.Now().UnixMilli()
	validityPeriod := s.getOTPValidityPeriodInMillis()

	return common.OTP{
		Value:                  token,
		GeneratedTimeInMillis:  currentTime,
		ValidityPeriodInMillis: validityPeriod,
		ExpiryTimeInMillis:     currentTime + validityPeriod,
	}, nil
}

// getOTPCharset returns the character set for OTP generation.
func (s *otpService) getOTPCharset() string {
	if s.useOnlyNumericChars() {
		return "9245378016"
	}
	return "KIGXHOYSPRWCEFMVUQLZDNABJT9245378016"
}

// getOTPLength returns the length of the OTP.
func (s *otpService) getOTPLength() int {
	// TODO: This needs to be configured as a property
	return 6
}

// useOnlyNumericChars determines whether to use only numeric characters.
func (s *otpService) useOnlyNumericChars() bool {
	// TODO: This needs to be configured as a property
	return otpUseOnlyNumericChars
}

// getOTPValidityPeriodInMillis returns the validity period of the OTP in milliseconds.
func (s *otpService) getOTPValidityPeriodInMillis() int64 {
	// TODO: This needs to be configured as a property
	return 120000 // 2 minutes
}

// sendSMSOTP sends an SMS OTP to the recipient.
func (s *otpService) sendSMSOTP(ctx context.Context, recipient, otp string,
	sender common.NotificationSenderDTO, logger *log.Logger) *serviceerror.ServiceError {
	expiryMinutes := strconv.FormatInt(s.getOTPValidityPeriodInMillis()/60000, 10)
	templateData := template.TemplateData{"otp": otp, "expiryMinutes": expiryMinutes}
	rendered, svcErr := s.templateService.Render(ctx, template.ScenarioOTP, template.TemplateTypeSMS, templateData)
	if svcErr != nil {
		logger.Error("Failed to render SMS OTP template", log.String("error", svcErr.Code))
		return &serviceerror.InternalServerError
	}

	_client, clientSvcErr := s.clientProvider.GetClient(sender)
	if clientSvcErr != nil {
		return clientSvcErr
	}

	if !_client.IsChannelSupported(common.ChannelTypeSMS) {
		return &ErrorUnsupportedChannel
	}

	notifData := common.NotificationData{Recipient: recipient, Body: rendered.Body}
	if err := _client.Send(common.ChannelTypeSMS, notifData); err != nil {
		logger.Error("Failed to send SMS OTP", log.Error(err))
		return &serviceerror.InternalServerError
	}

	return nil
}

// createSessionToken creates a JWT session token with OTP session data.
func (s *otpService) createSessionToken(ctx context.Context, sessionData common.OTPSessionData) (string, error) {
	claims := map[string]interface{}{
		"otp_data": sessionData,
	}

	// Use a short validity period for the token (same as OTP expiry)
	validityPeriod := (sessionData.ExpiryTime - time.Now().UnixMilli()) / 1000
	jwtConfig := config.GetServerRuntime().Config.JWT

	claims["aud"] = "otp-svc"
	token, _, err := s.jwtService.GenerateJWT(
		ctx, "otp-svc", jwtConfig.Issuer, validityPeriod, claims, jwt.TokenTypeJWT, "")
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT token: %v", err)
	}

	return token, nil
}

// verifyAndDecodeSessionToken verifies the JWT signature and decodes the session data.
func (s *otpService) verifyAndDecodeSessionToken(token string, logger *log.Logger) (
	*common.OTPSessionData, *serviceerror.ServiceError) {
	// Verify JWT signature
	jwtConfig := config.GetServerRuntime().Config.JWT
	svcErr := s.jwtService.VerifyJWT(token, "otp-svc", jwtConfig.Issuer)
	if svcErr != nil {
		logger.Debug("Invalid session token", log.String("error", svcErr.Error.DefaultValue))
		return nil, &ErrorInvalidSessionToken
	}

	// Parse and extract OTP session data
	payload, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		return nil, &ErrorInvalidSessionToken
	}

	otpDataClaim, ok := payload["otp_data"]
	if !ok {
		return nil, &ErrorInvalidSessionToken
	}

	otpDataBytes, err := json.Marshal(otpDataClaim)
	if err != nil {
		return nil, &ErrorInvalidSessionToken
	}

	var sessionData common.OTPSessionData
	err = json.Unmarshal(otpDataBytes, &sessionData)
	if err != nil {
		return nil, &ErrorInvalidSessionToken
	}

	return &sessionData, nil
}
