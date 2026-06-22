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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import EditGeneralSettings from '../EditGeneralSettings';

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    config: {
      client: {
        client_id: 'CONSOLE',
      },
    },
  }),
}));

// Mock the child components
vi.mock('../QuickCopySection', () => ({
  default: ({
    application,
    oauth2Config,
    copiedField,
  }: {
    application: Application;
    oauth2Config?: OAuth2Config;
    copiedField: string | null;
  }) => (
    <div data-testid="quick-copy-section">
      QuickCopySection - App: {application.id}, OAuth: {oauth2Config?.clientId ?? 'None'}, Copied:{' '}
      {copiedField ?? 'None'}
    </div>
  ),
}));

vi.mock('../AccessSection', () => ({
  default: ({
    application,
    editedApp,
    oauth2Config,
  }: {
    application: Application;
    editedApp: Partial<Application>;
    oauth2Config?: OAuth2Config;
  }) => (
    <div data-testid="access-section">
      AccessSection - App: {application.id}, Edited URL: {editedApp.url ?? 'None'}, OAuth:{' '}
      {oauth2Config?.clientId ?? 'None'}
    </div>
  ),
}));

vi.mock('../DangerZoneSection', () => ({
  default: ({
    onRegenerateClick,
    onDeleteClick,
    showRegenerateSecret,
  }: {
    onRegenerateClick?: () => void;
    onDeleteClick: () => void;
    showRegenerateSecret?: boolean;
  }) => (
    <div data-testid="danger-zone-section">
      {showRegenerateSecret && (
        <button type="button" onClick={onRegenerateClick} data-testid="regenerate-button">
          Regenerate Client Secret
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
    onClose,
    onSuccess,
  }: {
    open: boolean;
    applicationId: string | null;
    onClose: () => void;
    onSuccess?: (clientSecret: string) => void;
  }) =>
    open ? (
      <div data-testid="regenerate-dialog" data-application-id={applicationId}>
        <button type="button" onClick={onClose} data-testid="dialog-close">
          Close
        </button>
        <button type="button" onClick={() => onSuccess?.('new-test-secret')} data-testid="dialog-success">
          Trigger Success
        </button>
      </div>
    ) : null,
}));

vi.mock('../../../RegenerateAppSecretDialog', () => ({
  default: ({
    open,
    applicationId,
    onClose,
    onSuccess,
  }: {
    open: boolean;
    applicationId: string | null;
    onClose: () => void;
    onSuccess?: (appSecret: string) => void;
  }) =>
    open ? (
      <div data-testid="regenerate-app-secret-dialog" data-application-id={applicationId}>
        <button type="button" onClick={onClose} data-testid="app-secret-dialog-close">
          Close
        </button>
        <button
          type="button"
          onClick={() => onSuccess?.('new-test-app-secret')}
          data-testid="app-secret-dialog-success"
        >
          Trigger Success
        </button>
      </div>
    ) : null,
}));

vi.mock('../../../ClientSecretSuccessDialog', () => ({
  default: ({open, clientSecret, onClose}: {open: boolean; clientSecret: string; onClose: () => void}) =>
    open ? (
      <div data-testid="secret-dialog" data-client-secret={clientSecret}>
        <button type="button" onClick={onClose} data-testid="secret-dialog-close">
          Close Secret Dialog
        </button>
      </div>
    ) : null,
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

describe('EditGeneralSettings', () => {
  const mockOnFieldChange = vi.fn();
  const mockOnCopyToClipboard = vi.fn();
  const mockApplication: Application = {
    id: 'app-123',
    name: 'Test App',
    url: 'https://example.com',
  } as Application;

  const mockOAuth2Config: OAuth2Config = {
    clientId: 'client-123',
    clientSecret: 'secret-456',
    tokenEndpointAuthMethod: 'client_secret_basic',
  } as OAuth2Config;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render both QuickCopySection and AccessSection', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('quick-copy-section')).toBeInTheDocument();
      expect(screen.getByTestId('access-section')).toBeInTheDocument();
    });

    it('should pass application to child components', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('quick-copy-section')).toHaveTextContent('App: app-123');
      expect(screen.getByTestId('access-section')).toHaveTextContent('App: app-123');
    });

    it('should pass editedApp to AccessSection', () => {
      const editedApp = {url: 'https://edited.com'};

      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={editedApp}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('access-section')).toHaveTextContent('Edited URL: https://edited.com');
    });

    it('should pass oauth2Config to child components when provided', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={mockOAuth2Config}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('quick-copy-section')).toHaveTextContent('OAuth: client-123');
      expect(screen.getByTestId('access-section')).toHaveTextContent('OAuth: client-123');
    });

    it('should handle missing oauth2Config', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('quick-copy-section')).toHaveTextContent('OAuth: None');
      expect(screen.getByTestId('access-section')).toHaveTextContent('OAuth: None');
    });

    it('should pass copiedField to QuickCopySection', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField="app_id"
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('quick-copy-section')).toHaveTextContent('Copied: app_id');
    });

    it('should handle null copiedField', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('quick-copy-section')).toHaveTextContent('Copied: None');
    });
  });

  describe('Props Propagation', () => {
    it('should pass onFieldChange to AccessSection', () => {
      const {container} = render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(container.querySelector('[data-testid="access-section"]')).toBeInTheDocument();
    });

    it('should pass onCopyToClipboard to QuickCopySection', () => {
      const {container} = render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(container.querySelector('[data-testid="quick-copy-section"]')).toBeInTheDocument();
    });

    it('should pass all required props to both child components', () => {
      const editedApp = {url: 'https://new.com'};

      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={editedApp}
          onFieldChange={mockOnFieldChange}
          oauth2Config={mockOAuth2Config}
          copiedField="clientId"
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('quick-copy-section')).toBeInTheDocument();
      expect(screen.getByTestId('access-section')).toBeInTheDocument();
    });
  });

  describe('Layout', () => {
    it('should render sections in correct order for confidential clients', () => {
      const {container} = render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={mockOAuth2Config}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      const sections = container.querySelectorAll('[data-testid]');
      expect(sections[0]).toHaveAttribute('data-testid', 'quick-copy-section');
      expect(sections[1]).toHaveAttribute('data-testid', 'access-section');
      expect(sections[2]).toHaveAttribute('data-testid', 'danger-zone-section');
    });

    it('should render DangerZoneSection for confidential client', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={mockOAuth2Config}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
      expect(screen.getByTestId('regenerate-button')).toBeInTheDocument();
    });

    it('should render DangerZoneSection without regenerate for public client (none auth method)', () => {
      const publicClientConfig: OAuth2Config = {
        clientId: 'public-client-123',
        tokenEndpointAuthMethod: 'none',
      } as OAuth2Config;

      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={publicClientConfig}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
      expect(screen.queryByTestId('regenerate-button')).not.toBeInTheDocument();
    });

    it('should render DangerZoneSection without regenerate when no oauth2Config provided', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
      expect(screen.queryByTestId('regenerate-button')).not.toBeInTheDocument();
    });

    it('should render DangerZoneSection without regenerate for private_key_jwt auth method', () => {
      const pkjwtClientConfig: OAuth2Config = {
        clientId: 'pkjwt-client-123',
        tokenEndpointAuthMethod: 'private_key_jwt',
      } as OAuth2Config;

      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={pkjwtClientConfig}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
      expect(screen.queryByTestId('regenerate-button')).not.toBeInTheDocument();
    });

    it('should render DangerZoneSection for client_secret_post auth method', () => {
      const postClientConfig: OAuth2Config = {
        clientId: 'post-client-123',
        clientSecret: 'secret-456',
        tokenEndpointAuthMethod: 'client_secret_post',
      } as OAuth2Config;

      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={postClientConfig}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      expect(screen.getByTestId('danger-zone-section')).toBeInTheDocument();
      expect(screen.getByTestId('regenerate-button')).toBeInTheDocument();
    });
  });

  describe('Regenerate Secret Flow', () => {
    it('should open regenerate dialog when regenerate button is clicked', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={mockOAuth2Config}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      const regenerateButton = screen.getByTestId('regenerate-button');
      fireEvent.click(regenerateButton);

      expect(screen.getByTestId('regenerate-dialog')).toBeInTheDocument();
    });

    it('should pass application id to regenerate dialog', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={mockOAuth2Config}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      const regenerateButton = screen.getByTestId('regenerate-button');
      fireEvent.click(regenerateButton);

      expect(screen.getByTestId('regenerate-dialog')).toHaveAttribute('data-application-id', 'app-123');
    });

    it('should close regenerate dialog when close is triggered', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={mockOAuth2Config}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      const regenerateButton = screen.getByTestId('regenerate-button');
      fireEvent.click(regenerateButton);

      expect(screen.getByTestId('regenerate-dialog')).toBeInTheDocument();

      const closeButton = screen.getByTestId('dialog-close');
      fireEvent.click(closeButton);

      expect(screen.queryByTestId('regenerate-dialog')).not.toBeInTheDocument();
    });

    it('should open secret dialog when regeneration is successful', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={mockOAuth2Config}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      const regenerateButton = screen.getByTestId('regenerate-button');
      fireEvent.click(regenerateButton);

      const successButton = screen.getByTestId('dialog-success');
      fireEvent.click(successButton);

      expect(screen.getByTestId('secret-dialog')).toBeInTheDocument();
      expect(screen.getByTestId('secret-dialog')).toHaveAttribute('data-client-secret', 'new-test-secret');
    });

    it('should close secret dialog when close is triggered', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          oauth2Config={mockOAuth2Config}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      // Open regenerate dialog and trigger success
      const regenerateButton = screen.getByTestId('regenerate-button');
      fireEvent.click(regenerateButton);

      const successButton = screen.getByTestId('dialog-success');
      fireEvent.click(successButton);

      expect(screen.getByTestId('secret-dialog')).toBeInTheDocument();

      // Close secret dialog
      const closeSecretButton = screen.getByTestId('secret-dialog-close');
      fireEvent.click(closeSecretButton);

      expect(screen.queryByTestId('secret-dialog')).not.toBeInTheDocument();
    });
  });

  describe('Delete Application Flow', () => {
    it('should open delete dialog when delete button is clicked', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      const deleteButton = screen.getByTestId('delete-button');
      fireEvent.click(deleteButton);

      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });

    it('should pass application id to delete dialog', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      const deleteButton = screen.getByTestId('delete-button');
      fireEvent.click(deleteButton);

      expect(screen.getByTestId('delete-dialog')).toHaveAttribute('data-application-id', 'app-123');
    });

    it('should close delete dialog when cancel is triggered', () => {
      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
        />,
      );

      const deleteButton = screen.getByTestId('delete-button');
      fireEvent.click(deleteButton);

      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();

      const cancelButton = screen.getByTestId('delete-dialog-close');
      fireEvent.click(cancelButton);

      expect(screen.queryByTestId('delete-dialog')).not.toBeInTheDocument();
    });

    it('should call onDeleteSuccess when delete is confirmed', () => {
      const mockOnDeleteSuccess = vi.fn();

      render(
        <EditGeneralSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          copiedField={null}
          onCopyToClipboard={mockOnCopyToClipboard}
          onDeleteSuccess={mockOnDeleteSuccess}
        />,
      );

      const deleteButton = screen.getByTestId('delete-button');
      fireEvent.click(deleteButton);

      const confirmButton = screen.getByTestId('delete-dialog-success');
      fireEvent.click(confirmButton);

      expect(mockOnDeleteSuccess).toHaveBeenCalledTimes(1);
    });
  });
});
