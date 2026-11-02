// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import AgentGroupsSection from './AgentGroupsSection';
import AgentRolesSection from './AgentRolesSection';
import type {Agent} from '../../../models/agent';

interface EditAccessSettingsProps {
  agent: Agent;
}

export default function EditAccessSettings({agent}: EditAccessSettingsProps): JSX.Element {
  return (
    <Stack spacing={3}>
      <AgentGroupsSection agentId={agent.id} />
      <AgentRolesSection agentId={agent.id} />
    </Stack>
  );
}
