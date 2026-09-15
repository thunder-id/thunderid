// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

  /** User types list page */
  userTypes: "/console/user-types",

  /** Create new user type wizard page */
  userTypeCreate: "/console/user-types/create",

  /** Settings page */
  settings: "/console/settings",

  /** User profile settings page */
  profile: "/console/settings/profile",

  /** Welcome screen (landing page shown on first login) */
  welcome: "/console/welcome",

  /** Wayfinder "Secured Web Application" tryout page (embeds the Wayfinder sample setup) */
  welcomeTryoutApp: "/console/welcome/tryout/securing-application",

  /** Agents list page */
  agents: "/console/agents",

  /**
   * Agent details page
   * @param agentId - The agent identifier
   */
  agentDetails: (agentId: string) => `/console/agents/${agentId}`,
  /** Connections list page */
  connections: "/console/connections",

  /**
   * Configure wizard for a branded (singleton) connection vendor, e.g. "google"
   * @param type - The connection vendor type
   */
  connectionConfigure: (type: string) => `/console/connections/${type}/configure`,

  /**
   * Connection details page
   * @param type - The connection vendor type
   * @param id - The connection identifier
   */
  connectionDetails: (type: string, id: string) => `/console/connections/${type}/${id}`,
} as const;

export type ConsoleRoute = (typeof ConsoleRoutes)[keyof typeof ConsoleRoutes];

export default ConsoleRoutes;
