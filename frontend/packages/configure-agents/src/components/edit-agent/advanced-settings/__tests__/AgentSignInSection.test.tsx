// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent, OAuthAgentConfig} from '../../../../models/agent';
import AgentSignInSection from '../AgentSignInSection';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: string, options?: Record<string, string>) =>
      (fallback ?? key).replace(/{{(\w+)}}/g, (_match, name: string) => options?.[name] ?? ''),
  }),
}));

describe('AgentSignInSection', () => {
  const mockOnFieldChange = vi.fn();

  const agent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    display: 'Test Agent',
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

    expect(screen.getByText('Agent sign-in')).toBeInTheDocument();
    expect(
      screen.getByText('Allow agents of any agent type to sign in through this agent using the sign-in flow.'),
    ).toBeInTheDocument();
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

    expect(screen.getByLabelText('Enable agent sign-in')).not.toBeChecked();
  });

  it('should render the toggle on when every agent type is allowed', () => {
    render(
      <AgentSignInSection
        agent={{...agent, allowedAgentTypes: ['*']}}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByLabelText('Enable agent sign-in')).toBeChecked();
  });

  // A list saved before the wildcard existed still names the type it allowed, so it shows as on.
  it('renders the toggle on when an agent type is allowed', () => {
    render(
      <AgentSignInSection
        agent={{...agent, allowedAgentTypes: ['default']}}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByLabelText('Enable agent sign-in')).toBeChecked();
    expect(screen.getByText(/only agents of these agent types can sign in now: default\./i)).toBeInTheDocument();
  });

  it('does not list agent types when every agent type is allowed', () => {
    render(
      <AgentSignInSection
        agent={{...agent, allowedAgentTypes: ['*']}}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.queryByText(/only agents of these agent types/i)).not.toBeInTheDocument();
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

    expect(screen.getByLabelText('Enable agent sign-in')).not.toBeChecked();
  });

  it('should allow every agent type when the toggle is turned on', async () => {
    const user = userEvent.setup();
    render(
      <AgentSignInSection
        agent={agent}
        editedAgent={{}}
        oauth2Config={delegationEnabledConfig}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByLabelText('Enable agent sign-in'));

    expect(mockOnFieldChange).toHaveBeenCalledWith('allowedAgentTypes', ['*']);
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

    await user.click(screen.getByLabelText('Enable agent sign-in'));

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

    expect(screen.getByLabelText('Enable agent sign-in')).toBeDisabled();
  });
});
