/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * Console Routes Configuration
 *
 * Centralized route definitions for the Console.
 * All route paths should be defined here to ensure consistency across tests.
 *
 * @example
 * import { routes } from '../../fixtures';
 * await page.goto(`${baseUrl}${routes.home}`);
 *
 * @example
 * import { ConsoleRoutes } from '../configs/routes/console-routes';
 * await page.goto(`${baseUrl}${ConsoleRoutes.applications}`);
 */
export const ConsoleRoutes = {
  /** Sign-in page route */
  signin: "/gate/signin",

  /** Sign-out page route */
  signout: "/gate/signout",

  /** Console home page */
  home: "/console",

  /** Dashboard page */
  dashboard: "/console/dashboard",

  /** Applications list page */
  applications: "/console/applications",

  /** Application type gallery page */
  applicationTypes: "/console/applications/types",

  /** Create new application wizard page */
  applicationCreate: "/console/applications/create",

  /**
   * Application details page
   * @param appId - The application identifier
   */
  applicationDetails: (appId: string) => `/console/applications/${appId}`,

  /** APIs list page */
  apis: "/console/apis",

  /**
   * API details page
   * @param apiId - The API identifier
   */
  apiDetails: (apiId: string) => `/console/apis/${apiId}`,

  /** Users list page */
  users: "/console/users",

  /** Create new user page */
  userCreate: "/console/users/create",

  /**
   * User details page
   * @param userId - The user identifier
   */
  userDetails: (userId: string) => `/console/users/${userId}`,

  /** Settings page */
  settings: "/console/settings",

  /** User profile settings page */
  profile: "/console/settings/profile",
} as const;

export type ConsoleRoute = (typeof ConsoleRoutes)[keyof typeof ConsoleRoutes];

export default ConsoleRoutes;
