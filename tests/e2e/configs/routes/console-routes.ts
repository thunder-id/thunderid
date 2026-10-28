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
  home: "/admin",

  /** Dashboard page */
  dashboard: "/admin/dashboard",

  /** Applications list page */
  applications: "/admin/applications",

  /** Application type gallery page */
  applicationTypes: "/admin/applications/types",

  /** Create new application wizard page */
  applicationCreate: "/admin/applications/create",

  /**
   * Application details page
   * @param appId - The application identifier
   */
  applicationDetails: (appId: string) => `/admin/applications/${appId}`,

  /** APIs list page */
  apis: "/admin/apis",

  /**
   * API details page
   * @param apiId - The API identifier
   */
  apiDetails: (apiId: string) => `/admin/apis/${apiId}`,

  /** Users list page */
  users: "/admin/users",

  /** Create new user page */
  userCreate: "/admin/users/create",

  /**
   * User details page
   * @param userId - The user identifier
   */
  userDetails: (userId: string) => `/admin/users/${userId}`,

  /** User types list page */
  userTypes: "/admin/user-types",

  /** Create new user type wizard page */
  userTypeCreate: "/admin/user-types/create",

  /** Settings page */
  settings: "/admin/settings",

  /** User profile settings page */
  profile: "/admin/settings/profile",

  /** Welcome screen (landing page shown on first login) */
  welcome: "/admin/welcome",

  /** Wayfinder "Secured Web Application" tryout page (embeds the Wayfinder sample setup) */
  welcomeTryoutApp: "/admin/welcome/tryout/securing-application",

  /** Agents list page */
  agents: "/admin/agents",

  /** Agent onboarding page, which runs the configured agent onboarding flow */
  agentCreate: "/admin/agents/create",

  /**
   * Agent details page
   * @param agentId - The agent identifier
   */
  agentDetails: (agentId: string) => `/admin/agents/${agentId}`,
  /** Connections list page */
  connections: "/admin/connections",

  /**
   * Configure wizard for a branded (singleton) connection vendor, e.g. "google"
   * @param type - The connection vendor type
   */
  connectionConfigure: (type: string) => `/admin/connections/${type}/configure`,

  /**
   * Connection details page
   * @param type - The connection vendor type
   * @param id - The connection identifier
   */
  connectionDetails: (type: string, id: string) => `/admin/connections/${type}/${id}`,
} as const;

export type ConsoleRoute = (typeof ConsoleRoutes)[keyof typeof ConsoleRoutes];

export default ConsoleRoutes;
