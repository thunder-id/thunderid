// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent, OAuthAgentConfig} from '../../../../models/agent';
import AgentSignInSection from '../AgentSignInSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

describe('AgentSignInSection', () => {
  const mockOnFieldChange = vi.fn();

  const agent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    name: 'Test Agent',
  };

  const delegationEnabledConfig: OAuthAgentConfig = {
    grantTypes: ['authorization_code'],
    responseTypes: ['code'],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns null when oauth2Config is undefined', () => {
    const {container} = render(<AgentSignInSection agent={agent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(container.firstChild).toBeNull();
  });

  it('returns null when Delegated mode is off', () => {
    const {container} = render(
      <AgentSignInSection
        agent={agent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it('renders the section title and description when Delegated mode is on', () => {
    render(
      <AgentSignInSection
        agent={agent}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByText('Agent Sign-In')).toBeInTheDocument();
    expect(screen.getByText('Allow agents to sign in through this agent using the sign-in flow.')).toBeInTheDocument();
  });

  it('renders the toggle off when no agent type is allowed', () => {
    render(
      <AgentSignInSection
        agent={agent}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByLabelText('Enable Agent Sign-In')).not.toBeChecked();
  });

  it('renders the toggle on when an agent type is allowed', () => {
    render(
      <AgentSignInSection
        agent={{...agent, allowedAgentTypes: ['default']}}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByLabelText('Enable Agent Sign-In')).toBeChecked();
  });

  it('prioritizes editedAgent.allowedAgentTypes over agent.allowedAgentTypes', () => {
    render(
      <AgentSignInSection
        agent={{...agent, allowedAgentTypes: ['default']}}
        editedAgent={{allowedAgentTypes: []}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByLabelText('Enable Agent Sign-In')).not.toBeChecked();
  });

  it('allows the default agent type when the toggle is turned on', async () => {
    const user = userEvent.setup();
    render(
      <AgentSignInSection
        agent={agent}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByLabelText('Enable Agent Sign-In'));

    expect(mockOnFieldChange).toHaveBeenCalledWith('allowedAgentTypes', ['default']);
  });

  it('clears every allowed agent type when the toggle is turned off', async () => {
    const user = userEvent.setup();
    render(
      <AgentSignInSection
        agent={{...agent, allowedAgentTypes: ['default']}}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByLabelText('Enable Agent Sign-In'));

    expect(mockOnFieldChange).toHaveBeenCalledWith('allowedAgentTypes', []);
  });

  it('disables the toggle for a read-only agent', () => {
    render(
      <AgentSignInSection
        agent={{...agent, isReadOnly: true}}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByLabelText('Enable Agent Sign-In')).toBeDisabled();
  });
});
