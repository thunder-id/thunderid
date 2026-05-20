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

package provider

import (
	"time"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/config"
	systemhttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// InitializeAuthnProvider initializes the authentication provider.
func InitializeAuthnProvider(
	entitySvc entity.EntityServiceInterface,
	passkeySvc passkey.PasskeyServiceInterface,
	otpSvc otp.OTPAuthnServiceInterface,
	federatedAuths map[idp.IDPType]authncommon.FederatedAuthenticator,
) AuthnProviderInterface {
	authnProviderConfig := config.GetServerRuntime().Config.AuthnProvider
	switch authnProviderConfig.Type {
	case "rest":
		return initializeRestAuthnProvider()
	default:
		return initializeDefaultAuthnProvider(entitySvc, passkeySvc, otpSvc, federatedAuths)
	}
}

// initializeDefaultAuthnProvider initializes the default authentication provider.
func initializeDefaultAuthnProvider(
	entitySvc entity.EntityServiceInterface,
	passkeySvc passkey.PasskeyServiceInterface,
	otpSvc otp.OTPAuthnServiceInterface,
	federatedAuths map[idp.IDPType]authncommon.FederatedAuthenticator,
) AuthnProviderInterface {
	return newDefaultAuthnProvider(entitySvc, passkeySvc, otpSvc, federatedAuths)
}

// initializeRestAuthnProvider initializes the REST authentication provider.
func initializeRestAuthnProvider() AuthnProviderInterface {
	authnProviderConfig := config.GetServerRuntime().Config.AuthnProvider
	baseURL := authnProviderConfig.Rest.BaseURL
	apiKey := authnProviderConfig.Rest.Security.APIKey
	timeout := time.Duration(authnProviderConfig.Rest.Timeout) * time.Second
	if baseURL == "" {
		log.GetLogger().Fatal("AuthnProvider Rest BaseURL is required but found empty")
	}
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	httpClient := systemhttp.NewHTTPClientWithTimeout(timeout)
	return newRestAuthnProvider(baseURL, apiKey, httpClient)
}
