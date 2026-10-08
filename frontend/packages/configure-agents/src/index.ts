// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// API Hooks
export {default as useDeleteAgent} from './api/useDeleteAgent';
export {default as useGetAgent} from './api/useGetAgent';
export {default as useGetAgentGroups} from './api/useGetAgentGroups';
export type {UseGetAgentGroupsParams} from './api/useGetAgentGroups';
export {default as useGetAgentRoles} from './api/useGetAgentRoles';
export type {UseGetAgentRolesParams} from './api/useGetAgentRoles';
export {default as useGetAgents} from './api/useGetAgents';
export type {UseGetAgentsParams} from './api/useGetAgents';
export {default as useRegenerateAgentSecret} from './api/useRegenerateAgentSecret';
export type {RegenerateAgentSecretResult, RegenerateAgentSecretVariables} from './api/useRegenerateAgentSecret';
export {default as useUpdateAgent} from './api/useUpdateAgent';

// Models & Types
export {DEFAULT_AGENT_TYPE_HANDLE} from './models/agent';
export type {
  Agent,
  AgentGroup,
  AgentGroupListResponse,
  AgentInboundAuthConfig,
  AgentListResponse,
  AgentLoginConsentConfig,
  AgentRoleListResponse,
  BasicAgent,
  CreateAgentRequest,
  OAuthAgentConfig,
  UpdateAgentRequest,
} from './models/agent';

// Constants
export {default as AgentConstants} from './constants/agent-constants';
export {default as AgentQueryKeys} from './constants/agent-query-keys';

// Pages
export {default as AgentEditPage} from './pages/AgentEditPage';
export {default as AgentOnboardPage} from './pages/AgentOnboardPage';
export {default as AgentsListPage} from './pages/AgentsListPage';

// Routes
export {default as useAgentRoutes, defaultAgentRoutePaths} from './hooks/useAgentRoutes';
export type {AgentRoutePaths} from './hooks/useAgentRoutes';
