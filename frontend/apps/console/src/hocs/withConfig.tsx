/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useConfig} from '@thunderid/contexts';
import {ThunderIDProvider} from '@thunderid/react';
import type {ThunderIDProviderProps} from '@thunderid/react';
import {merge} from '@thunderid/utils';
import type {JSX, ComponentType} from 'react';

export default function withConfig<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithConfig(props: P): JSX.Element {
    const {
      getTrustedIssuerUrl,
      getTrustedIssuerClientId,
      getClientUrl,
      getTrustedIssuerScopes,
      getServerUrl,
      isTrustedIssuerGenericOidc,
      config,
    } = useConfig();

    const genericOidc = isTrustedIssuerGenericOidc();

    // Behavioral defaults derived from app config and runtime heuristics.
    // config.sdk values are deep-merged on top, so operators can override any of these.
    const sdkDefaults: Partial<ThunderIDProviderProps> = {
      discovery: {wellKnown: {enabled: true}},
      ...(config.trusted_issuer ? {signInOptions: {resource: getServerUrl()}} : {}),
      // When the trusted issuer is a generic OIDC provider, suppress the SDK's
      // product-specific bootstrap calls that would otherwise 404 / be CORS-blocked
      // at the external authorization server: flow metadata (`{baseUrl}/flow/meta`).
      // The user profile is already derived from ID token
      // claims under the ThunderID platform, so no additional profile configuration
      // is required.
      ...(genericOidc
        ? {
            preferences: {resolveFromMeta: false},
            // Generic OIDC authorization servers typically respond to the `/oauth/token`
            // endpoint with `access-control-allow-credentials: false`, so a credentialed
            // fetch from the browser is blocked by CORS and the SDK never sees the issued
            // token. The ThunderID SDK defaults `sendCookiesInRequests` to `true`, which
            // makes it issue the token request with `credentials: 'include'`. Forcing it
            // to `false` switches the request to `credentials: 'same-origin'`, which is
            // uncredentialed for cross-origin requests and therefore CORS-safe against any
            // compliant OIDC provider.
            //
            // The field is not surfaced in the v2 `ThunderIDProvider` prop destructuring
            // but is read from `...rest` at runtime (see @thunderid/browser DefaultConfig
            // and requestAccessToken), so it is part of the supported config shape.
            sendCookiesInRequests: false,
          }
        : {}),
    };

    const sdkProps = merge({}, sdkDefaults, config.sdk ?? {}) as Partial<ThunderIDProviderProps>;

    return (
      <ThunderIDProvider
        baseUrl={getTrustedIssuerUrl() ?? (import.meta.env.VITE_THUNDER_BASE_URL as string)}
        clientId={getTrustedIssuerClientId() ?? (import.meta.env.VITE_THUNDER_CLIENT_ID as string)}
        afterSignInUrl={getClientUrl() ?? (import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL as string)}
        scopes={getTrustedIssuerScopes().length > 0 ? getTrustedIssuerScopes() : undefined}
        {...sdkProps}
      >
        <WrappedComponent {...props} />
      </ThunderIDProvider>
    );
  };
}
