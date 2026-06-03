/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {useConfig} from '@thunderid/contexts';
import {Stack} from '@wso2/oxygen-ui';
import {useState, useCallback} from 'react';
import type {JSX} from 'react';
import AccessSection from './AccessSection';
import DangerZoneSection from './DangerZoneSection';
import QuickCopySection from './QuickCopySection';
import type {Application} from '../../../models/application';
import {TokenEndpointAuthMethods} from '../../../models/oauth';
import type {OAuth2Config} from '../../../models/oauth';
import ApplicationDeleteDialog from '../../ApplicationDeleteDialog';
import ClientSecretSuccessDialog from '../../ClientSecretSuccessDialog';
import RegenerateSecretDialog from '../../RegenerateSecretDialog';

/**
 * Props for the {@link EditGeneralSettings} component.
 */
interface EditGeneralSettingsProps {
  /**
   * The application being edited
   */
  application: Application;
  /**
   * Partial application object containing edited fields
   */
  editedApp: Partial<Application>;
  /**
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;
  /**
   * OAuth2 configuration for the application (optional)
   */
  oauth2Config?: OAuth2Config;
  /**
   * The name of the field that was recently copied to clipboard
   */
  copiedField: string | null;
  /**
   * Callback function to copy text to clipboard
   * @param text - The text to copy
   * @param fieldName - The name of the field being copied
   */
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
  /**
   * Callback invoked after the application is successfully deleted
   */
  onDeleteSuccess?: () => void;
}

/**
 * Container component for general application settings.
 *
 * Displays sections for:
 * - Quick copy of application credentials (ID, Client ID)
 * - Access configuration (URL, redirect URIs, allowed user types)
 * - Danger zone (regenerate client secret)
 *
 * @param props - Component props
 * @returns General settings sections wrapped in a Stack
 */
export default function EditGeneralSettings({
  application,
  editedApp,
  onFieldChange,
  oauth2Config = undefined,
  copiedField,
  onCopyToClipboard,
  onDeleteSuccess = undefined,
}: EditGeneralSettingsProps): JSX.Element {
  const {config} = useConfig();
  const [regenerateDialogOpen, setRegenerateDialogOpen] = useState(false);
  const [secretDialogOpen, setSecretDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [newClientSecret, setNewClientSecret] = useState<string>('');
  const systemConsoleClientId = (config?.client?.client_id ?? 'CONSOLE').toUpperCase();

  const isConfidentialClient =
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_BASIC ||
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_POST;

  const handleRegenerateClick = useCallback((): void => {
    setRegenerateDialogOpen(true);
  }, []);

  const handleRegenerateSuccess = useCallback((clientSecret: string): void => {
    setNewClientSecret(clientSecret);
    setSecretDialogOpen(true);
  }, []);

  const handleSecretDialogClose = useCallback((): void => {
    setSecretDialogOpen(false);
    setNewClientSecret('');
  }, []);

  return (
    <>
      <Stack spacing={3}>
        <QuickCopySection
          application={application}
          oauth2Config={oauth2Config}
          copiedField={copiedField}
          onCopyToClipboard={onCopyToClipboard}
        />
        <AccessSection
          application={application}
          editedApp={editedApp}
          oauth2Config={oauth2Config}
          onFieldChange={onFieldChange}
        />
        {!application.isReadOnly && oauth2Config?.clientId?.toUpperCase() !== systemConsoleClientId && (
          <DangerZoneSection
            showRegenerateSecret={isConfidentialClient}
            onRegenerateClick={handleRegenerateClick}
            onDeleteClick={() => setDeleteDialogOpen(true)}
          />
        )}
      </Stack>

      {/* Regenerate Client Secret Confirmation Dialog */}
      <RegenerateSecretDialog
        open={regenerateDialogOpen}
        applicationId={application.id}
        onClose={() => setRegenerateDialogOpen(false)}
        onSuccess={handleRegenerateSuccess}
      />

      {/* New Client Secret Success Dialog */}
      <ClientSecretSuccessDialog
        open={secretDialogOpen}
        clientSecret={newClientSecret}
        onClose={handleSecretDialogClose}
      />

      {/* Delete Application Confirmation Dialog */}
      <ApplicationDeleteDialog
        open={deleteDialogOpen}
        applicationId={application.id}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={onDeleteSuccess}
      />
    </>
  );
}
