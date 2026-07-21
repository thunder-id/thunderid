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
import {describe, it, expect, vi} from 'vitest';
import type {Application} from '../../../../../applications/models/application';
import type {Agent} from '../../../../models/agent';
import EditTokensSettings from '../EditTokensSettings';

vi.mock('../../../../../applications/components/edit-application/token-settings/EditTokenSettings', () => ({
  default: ({application}: {application: Application}) => (
    <div data-testid="token-settings" data-readonly={String(application.isReadOnly)} />
  ),
}));

vi.mock('../AgentAccessTokenSection', () => ({
  default: ({agent}: {agent: Agent}) => (
    <div data-testid="agent-access-token" data-readonly={String(agent.isReadOnly)} />
  ),
}));

describe('EditTokensSettings', () => {
  const mockOnFieldChange = vi.fn();
  const baseAgent: Agent = {id: 'agent-1', ouId: 'ou-1', type: 'default', name: 'Test Agent'};

  it('shows both "User" and "Agent" secondary tabs, defaulting to User', () => {
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['User', 'Agent']);
    expect(screen.getByTestId('token-settings')).toBeInTheDocument();
    expect(screen.queryByTestId('agent-access-token')).not.toBeInTheDocument();
  });

  it('switches to the Agent tab content when clicked', async () => {
    const user = userEvent.setup();
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'Agent'}));

    expect(screen.getByTestId('agent-access-token')).toBeInTheDocument();
    expect(screen.queryByTestId('token-settings')).not.toBeInTheDocument();
  });

  it('keeps token settings editable and hides the lock notice when Delegated mode is on', () => {
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('token-settings')).toHaveAttribute('data-readonly', 'false');
    expect(screen.queryByText(/These settings are frozen for this agent/)).not.toBeInTheDocument();
  });

  it('forces token settings read-only and shows the lock notice when Delegated mode is off', () => {
    render(
      <EditTokensSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('token-settings')).toHaveAttribute('data-readonly', 'true');
    expect(screen.getByText(/These settings are frozen for this agent/)).toBeInTheDocument();
  });

  it('stays read-only when the agent is already read-only, even with Delegated mode on', () => {
    render(
      <EditTokensSettings
        agent={{...baseAgent, isReadOnly: true}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('token-settings')).toHaveAttribute('data-readonly', 'true');
  });

  it('keeps the Agent tab fully editable regardless of Delegated mode', async () => {
    const user = userEvent.setup();
    render(
      <EditTokensSettings
        agent={{...baseAgent, isReadOnly: false}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByRole('tab', {name: 'Agent'}));

    expect(screen.getByTestId('agent-access-token')).toHaveAttribute('data-readonly', 'false');
    expect(screen.queryByText(/These settings are frozen for this agent/)).not.toBeInTheDocument();
  });
});
