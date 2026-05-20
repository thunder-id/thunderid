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
	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/authnprovider/provider"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/idp"
)

// InitializeAuthnProviderManager initializes and returns an AuthnProviderManagerInterface.
func InitializeAuthnProviderManager(entitySvc entity.EntityServiceInterface,
	passkeySvc passkey.PasskeyServiceInterface, otpSvc otp.OTPAuthnServiceInterface,
	federatedAuths map[idp.IDPType]authncommon.FederatedAuthenticator) AuthnProviderManagerInterface {
	p := provider.InitializeAuthnProvider(entitySvc, passkeySvc, otpSvc, federatedAuths)
	return newAuthnProviderManager(p)
}
