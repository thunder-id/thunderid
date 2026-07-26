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
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const otpSessionAudience = "otp-svc"

// otpSessionData holds the data encoded in the OTP session JWT.
// JSON field names match those used by authn/otp for session token compatibility.
type otpSessionData struct {
	Recipient     string `json:"recipient"`
	RecipientAttr string `json:"recipientAttr,omitempty"`
	OTPValue      string `json:"otp_value"`
	ExpiryTime    int64  `json:"expiry_time"`
	AttemptCount  int    `json:"attempt_count,omitempty"`
}

// generatedOTP holds the raw OTP value and its expiry timestamp.
type generatedOTP struct {
	Value              string
	ExpiryTimeInMillis int64
}

// OTPServiceInterface defines the interface for OTP operations.
type OTPServiceInterface interface {
	GenerateOTP(ctx context.Context, recipient, recipientAttr, previousSessionToken string) (
		sessionToken string, otpValue string, expirySeconds int64, svcErr *tidcommon.ServiceError)
	VerifyOTP(ctx context.Context, request common.VerifyOTPDTO) (
		*common.VerifyOTPResultDTO, *tidcommon.ServiceError)
}

// otpService implements the OTPServiceInterface.
type otpService struct {
	logger     *log.Logger
	jwtService jwt.JWTServiceInterface
}

// newOTPService returns a new instance of OTPServiceInterface.
func newOTPService(jwtSvc jwt.JWTServiceInterface) OTPServiceInterface {
	return &otpService{
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OTPService")),
		jwtService: jwtSvc,
	}
}

// GenerateOTP generates an OTP and session token for the recipient without delivering it.
func (s *otpService) GenerateOTP(ctx context.Context, recipient, recipientAttr, previousSessionToken string) (
	string, string, int64, *tidcommon.ServiceError) {
	logger := s.logger

	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return "", "", 0, &ErrorInvalidRecipient
	}

	attemptCount := 1
	if previousSessionToken != "" {
		prevData, svcErr := s.verifyAndDecodeSessionToken(ctx, previousSessionToken, logger)
		if svcErr != nil {
			return "", "", 0, svcErr
		}
		// Validate token binding to current request
		if prevData.Recipient != recipient || prevData.RecipientAttr != recipientAttr {
			logger.Debug(ctx, "Previous session token does not match current recipient or attribute")
			return "", "", 0, &ErrorInvalidSessionToken
		}
		attemptCount = prevData.AttemptCount + 1
	}

	maxAttempts := s.resolveOTPConfig().MaxGenerationAttempts
	if maxAttempts > 0 && attemptCount > maxAttempts {
		logger.Debug(ctx, "Maximum OTP generation attempts reached", log.Int("attemptCount", attemptCount))
		return "", "", 0, &ErrorMaxOTPAttemptsExceeded
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
		AttemptCount:  attemptCount,
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
