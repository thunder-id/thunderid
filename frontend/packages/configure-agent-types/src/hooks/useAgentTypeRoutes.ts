// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this package needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in Storybook or a unit test), `useAgentTypeRoutes` falls back to
 * `defaultAgentTypeRoutePaths` below.
 *
 * @public
 */
export interface AgentTypeRoutePaths {
  agentTypes: {
    list: () => string;
    detail: (id: string) => string;
    create: () => string;
  };
}

/**
 * Default agent type paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultAgentTypeRoutePaths: AgentTypeRoutePaths = {
  agentTypes: {
    list: () => '/agent-types',
    detail: (id) => `/agent-types/${id}`,
    create: () => '/agent-types/create',
  },
};

/**
 * Resolves the agent type route paths, preferring the host application's configuration
 * (supplied via `RoutesProvider`) and falling back to this package's own defaults.
 *
 * Components should never hardcode agent type destination paths; they should call this hook
 * and build the destination from the returned functions instead.
 *
 * @public
 */
export default function useAgentTypeRoutes(): AgentTypeRoutePaths['agentTypes'] {
  const routes = useRoutes<Partial<AgentTypeRoutePaths>>();
  return routes.agentTypes ?? defaultAgentTypeRoutePaths.agentTypes;
}
