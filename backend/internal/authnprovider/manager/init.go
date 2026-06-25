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

package manager

import (
	"context"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/openid4vp"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// InitializeAuthnProviderManager initializes and returns an AuthnProviderManagerInterface.
// Bad configuration (missing provider, mapping references an unregistered provider,
// REST provider missing base_url, etc.) is fatal at startup, mirroring the pattern
// used by other init functions in this package.
func InitializeAuthnProviderManager(entitySvc entity.EntityServiceInterface,
	passkeySvc passkey.PasskeyServiceInterface, otpSvc otp.OTPAuthnServiceInterface,
	magicLinkSvc magiclink.MagicLinkAuthnServiceInterface,
	openid4vpSvc openid4vp.OpenID4VPServiceInterface,
	federatedAuths map[idp.IDPType]authncommon.FederatedAuthenticator,
) AuthnProviderManagerInterface {
	deps := provider.AuthnProviderDependencies{
		EntitySvc:        entitySvc,
		PasskeyService:   passkeySvc,
		OTPService:       otpSvc,
		MagicLinkService: magicLinkSvc,
		OpenID4VPService: openid4vpSvc,
		FederatedAuths:   federatedAuths,
	}
	providers, err := provider.InitializeAuthnProviders(deps)
	if err != nil {
		// Provider initialization runs during application startup, outside any request.
		log.GetLogger().Fatal(context.Background(), "Failed to initialize authn providers", log.Error(err))
	}
	credMap := config.GetServerRuntime().Config.AuthnProvider.CredentialMapping
	mgr, err := newAuthnProviderManager(providers, credMap)
	if err != nil {
		log.GetLogger().Fatal(context.Background(), "Failed to initialize authn provider manager", log.Error(err))
	}
	return mgr
}
