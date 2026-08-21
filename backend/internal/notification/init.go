// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"

	"github.com/thunder-id/thunderid/internal/notification/client"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize creates and configures the notification service components. Declarative resource
// loading and sender-CRUD HTTP routing now happen in the connection package
// (/connections/{vendor}), which is the sole owner of the "connection" declarative resource
// type. This package no longer registers any HTTP routes; its services are consumed internally
// by authn, flow executors, and the connection/importer packages.
func Initialize(cacheManager cache.CacheManagerInterface, jwtService jwt.JWTServiceInterface) (
	NotificationSenderMgtSvcInterface, OTPServiceInterface, NotificationSenderServiceInterface, error) {
	var notificationStore notificationStoreInterface
	var tx providers.Transactioner

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
	otpService := newOTPService(cacheManager, jwtService)
	notificationSenderService := newNotificationSenderService(mgtService, clientFactory)

	return mgtService, otpService, notificationSenderService, nil
}
