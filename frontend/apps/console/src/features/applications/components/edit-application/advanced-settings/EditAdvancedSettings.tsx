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

import {Stack} from '@wso2/oxygen-ui';
import {useEffect, useState} from 'react';
import AttestationSection from './AttestationSection';
import CertificateSection from './CertificateSection';
import IdentityAssertionsSection from './IdentityAssertionsSection';
import MetadataSection from './MetadataSection';
import OAuth2ConfigSection from './OAuth2ConfigSection';
import type {Application} from '../../../models/application';
import type {ApplicationTemplate} from '../../../models/application-templates';
import type {InboundAuthConfig} from '../../../models/inbound-auth';
import type {AttestationConfig, OAuth2Config, OAuth2Token} from '../../../models/oauth';

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
   * Whether the platform attestation section is shown. Driven by the template's `attestation`
   * capability, so it appears only for templates that support it (e.g. mobile).
   */
  showAttestation?: boolean;
  /**
   * Callback to report whether any child section (identity assertions / ID-JAG, or platform
   * attestation) currently has validation errors (feeds the page's Save bar).
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

type OAuthCertificate = {type: string; value?: string} | null;

/**
 * Container component for advanced application settings.
 *
 * Displays sections for:
 * - OAuth2 configuration (grant types, response types, PKCE, public client)
 * - Certificate configuration (JWKS/JWKS URI)
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
  showAttestation = false,
  onValidationChange = undefined,
}: EditAdvancedSettingsProps) {
  // Identity assertions and attestation validate independently; each is tracked separately so one
  // resolving doesn't clobber the other's still-invalid state when both report to the single
  // upward onValidationChange prop.
  const [identityAssertionsInvalid, setIdentityAssertionsInvalid] = useState(false);
  const [attestationInvalid, setAttestationInvalid] = useState(false);

  useEffect(() => {
    onValidationChange?.(identityAssertionsInvalid || attestationInvalid);
  }, [identityAssertionsInvalid, attestationInvalid, onValidationChange]);

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

  // Attestation is a client-level (protocol-agnostic) setting, so it is stored at the top level of
  // the application rather than nested under the OAuth2 config. This lets any application type —
  // including embedded apps with no OAuth2 config — enable it.
  const handleAttestationChange = (attestation: AttestationConfig | null) => {
    onFieldChange('attestation', attestation);
  };

  // Prefer the edited value whenever it has been set — including an explicit null, which represents
  // the user clearing attestation. Only fall back to the stored value when the field is untouched.
  const currentAttestation = 'attestation' in editedApp ? editedApp.attestation : application.attestation;

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

  return (
    <Stack spacing={3}>
      <OAuth2ConfigSection
        oauth2Config={oauth2Config}
        oauth2Constraints={oauth2Constraints}
        onOAuth2ConfigChange={handleOAuth2ConfigChange}
        disabled={application.isReadOnly}
        allowedGrantTypes={allowedGrantTypes}
      />
      {oauth2Config && (
        <IdentityAssertionsSection
          oauth2Config={oauth2Config}
          onTokenConfigChange={handleTokenConfigChange}
          disabled={application.isReadOnly}
          onValidationChange={setIdentityAssertionsInvalid}
        />
      )}
      <CertificateSection
        certificate={oauth2Config?.certificate}
        onCertificateChange={handleCertificateChange}
        required={oauth2Config?.tokenEndpointAuthMethod === 'private_key_jwt'}
        disabled={application.isReadOnly}
      />
      {showAttestation && (
        <AttestationSection
          attestation={currentAttestation}
          onAttestationChange={handleAttestationChange}
          disabled={application.isReadOnly}
          onValidationChange={setAttestationInvalid}
        />
      )}
      <MetadataSection application={application} />
    </Stack>
  );
}
