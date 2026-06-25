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
	"errors"
	"time"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/openid4vp"
	systemhttp "github.com/thunder-id/thunderid/internal/system/http"
)

// AuthnProviderDependencies bundles upstream services available to every authn
// provider constructor. Add fields here when a new provider needs an upstream
// service that isn't already exposed.
type AuthnProviderDependencies struct {
	EntitySvc        entity.EntityServiceInterface
	PasskeyService   passkey.PasskeyServiceInterface
	OTPService       otp.OTPAuthnServiceInterface
	MagicLinkService magiclink.MagicLinkAuthnServiceInterface
	OpenID4VPService openid4vp.OpenID4VPServiceInterface
	FederatedAuths   map[idp.IDPType]authncommon.FederatedAuthenticator
}

// builtInAuthnProviderRegistrar constructs one authn provider. The properties
// argument is the per-provider config bag from YAML (nil when no entry is
// present); deps carries shared upstream services.
//
// A registrar may return (nil, nil) to indicate that the provider is not
// enabled in this deployment (e.g. the REST provider when no base_url is
// configured). Init treats such entries as silently skipped.
type builtInAuthnProviderRegistrar func(
	properties map[string]interface{},
	deps AuthnProviderDependencies,
) (AuthnProviderInterface, error)

// propsInt extracts an integer value from a property bag, tolerating both YAML
// (int) and JSON (float64) decoding. Returns 0 for missing or unrecognized types.
func propsInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// newBuiltInAuthnProviderRegistrars returns the catalog of built-in authn
// providers. The map key is both the catalog entry and the provider name used
// in AuthUser.state and in the credential_mapping config. To add a new
// provider, implement AuthnProviderInterface in this package and add one entry
// to this map.
func newBuiltInAuthnProviderRegistrars() map[string]builtInAuthnProviderRegistrar {
	return map[string]builtInAuthnProviderRegistrar{
		"default": func(_ map[string]interface{}, deps AuthnProviderDependencies) (AuthnProviderInterface, error) {
			return newDefaultAuthnProvider(deps.EntitySvc, deps.PasskeyService, deps.OTPService,
				deps.MagicLinkService, deps.OpenID4VPService, deps.FederatedAuths), nil
		},
		"rest": func(props map[string]interface{}, _ AuthnProviderDependencies) (AuthnProviderInterface, error) {
			// REST is opt-in: requires an explicit `enabled: true` flag, mirroring
			// other Thunder config blocks (e.g. consent). Absent/false leaves the
			// rest of the props inert so operators can stage config ahead of rollout.
			if enabled, _ := props["enabled"].(bool); !enabled {
				return nil, nil
			}
			baseURL, _ := props["base_url"].(string)
			if baseURL == "" {
				return nil, errors.New("base_url is required when enabled")
			}
			apiKey := ""
			if security, ok := props["security"].(map[string]interface{}); ok {
				apiKey, _ = security["api_key"].(string)
			}
			timeout := 10 * time.Second
			// YAML decodes integers as int; JSON decodes all numbers as float64; accept both.
			if t := propsInt(props["timeout"]); t > 0 {
				timeout = time.Duration(t) * time.Second
			}
			httpClient := systemhttp.NewHTTPClientWithTimeout(timeout)
			return newRestAuthnProvider(baseURL, apiKey, httpClient), nil
		},
	}
}
