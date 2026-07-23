/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package defaultprovider implements Thunder's built-in default authentication provider.
package defaultprovider

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/entity"
)

// Name is the name of the built-in default authn provider. It is the catch-all for any
// credential key not claimed by a named provider in the manager's routing table.
const Name = "default"

// Initialize constructs the default authn provider.
func Initialize(entitySvc entity.EntityServiceInterface,
	passkeySvc passkey.PasskeyServiceInterface, otpSvc otp.OTPAuthnServiceInterface,
	magicLinkSvc magiclink.MagicLinkAuthnServiceInterface,
	openid4vpSvc openid4vp.OpenID4VPServiceInterface,
	federatedAuths map[providers.IDPType]authncommon.FederatedAuthenticator) provider.AuthnProviderInterface {
	return newDefaultAuthnProvider(entitySvc, passkeySvc, otpSvc, magicLinkSvc, openid4vpSvc, federatedAuths)
}
