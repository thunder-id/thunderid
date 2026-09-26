// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';
import AgentOverview, {type AgentOverviewProps} from '@/features/agents/components/edit-agent/overview/AgentOverview';

/**
 * Renders the agent Overview tab, which tells a developer how to integrate against this agent.
 *
 * The tab prints the OAuth endpoints a client calls at runtime, built from the server this console
 * talks to. Only a Data Plane serves them, so this module is passed to the page by the Data Plane
 * console's route tree and by nothing else.
 */
export default function renderAgentOverview(props: AgentOverviewProps): JSX.Element {
  return <AgentOverview {...props} />;
}
