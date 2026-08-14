// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
      getResourceIdentifier,
      isTrustedIssuerGenericOidc,
      config,
    } = useConfig();

    const genericOidc = isTrustedIssuerGenericOidc();
    // Bind the client's tokens to a resource server via the RFC 8707 `resource` indicator.
    // In the trusted-issuer (federated) model, target the resource server URL; otherwise use the
    // configured resource identifier.
    const resourceIdentifier = config.trusted_issuer ? getServerUrl() : getResourceIdentifier();

    // In the trusted-issuer (federated) model `baseUrl` points at the authorization server (IdP),
    // so the SDK would otherwise send resource-server calls (flow execution, flow metadata, and the
    // user profile) to the IdP instead of the resource server that owns the users and flows. Point
    // those endpoints back at the resource server URL.
    const resourceServerUrl = config.trusted_issuer ? getServerUrl().replace(/\/+$/, '') : undefined;

    // Behavioral defaults derived from app config and runtime heuristics.
    // config.sdk values are deep-merged on top, so operators can override any of these.
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
      // Revoke the access token at the OP's revocation_endpoint before completing sign out, on top
      // of the SDK's default RP-Initiated Logout. Best-effort: revocation failures don't block
      // sign out. Override via config.js: `sdk: {tokenLifecycle: {revokeToken: {revokeOnSignOut: false}}}`.
      tokenLifecycle: {revokeToken: {revokeOnSignOut: true}},
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
        // Sign-out lands back on the console root, same as sign-in. Without this the SDK falls back
        // to the page origin, which drops the client base path (e.g. `/console`) and fails the
        // server's exact match against the application's registered post-logout redirect URIs.
        afterSignOutUrl={getClientUrl() ?? (import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL as string)}
        scopes={getTrustedIssuerScopes().length > 0 ? getTrustedIssuerScopes() : undefined}
        {...sdkProps}
      >
        <WrappedComponent {...props} />
      </ThunderIDProvider>
    );
  };
}
