// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package oauth

import (
	"time"

	"github.com/thunder-id/thunderid/internal/idp"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
)

// Initialize initializes the OAuth authentication service.
func Initialize(idpSvc idp.IDPServiceInterface,
	clients syshttp.OutboundClientProviderInterface) OAuthAuthnServiceInterface {
	// The requests carry credentials (the client secret and code, or the access token), so a redirect is
	// not followed: it could resend them to another host.
	httpClient := clients.NewHTTPClientWithoutRedirects(30 * time.Second)
	return newOAuthAuthnService(httpClient, idpSvc)
}
