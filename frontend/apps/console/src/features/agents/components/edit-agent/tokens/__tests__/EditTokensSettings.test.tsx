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

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState} from 'react';
import {describe, it, expect, vi} from 'vitest';
import type {Application} from '../../../../../applications/models/application';
import type {Agent} from '../../../../models/agent';
import EditTokensSettings from '../EditTokensSettings';

vi.mock('../../../../../applications/components/edit-application/token-settings/EditTokenSettings', () => ({
  default: ({application, sectionResetKey}: {application: Application; sectionResetKey?: number}) => (
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

describe('EditTokensSettings', () => {
  const mockOnFieldChange = vi.fn();
  const baseAgent: Agent = {id: 'agent-1', ouId: 'ou-1', type: 'default', name: 'Test Agent'};

  it('shows both "Agent" and "User" secondary tabs, defaulting to Agent', () => {
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['Agent', 'User']);
    expect(screen.getByTestId('agent-access-token')).toBeInTheDocument();
    expect(screen.queryByTestId('token-settings')).not.toBeInTheDocument();
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
    expect(screen.queryByText(/These settings are frozen for this agent/)).not.toBeInTheDocument();
  });

  it('forces token settings read-only and shows the lock notice when Delegated mode is off', async () => {
    const user = userEvent.setup();
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'User'}));

    expect(screen.getByTestId('token-settings')).toHaveAttribute('data-readonly', 'true');
    expect(screen.getByText(/These settings are frozen for this agent/)).toBeInTheDocument();
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

    it('forwards sectionResetKey to the User tab EditTokenSettings for in-place reset', () => {
      const {rerender} = render(
        <EditTokensSettings
          agent={baseAgent}
          editedAgent={{}}
          oauth2Config={oauth2Config}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

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
