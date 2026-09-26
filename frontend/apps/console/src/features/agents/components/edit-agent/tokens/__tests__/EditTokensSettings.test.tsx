// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {Application} from '@thunderid/configure-applications';
import {useState} from 'react';
import {describe, it, expect, vi} from 'vitest';
import type {Agent} from '../../../../models/agent';
import EditTokensSettings from '../EditTokensSettings';

vi.mock('@thunderid/configure-applications', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-applications')>()),
  EditTokenSettings: ({
    application,
    sectionResetKey = undefined,
  }: {
    application: Application;
    sectionResetKey?: number;
  }) => (
    <div
      data-testid="token-settings"
      data-readonly={String(application.isReadOnly)}
      data-section-reset-key={String(sectionResetKey)}
    />
  ),
}));

vi.mock('../AgentAccessTokenSection', () => ({
  // Carries local click state so a changed key (remount) is observable as the counter resetting.
  default: function MockAgentAccessTokenSection({agent}: {agent: Agent}) {
    const [clicks, setClicks] = useState(0);
    return (
      <div data-testid="agent-access-token" data-readonly={String(agent.isReadOnly)}>
        Clicks: {clicks}
        <button type="button" data-testid="agent-access-token-bump" onClick={() => setClicks((c) => c + 1)}>
          Bump
        </button>
      </div>
    );
  },
}));

const delegationLockMessage = /does not receive tokens on behalf of a user/i;

describe('EditTokensSettings', () => {
  const mockOnFieldChange = vi.fn();
  const baseAgent: Agent = {id: 'agent-1', ouId: 'ou-1', type: 'default', name: 'Test Agent'};

  it('shows both "Agent" and "User" audiences, defaulting to Agent', () => {
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getAllByRole('tab').map((tab) => tab.getAttribute('aria-label'))).toEqual(['Agent', 'User']);
    expect(screen.getByRole('tab', {name: 'Agent'})).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByTestId('agent-access-token')).toBeInTheDocument();
    expect(screen.queryByTestId('token-settings')).not.toBeInTheDocument();
  });

  it('describes each audience next to its label', () => {
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByText('Tokens for the agent acting on its own')).toBeInTheDocument();
    expect(screen.getByText('Tokens for the agent acting on behalf of a user')).toBeInTheDocument();
  });

  it('switches to the User tab content when clicked', async () => {
    const user = userEvent.setup();
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'User'}));

    expect(screen.getByTestId('token-settings')).toBeInTheDocument();
    expect(screen.queryByTestId('agent-access-token')).not.toBeInTheDocument();
  });

  it('keeps token settings editable and hides the lock notice when Delegated mode is on', async () => {
    const user = userEvent.setup();
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'User'}));

    expect(screen.getByTestId('token-settings')).toHaveAttribute('data-readonly', 'false');
    expect(screen.queryByText(delegationLockMessage)).not.toBeInTheDocument();
  });

  it('omits the User audience when Delegated mode is off', () => {
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    // Only the agent's own tokens apply, so there is no picker and no locked audience to explain.
    expect(screen.queryByRole('tablist', {name: 'Issued to'})).not.toBeInTheDocument();
    expect(screen.queryByText(delegationLockMessage)).not.toBeInTheDocument();
    expect(screen.queryByTestId('token-settings')).not.toBeInTheDocument();
  });

  it('stays read-only when the agent is already read-only, even with Delegated mode on', async () => {
    const user = userEvent.setup();
    render(
      <EditTokensSettings
        agent={{...baseAgent, isReadOnly: true}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'User'}));

    expect(screen.getByTestId('token-settings')).toHaveAttribute('data-readonly', 'true');
  });

  it('keeps the Agent tab fully editable regardless of Delegated mode', () => {
    render(
      <EditTokensSettings
        agent={{...baseAgent, isReadOnly: false}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('agent-access-token')).toHaveAttribute('data-readonly', 'false');
    expect(screen.queryByText(/These settings are frozen for this agent/)).not.toBeInTheDocument();
  });

  describe('section reset', () => {
    const oauth2Config = {grantTypes: ['authorization_code'], responseTypes: ['code']};

    it('forwards sectionResetKey to the User tab EditTokenSettings for in-place reset', async () => {
      const user = userEvent.setup();
      const {rerender} = render(
        <EditTokensSettings
          agent={baseAgent}
          editedAgent={{}}
          oauth2Config={oauth2Config}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      await user.click(screen.getByRole('tab', {name: 'User'}));

      expect(screen.getByTestId('token-settings')).toHaveAttribute('data-section-reset-key', '0');

      rerender(
        <EditTokensSettings
          agent={baseAgent}
          editedAgent={{}}
          oauth2Config={oauth2Config}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={1}
        />,
      );

      // Same element (no remount), new prop — proves in-place reset, preserving its own sub-tabs.
      expect(screen.getByTestId('token-settings')).toHaveAttribute('data-section-reset-key', '1');
    });

    it('remounts the Agent tab section, dropping its local state, when sectionResetKey changes', async () => {
      const user = userEvent.setup();
      const {rerender} = render(
        <EditTokensSettings
          agent={baseAgent}
          editedAgent={{}}
          oauth2Config={oauth2Config}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      await user.click(screen.getByRole('tab', {name: 'Agent'}));
      await user.click(screen.getByTestId('agent-access-token-bump'));
      expect(screen.getByTestId('agent-access-token')).toHaveTextContent('Clicks: 1');

      rerender(
        <EditTokensSettings
          agent={baseAgent}
          editedAgent={{}}
          oauth2Config={oauth2Config}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={1}
        />,
      );

      expect(screen.getByTestId('agent-access-token')).toHaveTextContent('Clicks: 0');
    });
  });
});
