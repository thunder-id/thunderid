// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * The paths the agents pages link to. A host app overrides them through `RoutesProvider`.
 */
export interface ManageAgentRoutePaths {
  agents: {
    create: () => string;
    details: (agentId: string) => string;
    list: () => string;
  };
}

export const defaultManageAgentRoutePaths: ManageAgentRoutePaths = {
  agents: {
    create: () => '/agents/new',
    details: (agentId: string) => `/agents/${encodeURIComponent(agentId)}`,
    list: () => '/agents',
  },
};

export default function useManageAgentRoutes(): ManageAgentRoutePaths['agents'] {
  const routes = useRoutes<Partial<ManageAgentRoutePaths>>();

  return routes.agents ?? defaultManageAgentRoutePaths.agents;
}
