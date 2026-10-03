// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/idp"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
)

// Initialize initializes the OAuth authentication service.
func Initialize(idpSvc idp.IDPServiceInterface,
	entityProvider entityprovider.EntityProviderInterface) OAuthAuthnServiceInterface {
	// The token and userinfo endpoints come from the connection config like
	// the JWKS resolver's targets, so they get the same SSRF dial guard.
	httpClient := syshttp.NewHTTPClient(syshttp.HTTPClientConfig{
		GuardSSRF: true,
	})
	return newOAuthAuthnService(httpClient, idpSvc, entityProvider)
}
