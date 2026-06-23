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

// Package otp implements the OTP authentication service.
package otp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	loggerComponentName       = "OTPAuthnService"
	userAttributeMobileNumber = "mobileNumber"
	otpSessionAudience        = "otp-svc"
)

// otpSessionData holds the data encoded in the OTP session JWT.
type otpSessionData struct {
	Recipient     string `json:"recipient"`
	RecipientAttr string `json:"recipientAttr,omitempty"`
	OTPValue      string `json:"otp_value"`
	ExpiryTime    int64  `json:"expiry_time"`
}

// generatedOTP holds the raw OTP value and its expiry.
type generatedOTP struct {
	Value              string
	ExpiryTimeInMillis int64
}

// OTPAuthnServiceInterface defines the interface for OTP authentication operations.
// Authenticate returns an error only for actual failures; a missing local user is NOT an error.
type OTPAuthnServiceInterface interface {
	GenerateOTP(ctx context.Context, recipient, recipientAttr string) (
		sessionToken string, otpValue string, expirySeconds int64, svcErr *serviceerror.ServiceError)
	Authenticate(ctx context.Context, sessionToken, otp string) (*common.AuthnResult, *serviceerror.ServiceError)
}

// otpAuthnService is the default implementation of OTPAuthnServiceInterface.
type otpAuthnService struct {
	jwtService jwt.JWTServiceInterface
}

// newOTPAuthnService creates a new instance of OTPAuthnService.
func newOTPAuthnService(jwtSvc jwt.JWTServiceInterface) OTPAuthnServiceInterface {
	return &otpAuthnService{
		jwtService: jwtSvc,
	}
}

// GenerateOTP generates an OTP and session token for the recipient without delivering it.
// recipientAttr identifies which attribute the recipient value represents (e.g. "userID",
// "mobileNumber"). Use authnprovidercm.UserAttributeUserID for authentication flows and
// the actual attribute name (e.g. "mobileNumber") for registration flows.
func (s *otpAuthnService) GenerateOTP(ctx context.Context,
	recipient, recipientAttr string) (string, string, int64, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Generating OTP", log.MaskedString("recipient", recipient))

	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return "", "", 0, &ErrorInvalidRecipient
	}
	if recipientAttr == "" {
		recipientAttr = authnprovidercm.UserAttributeUserID
	}

	otp, err := s.generateOTP()
	if err != nil {
		logger.Error(ctx, "Failed to generate OTP", log.Error(err))
		return "", "", 0, &serviceerror.InternalServerError
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
		return "", "", 0, &serviceerror.InternalServerError
	}

	expirySeconds := s.getOTPValidityPeriodInMillis() / 1000
	logger.Debug(ctx, "OTP generated successfully")
	return sessionToken, otp.Value, expirySeconds, nil
}

// Authenticate verifies the provided OTP against the session token and returns the authenticated user.
func (s *otpAuthnService) Authenticate(ctx context.Context, sessionToken,
	otpCode string) (*common.AuthnResult, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Verifying OTP for authentication")

	if svcErr := s.validateOTPVerifyRequest(sessionToken, otpCode); svcErr != nil {
		return nil, svcErr
	}

	sessionData, svcErr := s.verifyAndDecodeSessionToken(ctx, sessionToken, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	if time.Now().UnixMilli() > sessionData.ExpiryTime {
		logger.Debug(ctx, "OTP has expired")
		return nil, &ErrorIncorrectOTP
	}

	if cryptolib.GenerateThumbprintFromString(otpCode) != sessionData.OTPValue {
		logger.Debug(ctx, "Invalid OTP provided")
		return nil, &ErrorIncorrectOTP
	}

	return s.handleVerifyOTPResponse(ctx, sessionData, logger)
}

// validateOTPVerifyRequest validates the parameters for verifying an OTP.
func (s *otpAuthnService) validateOTPVerifyRequest(sessionToken, otp string) *serviceerror.ServiceError {
	if strings.TrimSpace(sessionToken) == "" {
		return &ErrorInvalidSessionToken
	}
	if strings.TrimSpace(otp) == "" {
		return &ErrorInvalidOTP
	}
	return nil
}

// handleVerifyOTPResponse constructs the authentication result from the verified session data.
func (s *otpAuthnService) handleVerifyOTPResponse(ctx context.Context, sessionData *otpSessionData,
	logger *log.Logger) (*common.AuthnResult, *serviceerror.ServiceError) {
	if sessionData.Recipient == "" {
		logger.Error(ctx, "Recipient not found in OTP session data")
		return nil, &serviceerror.InternalServerError
	}

	recipientAttr := sessionData.RecipientAttr
	if recipientAttr == "" {
		recipientAttr = userAttributeMobileNumber
	}

	return &common.AuthnResult{
		Token:               map[string]interface{}{recipientAttr: sessionData.Recipient},
		AuthenticatedClaims: map[string]interface{}{recipientAttr: sessionData.Recipient},
	}, nil
}

// generateOTP generates a random OTP value based on server configuration.
func (s *otpAuthnService) generateOTP() (generatedOTP, error) {
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
func (s *otpAuthnService) createSessionToken(ctx context.Context, sessionData otpSessionData) (string, error) {
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
func (s *otpAuthnService) verifyAndDecodeSessionToken(ctx context.Context, token string,
	logger *log.Logger) (*otpSessionData, *serviceerror.ServiceError) {
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

// getOTPCharset returns the character set for OTP generation.
func (s *otpAuthnService) getOTPCharset() string {
	if s.useOnlyNumericChars() {
		return "9245378016"
	}
	return "KIGXHOYSPRWCEFMVUQLZDNABJT9245378016"
}

// getOTPLength returns the configured OTP length.
func (s *otpAuthnService) getOTPLength() int {
	return config.GetServerRuntime().Config.Notification.OTP.Length
}

// useOnlyNumericChars returns whether to use only numeric characters.
func (s *otpAuthnService) useOnlyNumericChars() bool {
	return config.GetServerRuntime().Config.Notification.OTP.UseNumericOnly
}

// getOTPValidityPeriodInMillis returns the OTP validity period in milliseconds.
func (s *otpAuthnService) getOTPValidityPeriodInMillis() int64 {
	return int64(config.GetServerRuntime().Config.Notification.OTP.ValidityPeriodSeconds) * 1000
}
