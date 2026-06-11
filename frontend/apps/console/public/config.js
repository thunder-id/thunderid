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
    client_id: 'CONSOLE',
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
  server: {
    hostname: 'localhost',
    port: 8090,
    http_only: false,
  },
};
