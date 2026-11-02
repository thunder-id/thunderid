// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import type {OAuth2Config, OAuth2Token} from '@thunderid/configure-applications';
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

const TOKEN_EXCHANGE = 'urn:ietf:params:oauth:grant-type:token-exchange';
const JWT_BEARER = 'urn:ietf:params:oauth:grant-type:jwt-bearer';

// Stands in for the shared card, so the tests drive its two callbacks exactly as the card does.
vi.mock('@thunderid/configure-applications', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-applications')>()),
  IdentityAssertionsSection: ({
    oauth2Config = undefined,
    onTokenConfigChange,
    disabled = false,
    onValidationChange = undefined,
  }: {
    oauth2Config?: OAuth2Config;
    onTokenConfigChange: (tokenUpdates: Partial<OAuth2Token>, oauth2Updates?: Partial<OAuth2Config>) => void;
    disabled?: boolean;
    onValidationChange?: (hasErrors: boolean) => void;
  }) => (
    <div data-testid="identity-assertions-section" data-disabled={String(disabled)}>
      <button
        type="button"
        onClick={() =>
          onTokenConfigChange(
            {idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300}},
            {grantTypes: [...(oauth2Config?.grantTypes ?? []), TOKEN_EXCHANGE]},
          )
        }
      >
        Enable ID-JAG
      </button>
      <button type="button" onClick={() => onValidationChange?.(true)}>
        Report invalid ID-JAG
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
    display: 'Test Agent',
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
      expect(screen.getByText('This agent can act on behalf of a user.')).toBeInTheDocument();
      expect(screen.queryByText(/authorization code/i)).not.toBeInTheDocument();
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

    it('turns on Delegated mode with the default delegated grants and requires PKCE', async () => {
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
              grantTypes: ['client_credentials', 'authorization_code', 'refresh_token'],
              pkceRequired: true,
            }) as Record<string, unknown>,
          }),
        ]) as AgentInboundAuthConfig[],
      );
    });

    it.each([
      ['CIBA', 'urn:openid:params:grant-type:ciba'],
      ['token exchange', TOKEN_EXCHANGE],
      ['JWT bearer', JWT_BEARER],
    ])('shows the toggle checked for an agent delegated through %s alone', (_name, grant) => {
      render(
        <EditAdvancedSettings
          agent={mockAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['client_credentials', grant], responseTypes: []}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('switch', {name: 'Delegated mode'})).toBeChecked();
    });

    it('turns off Delegated mode for an agent delegated through CIBA and token exchange', async () => {
      const user = userEvent.setup();
      const grantTypes = [
        'client_credentials',
        'urn:openid:params:grant-type:ciba',
        'urn:ietf:params:oauth:grant-type:token-exchange',
        'refresh_token',
      ];
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, inboundAuthConfig: [{type: 'oauth2', config: {grantTypes, responseTypes: []}}]}}
          editedAgent={{}}
          oauth2Config={{grantTypes, responseTypes: []}}
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

    it('should remove JWT bearer when Delegated mode is turned off', async () => {
      const user = userEvent.setup();
      const grantTypes = ['client_credentials', JWT_BEARER];
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, inboundAuthConfig: [{type: 'oauth2', config: {grantTypes, responseTypes: []}}]}}
          editedAgent={{}}
          oauth2Config={{grantTypes, responseTypes: []}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(screen.getByRole('switch', {name: 'Delegated mode'}));

      expect(mockOnFieldChange).toHaveBeenCalledWith('inboundAuthConfig', [
        {type: 'oauth2', config: {grantTypes: ['client_credentials'], responseTypes: []}},
      ]);
    });

    it('should turn identity assertions off, keeping their settings, when Delegated mode is turned off', async () => {
      const user = userEvent.setup();
      const config: OAuthAgentConfig = {
        grantTypes: ['client_credentials', TOKEN_EXCHANGE, JWT_BEARER],
        responseTypes: [],
        token: {
          accessToken: {defaultAudience: 'https://api.example.com'},
          idJag: {enabled: true, allowedAudiences: ['https://mcp.example.com'], validityPeriod: 600},
        } as OAuth2Config['token'],
      };
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, inboundAuthConfig: [{type: 'oauth2', config}]}}
          editedAgent={{}}
          oauth2Config={config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(screen.getByRole('switch', {name: 'Delegated mode'}));

      expect(mockOnFieldChange).toHaveBeenCalledTimes(1);
      expect(mockOnFieldChange).toHaveBeenCalledWith('inboundAuthConfig', [
        {
          type: 'oauth2',
          config: {
            grantTypes: ['client_credentials'],
            responseTypes: [],
            token: {
              accessToken: {defaultAudience: 'https://api.example.com'},
              idJag: {enabled: false, allowedAudiences: ['https://mcp.example.com'], validityPeriod: 600},
            },
          },
        },
      ]);
    });

    it('should leave the token config alone when Delegated mode is turned off with identity assertions off', async () => {
      const user = userEvent.setup();
      const config: OAuthAgentConfig = {
        grantTypes: ['client_credentials', TOKEN_EXCHANGE],
        responseTypes: [],
        token: {idJag: {enabled: false, allowedAudiences: ['https://mcp.example.com']}} as OAuth2Config['token'],
      };
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, inboundAuthConfig: [{type: 'oauth2', config}]}}
          editedAgent={{}}
          oauth2Config={config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(screen.getByRole('switch', {name: 'Delegated mode'}));

      expect(mockOnFieldChange).toHaveBeenCalledWith('inboundAuthConfig', [
        {type: 'oauth2', config: {...config, grantTypes: ['client_credentials']}},
      ]);
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

  describe('Identity assertions (ID-JAG)', () => {
    it('should show the card, after the OAuth 2 configuration, in Delegated mode', () => {
      render(
        <EditAdvancedSettings
          agent={mockAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['client_credentials', JWT_BEARER], responseTypes: []}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const card = screen.getByTestId('identity-assertions-section');
      expect(card).toHaveAttribute('data-disabled', 'false');
      expect(
        screen.getByTestId('operation-modes-section').compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    });

    it('should hide the card when Delegated mode is off', () => {
      render(
        <EditAdvancedSettings
          agent={mockAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByTestId('identity-assertions-section')).not.toBeInTheDocument();
    });

    it('should hide the card for an agent with no OAuth2 inbound config', () => {
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, inboundAuthConfig: []}}
          editedAgent={{}}
          oauth2Config={undefined}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByTestId('identity-assertions-section')).not.toBeInTheDocument();
    });

    it('should disable the card for a read-only agent', () => {
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, isReadOnly: true}}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['authorization_code'], responseTypes: ['code']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('identity-assertions-section')).toHaveAttribute('data-disabled', 'true');
    });

    it('should merge the ID-JAG settings and the added token exchange grant into one change, keeping PKCE', async () => {
      const user = userEvent.setup();
      const config: OAuthAgentConfig = {
        grantTypes: ['client_credentials', 'authorization_code'],
        responseTypes: ['code'],
        redirectUris: ['https://example.com/cb'],
        token: {accessToken: {defaultAudience: 'https://api.example.com'}} as OAuth2Config['token'],
      };
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, inboundAuthConfig: [{type: 'oauth2', config}]}}
          editedAgent={{}}
          oauth2Config={config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(screen.getByRole('button', {name: 'Enable ID-JAG'}));

      expect(mockOnFieldChange).toHaveBeenCalledTimes(1);
      expect(mockOnFieldChange).toHaveBeenCalledWith('inboundAuthConfig', [
        {
          type: 'oauth2',
          config: {
            ...config,
            grantTypes: ['client_credentials', 'authorization_code', TOKEN_EXCHANGE],
            pkceRequired: true,
            token: {
              accessToken: {defaultAudience: 'https://api.example.com'},
              idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300},
            },
          },
        },
      ]);
    });

    it('should not require PKCE when the agent does not hold authorization code', async () => {
      const user = userEvent.setup();
      const config: OAuthAgentConfig = {grantTypes: ['client_credentials', JWT_BEARER], responseTypes: []};
      render(
        <EditAdvancedSettings
          agent={{...mockAgent, inboundAuthConfig: [{type: 'oauth2', config}]}}
          editedAgent={{}}
          oauth2Config={config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(screen.getByRole('button', {name: 'Enable ID-JAG'}));

      expect(mockOnFieldChange).toHaveBeenCalledWith('inboundAuthConfig', [
        {
          type: 'oauth2',
          config: {
            grantTypes: ['client_credentials', JWT_BEARER, TOKEN_EXCHANGE],
            responseTypes: [],
            token: {idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300}},
          },
        },
      ]);
    });

    it('should report the card validation state to the edit page', async () => {
      const user = userEvent.setup();
      const onIdentityAssertionsValidationChange = vi.fn();
      render(
        <EditAdvancedSettings
          agent={mockAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['client_credentials', TOKEN_EXCHANGE], responseTypes: []}}
          onFieldChange={mockOnFieldChange}
          onIdentityAssertionsValidationChange={onIdentityAssertionsValidationChange}
        />,
      );

      await user.click(screen.getByRole('button', {name: 'Report invalid ID-JAG'}));

      expect(onIdentityAssertionsValidationChange).toHaveBeenLastCalledWith(true);
    });

    it('should clear the report when the tab unmounts, since an uncommitted value goes with it', async () => {
      const user = userEvent.setup();
      const onIdentityAssertionsValidationChange = vi.fn();
      const {unmount} = render(
        <EditAdvancedSettings
          agent={mockAgent}
          editedAgent={{}}
          oauth2Config={{grantTypes: ['client_credentials', TOKEN_EXCHANGE], responseTypes: []}}
          onFieldChange={mockOnFieldChange}
          onIdentityAssertionsValidationChange={onIdentityAssertionsValidationChange}
        />,
      );
      await user.click(screen.getByRole('button', {name: 'Report invalid ID-JAG'}));

      unmount();

      expect(onIdentityAssertionsValidationChange).toHaveBeenLastCalledWith(false);
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
