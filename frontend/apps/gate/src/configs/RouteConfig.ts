/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
 * Route configuration for the whole Gate app.
 *
 * @public
 */
export interface RouteConfig {
  root: () => string;
  error: () => string;
  signIn: () => string;
  signUp: () => string;
  invite: () => string;
  callback: () => string;
  recovery: () => string;
  signout: () => string;
}

/**
 * Application route paths configuration.
 *
 * @example
 * ```tsx
 * import RouteConfig from './configs/RouteConfig';
 *
 * // Navigate to sign-in page
 * navigate(RouteConfig.signIn());
 * ```
 *
 * @public
 */
const RouteConfig: RouteConfig = {
  root: () => '/',
  error: () => '/error',
  signIn: () => '/signin',
  signUp: () => '/signup',
  invite: () => '/invite',
  callback: () => '/callback',
  recovery: () => '/recovery',
  signout: () => '/signout',
};

export default RouteConfig;
