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
	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notification/message"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// notificationClientProviderInterface defines the interface for obtaining notification clients.
type notificationClientProviderInterface interface {
	GetClient(sender common.NotificationSenderDTO) (message.NotificationClientInterface, *serviceerror.ServiceError)
}

// notificationClientProvider is the implementation of notificationClientProviderInterface.
type notificationClientProvider struct{}

// newNotificationClientProvider returns a new instance of notificationClientProviderInterface.
func newNotificationClientProvider() notificationClientProviderInterface {
	return &notificationClientProvider{}
}

// GetClient returns the notification client for the given sender.
func (p *notificationClientProvider) GetClient(sender common.NotificationSenderDTO) (
	message.NotificationClientInterface, *serviceerror.ServiceError) {
	var _client message.NotificationClientInterface
	var err error
	switch sender.Provider {
	case common.MessageProviderTypeVonage:
		_client, err = message.NewVonageClient(sender)
	case common.MessageProviderTypeTwilio:
		_client, err = message.NewTwilioClient(sender)
	case common.MessageProviderTypeCustom:
		_client, err = message.NewCustomClient(sender)
	default:
		return nil, &ErrorInvalidProvider
	}

	if err != nil {
		return nil, &serviceerror.InternalServerError
	}

	return _client, nil
}
