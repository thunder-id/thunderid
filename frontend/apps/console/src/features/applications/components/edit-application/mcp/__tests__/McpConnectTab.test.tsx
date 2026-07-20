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

vi.mock('../../general-settings/DangerZoneSection', () => ({
  default: ({
    onRegenerateClick,
    onDeleteClick,
    showRegenerateSecret,
    showRegenerateFlowSecret,
    onRegenerateFlowSecretClick,
  }: {
    onRegenerateClick?: () => void;
    onDeleteClick: () => void;
    showRegenerateSecret?: boolean;
    showRegenerateFlowSecret?: boolean;
    onRegenerateFlowSecretClick?: () => void;
  }) => (
    <div data-testid="danger-zone-section">
      {showRegenerateSecret && (
        <button type="button" onClick={onRegenerateClick} data-testid="danger-zone-regenerate-button">
          Regenerate Client Secret
        </button>
      )}
      {showRegenerateFlowSecret && (
        <button
          type="button"
          onClick={onRegenerateFlowSecretClick}
          data-testid="danger-zone-regenerate-flow-secret-button"
        >
          Regenerate Flow Secret
        </button>
      )}
      <button type="button" onClick={onDeleteClick} data-testid="delete-button">
        Delete Application
      </button>
    </div>
  ),
}));

vi.mock('../../../RegenerateSecretDialog', () => ({
  default: ({
    open,
    applicationId,
    onSuccess,
  }: {
    open: boolean;
    applicationId: string | null;
    onClose: () => void;
    onSuccess?: (clientSecret: string) => void;
  }) =>
    open ? (
      <div data-testid="regenerate-dialog" data-application-id={applicationId}>
        <button type="button" onClick={() => onSuccess?.('new-test-secret')} data-testid="dialog-success">
          Trigger Success
        </button>
      </div>
    ) : null,
}));

vi.mock('../../../RegenerateFlowSecretDialog', () => ({
  default: ({
    open,
    applicationId,
    onSuccess,
  }: {
    open: boolean;
    applicationId: string | null;
    onClose: () => void;
    onSuccess?: (flowSecret: string) => void;
  }) =>
    open ? (
      <div data-testid="regenerate-flow-secret-dialog" data-application-id={applicationId}>
        <button type="button" onClick={() => onSuccess?.('new-test-flow-secret')} data-testid="flow-dialog-success">
          Trigger Success
        </button>
      </div>
    ) : null,
}));

vi.mock('../../../ClientSecretSuccessDialog', () => ({
  default: ({open, clientSecret}: {open: boolean; clientSecret: string}) =>
    open ? (
      <div data-testid="secret-dialog" data-client-secret={clientSecret}>
        Secret dialog
      </div>
    ) : null,
}));

vi.mock('../../../ApplicationDeleteDialog', () => ({
  default: ({open, applicationId}: {open: boolean; applicationId: string}) =>
    open ? (
      <div data-testid="delete-dialog" data-application-id={applicationId}>
        Delete dialog
      </div>
    ) : null,
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

      expect(screen.getByText('On behalf of a user (Authorization Code + PKCE)')).toBeInTheDocument();
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

      expect(screen.getByText('On its own behalf (Client Credentials)')).toBeInTheDocument();
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

  describe('Client secret management', () => {
    it('shows the masked secret field and a Generate button for a confidential client', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByText('Client Secret')).toBeInTheDocument();
      expect(screen.getByDisplayValue('••••••••••••••••')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Generate'})).toBeInTheDocument();
    });

    it('hides the secret row for a public client', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.queryByDisplayValue('••••••••••••••••')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Generate'})).not.toBeInTheDocument();
    });

    it('opens the regenerate secret dialog when Generate is clicked', async () => {
      const user = userEvent.setup();
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      await user.click(screen.getByRole('button', {name: 'Generate'}));

      expect(screen.getByTestId('regenerate-dialog')).toBeInTheDocument();
    });

    it('shows the new secret once after the dialog reports success', async () => {
      const user = userEvent.setup();
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      await user.click(screen.getByRole('button', {name: 'Generate'}));
      await user.click(screen.getByTestId('dialog-success'));

      expect(screen.getByTestId('secret-dialog')).toHaveAttribute('data-client-secret', 'new-test-secret');
    });

    it('disables the Generate button when the application is read-only', () => {
      render(
        <McpConnectTab
          application={buildApplication({isReadOnly: true})}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly
        />,
      );

      expect(screen.getByRole('button', {name: 'Generate'})).toBeDisabled();
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

  describe('Danger zone', () => {
    it('does not show a regenerate secret entry in the danger zone', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
      expect(screen.queryByTestId('danger-zone-regenerate-button')).not.toBeInTheDocument();
    });

    it('does not show regenerate Flow Secret for a user-delegated (authorization_code) client', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.queryByTestId('danger-zone-regenerate-flow-secret-button')).not.toBeInTheDocument();
    });

    it('does not show regenerate Flow Secret for a machine-to-machine (client_credentials only) client', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      expect(screen.queryByTestId('danger-zone-regenerate-flow-secret-button')).not.toBeInTheDocument();
    });

    it('shows and wires regenerate Flow Secret for a flow-native oauth2Config', async () => {
      const user = userEvent.setup();
      const flowNativeOAuth2Config: OAuth2Config = {
        clientId: 'mcp-flow-native-client-id',
        grantTypes: ['refresh_token'],
        publicClient: false,
      } as OAuth2Config;

      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={flowNativeOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      await user.click(screen.getByTestId('danger-zone-regenerate-flow-secret-button'));
      expect(screen.getByTestId('regenerate-flow-secret-dialog')).toBeInTheDocument();

      await user.click(screen.getByTestId('flow-dialog-success'));
      expect(screen.getByTestId('secret-dialog')).toHaveAttribute('data-client-secret', 'new-test-flow-secret');
    });

    it('does not render the danger zone when the application is read-only', () => {
      render(
        <McpConnectTab
          application={buildApplication({isReadOnly: true})}
          oauth2Config={m2mOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly
        />,
      );

      expect(screen.queryByTestId('danger-zone-section')).not.toBeInTheDocument();
    });

    it('opens the delete dialog when the danger zone delete button is clicked', () => {
      render(
        <McpConnectTab
          application={buildApplication()}
          oauth2Config={userDelegatedOAuth2Config}
          onFieldChange={mockOnFieldChange}
          isReadOnly={false}
        />,
      );

      fireEvent.click(screen.getByTestId('delete-button'));

      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
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
});
