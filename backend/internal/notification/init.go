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

	"github.com/thunder-id/thunderid/internal/notification/client"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize creates and configures the notification service components. Declarative resource
// loading and sender-CRUD HTTP routing now happen in the connection package
// (/connections/{vendor}), which is the sole owner of the "connection" declarative resource
// type. This package no longer registers any HTTP routes; its services are consumed internally
// by authn, flow executors, and the connection/importer packages.
func Initialize(jwtService jwt.JWTServiceInterface) (
	NotificationSenderMgtSvcInterface, OTPServiceInterface, NotificationSenderServiceInterface, error) {
	var notificationStore notificationStoreInterface
	var tx transaction.Transactioner

	if config.GetServerRuntime().Config.DeclarativeResources.Enabled {
		notificationStore, tx = newNotificationFileBasedStore()
	} else {
		var err error
		notificationStore, tx, err = newNotificationStore()
		if err != nil {
			// Service initialization runs during application startup, outside any request.
			log.GetLogger().Error(context.Background(),
				"Failed to initialize notification store", log.Error(err))
			return nil, nil, nil, err
		}
	}

	mgtService := newNotificationSenderMgtService(notificationStore, tx)

	clientFactory := client.Initialize()
	otpService := newOTPService(jwtService)
	notificationSenderService := newNotificationSenderService(mgtService, clientFactory)

	return mgtService, otpService, notificationSenderService, nil
}
