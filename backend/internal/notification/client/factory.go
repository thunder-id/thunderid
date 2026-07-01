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

package client

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/notification/common"
)

// ClientFactoryInterface defines the interface for obtaining notification clients.
type ClientFactoryInterface interface {
	GetClient(ctx context.Context,
		sender common.NotificationSenderDTO) (NotificationClientInterface, *tidcommon.ServiceError)
}

// clientFactory is the implementation of ClientFactoryInterface.
type clientFactory struct{}

// newClientFactory returns a new instance of ClientFactoryInterface.
func newClientFactory() ClientFactoryInterface {
	return &clientFactory{}
}

// GetClient returns the notification client for the given sender.
func (p *clientFactory) GetClient(ctx context.Context, sender common.NotificationSenderDTO) (
	NotificationClientInterface, *tidcommon.ServiceError) {
	var _client NotificationClientInterface
	var err error
	switch sender.Provider {
	case common.MessageProviderTypeVonage:
		_client, err = newVonageClient(ctx, sender)
	case common.MessageProviderTypeTwilio:
		_client, err = newTwilioClient(ctx, sender)
	case common.MessageProviderTypeCustom:
		_client, err = newCustomClient(ctx, sender)
	default:
		return nil, &ErrorInvalidProvider
	}

	if err != nil {
		return nil, &tidcommon.InternalServerError
	}

	return _client, nil
}
