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
	"fmt"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/notification"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	loggerComponentName       = "OTPAuthnService"
	userAttributeMobileNumber = "mobileNumber"
)

var supportedChannels = []notifcommon.ChannelType{notifcommon.ChannelTypeSMS}

// OTPAuthnServiceInterface defines the interface for OTP authentication operations.
// This is a wrapper over the notification.OTPServiceInterface to perform user authentication.
// Authenticate returns an error only for actual failures; a missing local user is NOT an error.
type OTPAuthnServiceInterface interface {
	SendOTP(ctx context.Context, senderID string, channel notifcommon.ChannelType,
		recipient string) (string, *serviceerror.ServiceError)
	Authenticate(ctx context.Context, sessionToken, otp string) (*common.AuthnResult, *serviceerror.ServiceError)
}

// otpAuthnService is the default implementation of OTPAuthnServiceInterface.
type otpAuthnService struct {
	otpService     notification.OTPServiceInterface
	entityProvider entityprovider.EntityProviderInterface
}

// newOTPAuthnService creates a new instance of OTPAuthnService.
func newOTPAuthnService(otpSvc notification.OTPServiceInterface,
	entityProvider entityprovider.EntityProviderInterface) OTPAuthnServiceInterface {
	return &otpAuthnService{
		otpService:     otpSvc,
		entityProvider: entityProvider,
	}
}

// SendOTP sends an OTP to the specified recipient using the provided sender.
func (s *otpAuthnService) SendOTP(ctx context.Context, senderID string, channel notifcommon.ChannelType,
	recipient string) (string, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Sending OTP for authentication", log.MaskedString("recipient", recipient),
		log.String("channel", string(channel)))

	if svcErr := s.validateOTPSendRequest(senderID, channel, recipient); svcErr != nil {
		return "", svcErr
	}

	otpData := notifcommon.SendOTPDTO{
		SenderID:  senderID,
		Channel:   string(channel),
		Recipient: recipient,
	}
	result, svcErr := s.otpService.SendOTP(ctx, otpData)
	if svcErr != nil {
		return "", s.handleOTPServiceError(ctx, svcErr, false, logger)
	}

	logger.Debug(ctx, "OTP sent successfully, session token generated")
	return result.SessionToken, nil
}

// Authenticate verifies the provided OTP against the session token and returns the authenticated user.
func (s *otpAuthnService) Authenticate(ctx context.Context, sessionToken,
	otp string) (*common.AuthnResult, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Verifying OTP for authentication")

	if svcErr := s.validateOTPVerifyRequest(sessionToken, otp); svcErr != nil {
		return nil, svcErr
	}

	verifyData := notifcommon.VerifyOTPDTO{
		SessionToken: sessionToken,
		OTPCode:      otp,
	}
	result, svcErr := s.otpService.VerifyOTP(ctx, verifyData)
	if svcErr != nil {
		return nil, s.handleOTPServiceError(ctx, svcErr, true, logger)
	}

	return s.handleVerifyOTPResponse(ctx, result, logger)
}

// validateOTPSendRequest validates the parameters for sending an OTP.
func (s *otpAuthnService) validateOTPSendRequest(senderID string, channel notifcommon.ChannelType,
	recipient string) *serviceerror.ServiceError {
	if strings.TrimSpace(senderID) == "" {
		return &ErrorInvalidSenderID
	}
	if strings.TrimSpace(recipient) == "" {
		return &ErrorInvalidRecipient
	}
	if !slices.Contains(supportedChannels, channel) {
		return &ErrorUnsupportedChannel
	}
	return nil
}

// handleOTPServiceError handles errors from the OTP service.
func (s *otpAuthnService) handleOTPServiceError(ctx context.Context, svcErr *serviceerror.ServiceError, isVerify bool,
	logger *log.Logger) *serviceerror.ServiceError {
	if svcErr.Type == serviceerror.ClientErrorType {
		if isVerify {
			return serviceerror.CustomServiceError(ErrorClientErrorFromOTPService, core.I18nMessage{
				Key:          "error.otpauthnservice.error_verifying_otp_description",
				DefaultValue: fmt.Sprintf("Error verifying OTP: %s", svcErr.ErrorDescription.DefaultValue),
			})
		} else {
			return serviceerror.CustomServiceError(ErrorClientErrorFromOTPService, core.I18nMessage{
				Key:          "error.otpauthnservice.error_sending_otp_description",
				DefaultValue: fmt.Sprintf("Error sending OTP: %s", svcErr.ErrorDescription.DefaultValue),
			})
		}
	}

	if isVerify {
		logger.Error(ctx, "Error occurred while verifying OTP", log.Any("error", svcErr))
	} else {
		logger.Error(ctx, "Error occurred while sending OTP", log.Any("error", svcErr))
	}
	return &serviceerror.InternalServerError
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

// handleVerifyOTPResponse processes the OTP verification result and resolves the user.
func (s *otpAuthnService) handleVerifyOTPResponse(ctx context.Context, result *notifcommon.VerifyOTPResultDTO,
	logger *log.Logger) (*common.AuthnResult, *serviceerror.ServiceError) {
	if result.Status != notifcommon.OTPVerifyStatusVerified {
		return nil, &ErrorIncorrectOTP
	}

	if result.Recipient == "" {
		logger.Error(ctx, "Recipient not found in OTP verification result")
		return nil, &serviceerror.InternalServerError
	}

	return &common.AuthnResult{
		Token:               map[string]interface{}{userAttributeMobileNumber: result.Recipient},
		AuthenticatedClaims: map[string]interface{}{userAttributeMobileNumber: result.Recipient},
	}, nil
}
