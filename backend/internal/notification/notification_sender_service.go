// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/notification/client"
	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// NotificationSenderServiceInterface defines the interface for sending notifications.
type NotificationSenderServiceInterface interface {
	SendMessage(ctx context.Context, channel common.ChannelType, senderID string,
		data common.MessageData) *tidcommon.ServiceError
	SendEmail(ctx context.Context, senderID string, data common.EmailData) *tidcommon.ServiceError
}

// notificationSenderService implements NotificationSenderServiceInterface.
type notificationSenderService struct {
	senderMgtService NotificationSenderMgtSvcInterface
	clientFactory    client.ClientFactoryInterface
	logger           *log.Logger
}

// newNotificationSenderService returns a new instance of NotificationSenderServiceInterface.
func newNotificationSenderService(
	senderMgtService NotificationSenderMgtSvcInterface,
	clientFactory client.ClientFactoryInterface) NotificationSenderServiceInterface {
	return &notificationSenderService{
		senderMgtService: senderMgtService,
		clientFactory:    clientFactory,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderService")),
	}
}

// SendMessage looks up the sender by ID and dispatches the message via the specified channel.
func (s *notificationSenderService) SendMessage(ctx context.Context, channel common.ChannelType, senderID string,
	data common.MessageData) *tidcommon.ServiceError {
	sender, svcErr := s.senderMgtService.GetSender(ctx, senderID)
	if svcErr != nil {
		return svcErr
	}

	_client, svcErr := s.clientFactory.GetClient(ctx, *sender)
	if svcErr != nil {
		return svcErr
	}

	messageClient, ok := _client.(client.MessageClientInterface)
	if !ok {
		return &ErrorRequestedSenderIsNotOfExpectedType
	}

	if !messageClient.IsChannelSupported(channel) {
		return &ErrorUnsupportedChannel
	}

	if err := messageClient.Send(ctx, channel, data); err != nil {
		s.logger.Error(ctx, "Failed to send message",
			log.String("channel", string(channel)), log.Error(err))
		return &tidcommon.InternalServerError
	}

	return nil
}

// SendEmail looks up the sender by ID and dispatches the email. An email provider is always
// named explicitly: providers are managed through /connections/email-smtp and selected per flow node,
// so there is no deployment-wide default to fall back to.
func (s *notificationSenderService) SendEmail(ctx context.Context, senderID string,
	data common.EmailData) *tidcommon.ServiceError {
	if strings.TrimSpace(senderID) == "" {
		return &ErrorEmailSenderNotSpecified
	}

	sender, svcErr := s.senderMgtService.GetSender(ctx, senderID)
	if svcErr != nil {
		return svcErr
	}

	_client, svcErr := s.clientFactory.GetClient(ctx, *sender)
	if svcErr != nil {
		return svcErr
	}

	emailClient, ok := _client.(client.EmailClientInterface)
	if !ok {
		return &ErrorRequestedSenderIsNotOfExpectedType
	}

	if err := emailClient.Send(ctx, data); err != nil {
		s.logger.Error(ctx, "Failed to send email", log.Error(err))
		return &tidcommon.InternalServerError
	}

	return nil
}
