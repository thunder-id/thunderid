// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package usermgtprovider

import (
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the user management provider.
func Initialize(userSvc user.UserServiceInterface) providers.UserMgtProvider {
	userMgtProviderConfig := config.GetServerRuntime().Config.UserMgtProvider
	switch userMgtProviderConfig.Type {
	case "disabled":
		return initializeDisabledUserMgtProvider()
	default:
		return initializeDefaultUserMgtProvider(userSvc)
	}
}

// initializeDefaultUserMgtProvider initializes the default user management provider.
func initializeDefaultUserMgtProvider(userSvc user.UserServiceInterface) providers.UserMgtProvider {
	return newDefaultUserMgtProvider(userSvc)
}

// initializeDisabledUserMgtProvider initializes the disabled user management provider.
func initializeDisabledUserMgtProvider() providers.UserMgtProvider {
	return NewDisabledUserMgtProvider()
}
