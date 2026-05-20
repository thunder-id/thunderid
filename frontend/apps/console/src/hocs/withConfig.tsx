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

import {ThunderIDProvider} from '@thunderid/react';
import {useConfig} from '@thunderid/contexts';
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

    const signInOptions: Record<string, string> | undefined = config.trusted_issuer
      ? {resource: getServerUrl()}
      : undefined;

    // When the trusted issuer is a generic OIDC provider, suppress the SDK's
    // product specific bootstrap calls that would otherwise 404 / be CORS-blocked
    // at the external authorization server: flow metadata (`{baseUrl}/flow/meta`)
    // and branding preferences. The user profile is already derived from ID token
    // claims under the ThunderID platform, so no additional profile configuration
    // is required.
    const genericOidc = isTrustedIssuerGenericOidc();
    const preferences = genericOidc
      ? {
          resolveFromMeta: false,
          theme: {
            inheritFromBranding: false,
          },
        }
      : undefined;

    // Generic OIDC authorization servers typically respond to the `/oauth/token`
    // endpoint with `access-control-allow-credentials:
    // false`, so a credentialed fetch from the browser is blocked by CORS and the
    // SDK never sees the issued token. The ThunderID SDK defaults
    // `sendCookiesInRequests` to `true`, which makes it issue the token request
    // with `credentials: 'include'`. Forcing it to `false` switches the request
    // to `credentials: 'same-origin'`, which is uncredentialed for cross-origin
    // requests and therefore CORS-safe against any compliant OIDC provider.
    //
    // The field is declared on the SDK's legacy auth client config and read at
    // runtime (see @thunderid/browser DefaultConfig and requestAccessToken),
    // but it is not surfaced on the v2 `ThunderIDProvider` prop type. The
    // provider spreads unknown props through to the underlying client via
    // `...rest`, so passing it here is the SDK-supported escape hatch.
    const genericOidcExtraProps: {sendCookiesInRequests?: boolean} = genericOidc ? {sendCookiesInRequests: false} : {};

    return (
      <ThunderIDProvider
        baseUrl={getTrustedIssuerUrl() ?? (import.meta.env.VITE_THUNDER_BASE_URL as string)}
        clientId={getTrustedIssuerClientId() ?? (import.meta.env.VITE_THUNDER_CLIENT_ID as string)}
        afterSignInUrl={getClientUrl() ?? (import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL as string)}
        scopes={getTrustedIssuerScopes().length > 0 ? getTrustedIssuerScopes() : undefined}
        signInOptions={signInOptions}
        discovery={{
          wellKnown: {
            enabled: true,
          },
        }}
        preferences={preferences}
        {...genericOidcExtraProps}
      >
        <WrappedComponent {...props} />
      </ThunderIDProvider>
    );
  };
}
