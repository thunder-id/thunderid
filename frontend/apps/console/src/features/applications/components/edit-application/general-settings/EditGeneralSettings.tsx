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

import {TokenEndpointAuthMethods} from '@thunderid/configure-applications';
import type {Application, OAuth2Config} from '@thunderid/configure-applications';
import {useConfig} from '@thunderid/contexts';
import {Stack} from '@wso2/oxygen-ui';
import {useState, useCallback} from 'react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AccessSection from './AccessSection';
import DangerZoneSection from './DangerZoneSection';
import QuickCopySection from './QuickCopySection';
import resolveApplicationType, {isClientCredentialsOnlyGrantSet} from '../../../utils/resolveApplicationType';
import ApplicationDeleteDialog from '../../ApplicationDeleteDialog';
import ClientSecretSuccessDialog from '../../ClientSecretSuccessDialog';
import RegenerateFlowSecretDialog from '../../RegenerateFlowSecretDialog';
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
   * Bumped by the parent on Save/Reset to force AccessSection to remount and drop its local
   * redirect URI list state.
   */
  sectionResetKey?: number;
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
  /**
   * Callback function to handle validation changes
   * @param hasErrors - Boolean indicating if the general settings have validation errors
   */
  onValidationChange?: (hasErrors: boolean) => void;
  /**
   * Whether to show user-facing access config (allowed user types, redirect URIs). Hidden for
   * clients with no user-facing grant.
   */
  showUserAccessConfig?: boolean;
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
  sectionResetKey = 0,
  copiedField,
  onCopyToClipboard,
  onDeleteSuccess = undefined,
  onValidationChange = undefined,
  showUserAccessConfig = true,
}: EditGeneralSettingsProps): JSX.Element {
  const {config} = useConfig();
  const {t} = useTranslation();
  const [regenerateDialogOpen, setRegenerateDialogOpen] = useState(false);
  const [secretDialogOpen, setSecretDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [newClientSecret, setNewClientSecret] = useState<string>('');
  const [regenerateFlowSecretDialogOpen, setRegenerateFlowSecretDialogOpen] = useState(false);
  const [flowSecretDialogOpen, setFlowSecretDialogOpen] = useState(false);
  const [newFlowSecret, setNewFlowSecret] = useState<string>('');
  const systemConsoleClientId = (config?.client?.client_id ?? 'CONSOLE').toUpperCase();

  const isConfidentialClient =
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_BASIC ||
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_POST;

  // Only flow-native apps are issued a Flow Secret and can rotate it: full-stack, custom, or mcp
  // apps using the embedded (non-redirect) sign-in option. Browser (public redirect), mobile
  // (attestation), and m2m (direct token, including client_credentials-only mcp configs) apps never
  // hold one. The canonical application type is the discriminator, falling back to the OAuth config
  // shape for legacy/custom apps.
  const resolvedType = resolveApplicationType(application.type, oauth2Config);
  const grantTypes = oauth2Config?.grantTypes ?? [];
  const isFlowNativeClient =
    (resolvedType === 'fullstack' || resolvedType === 'custom' || resolvedType === 'mcp') &&
    (!oauth2Config ||
      (!oauth2Config.publicClient &&
        !grantTypes.includes('authorization_code') &&
        !isClientCredentialsOnlyGrantSet(grantTypes)));

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

  const handleRegenerateFlowSecretClick = useCallback((): void => {
    setRegenerateFlowSecretDialogOpen(true);
  }, []);

  const handleRegenerateFlowSecretSuccess = useCallback((flowSecret: string): void => {
    setNewFlowSecret(flowSecret);
    setFlowSecretDialogOpen(true);
  }, []);

  const handleFlowSecretDialogClose = useCallback((): void => {
    setFlowSecretDialogOpen(false);
    setNewFlowSecret('');
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
          key={sectionResetKey}
          application={application}
          editedApp={editedApp}
          oauth2Config={oauth2Config}
          onFieldChange={onFieldChange}
          onValidationChange={onValidationChange}
          showUserAccessConfig={showUserAccessConfig}
        />
        {!application.isReadOnly && oauth2Config?.clientId?.toUpperCase() !== systemConsoleClientId && (
          <DangerZoneSection
            showRegenerateSecret={isConfidentialClient}
            onRegenerateClick={handleRegenerateClick}
            showRegenerateFlowSecret={isFlowNativeClient}
            onRegenerateFlowSecretClick={handleRegenerateFlowSecretClick}
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

      {/* Regenerate Flow Secret Confirmation Dialog */}
      <RegenerateFlowSecretDialog
        open={regenerateFlowSecretDialogOpen}
        applicationId={application.id}
        onClose={() => setRegenerateFlowSecretDialogOpen(false)}
        onSuccess={handleRegenerateFlowSecretSuccess}
      />

      {/* New Flow Secret Success Dialog */}
      <ClientSecretSuccessDialog
        open={flowSecretDialogOpen}
        clientSecret={newFlowSecret}
        title={t('applications:regenerateFlowSecret.success.title')}
        subtitle={t('applications:regenerateFlowSecret.success.subtitle')}
        secretLabel={t('applications:regenerateFlowSecret.success.secretLabel')}
        copySecretLabel={t('applications:regenerateFlowSecret.success.copySecret')}
        securityReminderTitle={t('applications:regenerateFlowSecret.success.securityReminder.title')}
        securityReminderDescription={t('applications:regenerateFlowSecret.success.securityReminder.description')}
        onClose={handleFlowSecretDialogClose}
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
