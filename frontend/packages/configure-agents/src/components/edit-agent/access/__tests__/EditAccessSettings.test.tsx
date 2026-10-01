// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {Agent} from '../../../../models/agent';
import EditAccessSettings from '../EditAccessSettings';

vi.mock('../AgentGroupsSection', () => ({default: () => <div data-testid="agent-groups" />}));
vi.mock('../AgentRolesSection', () => ({default: () => <div data-testid="agent-roles" />}));

describe('EditAccessSettings', () => {
  const mockAgent: Agent = {id: 'agent-1', ouId: 'ou-1', type: 'default', name: 'Test Agent'};

  it('renders groups and roles directly, with no sub-tab experience', () => {
    render(<EditAccessSettings agent={mockAgent} />);

    expect(screen.getByTestId('agent-groups')).toBeInTheDocument();
    expect(screen.getByTestId('agent-roles')).toBeInTheDocument();
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
  });
});
