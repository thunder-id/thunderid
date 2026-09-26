// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import EditAdvancedSettings from '../EditAdvancedSettings';
import type {Application, OAuth2Config} from '@thunderid/configure-applications';

const {mockUseThunderID} = vi.hoisted(() => ({
  mockUseThunderID: vi.fn(() => ({discovery: null}) as unknown),
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: mockUseThunderID,
}));

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
    t: (key: string) => key,
  }),
}));

vi.mock('../../general-settings/DangerZoneSection', () => ({
  default: ({onDeleteClick}: {onDeleteClick: () => void}) => (
    <div data-testid="danger-zone-section">
      <button type="button" onClick={onDeleteClick} data-testid="delete-button">
        Delete Application
      </button>
    </div>
  ),
}));

vi.mock('../../../ApplicationDeleteDialog', () => ({
  default: ({
    open,
    applicationId,
    onClose,
    onSuccess,
  }: {
    open: boolean;
    applicationId: string;
    onClose: () => void;
    onSuccess?: () => void;
  }) =>
    open ? (
      <div data-testid="delete-dialog" data-application-id={applicationId}>
        <button type="button" onClick={onClose} data-testid="delete-dialog-close">
          Cancel
        </button>
        <button type="button" onClick={() => onSuccess?.()} data-testid="delete-dialog-success">
          Confirm Delete
        </button>
      </div>
    ) : null,
}));

describe('EditAdvancedSettings', () => {
  const mockApplication: Application = {
    id: 'test-app-id',
    name: 'Test Application',
    description: 'Test Description',
    template: 'custom',
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-15T00:00:00Z',
  } as Application;

  const mockOAuth2Config: OAuth2Config = {
    grantTypes: ['authorization_code', 'refresh_token'],
    responseTypes: ['code'],
    pkceRequired: true,
    publicClient: false,
  };

  const mockOnFieldChange = vi.fn();

  describe('Rendering', () => {
    it('should render the OAuth 2 Configuration and Metadata sections', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.oauth2Config')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.metadata')).toBeInTheDocument();
    });

    it('should render without OAuth2 config when not provided', () => {
      render(<EditAdvancedSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByText('applications:edit.advanced.labels.oauth2Config')).not.toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.metadata')).toBeInTheDocument();
    });

    it('should render without metadata section when timestamps are missing', () => {
      const appWithoutMetadata = {...mockApplication};
      delete (appWithoutMetadata as Partial<Application>).createdAt;
      delete (appWithoutMetadata as Partial<Application>).updatedAt;

      render(
        <EditAdvancedSettings
          application={appWithoutMetadata}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.oauth2Config')).toBeInTheDocument();
      expect(screen.queryByText('applications:edit.advanced.labels.metadata')).not.toBeInTheDocument();
    });
  });

  describe('Section Integration', () => {
    it('should pass correct props to OAuth2ConfigSection', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('authorization_code')).toBeInTheDocument();
      expect(screen.getByText('refresh_token')).toBeInTheDocument();
      expect(screen.getByText('code')).toBeInTheDocument();
    });

    it('should pass correct props to MetadataSection', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.createdAt')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.updatedAt')).toBeInTheDocument();
    });
  });

  describe('Layout', () => {
    it('should render sections in a Stack with spacing', () => {
      const {container} = render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const stack = container.firstChild;
      expect(stack).toHaveClass('MuiStack-root');
    });
  });

  describe('Edge Cases', () => {
    it('should handle undefined oauth2Config', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={undefined}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByText('applications:edit.advanced.labels.oauth2Config')).not.toBeInTheDocument();
    });

    it('should handle empty editedApp', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.metadata')).toBeInTheDocument();
    });

    it('should render with minimal application data', () => {
      const minimalApp = {
        id: 'minimal-id',
        name: 'Minimal App',
        template: 'custom',
      } as Application;

      render(<EditAdvancedSettings application={minimalApp} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.advanced.labels.passkeys')).toBeInTheDocument();
    });
  });

  describe('AcrValuesSection Integration', () => {
    beforeEach(() => {
      mockUseThunderID.mockReturnValue({
        discovery: {
          wellKnown: {
            acr_values_supported: ['urn:acr:loa1', 'urn:acr:loa2'],
            grant_types_supported: ['authorization_code', 'refresh_token', 'client_credentials'],
            response_types_supported: ['code', 'token'],
            token_endpoint_auth_methods_supported: ['client_secret_basic', 'client_secret_post', 'none'],
          },
        },
      });
    });

    afterEach(() => {
      mockUseThunderID.mockReturnValue({discovery: null});
    });

    it('should render AcrValuesSection when discovery has acr_values_supported', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getAllByText('applications:edit.advanced.labels.acrValues').length).toBeGreaterThan(0);
    });

    it('should not render AcrValuesSection when discovery has no acr_values_supported', () => {
      mockUseThunderID.mockReturnValue({discovery: null});

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByText('applications:edit.advanced.labels.acrValues')).not.toBeInTheDocument();
    });

    it('should propagate AcrValues changes to onFieldChange as inboundAuthConfig update', () => {
      const appWithInboundAuth = {
        ...mockApplication,
        inboundAuthConfig: [{type: 'oauth2', config: {...mockOAuth2Config}}],
      } as Application;

      render(
        <EditAdvancedSettings
          application={appWithInboundAuth}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Open the MUI Select dropdown
      const selectButton = document.getElementById('acr_values')!;
      fireEvent.mouseDown(selectButton);

      // Click an ACR value option in the dropdown
      const option = screen.getByText('urn:acr:loa1');
      fireEvent.click(option);

      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({acrValues: ['urn:acr:loa1']}) as unknown,
          }),
        ]),
      );
    });
  });

  describe('ID-JAG Integration', () => {
    it('should apply the idJag enable and the added token-exchange grant type in a single onFieldChange call on the first click', () => {
      const oauth2ConfigWithoutTokenExchange: OAuth2Config = {
        ...mockOAuth2Config,
        grantTypes: ['authorization_code'],
      };
      const appWithInboundAuth = {
        ...mockApplication,
        inboundAuthConfig: [{type: 'oauth2', config: {...oauth2ConfigWithoutTokenExchange}}],
      } as Application;

      mockOnFieldChange.mockClear();

      render(
        <EditAdvancedSettings
          application={appWithInboundAuth}
          editedApp={{}}
          oauth2Config={oauth2ConfigWithoutTokenExchange}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const toggle = screen.getByLabelText('applications:edit.advanced.idJag.title');
      fireEvent.click(toggle);

      expect(mockOnFieldChange).toHaveBeenCalledTimes(1);
      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({
              grantTypes: expect.arrayContaining([
                'authorization_code',
                'urn:ietf:params:oauth:grant-type:token-exchange',
              ]) as unknown,
              token: expect.objectContaining({
                idJag: expect.objectContaining({enabled: true}) as unknown,
              }) as unknown,
            }) as unknown,
          }),
        ]),
      );
    });

    it('should forward onValidationChange to IdentityAssertionsSection', () => {
      const onValidationChange = vi.fn();
      const oauth2ConfigWithIdJagError: OAuth2Config = {
        ...mockOAuth2Config,
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300},
        },
      };

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={oauth2ConfigWithIdJagError}
          onFieldChange={mockOnFieldChange}
          onValidationChange={onValidationChange}
        />,
      );

      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });
  });

  describe('PasskeysSection Integration', () => {
    it('renders the Passkeys section', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.passkeys')).toBeInTheDocument();
    });

    it('displays existing passkeyAllowedOrigins from the application', () => {
      const appWithOrigins: Application = {
        ...mockApplication,
        passkeyAllowedOrigins: ['https://app.example.com'],
      };

      render(
        <EditAdvancedSettings
          application={appWithOrigins}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByDisplayValue('https://app.example.com')).toBeInTheDocument();
    });

    it('prefers editedApp.passkeyAllowedOrigins over application.passkeyAllowedOrigins', () => {
      const appWithOrigins: Application = {
        ...mockApplication,
        passkeyAllowedOrigins: ['https://old.example.com'],
      };

      render(
        <EditAdvancedSettings
          application={appWithOrigins}
          editedApp={{passkeyAllowedOrigins: ['https://new.example.com']}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByDisplayValue('https://new.example.com')).toBeInTheDocument();
      expect(screen.queryByDisplayValue('https://old.example.com')).not.toBeInTheDocument();
    });

    it('calls onFieldChange with passkeyAllowedOrigins when origins change', async () => {
      const user = userEvent.setup();
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(
        screen.getByRole('button', {name: /applications:edit.advanced.passkeys.allowedOrigins.addOrigin/i}),
      );

      expect(mockOnFieldChange).toHaveBeenCalledWith('passkeyAllowedOrigins', ['', '']);
    });

    it('passes disabled=true to PasskeysSection when the application is read-only', () => {
      const readOnlyApp: Application = {...mockApplication, isReadOnly: true};

      render(
        <EditAdvancedSettings
          application={readOnlyApp}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // In read-only mode the Add Origin button should not be present
      expect(screen.queryByRole('button', {name: /Add Origin/i})).not.toBeInTheDocument();
    });

    it('reports hasErrors=true via onValidationChange when an origin is empty', () => {
      const onValidationChange = vi.fn();

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{passkeyAllowedOrigins: ['']}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
          onValidationChange={onValidationChange}
        />,
      );

      // An empty string origin is immediately invalid; the effect fires on mount.
      expect(onValidationChange).toHaveBeenCalledWith(true);
    });

    it('reports hasErrors=true via onValidationChange when an origin is not a valid URL', () => {
      const onValidationChange = vi.fn();

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{passkeyAllowedOrigins: ['not-a-url']}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
          onValidationChange={onValidationChange}
        />,
      );

      expect(onValidationChange).toHaveBeenCalledWith(true);
    });

    it('reports hasErrors=false via onValidationChange when all origins are valid URLs', () => {
      const onValidationChange = vi.fn();

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{passkeyAllowedOrigins: ['https://app.example.com']}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
          onValidationChange={onValidationChange}
        />,
      );

      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });

    it('reports hasErrors=false via onValidationChange when the origins list is empty', () => {
      const onValidationChange = vi.fn();

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{passkeyAllowedOrigins: []}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
          onValidationChange={onValidationChange}
        />,
      );

      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });
  });

  describe('Danger Zone', () => {
    const confidentialConfig: OAuth2Config = {
      clientId: 'client-123',
      clientSecret: 'secret-456',
      tokenEndpointAuthMethod: 'client_secret_basic',
    } as OAuth2Config;

    it('should render DangerZoneSection', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={confidentialConfig}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
    });

    it('should not render DangerZoneSection for the system console client', () => {
      const consoleConfig: OAuth2Config = {
        clientId: 'console',
        tokenEndpointAuthMethod: 'client_secret_basic',
      } as OAuth2Config;

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={consoleConfig}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByTestId('danger-zone-section')).not.toBeInTheDocument();
    });

    it('should not render DangerZoneSection for a read-only application', () => {
      const readOnlyApp: Application = {...mockApplication, isReadOnly: true};

      render(
        <EditAdvancedSettings
          application={readOnlyApp}
          editedApp={{}}
          oauth2Config={confidentialConfig}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByTestId('danger-zone-section')).not.toBeInTheDocument();
    });

    it('should open delete dialog when delete button is clicked', () => {
      render(<EditAdvancedSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      fireEvent.click(screen.getByTestId('delete-button'));

      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });

    it('should pass application id to delete dialog', () => {
      render(<EditAdvancedSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      fireEvent.click(screen.getByTestId('delete-button'));

      expect(screen.getByTestId('delete-dialog')).toHaveAttribute('data-application-id', 'test-app-id');
    });

    it('should close delete dialog when cancel is triggered', () => {
      render(<EditAdvancedSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      fireEvent.click(screen.getByTestId('delete-button'));
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();

      fireEvent.click(screen.getByTestId('delete-dialog-close'));

      expect(screen.queryByTestId('delete-dialog')).not.toBeInTheDocument();
    });

    it('should call onDeleteSuccess when delete is confirmed', () => {
      const mockOnDeleteSuccess = vi.fn();

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          onDeleteSuccess={mockOnDeleteSuccess}
        />,
      );

      fireEvent.click(screen.getByTestId('delete-button'));
      fireEvent.click(screen.getByTestId('delete-dialog-success'));

      expect(mockOnDeleteSuccess).toHaveBeenCalledTimes(1);
    });
  });
});
