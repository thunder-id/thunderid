// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Route configuration for the whole Gate app.
 *
 * @public
 */
export interface RouteConfig {
  root: () => string;
  error: () => string;
  signIn: () => string;
  device: () => string;
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
  device: () => '/device',
  signUp: () => '/signup',
  invite: () => '/invite',
  callback: () => '/callback',
  recovery: () => '/recovery',
  signout: () => '/signout',
};

export default RouteConfig;
