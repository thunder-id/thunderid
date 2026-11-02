// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package consent

import (
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the consent enforcer service and other related components.
func Initialize(jwtSvc jwt.JWTServiceInterface,
	detailTypes providers.AuthorizationDetailTypeProvider) ConsentEnforcerService {
	return newConsentEnforcerService(jwtSvc, detailTypes)
}
