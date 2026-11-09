// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

window.__THUNDERID_RUNTIME_CONFIG__ = {
  brand: {
    product_name: 'ThunderID',
    favicon: {
      light: 'assets/images/favicon.ico',
      dark: 'assets/images/favicon-inverted.ico',
    },
  },
  documentation: {
    baseUrl: 'https://thunderid.dev/docs/v1.0.x',
    releasesUrl: 'https://thunderid.dev/data/releases.json',
    links: {
      users: '',
    },
  },
  client: {
    base: '/console',
    client_id: 'CONSOLE',
    resource_identifier: 'https://localhost:8090/mcp',
    scopes: [
      'openid',
      'profile',
      'email',
      'ou',
      'system',
      'org',
      'org:ou',
      'org:ou:view',
      'org:ou:create',
      'org:ou:update',
      'org:ou:delete',
      'org:user',
      'org:user:view',
      'org:user:create',
      'org:user:update',
      'org:user:delete',
      'org:group',
      'org:group:view',
      'org:group:create',
      'org:group:update',
      'org:group:delete',
      'org:agent',
      'org:agent:view',
      'org:agent:create',
      'org:agent:update',
      'org:agent:delete',
      'org:usertype',
      'org:usertype:view',
      'org:usertype:create',
      'org:usertype:update',
      'org:usertype:delete',
      'org:agenttype',
      'org:agenttype:view',
      'org:agenttype:create',
      'org:agenttype:update',
      'org:agenttype:delete',
    ],
  },
  // Defaults to the origin this app is served from. Add a `server` block with `public_url`
  // (or `hostname`, `port`, `http_only`) to target a different backend.

  // Optional: location of the login gate, used to build the OAuth redirect URI shown when
  // configuring social/OIDC connections. Omit to default to `${served origin}/gate/callback`.
  // gate_client: {
  //   public_url: 'https://gate.example.com',   // or hostname/port/scheme
  // },
};
