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

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent, AgentInboundAuthConfig, OAuthAgentConfig} from '../../../../models/agent';
import EditAdvancedSettings from '../EditAdvancedSettings';

// Mock the child sections to capture wiring without exercising MUI internals.
vi.mock('../OperationModesSection', () => ({
  default: ({
    oauth2Config,
    onOAuth2ConfigChange,
  }: {
    oauth2Config?: OAuthAgentConfig;
    onOAuth2ConfigChange?: (updates: Partial<OAuthAgentConfig>) => void;
  }) => (
    <div data-testid="operation-modes-section">
      <span data-testid="oauth2-grants">{(oauth2Config?.grantTypes ?? []).join(',')}</span>
      <button type="button" onClick={() => onOAuth2ConfigChange?.({grantTypes: ['client_credentials']})}>
        Trigger OAuth Change
      </button>
    </div>
  ),
}));

vi.mock('../SecuritySection', () => ({
  default: () => <div data-testid="security-section">Security</div>,
}));

vi.mock('../OwnerSection', () => ({
  default: () => <div data-testid="owner-section">Owner</div>,
}));

vi.mock('../AllowedUserTypesSection', () => ({
  default: () => <div data-testid="allowed-user-types-section">Allowed User Types</div>,
}));

vi.mock('../TokenEndpointAuthMethodSection', () => ({
  default: () => <div data-testid="token-endpoint-auth-method-section">Token Endpoint Auth Method</div>,
}));

describe('EditAdvancedSettings', () => {
  const mockAgent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    name: 'Test Agent',
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {grantTypes: ['authorization_code'], responseTypes: ['code']},
      },
    ],
  };

  const mockOnFieldChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders all sections', () => {
    render(
      <EditAdvancedSettings
        agent={mockAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('operation-modes-section')).toBeInTheDocument();
    expect(screen.getByTestId('security-section')).toBeInTheDocument();
    expect(screen.getByTestId('owner-section')).toBeInTheDocument();
    expect(screen.getByTestId('allowed-user-types-section')).toBeInTheDocument();
    expect(screen.getByTestId('token-endpoint-auth-method-section')).toBeInTheDocument();
  });

  it('passes the oauth2Config down to OperationModesSection', () => {
    render(
      <EditAdvancedSettings
        agent={mockAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByTestId('oauth2-grants')).toHaveTextContent('authorization_code');
  });

  it('updates the agent inboundAuthConfig when OAuth2 config changes', async () => {
    const user = userEvent.setup();
    render(
      <EditAdvancedSettings
        agent={mockAgent}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByText('Trigger OAuth Change'));

    expect(mockOnFieldChange).toHaveBeenCalledWith(
      'inboundAuthConfig',
      expect.arrayContaining([
        expect.objectContaining({
          type: 'oauth2',
          config: expect.objectContaining({grantTypes: ['client_credentials']}) as Record<string, unknown>,
        }),
      ]) as AgentInboundAuthConfig[],
    );
  });

  it('uses editedAgent.inboundAuthConfig when present', async () => {
    const user = userEvent.setup();
    const editedInbound: AgentInboundAuthConfig[] = [
      {type: 'oauth2', config: {grantTypes: ['refresh_token'], responseTypes: []}},
    ];
    render(
      <EditAdvancedSettings
        agent={mockAgent}
        editedAgent={{inboundAuthConfig: editedInbound}}
        oauth2Config={{grantTypes: ['refresh_token'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByText('Trigger OAuth Change'));

    expect(mockOnFieldChange).toHaveBeenCalledWith(
      'inboundAuthConfig',
      expect.arrayContaining([
        expect.objectContaining({
          type: 'oauth2',
          config: expect.objectContaining({grantTypes: ['client_credentials']}) as Record<string, unknown>,
        }),
      ]) as AgentInboundAuthConfig[],
    );
  });

  it('handles a missing inboundAuthConfig gracefully', async () => {
    const user = userEvent.setup();
    const agentWithoutAuth: Agent = {...mockAgent, inboundAuthConfig: undefined};
    render(
      <EditAdvancedSettings
        agent={agentWithoutAuth}
        editedAgent={{}}
        oauth2Config={undefined}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByText('Trigger OAuth Change'));

    // No OAuth2 entry to merge into; should still call with an empty array
    expect(mockOnFieldChange).toHaveBeenCalledWith('inboundAuthConfig', []);
  });
});
