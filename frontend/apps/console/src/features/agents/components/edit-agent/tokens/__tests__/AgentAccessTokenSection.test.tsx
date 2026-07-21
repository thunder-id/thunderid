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

/* eslint-disable @typescript-eslint/no-unsafe-return */
import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent, AgentInboundAuthConfig} from '../../../../models/agent';
import AgentAccessTokenSection from '../AgentAccessTokenSection';

const {mockUseGetAgentTypes, mockUseGetAgentType} = vi.hoisted(() => ({
  mockUseGetAgentTypes: vi.fn(),
  mockUseGetAgentType: vi.fn(),
}));

vi.mock('@thunderid/configure-agent-types', () => ({
  useGetAgentTypes: () => mockUseGetAgentTypes(),
  useGetAgentType: (id?: string) => mockUseGetAgentType(id),
}));

vi.mock('../../../../../applications/components/edit-application/token-settings/JwtPreview', () => ({
  default: ({payload}: {payload: Record<string, unknown>}) => (
    <pre data-testid="jwt-preview">{JSON.stringify(payload)}</pre>
  ),
}));

describe('AgentAccessTokenSection', () => {
  const mockOnFieldChange = vi.fn();
  const baseAgent: Agent = {id: 'agent-1', ouId: 'ou-1', type: 'default', name: 'Test Agent'};
  const baseInboundAuthConfig: AgentInboundAuthConfig[] = [
    {type: 'oauth2', config: {grantTypes: ['client_credentials'], responseTypes: []}},
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetAgentTypes.mockReturnValue({
      data: {types: [{id: 'schema-1', name: 'default', ouId: 'ou-1'}]},
    });
    mockUseGetAgentType.mockReturnValue({
      data: {
        id: 'schema-1',
        name: 'default',
        ouId: 'ou-1',
        schema: {
          department: {type: 'string'},
          apiKey: {type: 'string', credential: true},
        },
      },
      isLoading: false,
    });
  });

  it('lists the agent schema attributes as selectable chips, excluding credential fields', () => {
    render(
      <AgentAccessTokenSection
        agent={{...baseAgent, inboundAuthConfig: baseInboundAuthConfig}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByText('department')).toBeInTheDocument();
    expect(screen.queryByText('apiKey')).not.toBeInTheDocument();
  });

  it('does not show a scopes section', () => {
    render(
      <AgentAccessTokenSection
        agent={{...baseAgent, inboundAuthConfig: baseInboundAuthConfig}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.queryByText(/scope/i)).not.toBeInTheDocument();
  });

  it('adds an attribute to clientConfig when its chip is clicked', async () => {
    const user = userEvent.setup();
    render(
      <AgentAccessTokenSection
        agent={{...baseAgent, inboundAuthConfig: baseInboundAuthConfig}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByText('department'));

    expect(mockOnFieldChange).toHaveBeenCalledWith(
      'inboundAuthConfig',
      expect.arrayContaining([
        expect.objectContaining({
          type: 'oauth2',
          config: expect.objectContaining({
            token: expect.objectContaining({
              accessToken: expect.objectContaining({
                clientConfig: expect.objectContaining({attributes: ['department']}) as Record<string, unknown>,
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }) as Record<string, unknown>,
        }),
      ]),
    );
  });

  it('shows the current validity period, defaulting to 3600', () => {
    render(
      <AgentAccessTokenSection
        agent={{...baseAgent, inboundAuthConfig: baseInboundAuthConfig}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(document.getElementById('agent-access-token-validity')!).toHaveValue(3600);
  });

  it('commits a valid validity period change', async () => {
    const user = userEvent.setup();
    render(
      <AgentAccessTokenSection
        agent={{...baseAgent, inboundAuthConfig: baseInboundAuthConfig}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    const input = document.getElementById('agent-access-token-validity')!;
    await user.clear(input);
    await user.type(input, '7200');

    expect(mockOnFieldChange).toHaveBeenLastCalledWith(
      'inboundAuthConfig',
      expect.arrayContaining([
        expect.objectContaining({
          type: 'oauth2',
          config: expect.objectContaining({
            token: expect.objectContaining({
              accessToken: expect.objectContaining({
                clientConfig: expect.objectContaining({validityPeriod: 7200}) as Record<string, unknown>,
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }) as Record<string, unknown>,
        }),
      ]),
    );
  });

  it('shows a validation error and reports it, without committing, when the validity period is cleared', async () => {
    const user = userEvent.setup();
    const onValidationChange = vi.fn();
    render(
      <AgentAccessTokenSection
        agent={{...baseAgent, inboundAuthConfig: baseInboundAuthConfig}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
        onValidationChange={onValidationChange}
      />,
    );

    mockOnFieldChange.mockClear();
    const input = document.getElementById('agent-access-token-validity')!;
    await user.clear(input);

    expect(screen.getByText('Enter a validity period of at least 1 second.')).toBeInTheDocument();
    expect(onValidationChange).toHaveBeenLastCalledWith(true);
    expect(mockOnFieldChange).not.toHaveBeenCalled();
  });

  it('disables attribute selection and the validity input for read-only agents', () => {
    render(
      <AgentAccessTokenSection
        agent={{...baseAgent, isReadOnly: true, inboundAuthConfig: baseInboundAuthConfig}}
        editedAgent={{}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(document.getElementById('agent-access-token-validity')!).toBeDisabled();
  });
});
