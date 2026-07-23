/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this package needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in Storybook or a unit test), `useAgentTypeRoutes` falls back to
 * `defaultAgentTypeRoutePaths` below.
 *
 * Includes `agents` alongside `agentTypes`: there is no agent-types listing page, so
 * `ViewAgentTypePage`'s back button returns to the host application's agent listing instead,
 * a destination this package does not own.
 *
 * @public
 */
export interface AgentTypeRoutePaths {
  agentTypes: {
    detail: (id: string) => string;
  };
  agents: {
    list: () => string;
  };
}

/**
 * Default agent type (and agent listing) paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultAgentTypeRoutePaths: AgentTypeRoutePaths = {
  agentTypes: {
    detail: (id) => `/agent-types/${id}`,
  },
  agents: {
    list: () => '/agents',
  },
};

/**
 * Resolves the agent type (and agent listing) route paths, preferring the host application's
 * configuration (supplied via `RoutesProvider`) and falling back to this package's own defaults.
 *
 * Components should never hardcode these destination paths; they should call this hook and
 * build the destination from the returned functions instead.
 *
 * @public
 */
export default function useAgentTypeRoutes(): AgentTypeRoutePaths {
  const routes = useRoutes<Partial<AgentTypeRoutePaths>>();
  return {
    agentTypes: routes.agentTypes ?? defaultAgentTypeRoutePaths.agentTypes,
    agents: routes.agents ?? defaultAgentTypeRoutePaths.agents,
  };
}
