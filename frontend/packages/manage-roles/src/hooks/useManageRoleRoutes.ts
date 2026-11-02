// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * The paths the roles pages link to. A host app overrides them through `RoutesProvider`.
 */
export interface ManageRoleRoutePaths {
  roles: {
    create: () => string;
    details: (roleId: string) => string;
    list: () => string;
  };
}

export const defaultManageRoleRoutePaths: ManageRoleRoutePaths = {
  roles: {
    create: () => '/roles/new',
    details: (roleId: string) => `/roles/${encodeURIComponent(roleId)}`,
    list: () => '/roles',
  },
};

export default function useManageRoleRoutes(): ManageRoleRoutePaths['roles'] {
  const routes = useRoutes<Partial<ManageRoleRoutePaths>>();

  return routes.roles ?? defaultManageRoleRoutePaths.roles;
}
