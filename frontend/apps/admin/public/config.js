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
  client: {
    base: '/admin',
    client_id: 'ADMIN_CONSOLE',
    resource_identifier: 'https://localhost:8090/mcp',
    // Tokens carry only the requested scopes the signed-in user holds. Root admins get `system`;
    // admins of an organization unit get the narrower permissions their roles grant.
    scopes: [
      'openid',
      'profile',
      'email',
      'ou',
      'system',
      'system:ou:view',
      'system:user',
      'system:user:view',
      'system:group',
      'system:group:view',
      'system:role',
      'system:role:view',
      'system:agent',
      'system:agent:view',
      'system:usertype:view',
      'system:agenttype:view',
    ],
  },
  sdk: {
    applicationId: '01900000-0000-7000-8000-0000000000f1',
  },
  // Defaults to the origin this app is served from. Add a `server` block with `public_url`
  // (or `hostname`, `port`, `http_only`) to target a different backend.
};
