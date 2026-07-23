/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

window.__THUNDERID_RUNTIME_CONFIG__ = {
  brand: {
    product_name: 'ThunderID',
    favicon: {
      light: 'assets/images/favicon.ico',
      dark: 'assets/images/favicon-inverted.ico',
    },
  },
  // Documentation site used for release info and per-page "Learn more" links. `links` maps a
  // feature/section id to a path resolved against `baseUrl` (or a full URL). Omit a key, or the
  // whole `documentation` block, to hide the corresponding link(s).
  documentation: {
    baseUrl: 'https://thunderid.dev/docs/next',
    releasesUrl: 'https://thunderid.dev/data/releases.json',
    links: {
      users: '/guides/users',
      applications: '/guides/applications',
      agents: '/guides/agents',
      design: '/guides/design',
      flows: '/guides/flows',
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
      'system:user',
      'system:group',
      'system:ou:view',
      'system:usertype:view',
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
