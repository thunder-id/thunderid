// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {ThunderIDProvider} from '@thunderid/react';
import type {ThunderIDProviderProps} from '@thunderid/react';
import {merge} from '@thunderid/utils';
import type {JSX, ComponentType} from 'react';

/**
 * Wraps the app in `ThunderIDProvider`, configured from the runtime config (`config.js`): the client
 * ID the app signs in with, the URL it returns to, and the server it talks to.
 *
 * In the trusted-issuer (federated) model `baseUrl` points at the authorization server, so the
 * self-service endpoints the SDK components call are pointed back at the resource server.
 */
export default function withConfig<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithConfig(props: P): JSX.Element {
    const {
      getTrustedIssuerUrl,
      getTrustedIssuerClientId,
      getClientUrl,
      getTrustedIssuerScopes,
      getServerUrl,
      getResourceIdentifier,
      config,
    } = useConfig();

    // An explicitly configured resource identifier always wins; only without one does the
    // trusted-issuer model fall back to this server's own origin.
    const resourceIdentifier = getResourceIdentifier() ?? (config.trusted_issuer ? getServerUrl() : undefined);
    const resourceServerUrl = config.trusted_issuer ? getServerUrl().replace(/\/+$/, '') : undefined;

    // config.sdk values are deep-merged on top, so a deployment can override any of these.
    const sdkDefaults: Partial<ThunderIDProviderProps> = {
      discovery: {wellKnown: {enabled: true}},
      ...(resourceIdentifier ? {signInOptions: {resource: resourceIdentifier}} : {}),
      ...(resourceServerUrl
        ? {
            endpoints: {
              flowExecute: `${resourceServerUrl}/flow/execute`,
              flowMeta: `${resourceServerUrl}/flow/meta`,
              usersMe: `${resourceServerUrl}/users/me`,
            },
          }
        : {}),
      sendIdTokenInLogoutRequest: false,
      tokenLifecycle: {revokeToken: {revokeOnSignOut: true}},
    };

    const sdkProps = merge({}, sdkDefaults, config.sdk ?? {}) as Partial<ThunderIDProviderProps>;

    return (
      <ThunderIDProvider
        baseUrl={getTrustedIssuerUrl() ?? (import.meta.env.VITE_THUNDER_BASE_URL as string)}
        clientId={getTrustedIssuerClientId() ?? (import.meta.env.VITE_THUNDER_CLIENT_ID as string)}
        afterSignInUrl={getClientUrl() ?? (import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL as string)}
        // Sign-out lands back on the app root, same as sign-in, so the base path (`/myaccount`) stays
        // in the post-logout redirect URI the server matches exactly.
        afterSignOutUrl={getClientUrl() ?? (import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL as string)}
        scopes={getTrustedIssuerScopes().length > 0 ? getTrustedIssuerScopes() : undefined}
        {...sdkProps}
      >
        <WrappedComponent {...props} />
      </ThunderIDProvider>
    );
  };
}
