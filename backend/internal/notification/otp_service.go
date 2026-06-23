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
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/notification/client"
	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
)

const otpSessionAudience = "otp-svc"

// otpSessionData holds the data encoded in the OTP session JWT.
// JSON field names match those used by authn/otp for session token compatibility.
type otpSessionData struct {
	Recipient     string `json:"recipient"`
	RecipientAttr string `json:"recipientAttr,omitempty"`
	OTPValue      string `json:"otp_value"`
	ExpiryTime    int64  `json:"expiry_time"`
}

// generatedOTP holds the raw OTP value and its expiry timestamp.
type generatedOTP struct {
	Value              string
	ExpiryTimeInMillis int64
}

// OTPServiceInterface defines the interface for OTP operations.
type OTPServiceInterface interface {
	GenerateOTP(ctx context.Context, recipient, recipientAttr string) (
		sessionToken string, otpValue string, expirySeconds int64, svcErr *tidcommon.ServiceError)
	SendOTP(ctx context.Context, request common.SendOTPDTO) (*common.SendOTPResultDTO, *tidcommon.ServiceError)
	VerifyOTP(ctx context.Context, request common.VerifyOTPDTO) (
		*common.VerifyOTPResultDTO, *tidcommon.ServiceError)
}

// otpService implements the OTPServiceInterface.
type otpService struct {
	logger           *log.Logger
	jwtService       jwt.JWTServiceInterface
	senderMgtService NotificationSenderMgtSvcInterface
	clientFactory    client.ClientFactoryInterface
	templateService  template.TemplateServiceInterface
}

// newOTPService returns a new instance of OTPServiceInterface.
func newOTPService(notifSenderSvc NotificationSenderMgtSvcInterface,
	jwtSvc jwt.JWTServiceInterface, templateSvc template.TemplateServiceInterface,
	clientFactory client.ClientFactoryInterface) OTPServiceInterface {
	return &otpService{
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OTPService")),
		jwtService:       jwtSvc,
		senderMgtService: notifSenderSvc,
		clientFactory:    clientFactory,
		templateService:  templateSvc,
	}
}

// GenerateOTP generates an OTP and session token for the recipient without delivering it.
func (s *otpService) GenerateOTP(ctx context.Context, recipient, recipientAttr string) (
	string, string, int64, *tidcommon.ServiceError) {
	logger := s.logger

	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return "", "", 0, &ErrorInvalidRecipient
	}

	otp, err := s.generateOTP()
	if err != nil {
		logger.Error(ctx, "Failed to generate OTP", log.Error(err))
		return "", "", 0, &tidcommon.InternalServerError
	}

	sessionData := otpSessionData{
		Recipient:     recipient,
		RecipientAttr: recipientAttr,
		OTPValue:      cryptolib.GenerateThumbprintFromString(otp.Value),
		ExpiryTime:    otp.ExpiryTimeInMillis,
	}

	sessionToken, err := s.createSessionToken(ctx, sessionData)
	if err != nil {
		logger.Error(ctx, "Failed to create OTP session token", log.Error(err))
		return "", "", 0, &tidcommon.InternalServerError
	}

	expirySeconds := s.getOTPValidityPeriodInMillis() / 1000
	logger.Debug(ctx, "OTP generated successfully", log.MaskedString("recipient", recipient))
	return sessionToken, otp.Value, expirySeconds, nil
}

// SendOTP sends an OTP to the specified recipient using the provided sender.
func (s *otpService) SendOTP(
	ctx context.Context, otpDTO common.SendOTPDTO) (*common.SendOTPResultDTO, *tidcommon.ServiceError) {
	logger := s.logger
	logger.Debug(ctx, "Sending OTP", log.MaskedString("recipient", otpDTO.Recipient),
		log.String("channel", otpDTO.Channel), log.String("senderId", otpDTO.SenderID))

	if otpDTO.Channel == "" {
		otpDTO.Channel = string(common.ChannelTypeSMS)
	}

	if err := s.validateOTPSendRequest(otpDTO); err != nil {
		return nil, err
	}

	sender, svcErr := s.senderMgtService.GetSender(ctx, otpDTO.SenderID)
	if svcErr != nil {
		if svcErr.Code == ErrorSenderNotFound.Code {
			return nil, &ErrorSenderNotFound
		}
		return nil, &tidcommon.InternalServerError
	}
	if sender == nil {
		return nil, &ErrorSenderNotFound
	}

	sessionToken, otpValue, _, otpErr := s.GenerateOTP(ctx, otpDTO.Recipient, "mobile_number")
	if otpErr != nil {
		logger.Error(ctx, "Failed to generate OTP", log.String("error", otpErr.Code))
		return nil, &tidcommon.InternalServerError
	}

	switch common.ChannelType(otpDTO.Channel) {
	case common.ChannelTypeSMS:
		if svcErr := s.sendSMSOTP(ctx, otpDTO.Recipient, otpValue, *sender, logger); svcErr != nil {
			return nil, svcErr
		}
	default:
		return nil, &ErrorUnsupportedChannel
	}

	logger.Debug(ctx, "OTP sent successfully", log.MaskedString("recipient", otpDTO.Recipient))

	return &common.SendOTPResultDTO{
		SessionToken: sessionToken,
	}, nil
}

// VerifyOTP verifies the provided OTP against the session token.
func (s *otpService) VerifyOTP(
	ctx context.Context, otpDTO common.VerifyOTPDTO) (*common.VerifyOTPResultDTO, *tidcommon.ServiceError) {
	logger := s.logger
	logger.Debug(ctx, "Verifying OTP")

	if err := s.validateOTPVerifyRequest(otpDTO); err != nil {
		return nil, err
	}

	sessionData, svcErr := s.verifyAndDecodeSessionToken(ctx, otpDTO.SessionToken, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	if time.Now().UnixMilli() > sessionData.ExpiryTime {
		logger.Debug(ctx, "OTP has expired")
		return &common.VerifyOTPResultDTO{
			Status:        common.OTPVerifyStatusInvalid,
			Recipient:     sessionData.Recipient,
			RecipientAttr: sessionData.RecipientAttr,
		}, nil
	}

	if cryptolib.GenerateThumbprintFromString(otpDTO.OTPCode) != sessionData.OTPValue {
		logger.Debug(ctx, "Invalid OTP provided")
		return &common.VerifyOTPResultDTO{
			Status:        common.OTPVerifyStatusInvalid,
			Recipient:     sessionData.Recipient,
			RecipientAttr: sessionData.RecipientAttr,
		}, nil
	}

	return &common.VerifyOTPResultDTO{
		Status:        common.OTPVerifyStatusVerified,
		Recipient:     sessionData.Recipient,
		RecipientAttr: sessionData.RecipientAttr,
	}, nil
}

// validateOTPSendRequest validates the OTP send request.
func (s *otpService) validateOTPSendRequest(request common.SendOTPDTO) *tidcommon.ServiceError {
	if strings.TrimSpace(request.Recipient) == "" {
		return &ErrorInvalidRecipient
	}
	if request.SenderID == "" {
		return &ErrorInvalidSenderID
	}
	if request.Channel != string(common.ChannelTypeSMS) {
		return &ErrorUnsupportedChannel
	}
	return nil
}

// validateOTPVerifyRequest validates the OTP verify request.
func (s *otpService) validateOTPVerifyRequest(request common.VerifyOTPDTO) *tidcommon.ServiceError {
	if request.SessionToken == "" {
		return &ErrorInvalidSessionToken
	}
	if request.OTPCode == "" {
		return &ErrorInvalidOTP
	}
	return nil
}

// sendSMSOTP sends an SMS OTP to the recipient.
func (s *otpService) sendSMSOTP(ctx context.Context, recipient, otpVal string,
	sender common.NotificationSenderDTO, logger *log.Logger) *tidcommon.ServiceError {
	otpCfg := s.resolveOTPConfig()
	expiryMinutes := strconv.FormatInt(int64(otpCfg.ValidityPeriodSeconds)/60, 10)
	templateData := template.TemplateData{"otpCode": otpVal, "expiryMinutes": expiryMinutes}
	rendered, svcErr := s.templateService.Render(ctx, template.ScenarioOTP, template.TemplateTypeSMS, templateData)
	if svcErr != nil {
		logger.Error(ctx, "Failed to render SMS OTP template", log.String("error", svcErr.Code))
		return &tidcommon.InternalServerError
	}

	_client, clientSvcErr := s.clientFactory.GetClient(ctx, sender)
	if clientSvcErr != nil {
		return clientSvcErr
	}

	if !_client.IsChannelSupported(common.ChannelTypeSMS) {
		return &ErrorUnsupportedChannel
	}

	notifData := common.NotificationData{Recipient: recipient, Body: rendered.Body}
	if err := _client.Send(ctx, common.ChannelTypeSMS, notifData); err != nil {
		logger.Error(ctx, "Failed to send SMS OTP", log.Error(err))
		return &tidcommon.InternalServerError
	}

	return nil
}

// generateOTP generates a random OTP value based on server configuration.
func (s *otpService) generateOTP() (generatedOTP, error) {
	charSet := s.getOTPCharset()
	length := s.getOTPLength()

	chars := []rune(charSet)
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return generatedOTP{}, fmt.Errorf("failed to generate random number: %w", err)
		}
		result[i] = chars[n.Int64()]
	}

	now := time.Now().UnixMilli()
	validity := s.getOTPValidityPeriodInMillis()
	return generatedOTP{
		Value:              string(result),
		ExpiryTimeInMillis: now + validity,
	}, nil
}

// createSessionToken creates a JWT containing the OTP session data.
func (s *otpService) createSessionToken(ctx context.Context, sessionData otpSessionData) (string, error) {
	claims := map[string]interface{}{
		"otp_data": sessionData,
		"aud":      otpSessionAudience,
	}
	validitySeconds := (sessionData.ExpiryTime - time.Now().UnixMilli()) / 1000
	jwtConfig := config.GetServerRuntime().Config.JWT

	token, _, err := s.jwtService.GenerateJWT(
		ctx, otpSessionAudience, jwtConfig.Issuer, validitySeconds, claims, jwt.TokenTypeJWT, "")
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT token: %v", err)
	}
	return token, nil
}

// verifyAndDecodeSessionToken verifies the JWT signature and decodes the embedded OTP session data.
func (s *otpService) verifyAndDecodeSessionToken(ctx context.Context, token string,
	logger *log.Logger) (*otpSessionData, *tidcommon.ServiceError) {
	jwtConfig := config.GetServerRuntime().Config.JWT
	if svcErr := s.jwtService.VerifyJWT(ctx, token, otpSessionAudience, jwtConfig.Issuer); svcErr != nil {
		logger.Debug(ctx, "Invalid OTP session token", log.String("error", svcErr.Error.DefaultValue))
		return nil, &ErrorInvalidSessionToken
	}

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

	var sessionData otpSessionData
	if err := json.Unmarshal(otpDataBytes, &sessionData); err != nil {
		return nil, &ErrorInvalidSessionToken
	}

	return &sessionData, nil
}

// resolveOTPConfig returns the effective OTP configuration for this otpService.
// This is the single point of config access; future callers can pass flow-level overrides here.
func (s *otpService) resolveOTPConfig() config.OTPConfig {
	return config.GetServerRuntime().Config.Notification.OTP
}

// getOTPCharset returns the character set for OTP generation.
func (s *otpService) getOTPCharset() string {
	if s.resolveOTPConfig().UseNumericOnly {
		return "9245378016"
	}
	return "KIGXHOYSPRWCEFMVUQLZDNABJT9245378016"
}

// getOTPLength returns the configured OTP length.
func (s *otpService) getOTPLength() int {
	return s.resolveOTPConfig().Length
}

// getOTPValidityPeriodInMillis returns the OTP validity period in milliseconds.
func (s *otpService) getOTPValidityPeriodInMillis() int64 {
	return int64(s.resolveOTPConfig().ValidityPeriodSeconds) * 1000
}
