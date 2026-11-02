// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes, useAdministrationOperation} from '@thunderid/contexts';
import {type JSX} from 'react';
import AgentAddFormPage from './AgentAddFormPage';
import AgentOnboardFlowPage from './AgentOnboardPage';

/**
 * Chooses how an agent is added, by how the `agents` `create` operation runs.
 *
 * Native creation is a plain write, so it gets a form that posts to the agents API. Otherwise the
 * configured agent onboarding flow runs, and its provisioning node creates the agent.
 */
export default function AgentAddRoute(): JSX.Element {
  const mode = useAdministrationOperation('agents', 'create');

  return mode === AdministrationModes.NATIVE ? <AgentAddFormPage /> : <AgentOnboardFlowPage />;
}
