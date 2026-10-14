// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this package needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in a unit test), `useSessionRoutes` falls back to
 * `defaultSessionRoutePaths` below.
 *
 * @public
 */
export interface SessionRoutePaths {
  users: {
    detail: (userId: string) => string;
  };
}

/**
 * Default session paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultSessionRoutePaths: SessionRoutePaths = {
  users: {
    detail: (userId) => `/users/${userId}`,
  },
};

/**
 * Resolves the session route paths, preferring the host application's configuration (supplied via
 * `RoutesProvider`) and falling back to this package's own defaults.
 *
 * @public
 */
export default function useSessionRoutes(): SessionRoutePaths {
  const routes = useRoutes<Partial<SessionRoutePaths>>();
  return {
    users: routes.users ?? defaultSessionRoutePaths.users,
  };
}
