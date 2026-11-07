// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * Authorization detail type route paths this package needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in Storybook or a unit test), `useAuthorizationDetailTypeRoutes`
 * falls back to `defaultAuthorizationDetailTypeRoutePaths` below.
 *
 * @public
 */
export interface AuthorizationDetailTypeRoutePaths {
  authorizationDetailTypes: {
    list: () => string;
    detail: (id: string) => string;
    create: () => string;
  };
}

/**
 * Default authorization detail type paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultAuthorizationDetailTypeRoutePaths: AuthorizationDetailTypeRoutePaths = {
  authorizationDetailTypes: {
    list: () => '/authorization-detail-types',
    detail: (id) => `/authorization-detail-types/${id}`,
    create: () => '/authorization-detail-types/create',
  },
};

/**
 * Resolves the authorization detail type route paths, preferring the host application's
 * configuration (supplied via `RoutesProvider`) and falling back to this package's own defaults.
 *
 * @public
 */
export default function useAuthorizationDetailTypeRoutes(): AuthorizationDetailTypeRoutePaths['authorizationDetailTypes'] {
  const routes = useRoutes<Partial<AuthorizationDetailTypeRoutePaths>>();
  return routes.authorizationDetailTypes ?? defaultAuthorizationDetailTypeRoutePaths.authorizationDetailTypes;
}
