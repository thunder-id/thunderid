// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';
import {useMemo} from 'react';

/**
 * Route paths this package needs from the host application, including the agent type, group,
 * role, and onboarding destinations the agent pages link to.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in a unit test), `useAgentRoutes` falls back to
 * `defaultAgentRoutePaths` below.
 *
 * @public
 */
export interface AgentRoutePaths {
  agents: {
    list: () => string;
    detail: (id: string) => string;
    create: () => string;
  };
  agentTypes: {
    detail: (id: string) => string;
  };
  groups: {
    list: () => string;
  };
  roles: {
    list: () => string;
  };
  welcome: {
    getStarted: () => string;
    getStartedAgentsCreate: () => string;
  };
}

/**
 * Default agent paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultAgentRoutePaths: AgentRoutePaths = {
  agents: {
    list: () => '/agents',
    detail: (id) => `/agents/${id}`,
    create: () => '/agents/create',
  },
  agentTypes: {
    detail: (id) => `/agent-types/${id}`,
  },
  groups: {
    list: () => '/groups',
  },
  roles: {
    list: () => '/roles',
  },
  welcome: {
    getStarted: () => '/welcome/get-started',
    getStartedAgentsCreate: () => '/welcome/get-started/agents/create',
  },
};

/**
 * Resolves the agent route paths, preferring the host application's configuration (supplied via
 * `RoutesProvider`) and falling back to this package's own defaults.
 *
 * @public
 */
export default function useAgentRoutes(): AgentRoutePaths {
  const {agents, agentTypes, groups, roles, welcome} = useRoutes<Partial<AgentRoutePaths>>();
  return useMemo(
    () => ({
      agents: agents ?? defaultAgentRoutePaths.agents,
      agentTypes: agentTypes ?? defaultAgentRoutePaths.agentTypes,
      groups: groups ?? defaultAgentRoutePaths.groups,
      roles: roles ?? defaultAgentRoutePaths.roles,
      welcome: welcome ?? defaultAgentRoutePaths.welcome,
    }),
    [agents, agentTypes, groups, roles, welcome],
  );
}
