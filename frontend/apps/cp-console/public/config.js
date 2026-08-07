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

// Control Plane console runtime configuration.
//   - plane: 'cp'    : authoring surface. DashboardLayout hides Data Plane runtime entries.
//   - server         : management API base -> Control Plane backend (:8095).
//   - trusted_issuer : OAuth/OIDC auth base -> external IdP, which issues tokens.
window.__THUNDERID_RUNTIME_CONFIG__ = {
  plane: 'cp',
  brand: {
    product_name: 'ThunderID',
    documentation: {
      baseUrl: 'https://thunderid.dev/docs/next',
      releasesUrl: 'https://thunderid.dev/data/releases.json',
    },
    favicon: {
      light: 'assets/images/favicon.ico',
      dark: 'assets/images/favicon-inverted.ico',
    },
  },
  client: {
    base: '/console',
    client_id: 'FlMp9nOYYkqwRierCHBlJfdwsaYa',
    resource_identifier: 'https://localhost:8095',
    // System scopes carry the tenant_instance prefix, matching security.system_permission_prefix in
    // the Control Plane's deployment.yaml. A scope names the instance it grants against, so a bare
    // "system" from the trusted issuer would not satisfy the permissions this plane checks.
    scopes: [
      'openid',
      'profile',
      'email',
      'ou',
      'tenant_instance:system',
      'tenant_instance:system:user',
      'tenant_instance:system:group',
      'tenant_instance:system:ou:view',
      'tenant_instance:system:usertype:view',
    ],
  },
  server: {
    public_url: 'https://localhost:8095',
  },
  // Environment manager service, which holds configuration versions and drives promotion between
  // environments. Omit this block to hide the Promotions feature.
  env_manager: {
    public_url: 'https://localhost:8095',
  },
  trusted_issuer: {
    type: 'generic',
    public_url: 'https://api.asgardeo.io/t/b1zt/oauth2/token',
    client_id: 'FlMp9nOYYkqwRierCHBlJfdwsaYa',
    scopes: ['openid', 'profile', 'email', 'tenant_instance:system'],
  },
};
