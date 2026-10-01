// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Box, Button, Chip, FormControl, FormLabel, Stack, TextField} from '@wso2/oxygen-ui';
import {Bot, UserRound} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useCallback, useState} from 'react';
import {useTranslation} from 'react-i18next';
import McpAccessSection from './McpAccessSection';
import type {Application} from '../../../models/application';
import type {OAuth2Config} from '../../../models/oauth';
import {TokenEndpointAuthMethods} from '../../../models/oauth';
import resolveApplicationType from '../../../utils/resolveApplicationType';
import ClientSecretSuccessDialog from '../../ClientSecretSuccessDialog';
import CopyableField from '../../common/CopyableField';
import RegenerateSecretDialog from '../../RegenerateSecretDialog';

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
   * Callback to report whether the access section currently has validation errors
   * (feeds the Save bar). Only invoked for user-delegated clients — machine-to-machine
   * clients never render the access section.
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * The Connect tab of the mcp-client template's edit page, replacing the generic
 * General tab. Composed of, top to bottom: an identity card (OAuth profile badge, Application
 * ID, Client ID, and — for confidential clients — a client secret row with a Generate action)
 * and an access card (allowed user types, client URI, and authorized redirect URIs) — shown only
 * for user-delegated clients. The shared danger zone (Delete Application and, for flow-native
 * clients, Flow Secret regeneration) lives in the Advanced tab.
 *
 * Every edit is routed through `onFieldChange` and, for `inboundAuthConfig`, spreads the
 * existing `oauth2Config` so backend fields not modeled on the frontend survive round-trips.
 *
 * @param props - The component props
 * @param props.application - The application being edited
 * @param props.oauth2Config - OAuth2 configuration for the application
 * @param props.onFieldChange - Callback invoked when a field value changes
 * @param props.isReadOnly - Whether the application is read-only
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
  onValidationChange = undefined,
}: McpConnectTabProps): JSX.Element {
  const {t} = useTranslation();
  const [regenerateDialogOpen, setRegenerateDialogOpen] = useState(false);
  const [secretDialogOpen, setSecretDialogOpen] = useState(false);
  const [newClientSecret, setNewClientSecret] = useState<string>('');

  // The canonical application type (explicit type, falling back to config shape) drives the
  // client-type badge, rather than inferring M2M from grant shape.
  const resolvedType = resolveApplicationType(application.type, oauth2Config);
  const isM2m = resolvedType === 'm2m';

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
    </>
  );
}
