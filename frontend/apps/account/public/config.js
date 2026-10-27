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
    base: '/account',
    client_id: 'ACCOUNT',
    resource_identifier: 'https://localhost:8090/mcp',
    scopes: ['openid', 'profile', 'email', 'ou'],
  },
  sdk: {
    applicationId: '01900000-0000-7000-8000-0000000000f6',
  },
  // Defaults to the origin this app is served from. Add a `server` block with `public_url`
  // (or `hostname`, `port`, `http_only`) to target a different backend.
};
