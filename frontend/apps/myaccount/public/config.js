// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Read at runtime, so a deployment can rewrite this file after packaging (apps/myaccount/config.js)
// to point the app at another application or backend.
window.__THUNDERID_RUNTIME_CONFIG__ = {
  brand: {
    product_name: 'ThunderID',
    favicon: {
      light: 'assets/images/favicon.ico',
      dark: 'assets/images/favicon-inverted.ico',
    },
    // Optional: the logo the top bar shows, ahead of the one the Console's MyAccount builder sets
    // and the logo of the application the app signs in with. An http(s) URL, or a path under
    // /myaccount such as 'assets/images/brand.svg'.
    // logo: 'https://example.com/logo.svg',
  },
  client: {
    base: '/myaccount',
    // The OAuth2 client the app signs in with. Its application needs `${origin}/myaccount` as a
    // redirect URI and a post-logout redirect URI.
    client_id: 'MYACCOUNT',
    resource_identifier: 'https://localhost:8090/mcp',
    scopes: ['openid', 'profile', 'email', 'ou'],
  },
  sdk: {
    // The application whose Design and logo the app takes. Set it to the application of client_id.
    applicationId: '01900000-0000-7000-8000-0000000000f6',
  },
  // Defaults to the origin this app is served from. Add a `server` block with `public_url`
  // (or `hostname`, `port`, `http_only`) to target a different backend.
};
