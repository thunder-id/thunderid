// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this feature needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * feature rendered standalone in a unit test), `useApplicationRoutes` falls back to
 * `defaultApplicationRoutePaths` below.
 *
 * @public
 */
export interface ApplicationRoutePaths {
  applications: {
    list: () => string;
    detail: (id: string) => string;
    types: () => string;
    create: () => string;
  };
}

/**
 * Default application paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultApplicationRoutePaths: ApplicationRoutePaths = {
  applications: {
    list: () => '/applications',
    detail: (id) => `/applications/${id}`,
    types: () => '/applications/types',
    create: () => '/applications/create',
  },
};

/**
 * Resolves the application route paths, preferring the host application's configuration (supplied
 * via `RoutesProvider`) and falling back to this feature's own defaults.
 *
 * Components should never hardcode application destination paths; they should call this hook and
 * build the destination from the returned functions instead.
 *
 * @public
 */
export default function useApplicationRoutes(): ApplicationRoutePaths {
  const routes = useRoutes<Partial<ApplicationRoutePaths>>();
  return {applications: routes.applications ?? defaultApplicationRoutePaths.applications};
}
