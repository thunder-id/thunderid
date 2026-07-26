/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/notification"
	notifcommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	loggerComponentName       = "OTPAuthnService"
	userAttributeMobileNumber = "mobile_number"
)

// OTPAuthnServiceInterface defines the interface for OTP authentication operations.
// Authenticate returns an error only for actual failures; a missing local user is NOT an error.
type OTPAuthnServiceInterface interface {
	GenerateOTP(ctx context.Context, recipient, recipientAttr, previousSessionToken string) (
		sessionToken string, otpValue string, expirySeconds int64, svcErr *tidcommon.ServiceError)
	Authenticate(ctx context.Context, sessionToken, otp string) (*common.AuthnResult, *tidcommon.ServiceError)
}

// otpAuthnService is the default implementation of OTPAuthnServiceInterface.
type otpAuthnService struct {
	notifOTPService notification.OTPServiceInterface
}

// newOTPAuthnService creates a new instance of OTPAuthnService.
func newOTPAuthnService(notifOTPSvc notification.OTPServiceInterface) OTPAuthnServiceInterface {
	return &otpAuthnService{
		notifOTPService: notifOTPSvc,
	}
}

// GenerateOTP validates the recipient and delegates OTP generation to the notification service.
func (s *otpAuthnService) GenerateOTP(ctx context.Context,
	recipient, recipientAttr, previousSessionToken string) (string, string, int64, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Generating OTP", log.MaskedString("recipient", recipient))

	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return "", "", 0, &ErrorInvalidRecipient
	}
	if recipientAttr == "" {
		recipientAttr = authnprovidercm.UserAttributeUserID
	}

	sessionToken, otpValue, expirySeconds, svcErr := s.notifOTPService.GenerateOTP(
		ctx, recipient, recipientAttr, previousSessionToken)
	if svcErr != nil {
		if svcErr.Code == notification.ErrorMaxOTPAttemptsExceeded.Code {
			return "", "", 0, &ErrorMaxOTPAttemptsExceeded
		}
		if svcErr.Type == tidcommon.ClientErrorType {
			return "", "", 0, &ErrorClientErrorFromOTPService
		}
		return "", "", 0, &tidcommon.InternalServerError
	}
	return sessionToken, otpValue, expirySeconds, nil
}

// Authenticate verifies the provided OTP against the session token and returns the authenticated user.
func (s *otpAuthnService) Authenticate(ctx context.Context, sessionToken,
	otpCode string) (*common.AuthnResult, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Verifying OTP for authentication")

	if strings.TrimSpace(sessionToken) == "" {
		return nil, &ErrorInvalidSessionToken
	}
	if strings.TrimSpace(otpCode) == "" {
		return nil, &ErrorInvalidOTP
	}

	result, svcErr := s.notifOTPService.VerifyOTP(ctx, notifcommon.VerifyOTPDTO{
		SessionToken: sessionToken,
		OTPCode:      otpCode,
	})
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &tidcommon.InternalServerError
		}
		switch svcErr.Code {
		case notification.ErrorInvalidSessionToken.Code:
			return nil, &ErrorInvalidSessionToken
		case notification.ErrorInvalidOTP.Code:
			return nil, &ErrorInvalidOTP
		default:
			return nil, &ErrorClientErrorFromOTPService
		}
	}

	if result.Status != notifcommon.OTPVerifyStatusVerified {
		return nil, &ErrorIncorrectOTP
	}
	if result.Recipient == "" {
		logger.Error(ctx, "Recipient not found in OTP verification result")
		return nil, &tidcommon.InternalServerError
	}

	recipientAttr := result.RecipientAttr
	if recipientAttr == "" {
		recipientAttr = userAttributeMobileNumber
	}

	return &common.AuthnResult{
		Token:               map[string]interface{}{recipientAttr: result.Recipient},
		AuthenticatedClaims: map[string]interface{}{recipientAttr: result.Recipient},
	}, nil
}
