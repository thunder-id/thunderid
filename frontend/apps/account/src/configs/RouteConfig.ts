// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The pages of the Account App, in navigation order. `App.tsx`
 * mounts a route for each and every link is built from `RouteConfig`, so the two cannot drift apart.
 */
export type AccountPage = 'profile' | 'security';

export const ACCOUNT_PAGES: AccountPage[] = ['profile', 'security'];

/**
 * Every place the navigation can point at: the home page, which is always there, and the pages.
 */
export type AccountDestination = 'home' | AccountPage;

const RouteConfig: Record<AccountDestination, () => string> = {
  home: (): string => '/',
  profile: (): string => '/profile',
  security: (): string => '/security',
};

export default RouteConfig;
