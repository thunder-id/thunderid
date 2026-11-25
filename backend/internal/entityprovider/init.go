// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entityprovider

import (
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// InitializeEntityProvider initializes the entity provider.
func InitializeEntityProvider(
	entitySvc entity.EntityServiceInterface,
	authzSvc sysauthz.SystemAuthorizationServiceInterface,
) EntityProviderInterface {
	entityProviderConfig := config.GetServerRuntime().Config.EntityProvider
	switch entityProviderConfig.Type {
	case "disabled":
		return initializeDisabledEntityProvider()
	default:
		return initializeDefaultEntityProvider(entitySvc, authzSvc)
	}
}

// initializeDefaultEntityProvider initializes the default entity provider.
func initializeDefaultEntityProvider(
	entitySvc entity.EntityServiceInterface,
	authzSvc sysauthz.SystemAuthorizationServiceInterface,
) EntityProviderInterface {
	return newDefaultEntityProvider(entitySvc, authzSvc)
}

// initializeDisabledEntityProvider initializes the disabled entity provider.
func initializeDisabledEntityProvider() EntityProviderInterface {
	return newDisabledEntityProvider()
}
