// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * The paths the groups pages link to. A host app overrides them through `RoutesProvider`.
 */
export interface ManageGroupRoutePaths {
  groups: {
    create: () => string;
    details: (groupId: string) => string;
    list: () => string;
  };
}

export const defaultManageGroupRoutePaths: ManageGroupRoutePaths = {
  groups: {
    create: () => '/groups/new',
    details: (groupId: string) => `/groups/${encodeURIComponent(groupId)}`,
    list: () => '/groups',
  },
};

export default function useManageGroupRoutes(): ManageGroupRoutePaths['groups'] {
  const routes = useRoutes<Partial<ManageGroupRoutePaths>>();

  return routes.groups ?? defaultManageGroupRoutePaths.groups;
}
