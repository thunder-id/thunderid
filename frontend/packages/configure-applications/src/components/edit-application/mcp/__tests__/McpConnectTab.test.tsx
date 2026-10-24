// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import McpConnectTab from '../McpConnectTab';

vi.mock('../McpAccessSection', () => ({
  default: function MockMcpAccessSection({onValidationChange}: {onValidationChange?: (hasErrors: boolean) => void}) {
    // Mimics McpAccessSection's real redirect-URI list, which lives in local state — used to
    // prove that a changed sectionResetKey remounts (rather than just re-renders) it.
    const [clicks, setClicks] = useState(0);
    return (
      <div data-testid="mcp-access-section">
        McpAccessSection, Clicks: {clicks}
        <button
          type="button"
          data-testid="mcp-access-section-report-invalid"
          onClick={() => onValidationChange?.(true)}
        >
          Report invalid
        </button>
        <button type="button" data-testid="mcp-access-section-bump" onClick={() => setClicks((c) => c + 1)}>
          Bump
        </button>
      </div>
    );
  },
}));

vi.mock('../../cimd/CimdDocumentSection', () => ({
  default: () => <div data-testid="cimd-document-section">CimdDocumentSection</div>,
}));

describe('McpConnectTab', () => {
  const mockOnFieldChange = vi.fn();

  const buildApplication = (overrides: Partial<Application> = {}): Application =>
    ({
      id: 'app-mcp-123',
      name: 'My MCP Client',
      isReadOnly: false,
      inboundAuthConfig: [
        {
          type: 'oauth2',
          config: {
            clientId: 'mcp-client-id',
            grantTypes: ['authorization_code', 'refresh_token'],
            redirectUris: ['http://127.0.0.1:8080/callback'],
            publicClient: true,
            tokenEndpointAuthMethod: 'none',
          },
        },
      ],
      ...overrides,
    }) as Application;

  const userDelegatedOAuth2Config: OAuth2Config = {
    clientId: 'mcp-client-id',
    grantTypes: ['authorization_code', 'refresh_token'],
    redirectUris: ['http://127.0.0.1:8080/callback'],
    publicClient: true,
    tokenEndpointAuthMethod: 'none',
  } as OAuth2Config;

  const m2mOAuth2Config: OAuth2Config = {
    clientId: 'mcp-m2m-client-id',
    grantTypes: ['client_credentials'],
    tokenEndpointAuthMethod: 'client_secret_basic',
  } as OAuth2Config;

  const jwtBearerOnlyOAuth2Config: OAuth2Config = {
    clientId: 'mcp-jwt-bearer-client-id',
    grantTypes: ['urn:ietf:params:oauth:grant-type:jwt-bearer'],
    publicClient: false,
    tokenEndpointAuthMethod: 'client_secret_basic',
  } as OAuth2Config;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('OAuth profile badge', () => {
    it('renders the user-delegated badge for an authorization_code client', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByText('On behalf of a user (authorization code + PKCE)')).toBeInTheDocument();
    });

    it('renders the machine-to-machine badge for a client_credentials-only client', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByText('On its own behalf (client credentials)')).toBeInTheDocument();
    });

    it('renders the JWT bearer badge for a client that only redeems assertions', () => {
      render(
        <McpConnectTab
          application={buildApplication({type: 'mcp'})}
          oauth2Config={jwtBearerOnlyOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByText('On behalf of a user (JWT bearer with ID-JAG)')).toBeInTheDocument();
      expect(screen.queryByText('On behalf of a user (authorization code + PKCE)')).not.toBeInTheDocument();
    });

    it('keeps the user-delegated badge when the JWT bearer grant is paired with authorization_code', () => {
      render(
        <McpConnectTab
          application={buildApplication({type: 'mcp'})}
          oauth2Config={{
            ...jwtBearerOnlyOAuth2Config,
            grantTypes: ['authorization_code', 'urn:ietf:params:oauth:grant-type:jwt-bearer'],
          }}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByText('On behalf of a user (authorization code + PKCE)')).toBeInTheDocument();
    });

    it('keeps the machine-to-machine badge for an m2m application that also has the JWT bearer grant', () => {
      render(
        <McpConnectTab
          application={buildApplication({type: 'm2m'})}
          oauth2Config={jwtBearerOnlyOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByText('On its own behalf (client credentials)')).toBeInTheDocument();
    });
  });

  describe('Identity fields', () => {
    it('renders the Application ID and Client ID as read-only copyable fields', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByDisplayValue('app-mcp-123')).toBeInTheDocument();
      expect(screen.getByDisplayValue('mcp-client-id')).toBeInTheDocument();
    });
  });

  describe('Credentials tab pointer', () => {
    const pointerText = /Manage the client secret and keys on the/;

    it('points a confidential client to the Credentials tab and switches to it when clicked', async () => {
      const user = userEvent.setup();
      const onGoToCredentials = vi.fn();
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onGoToCredentials={onGoToCredentials}
        />,
      );

      expect(screen.getByText(pointerText)).toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: 'Credentials tab'}));

      expect(onGoToCredentials).toHaveBeenCalledTimes(1);
    });

    it('points a private_key_jwt client to the Credentials tab, where its keys are managed', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={{...m2mOAuth2Config, tokenEndpointAuthMethod: 'private_key_jwt'}}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onGoToCredentials={vi.fn()}
        />,
      );

      expect(screen.getByRole('button', {name: 'Credentials tab'})).toBeInTheDocument();
    });

    it('no longer shows the client secret field or a Generate action', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onGoToCredentials={vi.fn()}
        />,
      );

      expect(screen.queryByDisplayValue('••••••••••••••••')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Generate'})).not.toBeInTheDocument();
    });

    it('does not point a public client to the Credentials tab', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onGoToCredentials={vi.fn()}
        />,
      );

      expect(screen.queryByText(pointerText)).not.toBeInTheDocument();
    });

    it('does not show the pointer when there is no Credentials tab to go to', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.queryByText(pointerText)).not.toBeInTheDocument();
    });
  });

  describe('Access section', () => {
    it('renders the access section for a user-delegated client', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByTestId('mcp-access-section')).toBeInTheDocument();
    });

    it('renders the access section for a JWT bearer-only client', () => {
      render(
        <McpConnectTab
          application={buildApplication({type: 'mcp'})}
          oauth2Config={jwtBearerOnlyOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByTestId('mcp-access-section')).toBeInTheDocument();
    });

    it('hides the access section for a machine-to-machine client', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.queryByTestId('mcp-access-section')).not.toBeInTheDocument();
    });

    it('forwards onValidationChange to the access section for a user-delegated client', async () => {
      const mockOnValidationChange = vi.fn();
      const user = userEvent.setup();
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          onValidationChange={mockOnValidationChange}
        />,
      );

      await user.click(screen.getByTestId('mcp-access-section-report-invalid'));

      expect(mockOnValidationChange).toHaveBeenCalledWith(true);
    });
  });

  describe('Access section reset', () => {
    it('remounts McpAccessSection, dropping its local state, when sectionResetKey changes', () => {
      const {rerender} = render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          sectionResetKey={0}
        />,
      );

      fireEvent.click(screen.getByTestId('mcp-access-section-bump'));
      expect(screen.getByTestId('mcp-access-section')).toHaveTextContent('Clicks: 1');

      rerender(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          sectionResetKey={1}
        />,
      );

      expect(screen.getByTestId('mcp-access-section')).toHaveTextContent('Clicks: 0');
    });

    it('keeps McpAccessSection mounted when sectionResetKey stays the same', () => {
      const {rerender} = render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          sectionResetKey={0}
        />,
      );

      fireEvent.click(screen.getByTestId('mcp-access-section-bump'));
      expect(screen.getByTestId('mcp-access-section')).toHaveTextContent('Clicks: 1');

      rerender(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
          sectionResetKey={0}
        />,
      );

      expect(screen.getByTestId('mcp-access-section')).toHaveTextContent('Clicks: 1');
    });
  });

  describe('Client ID Metadata Document clients', () => {
    const cimdConfig: OAuth2Config = {
      clientId: 'https://vscode.dev/oauth/client-metadata.json',
      clientIdMetadataDocument: true,
      redirectUris: ['http://127.0.0.1/callback'],
      grantTypes: ['authorization_code', 'refresh_token'],
      responseTypes: ['code'],
      tokenEndpointAuthMethod: 'none',
      publicClient: true,
      pkceRequired: true,
    };

    it('shows the metadata document badge and section instead of the editable access section', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={cimdConfig}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByText('CIMD')).toBeInTheDocument();
      expect(screen.getByTestId('cimd-document-section')).toBeInTheDocument();
      expect(screen.queryByTestId('mcp-access-section')).not.toBeInTheDocument();
    });

    it('treats an HTTPS client ID without the marker as an ordinary client', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={{...cimdConfig, clientIdMetadataDocument: false}}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.queryByTestId('cimd-document-section')).not.toBeInTheDocument();
      expect(screen.getByTestId('mcp-access-section')).toBeInTheDocument();
    });
  });
});
