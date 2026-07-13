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
import EditFlowsSettings from '../EditFlowsSettings';

vi.mock('../../../../../applications/components/edit-application/flows-settings/AuthenticationFlowSection', () => ({
  default: ({application}: {application: Application}) => (
    <div data-testid="auth-flow" data-readonly={String(application.isReadOnly)} />
  ),
}));
vi.mock('../../../../../applications/components/edit-application/flows-settings/RegistrationFlowSection', () => ({
  default: ({application}: {application: Application}) => (
    <div data-testid="registration-flow" data-readonly={String(application.isReadOnly)} />
  ),
}));
vi.mock('../../../../../applications/components/edit-application/flows-settings/RecoveryFlowSection', () => ({
  default: ({application}: {application: Application}) => (
    <div data-testid="recovery-flow" data-readonly={String(application.isReadOnly)} />
  ),
}));

describe('EditFlowsSettings', () => {
  const mockOnFieldChange = vi.fn();
  const baseAgent: Agent = {id: 'agent-1', ouId: 'ou-1', type: 'default', name: 'Test Agent'};

  it('renders all three flow sections', () => {
    render(
      <EditFlowsSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('auth-flow')).toBeInTheDocument();
    expect(screen.getByTestId('registration-flow')).toBeInTheDocument();
    expect(screen.getByTestId('recovery-flow')).toBeInTheDocument();
  });

  it('keeps flows editable and hides the lock notice when Delegated mode is on', () => {
    render(
      <EditFlowsSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('auth-flow')).toHaveAttribute('data-readonly', 'false');
    expect(screen.queryByText(/These settings are frozen for this agent/)).not.toBeInTheDocument();
  });

  it('forces the flow sections read-only and shows the lock notice when Delegated mode is off', () => {
    render(
      <EditFlowsSettings
        agent={baseAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('auth-flow')).toHaveAttribute('data-readonly', 'true');
    expect(screen.getByTestId('registration-flow')).toHaveAttribute('data-readonly', 'true');
    expect(screen.getByTestId('recovery-flow')).toHaveAttribute('data-readonly', 'true');
    expect(screen.getByText(/These settings are frozen for this agent/)).toBeInTheDocument();
  });

  it('stays read-only when the agent is already read-only, even with Delegated mode on', () => {
    render(
      <EditFlowsSettings
        agent={{...baseAgent, isReadOnly: true}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('auth-flow')).toHaveAttribute('data-readonly', 'true');
  });

  describe('Delegated mode toggle', () => {
    it('shows the toggle checked when Delegated mode is on', () => {
      render(
        <EditFlowsSettings
          agent={baseAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('switch', {name: 'Delegated mode'})).toBeChecked();
    });

    it('shows the toggle unchecked when Delegated mode is off', () => {
      render(
        <EditFlowsSettings
          agent={baseAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('switch', {name: 'Delegated mode'})).not.toBeChecked();
    });

    it('turns on Delegated mode by adding authorization_code and requiring PKCE', async () => {
      const user = userEvent.setup();
      render(
        <EditFlowsSettings
          agent={{
            ...baseAgent,
            inboundAuthConfig: [{type: 'oauth2', config: {grantTypes: ['client_credentials'], responseTypes: []}}],
          }}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(screen.getByRole('switch', {name: 'Delegated mode'}));

      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({
              grantTypes: expect.arrayContaining(['client_credentials', 'authorization_code']) as string[],
              pkceRequired: true,
            }) as Record<string, unknown>,
          }),
        ]),
      );
    });

    it('turns off Delegated mode by dropping the delegated-only grants', async () => {
      const user = userEvent.setup();
      render(
        <EditFlowsSettings
          agent={{
            ...baseAgent,
            inboundAuthConfig: [
              {
                type: 'oauth2',
                config: {grantTypes: ['client_credentials', 'authorization_code'], responseTypes: ['code']},
              },
            ],
          }}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['client_credentials', 'authorization_code'], responseTypes: ['code']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(screen.getByRole('switch', {name: 'Delegated mode'}));

      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({grantTypes: ['client_credentials']}) as Record<string, unknown>,
          }),
        ]),
      );
    });

    it('disables the toggle for read-only agents', () => {
      render(
        <EditFlowsSettings
          agent={{...baseAgent, isReadOnly: true}}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('switch', {name: 'Delegated mode'})).toBeDisabled();
    });
  });
});
