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

/**
 * Query key constants for connections feature cache management.
 */
const ConnectionQueryKeys = {
  /**
   * Base key for all identity-provider domain queries (consumed by useIdentityProviders)
   */
  INTEGRATIONS: 'integrations',

  /**
   * Key for identity providers queries
   */
  IDENTITY_PROVIDERS: 'identity-providers',

  /**
   * Key for the connection type summaries list (GET /connections)
   */
  CONNECTION_TYPES: 'connection-types',

  /**
   * Key for configured instances of a connection type (GET /connections/{type})
   */
  CONNECTION_INSTANCES: 'connection-instances',

  /**
   * Key for a single connection instance (GET /connections/{type}/{id})
   */
  CONNECTION: 'connection',
} as const;

export default ConnectionQueryKeys;
