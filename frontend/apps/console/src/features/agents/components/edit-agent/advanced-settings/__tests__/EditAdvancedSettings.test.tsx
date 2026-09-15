// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

vi.mock('../OwnerSection', () => ({
  default: () => <div data-testid="owner-section">Owner</div>,
}));

vi.mock('../AllowedUserTypesSection', () => ({
  default: () => <div data-testid="allowed-user-types-section">Allowed User Types</div>,
}));

vi.mock('../AgentSignInSection', () => ({
  default: () => <div data-testid="agent-sign-in-section">Agent Sign-In</div>,
}));

vi.mock('../DangerZoneSection', () => ({
  default: ({onDeleteClick}: {onDeleteClick: () => void}) => (
    <button type="button" data-testid="danger-zone-section" onClick={onDeleteClick}>
      Delete Agent
    </button>
  ),
}));

vi.mock('../../../AgentDeleteDialog', () => ({
  default: ({open, onSuccess}: {open: boolean; onSuccess?: () => void}) =>
    open ? (
      <button type="button" data-testid="agent-delete-dialog" onClick={() => onSuccess?.()}>
        Confirm Delete
      </button>
    ) : null,
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
    expect(screen.getByTestId('owner-section')).toBeInTheDocument();
    expect(screen.getByTestId('allowed-user-types-section')).toBeInTheDocument();
    expect(screen.getByTestId('agent-sign-in-section')).toBeInTheDocument();
    expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
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

  describe('Delegated mode toggle', () => {
    it('shows the toggle checked, and the delegated description, when Delegated mode is on', () => {
      render(
        <EditAdvancedSettings
          agent={mockAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('switch', {name: 'Delegated mode'})).toBeChecked();
      expect(screen.getByText(/acts on behalf of a signed-in user/i)).toBeInTheDocument();
    });

    it('shows the toggle unchecked, and the own-behalf description, when Delegated mode is off', () => {
      render(
        <EditAdvancedSettings
          agent={mockAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('switch', {name: 'Delegated mode'})).not.toBeChecked();
      expect(screen.getByText(/authenticates with its own credentials/i)).toBeInTheDocument();
    });

    it('turns on Delegated mode by adding authorization_code and requiring PKCE', async () => {
      const user = userEvent.setup();
      render(
        <EditAdvancedSettings
          agent={{
            ...mockAgent,
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
        ]) as AgentInboundAuthConfig[],
      );
    });

    it('turns off Delegated mode by dropping the delegated-only grants', async () => {
      const user = userEvent.setup();
      render(
        <EditAdvancedSettings
          agent={{
            ...mockAgent,
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
        ]) as AgentInboundAuthConfig[],
      );
    });

    it('disables the toggle for read-only agents', () => {
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, isReadOnly: true}}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('switch', {name: 'Delegated mode'})).toBeDisabled();
    });
  });

  describe('Danger Zone', () => {
    it('renders the Danger Zone for an editable agent', () => {
      render(
        <EditAdvancedSettings
          agent={mockAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
    });

    it('does not render the Danger Zone for a read-only agent', () => {
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, isReadOnly: true}}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByTestId('danger-zone-section')).not.toBeInTheDocument();
    });

    it('opens the delete dialog when Delete Agent is clicked and reports success', async () => {
      const user = userEvent.setup();
      const mockOnDeleteSuccess = vi.fn();
      render(
        <EditAdvancedSettings
          agent={mockAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
          onFieldChange={mockOnFieldChange}
          onDeleteSuccess={mockOnDeleteSuccess}
        />,
      );

      expect(screen.queryByTestId('agent-delete-dialog')).not.toBeInTheDocument();

      await user.click(screen.getByTestId('danger-zone-section'));
      expect(screen.getByTestId('agent-delete-dialog')).toBeInTheDocument();

      await user.click(screen.getByTestId('agent-delete-dialog'));
      expect(mockOnDeleteSuccess).toHaveBeenCalledTimes(1);
    });
  });

  describe('Agents with no OAuth2 inbound config', () => {
    it('still renders the Owner section and Danger Zone', () => {
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, inboundAuthConfig: []}}
          editedAgent={{}}
          oauth2Config={undefined}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('owner-section')).toBeInTheDocument();
      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
    });
  });
});
