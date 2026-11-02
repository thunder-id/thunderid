// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package defaultprovider implements Thunder's built-in default authentication provider.
package defaultprovider

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
)

// Name is the name of the built-in default authn provider. It is the catch-all for any
// credential key not claimed by a named provider in the manager's routing table.
const Name = "default"

// Initialize constructs the default authn provider.
func Initialize(entitySvc entity.EntityServiceInterface,
	passkeySvc passkey.PasskeyServiceInterface, otpSvc otp.OTPAuthnServiceInterface,
	magicLinkSvc magiclink.MagicLinkAuthnServiceInterface,
	openid4vpSvc openid4vp.OpenID4VPServiceInterface,
	federatedAuths map[providers.IDPType]authncommon.FederatedAuthenticator,
	governance identitygovernance.IdentityGovernanceServiceInterface) providers.AuthnProviderInterface {
	return newDefaultAuthnProvider(entitySvc, passkeySvc, otpSvc, magicLinkSvc, openid4vpSvc,
		federatedAuths, governance)
}
