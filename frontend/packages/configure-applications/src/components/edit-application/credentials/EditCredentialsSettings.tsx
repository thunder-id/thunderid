// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {Button, FormControl, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useCallback, useState} from 'react';
import {useTranslation} from 'react-i18next';
import AttestationSection from './AttestationSection';
import CertificateSection from './CertificateSection';
import type {Application} from '../../../models/application';
import type {InboundAuthConfig} from '../../../models/inbound-auth';
import type {AttestationConfig, OAuth2Config} from '../../../models/oauth';
import {TokenEndpointAuthMethods} from '../../../models/oauth';
import resolveApplicationType, {isClientCredentialsOnlyGrantSet} from '../../../utils/resolveApplicationType';
import ClientSecretSuccessDialog from '../../ClientSecretSuccessDialog';
import CopyableField from '../../common/CopyableField';
import RegenerateFlowSecretDialog from '../../RegenerateFlowSecretDialog';
import RegenerateSecretDialog from '../../RegenerateSecretDialog';

type OAuthCertificate = {type: string; value?: string} | null;

/**
 * Props for the {@link EditCredentialsSettings} component.
 */
interface EditCredentialsSettingsProps {
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
   * Callback function to handle field value changes
   * @param field - The application field being updated
   * @param value - The new value for the field
   */
  onFieldChange: (field: keyof Application, value: unknown) => void;
  /**
   * Whether the platform attestation section is shown. Driven by the template's `attestation`
   * capability, so it appears only for templates that support it (e.g. mobile).
   */
  showAttestation?: boolean;
  /**
   * Callback to report whether the platform attestation section currently has validation errors
   * (feeds the page's Save bar).
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * Container component for the Credentials tab of the application edit page.
 *
 * Displays sections for:
 * - Identifier (Client ID for redirect-based apps, Application ID for app-native/mobile apps)
 * - Secret (Client Secret for confidential redirect-based apps, Flow Secret for flow-native apps)
 * - Certificate configuration (JWKS/JWKS URI)
 * - Platform attestation (for templates that support it, e.g. mobile)
 *
 * @param props - Component props
 * @returns Credentials settings sections
 */
export default function EditCredentialsSettings({
  application,
  editedApp,
  oauth2Config = undefined,
  onFieldChange,
  showAttestation = false,
  onValidationChange = undefined,
}: EditCredentialsSettingsProps) {
  const {config} = useConfig();
  const {t} = useTranslation();

  const [regenerateDialogOpen, setRegenerateDialogOpen] = useState(false);
  const [secretDialogOpen, setSecretDialogOpen] = useState(false);
  const [newClientSecret, setNewClientSecret] = useState<string>('');
  const [regenerateFlowSecretDialogOpen, setRegenerateFlowSecretDialogOpen] = useState(false);
  const [flowSecretDialogOpen, setFlowSecretDialogOpen] = useState(false);
  const [newFlowSecret, setNewFlowSecret] = useState<string>('');

  const systemConsoleClientId = (config?.client?.client_id ?? 'CONSOLE').toUpperCase();
  const isSystemConsoleClient = oauth2Config?.clientId?.toUpperCase() === systemConsoleClientId;

  // App-native (mobile/attestation) clients authenticate via platform attestation rather than a
  // redirect-issued client, so the Application ID is the more meaningful identifier for them.
  const resolvedType = resolveApplicationType(application.type, oauth2Config);
  const isMobile = resolvedType === 'mobile';

  const isConfidentialClient =
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_BASIC ||
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_POST;

  // Only flow-native apps are issued a Flow Secret and can rotate it: full-stack, custom, or mcp
  // apps using the embedded (non-redirect) sign-in option. Browser (public redirect), mobile
  // (attestation), and m2m (direct token, including client_credentials-only mcp configs) apps never
  // hold one. The canonical application type is the discriminator, falling back to the OAuth config
  // shape for legacy/custom apps.
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

  const handleOAuth2ConfigChange = (updates: Partial<OAuth2Config>) => {
    const currentInboundAuth: InboundAuthConfig[] = editedApp.inboundAuthConfig ?? application.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: {...auth.config, ...updates}} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleCertificateChange = (cert: OAuthCertificate) => {
    handleOAuth2ConfigChange({certificate: cert});
  };

  // Encrypted ID token / UserInfo formats are encrypted to the client certificate, so removing the
  // certificate while one is selected would produce an invalid config. Used to block that removal.
  const idTokenResponseType = oauth2Config?.token?.idToken?.responseType;
  const userInfoResponseType = oauth2Config?.userInfo?.responseType;
  const encryptionDependsOnCert =
    idTokenResponseType === 'JWE' ||
    idTokenResponseType === 'NESTED_JWT' ||
    userInfoResponseType === 'JWE' ||
    userInfoResponseType === 'NESTED_JWT';

  // Attestation is a client-level (protocol-agnostic) setting, so it is stored at the top level of
  // the application rather than nested under the OAuth2 config. This lets any application type —
  // including embedded apps with no OAuth2 config — enable it.
  const handleAttestationChange = (attestation: AttestationConfig | null) => {
    onFieldChange('attestation', attestation);
  };

  // Prefer the edited value whenever it has been set — including an explicit null, which represents
  // the user clearing attestation. Only fall back to the stored value when the field is untouched.
  const currentAttestation = 'attestation' in editedApp ? editedApp.attestation : application.attestation;

  const applicationIdLabel = t('applications:edit.general.labels.applicationId', 'Application ID');
  const clientIdLabel = t('applications:edit.general.labels.clientId', 'Client ID');
  const applicationIdHint = t(
    'applications:edit.general.hints.applicationId',
    "ThunderID's internal identifier for this application. Use it when calling the Management API.",
  );
  const clientIdHint = t(
    'applications:edit.general.hints.clientId',
    'The public OAuth2 client identifier this application uses to authenticate as a client.',
  );
  const copyLabel = t('common:actions.copy');

  // Redirect-based apps are identified by their OAuth Client ID; app-native (mobile) apps and
  // apps with no OAuth profile at all fall back to the Application ID.
  const useClientIdAsIdentifier = !isMobile && Boolean(oauth2Config?.clientId);
  const identifierLabel = useClientIdAsIdentifier ? clientIdLabel : applicationIdLabel;
  const identifierHint = useClientIdAsIdentifier ? clientIdHint : applicationIdHint;
  const identifierValue = useClientIdAsIdentifier && oauth2Config?.clientId ? oauth2Config.clientId : application.id;

  const showSecretSection = isConfidentialClient || isFlowNativeClient;
  const secretLabel = isConfidentialClient
    ? t('applications:clientSecret.clientSecretLabel', 'Client Secret')
    : t('applications:edit.credentials.sections.secret.flowSecretLabel', 'Flow Secret');
  const secretHint = isConfidentialClient
    ? t(
        'applications:edit.credentials.sections.secret.hints.clientSecret',
        'A confidential credential used with the Client ID to authenticate this application. Keep it secret.',
      )
    : t(
        'applications:edit.credentials.sections.secret.hints.flowSecret',
        "A confidential credential used to authenticate this application's embedded sign-in flow. Keep it secret.",
      );
  const handleSecretRegenerateClick = isConfidentialClient ? handleRegenerateClick : handleRegenerateFlowSecretClick;

  return (
    <>
      <Stack spacing={3}>
        <SettingsCard
          title={t('applications:edit.credentials.sections.identifier.title', 'Identifier')}
          description={t(
            'applications:edit.credentials.sections.identifier.description',
            'Unique identifier used to reference this application.',
          )}
        >
          <CopyableField
            id="app-credentials-identifier"
            label={identifierLabel}
            value={identifierValue}
            copyAriaLabel={`${copyLabel} ${identifierLabel}`}
            hint={identifierHint}
          />
        </SettingsCard>

        {showSecretSection && (
          <SettingsCard
            title={t('applications:edit.credentials.sections.secret.title', 'Secret')}
            description={t(
              'applications:edit.credentials.sections.secret.description',
              'Regenerating the secret immediately invalidates the current one and cannot be undone.',
            )}
          >
            <FormControl fullWidth>
              <FormLabel htmlFor="app-credentials-secret">{secretLabel}</FormLabel>
              <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
                {secretHint}
              </Typography>
              <Stack direction="row" spacing={1}>
                <TextField
                  fullWidth
                  id="app-credentials-secret"
                  value="••••••••••••••••"
                  InputProps={{readOnly: true}}
                  disabled
                  sx={{flex: '0 0 80%', '& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                />
                <Button
                  variant="contained"
                  color="error"
                  onClick={handleSecretRegenerateClick}
                  disabled={Boolean(application.isReadOnly) || isSystemConsoleClient}
                  sx={{flex: '0 0 20%'}}
                >
                  {t('applications:edit.general.sections.dangerZone.regenerateSecret.button', 'Regenerate')}
                </Button>
              </Stack>
            </FormControl>
          </SettingsCard>
        )}

        <CertificateSection
          certificate={oauth2Config?.certificate}
          onCertificateChange={handleCertificateChange}
          required={oauth2Config?.tokenEndpointAuthMethod === 'private_key_jwt'}
          encryptionDependsOnCert={encryptionDependsOnCert}
          disabled={application.isReadOnly}
        />

        {showAttestation && (
          <AttestationSection
            attestation={currentAttestation}
            onAttestationChange={handleAttestationChange}
            disabled={application.isReadOnly}
            onValidationChange={onValidationChange}
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
    </>
  );
}
