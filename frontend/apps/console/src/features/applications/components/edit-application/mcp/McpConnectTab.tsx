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

import {SettingsCard} from '@thunderid/components';
import {Box, Button, Chip, FormControl, FormLabel, Stack, TextField} from '@wso2/oxygen-ui';
import {Bot, UserRound} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useCallback, useState} from 'react';
import {useTranslation} from 'react-i18next';
import McpAccessSection from './McpAccessSection';
import type {Application} from '../../../models/application';
import {McpClientTypes} from '../../../models/mcp-client';
import {TokenEndpointAuthMethods} from '../../../models/oauth';
import type {OAuth2Config} from '../../../models/oauth';
import deriveMcpClientType from '../../../utils/deriveMcpClientType';
import ApplicationDeleteDialog from '../../ApplicationDeleteDialog';
import ClientSecretSuccessDialog from '../../ClientSecretSuccessDialog';
import CopyableField from '../../common/CopyableField';
import RegenerateFlowSecretDialog from '../../RegenerateFlowSecretDialog';
import RegenerateSecretDialog from '../../RegenerateSecretDialog';
import DangerZoneSection from '../general-settings/DangerZoneSection';

/**
 * Props for the {@link McpConnectTab} component.
 *
 * @public
 */
export interface McpConnectTabProps {
  /**
   * The application being edited
   */
  application: Application;

  /**
   * OAuth2 configuration for the application (optional)
   */
  oauth2Config?: OAuth2Config;

  /**
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;

  /**
   * Bumped by the parent on Save/Reset to force McpAccessSection to remount and drop its local
   * redirect URI list state.
   */
  sectionResetKey?: number;

  /**
   * Whether the application is read-only, disabling all inputs and actions
   */
  isReadOnly: boolean;

  /**
   * Callback invoked after the application is successfully deleted
   */
  onDeleteSuccess?: () => void;

  /**
   * Callback to report whether the access section currently has validation errors
   * (feeds the Save bar). Only invoked for user-delegated clients — machine-to-machine
   * clients never render the access section.
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * The Connect tab of the mcp-client template's edit page, replacing the generic
 * General tab. Composed of, top to bottom: an identity card (OAuth profile badge, Application
 * ID, Client ID, and — for confidential clients — a client secret row with a Generate action),
 * an access card (allowed user types, client URI, and authorized redirect URIs) — shown only
 * for user-delegated clients — and the shared danger zone (Delete Application and, for
 * flow-native clients, Flow Secret regeneration).
 *
 * Every edit is routed through `onFieldChange` and, for `inboundAuthConfig`, spreads the
 * existing `oauth2Config` so backend fields not modeled on the frontend survive round-trips.
 *
 * @param props - The component props
 * @param props.application - The application being edited
 * @param props.oauth2Config - OAuth2 configuration for the application
 * @param props.onFieldChange - Callback invoked when a field value changes
 * @param props.isReadOnly - Whether the application is read-only
 * @param props.onDeleteSuccess - Callback invoked after the application is successfully deleted
 * @param props.onValidationChange - Callback invoked with whether the access section has validation errors
 *
 * @returns JSX element displaying the Connect tab
 *
 * @example
 * ```tsx
 * <McpConnectTab
 *   application={application}
 *   oauth2Config={oauth2Config}
 *   onFieldChange={handleFieldChange}
 *   isReadOnly={application.isReadOnly === true}
 * />
 * ```
 *
 * @public
 */
export default function McpConnectTab({
  application,
  oauth2Config = undefined,
  onFieldChange,
  sectionResetKey = 0,
  isReadOnly,
  onDeleteSuccess = undefined,
  onValidationChange = undefined,
}: McpConnectTabProps): JSX.Element {
  const {t} = useTranslation();
  const [regenerateDialogOpen, setRegenerateDialogOpen] = useState(false);
  const [secretDialogOpen, setSecretDialogOpen] = useState(false);
  const [newClientSecret, setNewClientSecret] = useState<string>('');
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [regenerateFlowSecretDialogOpen, setRegenerateFlowSecretDialogOpen] = useState(false);
  const [flowSecretDialogOpen, setFlowSecretDialogOpen] = useState(false);
  const [newFlowSecret, setNewFlowSecret] = useState<string>('');

  const clientType = deriveMcpClientType(oauth2Config?.grantTypes);
  const isM2m = clientType === McpClientTypes.M2M;

  const isConfidentialClient =
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_BASIC ||
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_POST;

  // Only flow-native apps are issued a Flow Secret and can rotate it: embedded apps with no OAuth
  // profile, or confidential non-redirect apps. Public, redirect (authorization_code), and
  // machine-to-machine (client_credentials as the only grant) apps get no Flow Secret.
  const grantTypes = oauth2Config?.grantTypes ?? [];
  const isM2MClient = grantTypes.length === 1 && grantTypes[0] === 'client_credentials';
  const isFlowNativeClient =
    !oauth2Config || (!oauth2Config.publicClient && !grantTypes.includes('authorization_code') && !isM2MClient);

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

  const copyLabel = t('common:actions.copy');
  const applicationIdLabel = t('applications:edit.general.labels.applicationId', 'Application ID');
  const clientIdLabel = t('applications:edit.general.labels.clientId', 'Client ID');

  return (
    <>
      <Stack spacing={3}>
        <SettingsCard
          title={t('applications:edit.mcp.connect.sections.identity', 'Connection')}
          description={t(
            'applications:edit.mcp.connect.sections.identity.description',
            'Client identity and credentials for connecting to MCP servers.',
          )}
        >
          <Stack spacing={3}>
            <Box>
              <Chip
                variant="outlined"
                color="default"
                icon={isM2m ? <Bot size={14} /> : <UserRound size={14} />}
                label={
                  isM2m
                    ? t('applications:edit.mcp.connect.profileBadge.m2m', 'On its own behalf (Client Credentials)')
                    : t(
                        'applications:edit.mcp.connect.profileBadge.userDelegated',
                        'On behalf of a user (Authorization Code + PKCE)',
                      )
                }
              />
            </Box>

            <CopyableField
              id="mcp-connect-application-id"
              label={applicationIdLabel}
              value={application.id}
              copyAriaLabel={`${copyLabel} ${applicationIdLabel}`}
            />

            {oauth2Config?.clientId && (
              <CopyableField
                id="mcp-connect-client-id"
                label={clientIdLabel}
                value={oauth2Config.clientId}
                copyAriaLabel={`${copyLabel} ${clientIdLabel}`}
              />
            )}

            {isConfidentialClient && (
              <FormControl fullWidth>
                <FormLabel htmlFor="mcp-connect-client-secret">
                  {t('applications:clientSecret.clientSecretLabel', 'Client Secret')}
                </FormLabel>
                <Stack direction="row" spacing={1}>
                  <TextField
                    fullWidth
                    id="mcp-connect-client-secret"
                    value="••••••••••••••••"
                    InputProps={{readOnly: true}}
                    disabled
                    sx={{flex: '0 0 80%', '& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                  />
                  <Button
                    variant="contained"
                    color="error"
                    onClick={handleRegenerateClick}
                    disabled={isReadOnly}
                    sx={{flex: '0 0 20%'}}
                  >
                    {t('applications:edit.mcp.connect.generateSecret', 'Generate')}
                  </Button>
                </Stack>
              </FormControl>
            )}
          </Stack>
        </SettingsCard>

        {!isM2m && (
          <McpAccessSection
            key={sectionResetKey}
            application={application}
            oauth2Config={oauth2Config}
            onFieldChange={onFieldChange}
            isReadOnly={isReadOnly}
            onValidationChange={onValidationChange}
          />
        )}

        {!isReadOnly && (
          <DangerZoneSection
            showRegenerateFlowSecret={isFlowNativeClient}
            onRegenerateFlowSecretClick={handleRegenerateFlowSecretClick}
            onDeleteClick={() => setDeleteDialogOpen(true)}
          />
        )}
      </Stack>

      <RegenerateSecretDialog
        open={regenerateDialogOpen}
        applicationId={application.id}
        onClose={() => setRegenerateDialogOpen(false)}
        onSuccess={handleRegenerateSuccess}
      />

      <ClientSecretSuccessDialog
        open={secretDialogOpen}
        clientSecret={newClientSecret}
        onClose={handleSecretDialogClose}
      />

      <RegenerateFlowSecretDialog
        open={regenerateFlowSecretDialogOpen}
        applicationId={application.id}
        onClose={() => setRegenerateFlowSecretDialogOpen(false)}
        onSuccess={handleRegenerateFlowSecretSuccess}
      />

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

      <ApplicationDeleteDialog
        open={deleteDialogOpen}
        applicationId={application.id}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={onDeleteSuccess}
      />
    </>
  );
}
