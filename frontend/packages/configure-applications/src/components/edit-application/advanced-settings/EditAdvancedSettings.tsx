// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {Stack} from '@wso2/oxygen-ui';
import {useEffect, useState} from 'react';
import AudienceSection from './AudienceSection';
import IdentityAssertionsSection from './IdentityAssertionsSection';
import MetadataSection from './MetadataSection';
import OAuth2ConfigSection from './OAuth2ConfigSection';
import PasskeysSection from './PasskeysSection';
import type {ApplicationTemplate} from '../../../models/application-templates';
import ApplicationDeleteDialog from '../../ApplicationDeleteDialog';
import DangerZoneSection from '../general-settings/DangerZoneSection';
import type {Application, InboundAuthConfig, OAuth2Config, OAuth2Token} from '@thunderid/configure-applications';

/**
 * Props for the {@link EditAdvancedSettings} component.
 */
interface EditAdvancedSettingsProps {
  /**
   * The application being edited
   */
  application: Application;
  /**
   * Partial application object containing edited fields
   */
  editedApp: Partial<Application>;
  /**
   * OAuth2 configuration for the application (optional)
   */
  oauth2Config?: OAuth2Config;
  /**
   * Template-driven field constraints for OAuth2 fields (optional)
   */
  oauth2Constraints?: NonNullable<ApplicationTemplate['fieldConstraints']>['oauth2'];
  /**
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;
  /**
   * When set, restricts the offered grant types to the intersection of this list and
   * discovery's `grant_types_supported`. Omit to offer every discovery-advertised grant type.
   */
  allowedGrantTypes?: string[];
  /**
   * Whether to show the redirect URI and post-logout redirect URI fields on the OAuth2
   * Configuration section. Hidden for clients with no user-facing grant, and for MCP clients,
   * which manage their redirect URIs on their own Connect tab.
   */
  showRedirectUris?: boolean;
  /**
   * Bumped by the parent on Save/Reset to force the OAuth 2 Configuration section to remount and
   * drop its local redirect URI list state.
   */
  sectionResetKey?: number;
  /**
   * Callback to report whether any child section (identity assertions / ID-JAG) currently has
   * validation errors (feeds the page's Save bar).
   */
  onValidationChange?: (hasErrors: boolean) => void;
  /**
   * Callback invoked after the application is successfully deleted from the Danger Zone.
   */
  onDeleteSuccess?: () => void;
}

/**
 * Container component for advanced application settings.
 *
 * Displays sections for:
 * - OAuth2 configuration (grant types, response types, PKCE, public client)
 * - Application metadata (created/updated timestamps)
 *
 * @param props - Component props
 * @returns Advanced settings sections wrapped in a Stack
 */
export default function EditAdvancedSettings({
  application,
  editedApp,
  oauth2Config = undefined,
  oauth2Constraints = undefined,
  onFieldChange,
  allowedGrantTypes = undefined,
  showRedirectUris = true,
  sectionResetKey = 0,
  onValidationChange = undefined,
  onDeleteSuccess = undefined,
}: EditAdvancedSettingsProps) {
  const {config} = useConfig();

  // Redirect URIs, identity assertions, and passkeys validate independently; each is tracked
  // separately so one resolving doesn't clobber the other's still-invalid state when both report
  // to the single upward onValidationChange prop.
  const [redirectUrisInvalid, setRedirectUrisInvalid] = useState(false);
  const [identityAssertionsInvalid, setIdentityAssertionsInvalid] = useState(false);
  const [passkeysInvalid, setPasskeysInvalid] = useState(false);

  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  useEffect(() => {
    onValidationChange?.(redirectUrisInvalid || identityAssertionsInvalid || passkeysInvalid);
  }, [redirectUrisInvalid, identityAssertionsInvalid, passkeysInvalid, onValidationChange]);

  const systemConsoleClientId = (config?.client?.client_id ?? 'CONSOLE').toUpperCase();

  const handleOAuth2ConfigChange = (updates: Partial<OAuth2Config>) => {
    const currentInboundAuth: InboundAuthConfig[] = editedApp.inboundAuthConfig ?? application.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: {...auth.config, ...updates}} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handlePasskeysChange = (origins: string[]): void => {
    onFieldChange('passkeyAllowedOrigins', origins);
  };

  const currentPasskeyOrigins = editedApp.passkeyAllowedOrigins ?? application.passkeyAllowedOrigins ?? [];

  const handleTokenConfigChange = (tokenUpdates: Partial<OAuth2Token>, oauth2Updates: Partial<OAuth2Config> = {}) => {
    const currentInboundAuth: InboundAuthConfig[] = editedApp.inboundAuthConfig ?? application.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2'
        ? {
            ...auth,
            config: {
              ...auth.config,
              ...oauth2Updates,
              token: {...auth.config?.token, ...tokenUpdates},
            },
          }
        : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleDefaultAudienceChange = (audience: string) => {
    handleTokenConfigChange({
      accessToken: {...oauth2Config?.token?.accessToken, defaultAudience: audience},
    });
  };

  return (
    <>
      <Stack spacing={3}>
        <OAuth2ConfigSection
          key={sectionResetKey}
          oauth2Config={oauth2Config}
          oauth2Constraints={oauth2Constraints}
          onOAuth2ConfigChange={handleOAuth2ConfigChange}
          disabled={application.isReadOnly}
          allowedGrantTypes={allowedGrantTypes}
          showRedirectUris={showRedirectUris}
          onValidationChange={setRedirectUrisInvalid}
        />
        {oauth2Config && (
          <IdentityAssertionsSection
            oauth2Config={oauth2Config}
            onTokenConfigChange={handleTokenConfigChange}
            disabled={application.isReadOnly}
            onValidationChange={setIdentityAssertionsInvalid}
          />
        )}
        {oauth2Config && (
          <AudienceSection
            audience={oauth2Config.token?.accessToken?.defaultAudience ?? ''}
            onAudienceChange={handleDefaultAudienceChange}
            disabled={application.isReadOnly}
          />
        )}
        <PasskeysSection
          allowedOrigins={currentPasskeyOrigins}
          onPasskeysChange={application.isReadOnly ? undefined : handlePasskeysChange}
          disabled={application.isReadOnly}
          onValidationChange={setPasskeysInvalid}
        />
        <MetadataSection application={application} />
        {!application.isReadOnly && oauth2Config?.clientId?.toUpperCase() !== systemConsoleClientId && (
          <DangerZoneSection onDeleteClick={() => setDeleteDialogOpen(true)} />
        )}
      </Stack>

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
