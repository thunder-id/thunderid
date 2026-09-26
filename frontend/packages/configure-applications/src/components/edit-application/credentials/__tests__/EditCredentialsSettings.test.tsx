// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import EditCredentialsSettings from '../EditCredentialsSettings';
import type {Application, OAuth2Config} from '@thunderid/configure-applications';

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    config: {
      client: {
        client_id: 'CONSOLE',
      },
    },
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
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

describe('EditCredentialsSettings', () => {
  const mockOnFieldChange = vi.fn();

  const buildApplication = (overrides: Partial<Application> = {}): Application =>
    ({
      id: 'app-123',
      name: 'Test App',
      isReadOnly: false,
      type: 'fullstack',
      inboundAuthConfig: [
        {
          type: 'oauth2',
          config: {
            clientId: 'client-123',
            tokenEndpointAuthMethod: 'none',
          },
        },
      ],
      ...overrides,
    }) as Application;

  const confidentialOAuth2Config: OAuth2Config = {
    clientId: 'client-123',
    tokenEndpointAuthMethod: 'client_secret_basic',
  } as OAuth2Config;

  const publicOAuth2Config: OAuth2Config = {
    clientId: 'client-123',
    tokenEndpointAuthMethod: 'none',
  } as OAuth2Config;

  // Fullstack app, no redirect grant, no client_credentials-only grant, not public — flow-native.
  const flowNativeOAuth2Config: OAuth2Config = {
    clientId: 'flow-native-123',
    tokenEndpointAuthMethod: 'none',
    publicClient: false,
    grantTypes: ['refresh_token'],
  } as OAuth2Config;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Identifier section', () => {
    it('shows the Client ID for a redirect-based app', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={publicOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByLabelText('Client ID')).toHaveDisplayValue('client-123');
      expect(screen.queryByLabelText('Application ID')).not.toBeInTheDocument();
    });

    it('shows the Application ID for an app-native (mobile) app', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication({type: 'mobile'})}
          editedApp={{}}
          oauth2Config={publicOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByLabelText('Application ID')).toHaveDisplayValue('app-123');
      expect(screen.queryByLabelText('Client ID')).not.toBeInTheDocument();
    });

    it('falls back to the Application ID when there is no OAuth2 client ID', () => {
      render(
        <EditCredentialsSettings application={buildApplication()} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByLabelText('Application ID')).toHaveDisplayValue('app-123');
    });
  });

  describe('Secret section — redirect-based (Client Secret)', () => {
    it('shows the masked secret field and a Regenerate button for a confidential client', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={confidentialOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByLabelText('Client Secret')).toHaveDisplayValue('••••••••••••••••');
      expect(screen.getByRole('button', {name: /regenerate/i})).toBeInTheDocument();
    });

    it('hides the secret section for a public browser (redirect) client', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication({type: 'browser'})}
          editedApp={{}}
          oauth2Config={{...publicOAuth2Config, publicClient: true, grantTypes: ['authorization_code']}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByDisplayValue('••••••••••••••••')).not.toBeInTheDocument();
    });

    it('opens the regenerate secret dialog when Regenerate is clicked', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={confidentialOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      fireEvent.click(screen.getByRole('button', {name: /regenerate/i}));

      expect(screen.getByTestId('regenerate-dialog')).toBeInTheDocument();
    });

    it('shows the new secret once after the dialog reports success', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={confidentialOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      fireEvent.click(screen.getByRole('button', {name: /regenerate/i}));
      fireEvent.click(screen.getByTestId('dialog-success'));

      expect(screen.getByTestId('secret-dialog')).toHaveAttribute('data-client-secret', 'new-test-secret');
    });

    it('disables the Regenerate button when the application is read-only', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication({isReadOnly: true})}
          editedApp={{}}
          oauth2Config={confidentialOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('button', {name: /regenerate/i})).toBeDisabled();
    });

    it('disables the Regenerate button for the system console client', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={{...confidentialOAuth2Config, clientId: 'console'}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('button', {name: /regenerate/i})).toBeDisabled();
    });
  });

  describe('Secret section — flow-native (Flow Secret)', () => {
    it('shows the Flow Secret label and a Regenerate button for a flow-native app', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={flowNativeOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByLabelText('Flow Secret')).toHaveDisplayValue('••••••••••••••••');
      expect(screen.getByRole('button', {name: /regenerate/i})).toBeInTheDocument();
    });

    it('opens the regenerate flow secret dialog when Regenerate is clicked', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={flowNativeOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      fireEvent.click(screen.getByRole('button', {name: /regenerate/i}));

      expect(screen.getByTestId('regenerate-flow-secret-dialog')).toBeInTheDocument();
    });

    it('shows the new flow secret once after the dialog reports success', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={flowNativeOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      fireEvent.click(screen.getByRole('button', {name: /regenerate/i}));
      fireEvent.click(screen.getByTestId('flow-dialog-success'));

      expect(screen.getByTestId('secret-dialog')).toHaveAttribute('data-client-secret', 'new-test-flow-secret');
    });

    it('does not show a secret section for an M2M (client_credentials only) app', () => {
      const m2mConfig: OAuth2Config = {
        clientId: 'm2m-123',
        tokenEndpointAuthMethod: 'none',
        publicClient: false,
        grantTypes: ['client_credentials'],
      } as OAuth2Config;

      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={m2mConfig}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByDisplayValue('••••••••••••••••')).not.toBeInTheDocument();
    });

    it('does not show a secret section for a redirect (authorization_code) app', () => {
      const redirectConfig: OAuth2Config = {
        clientId: 'redirect-123',
        tokenEndpointAuthMethod: 'none',
        publicClient: false,
        grantTypes: ['authorization_code'],
      } as OAuth2Config;

      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={redirectConfig}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByDisplayValue('••••••••••••••••')).not.toBeInTheDocument();
    });
  });

  describe('Certificate section', () => {
    it('renders the Certificate section', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={publicOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.certificate')).toBeInTheDocument();
    });
  });

  describe('Attestation section', () => {
    it('does not render the attestation section by default', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={publicOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByText('Platform Attestation')).not.toBeInTheDocument();
    });

    it('renders the attestation section when showAttestation is true', () => {
      render(
        <EditCredentialsSettings
          application={buildApplication()}
          editedApp={{}}
          oauth2Config={publicOAuth2Config}
          onFieldChange={mockOnFieldChange}
          showAttestation
        />,
      );

      expect(screen.getByText('Platform Attestation')).toBeInTheDocument();
    });
  });
});
