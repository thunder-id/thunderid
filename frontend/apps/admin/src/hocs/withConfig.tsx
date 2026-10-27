// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {ThunderIDProvider} from '@thunderid/react';
import type {ThunderIDProviderProps} from '@thunderid/react';
import {merge} from '@thunderid/utils';
import type {JSX, ComponentType} from 'react';

/**
 * Wraps the app in `ThunderIDProvider`, configured from the runtime config.
 *
 * In the trusted-issuer (federated) model `baseUrl` points at the authorization server, so every
 * management collection the SDK components call is pointed back at the resource server.
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

    const resourceIdentifier = config.trusted_issuer ? getServerUrl() : getResourceIdentifier();
    const resourceServerUrl = config.trusted_issuer ? getServerUrl().replace(/\/+$/, '') : undefined;

    const sdkDefaults: Partial<ThunderIDProviderProps> = {
      discovery: {wellKnown: {enabled: true}},
      ...(resourceIdentifier ? {signInOptions: {resource: resourceIdentifier}} : {}),
      ...(resourceServerUrl
        ? {
            endpoints: {
              agentTypes: `${resourceServerUrl}/agent-types`,
              agents: `${resourceServerUrl}/agents`,
              flowExecute: `${resourceServerUrl}/flow/execute`,
              flowMeta: `${resourceServerUrl}/flow/meta`,
              groups: `${resourceServerUrl}/groups`,
              roles: `${resourceServerUrl}/roles`,
              userTypes: `${resourceServerUrl}/user-types`,
              users: `${resourceServerUrl}/users`,
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
        afterSignOutUrl={getClientUrl() ?? (import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL as string)}
        scopes={getTrustedIssuerScopes().length > 0 ? getTrustedIssuerScopes() : undefined}
        {...sdkProps}
      >
        <WrappedComponent {...props} />
      </ThunderIDProvider>
    );
  };
}
