// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extensionauth

import (
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the extension authentication service.
func Initialize(idpService providers.IDPProvider,
	oauthSvc authnoauth.OAuthAuthnCoreServiceInterface) ExtensionAuthnServiceInterface {
	return newExtensionAuthnService(idpService, oauthSvc)
}
