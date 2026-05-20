/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// NotificationSenderServiceInterface defines the interface for sending notification messages.
type NotificationSenderServiceInterface interface {
	Send(ctx context.Context, channel common.ChannelType, senderID string,
		data common.NotificationData) *serviceerror.ServiceError
}

// notificationSenderService implements NotificationSenderServiceInterface.
type notificationSenderService struct {
	senderMgtService NotificationSenderMgtSvcInterface
	clientProvider   notificationClientProviderInterface
	logger           *log.Logger
}

// newNotificationSenderService returns a new instance of NotificationSenderServiceInterface.
func newNotificationSenderService(
	senderMgtService NotificationSenderMgtSvcInterface) NotificationSenderServiceInterface {
	return &notificationSenderService{
		senderMgtService: senderMgtService,
		clientProvider:   newNotificationClientProvider(),
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "NotificationSenderService")),
	}
}

// Send looks up the sender by ID and dispatches the notification via the specified channel.
func (s *notificationSenderService) Send(ctx context.Context, channel common.ChannelType, senderID string,
	data common.NotificationData) *serviceerror.ServiceError {
	sender, svcErr := s.senderMgtService.GetSender(ctx, senderID)
	if svcErr != nil {
		return svcErr
	}

	if sender.Type != common.NotificationSenderTypeMessage {
		return &ErrorRequestedSenderIsNotOfExpectedType
	}

	_client, svcErr := s.clientProvider.GetClient(*sender)
	if svcErr != nil {
		return svcErr
	}

	if !_client.IsChannelSupported(channel) {
		return &ErrorUnsupportedChannel
	}

	if err := _client.Send(channel, data); err != nil {
		s.logger.Error("Failed to send notification", log.String("channel", string(channel)), log.Error(err))
		return &serviceerror.InternalServerError
	}

	return nil
}
