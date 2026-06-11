/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import CertificateSection from './CertificateSection';
import MetadataSection from './MetadataSection';
import OAuth2ConfigSection from './OAuth2ConfigSection';
import type {Application} from '../../../models/application';
import type {ApplicationTemplate} from '../../../models/application-templates';
import type {InboundAuthConfig} from '../../../models/inbound-auth';
import type {OAuth2Config} from '../../../models/oauth';

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
}: EditAdvancedSettingsProps) {
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

  return (
    <Stack spacing={3}>
      <OAuth2ConfigSection
        oauth2Config={oauth2Config}
        oauth2Constraints={oauth2Constraints}
        onOAuth2ConfigChange={handleOAuth2ConfigChange}
        disabled={application.isReadOnly}
      />
      <CertificateSection
        certificate={oauth2Config?.certificate}
        onCertificateChange={handleCertificateChange}
        required={oauth2Config?.tokenEndpointAuthMethod === 'private_key_jwt'}
        disabled={application.isReadOnly}
      />
      <MetadataSection application={application} />
    </Stack>
  );
}
