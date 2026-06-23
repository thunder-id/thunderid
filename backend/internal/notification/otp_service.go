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
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/notification/client"
	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/template"
)

// OTPServiceInterface defines the interface for OTP operations.
type OTPServiceInterface interface {
	SendOTP(ctx context.Context, request common.SendOTPDTO) (*common.SendOTPResultDTO, *serviceerror.ServiceError)
	VerifyOTP(ctx context.Context, request common.VerifyOTPDTO) (
		*common.VerifyOTPResultDTO, *serviceerror.ServiceError)
}

// otpService implements the OTPServiceInterface.
type otpService struct {
	otpAuthnSvc      otp.OTPAuthnServiceInterface
	senderMgtService NotificationSenderMgtSvcInterface
	clientFactory    client.ClientFactoryInterface
	templateService  template.TemplateServiceInterface
}

// newOTPService returns a new instance of OTPServiceInterface.
func newOTPService(otpAuthnSvc otp.OTPAuthnServiceInterface,
	notifSenderMgtSvc NotificationSenderMgtSvcInterface,
	templateSvc template.TemplateServiceInterface,
	clientFactory client.ClientFactoryInterface) OTPServiceInterface {
	return &otpService{
		otpAuthnSvc:      otpAuthnSvc,
		senderMgtService: notifSenderMgtSvc,
		clientFactory:    clientFactory,
		templateService:  templateSvc,
	}
}

// SendOTP sends an OTP to the specified recipient using the provided sender.
func (s *otpService) SendOTP(
	ctx context.Context, otpDTO common.SendOTPDTO) (*common.SendOTPResultDTO, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OTPService"))
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
		return nil, &serviceerror.InternalServerError
	}
	if sender == nil {
		return nil, &ErrorSenderNotFound
	}

	sessionToken, otpValue, _, otpErr := s.otpAuthnSvc.GenerateOTP(ctx, otpDTO.Recipient, "mobileNumber")
	if otpErr != nil {
		logger.Error(ctx, "Failed to generate OTP", log.String("error", otpErr.Code))
		return nil, &serviceerror.InternalServerError
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
	ctx context.Context, otpDTO common.VerifyOTPDTO) (*common.VerifyOTPResultDTO, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OTPService"))
	logger.Debug(ctx, "Verifying OTP")

	if err := s.validateOTPVerifyRequest(otpDTO); err != nil {
		return nil, err
	}

	_, svcErr := s.otpAuthnSvc.Authenticate(ctx, otpDTO.SessionToken, otpDTO.OTPCode)
	if svcErr != nil {
		if svcErr.Code == otp.ErrorIncorrectOTP.Code || svcErr.Code == otp.ErrorInvalidSessionToken.Code {
			return &common.VerifyOTPResultDTO{Status: common.OTPVerifyStatusInvalid}, nil
		}
		return nil, &serviceerror.InternalServerError
	}

	return &common.VerifyOTPResultDTO{Status: common.OTPVerifyStatusVerified}, nil
}

// validateOTPSendRequest validates the OTP send request.
func (s *otpService) validateOTPSendRequest(request common.SendOTPDTO) *serviceerror.ServiceError {
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
func (s *otpService) validateOTPVerifyRequest(request common.VerifyOTPDTO) *serviceerror.ServiceError {
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
	sender common.NotificationSenderDTO, logger *log.Logger) *serviceerror.ServiceError {
	otpCfg := config.GetServerRuntime().Config.Notification.OTP
	expiryMinutes := strconv.FormatInt(int64(otpCfg.ValidityPeriodSeconds)/60, 10)
	templateData := template.TemplateData{"otp": otpVal, "expiryMinutes": expiryMinutes}
	rendered, svcErr := s.templateService.Render(ctx, template.ScenarioOTP, template.TemplateTypeSMS, templateData)
	if svcErr != nil {
		logger.Error(ctx, "Failed to render SMS OTP template", log.String("error", svcErr.Code))
		return &serviceerror.InternalServerError
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
		return &serviceerror.InternalServerError
	}

	return nil
}
