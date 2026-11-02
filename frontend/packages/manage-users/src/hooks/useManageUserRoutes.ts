// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * The paths the users pages link to. A host app overrides them through `RoutesProvider`.
 */
export interface ManageUserRoutePaths {
  users: {
    create: () => string;
    details: (userId: string) => string;
    list: () => string;
  };
}

export const defaultManageUserRoutePaths: ManageUserRoutePaths = {
  users: {
    create: () => '/users/new',
    details: (userId: string) => `/users/${encodeURIComponent(userId)}`,
    list: () => '/users',
  },
};

export default function useManageUserRoutes(): ManageUserRoutePaths['users'] {
  const routes = useRoutes<Partial<ManageUserRoutePaths>>();

  return routes.users ?? defaultManageUserRoutePaths.users;
}
