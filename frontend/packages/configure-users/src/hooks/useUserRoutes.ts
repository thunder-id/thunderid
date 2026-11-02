// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this package needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in Storybook or a unit test), `useUserRoutes` falls back to
 * `defaultUserRoutePaths` below.
 *
 * @public
 */
export interface UserRoutePaths {
  users: {
    list: () => string;
    detail: (userId: string) => string;
    add: () => string;
  };
}

/**
 * Default user paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultUserRoutePaths: UserRoutePaths = {
  users: {
    list: () => '/users',
    detail: (userId) => `/users/${userId}`,
    add: () => '/users/add',
  },
};

/**
 * Resolves the user route paths, preferring the host application's configuration (supplied via
 * `RoutesProvider`) and falling back to this package's own defaults.
 *
 * Components should never hardcode user destination paths; they should call this hook and build
 * the destination from the returned functions instead.
 *
 * @public
 */
export default function useUserRoutes(): UserRoutePaths['users'] {
  const routes = useRoutes<Partial<UserRoutePaths>>();
  return routes.users ?? defaultUserRoutePaths.users;
}
