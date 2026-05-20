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
 * Interface representing the application route structure.
 */
export interface Routes {
  /**
   * Root path of the application.
   */
  ROOT: string;
  /**
   * Authentication-related routes.
   */
  AUTH: {
    /**
     * Error page route.
     */
    ERROR: string;
    /**
     * Sign-in page route.
     */
    SIGN_IN: string;
    /**
     * Sign-up page route.
     */
    SIGN_UP: string;
    /**
     * Invite acceptance page route.
     */
    INVITE: string;
    /**
     * OAuth callback page route.
     */
    CALLBACK: string;
    /**
     * Recovery page route.
     */
    RECOVERY: string;
  };
}

/**
 * Application route paths configuration.
 *
 * @constant
 * @type {Routes}
 *
 * @example
 * ```tsx
 * import ROUTES from './constants/routes';
 *
 * // Navigate to sign-in page
 * navigate(ROUTES.AUTH.SIGN_IN);
 * ```
 */
const ROUTES: Routes = {
  ROOT: '/',
  AUTH: {
    ERROR: '/error',
    SIGN_IN: '/signin',
    SIGN_UP: '/signup',
    INVITE: '/invite',
    CALLBACK: '/callback',
    RECOVERY: '/recovery',
  },
} as const;

export default ROUTES;
