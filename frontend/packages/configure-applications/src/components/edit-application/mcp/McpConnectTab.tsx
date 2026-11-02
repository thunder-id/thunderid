// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Box, Chip, Link, Stack, Typography} from '@wso2/oxygen-ui';
import {Bot, FileBadge, UserRound} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import McpAccessSection from './McpAccessSection';
import type {Application} from '../../../models/application';
import type {OAuth2Config} from '../../../models/oauth';
import {TokenEndpointAuthMethods} from '../../../models/oauth';
import isJwtBearerOnlyGrantSet from '../../../utils/isJwtBearerOnlyGrantSet';
import resolveApplicationType from '../../../utils/resolveApplicationType';
import CopyableField from '../../common/CopyableField';
import CimdDocumentSection from '../cimd/CimdDocumentSection';

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

  /**
   * Navigates to the application's Credentials tab, where a confidential client's secret and keys
   * are managed. Omit when the client has no Credentials tab (e.g. a metadata-document client).
   */
  onGoToCredentials?: () => void;
}

/**
 * The Connect tab of the mcp-client template's edit page, replacing the generic
 * General tab. Composed of, top to bottom: an identity card (OAuth profile badge, Application
 * ID, Client ID, and, for confidential clients, a pointer to the Credentials tab where the client
 * secret and keys are managed) and an access card (allowed user types, client URI, and authorized
 * redirect URIs), shown only for user-delegated clients. The shared danger zone (Delete
 * Application) lives in the Advanced tab.
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
 * @param props.onGoToCredentials - Callback invoked to switch to the Credentials tab
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
  onGoToCredentials = undefined,
}: McpConnectTabProps): JSX.Element {
  const {t} = useTranslation();

  // The canonical application type (explicit type, falling back to config shape) drives the
  // client-type badge, rather than inferring M2M from grant shape.
  const resolvedType = resolveApplicationType(application.type, oauth2Config);
  const isM2m = resolvedType === 'm2m';
  // A JWT bearer-only client redeems assertions (e.g. an ID-JAG) issued for a signed-in user.
  const isJwtBearerOnly = !isM2m && isJwtBearerOnlyGrantSet(oauth2Config?.grantTypes);
  // A metadata-document client's redirect URIs come from its document rather than this tab.
  const isCimd = oauth2Config?.clientIdMetadataDocument === true;

  const isConfidentialClient =
    Boolean(oauth2Config) &&
    oauth2Config?.publicClient !== true &&
    oauth2Config?.tokenEndpointAuthMethod !== TokenEndpointAuthMethods.NONE;

  const getProfileBadgeLabel = (): string => {
    if (isM2m) {
      return t('applications:edit.mcp.connect.profileBadge.m2m', 'On its own behalf (client credentials)');
    }
    if (isJwtBearerOnly) {
      return t('applications:edit.mcp.connect.profileBadge.jwtBearer', 'On behalf of a user (JWT bearer with ID-JAG)');
    }
    return t(
      'applications:edit.mcp.connect.profileBadge.userDelegated',
      'On behalf of a user (authorization code + PKCE)',
    );
  };

  const copyLabel = t('common:actions.copy');
  const applicationIdLabel = t('applications:edit.general.labels.applicationId', 'Application ID');
  const clientIdLabel = t('applications:edit.general.labels.clientId', 'Client ID');

  return (
    <Stack spacing={3}>
      <SettingsCard
        title={t('applications:edit.mcp.connect.sections.identity', 'Connection')}
        description={t(
          'applications:edit.mcp.connect.sections.identity.description',
          'Client identity for connecting to MCP servers.',
        )}
      >
        <Stack spacing={3}>
          <Box sx={{display: 'flex', gap: 1, flexWrap: 'wrap'}}>
            {isCimd && (
              <Chip
                variant="outlined"
                color="primary"
                icon={<FileBadge size={14} />}
                label={t('applications:cimd.edit.badge', 'CIMD')}
              />
            )}
            <Chip
              variant="outlined"
              color="default"
              icon={isM2m ? <Bot size={14} /> : <UserRound size={14} />}
              label={getProfileBadgeLabel()}
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

          {isConfidentialClient && onGoToCredentials && (
            <Typography variant="body2" color="text.secondary">
              <Trans
                i18nKey="applications:edit.mcp.connect.credentialsPointer"
                defaults="Manage the client secret and keys on the <credentialsLink>Credentials tab</credentialsLink>."
                components={{
                  credentialsLink: (
                    <Link component="button" type="button" underline="always" onClick={onGoToCredentials} />
                  ),
                }}
              />
            </Typography>
          )}
        </Stack>
      </SettingsCard>

      {isCimd && oauth2Config && (
        <CimdDocumentSection
          application={application}
          oauth2Config={oauth2Config}
          onFieldChange={onFieldChange}
          isReadOnly={isReadOnly}
        />
      )}

      {!isM2m && !isCimd && (
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
  );
}
