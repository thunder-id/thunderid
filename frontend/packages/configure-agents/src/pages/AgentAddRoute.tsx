// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes, useAdministrationOperation, useProjectScope} from '@thunderid/contexts';
import {type JSX} from 'react';
import {Navigate} from 'react-router';
import AgentAddFormPage from './AgentAddFormPage';
import AgentOnboardFlowPage from './AgentOnboardPage';
import useAgentRoutes from '../hooks/useAgentRoutes';

/**
 * Props for the {@link AgentAddRoute} component.
 */
export interface AgentAddRouteProps {
  /**
   * Whether an agent is created in a project only. When set, the page returns to the list outside a selected
   * project, since an agent created there would be refused.
   */
  createInProjectsOnly?: boolean;
}

/**
 * Chooses how an agent is added, by how the `agents` `create` operation runs.
 *
 * Native creation is a plain write, so it gets a form that posts to the agents API. Otherwise the
 * configured agent onboarding flow runs, and its provisioning node creates the agent.
 */
export default function AgentAddRoute({createInProjectsOnly = false}: AgentAddRouteProps): JSX.Element {
  const mode = useAdministrationOperation('agents', 'create');
  const {projectId} = useProjectScope();
  const routes = useAgentRoutes();

  if (createInProjectsOnly && !projectId) {
    return <Navigate to={routes.agents.list()} replace />;
  }

  return mode === AdministrationModes.NATIVE ? <AgentAddFormPage /> : <AgentOnboardFlowPage />;
}
