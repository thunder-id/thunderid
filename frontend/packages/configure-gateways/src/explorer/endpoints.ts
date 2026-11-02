// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {GraphNode} from './model';

/** One URL a resource is reached at on a gateway, or one the gateway is reached at. */
export interface Endpoint {
  key: string;
  /** The label's i18n key and default. */
  label: [string, string];
  url: string;
}

const join = (baseUrl: string, path: string): string => `${baseUrl.replace(/\/+$/, '')}${path}`;

/** The endpoints a client, application or agent alike, signs in and gets tokens at. */
const CLIENT_PATHS: readonly [key: string, label: string, path: string][] = [
  ['discovery', 'Discovery', '/.well-known/openid-configuration'],
  ['authorize', 'Authorization', '/oauth2/authorize'],
  ['token', 'Token', '/oauth2/token'],
  ['userinfo', 'User info', '/oauth2/userinfo'],
  ['jwks', 'JWKS', '/oauth2/jwks'],
  ['logout', 'Logout', '/oauth2/logout'],
  ['introspect', 'Introspection', '/oauth2/introspect'],
  ['revoke', 'Revocation', '/oauth2/revoke'],
];

/**
 * The URLs a resource is reached at on one gateway, which differ from gateway to gateway since each is
 * reached at its own address: an application's or agent's sign-in and token endpoints, and the redirect
 * URI a connection's provider is told to return to. Other resources have none.
 */
export function endpointsOf(node: GraphNode, baseUrl: string | undefined): Endpoint[] {
  if (!baseUrl) return [];
  if (node.resourceType === 'application' || node.resourceType === 'agent') {
    return CLIENT_PATHS.map(([key, label, path]) => ({
      key,
      label: [`explorer.endpoints.${key}`, label],
      url: join(baseUrl, path),
    }));
  }
  if (node.resourceType === 'connection' && node.federated) {
    // The sign-in pages are served under /gate by the gateway itself, so a provider returns there.
    return [
      {key: 'redirect', label: ['explorer.endpoints.redirect', 'Redirect URI'], url: join(baseUrl, '/gate/callback')},
    ];
  }
  return [];
}

/** The URLs that describe a gateway as a whole: where clients discover it and verify its tokens. */
export function gatewayEndpoints(baseUrl: string | undefined): Endpoint[] {
  if (!baseUrl) return [];
  return [
    {
      key: 'discovery',
      label: ['explorer.endpoints.discovery', 'Discovery'],
      url: join(baseUrl, '/.well-known/openid-configuration'),
    },
    {key: 'jwks', label: ['explorer.endpoints.jwks', 'JWKS'], url: join(baseUrl, '/oauth2/jwks')},
  ];
}
