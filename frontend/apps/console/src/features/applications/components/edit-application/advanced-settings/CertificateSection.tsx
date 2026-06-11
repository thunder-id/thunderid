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

import {SettingsCard} from '@thunderid/components';
import {Stack, TextField, FormControl, FormLabel, Autocomplete, FormHelperText} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import CertificateTypes from '../../../constants/certificate-types';

/**
 * Props for the {@link CertificateSection} component.
 */
interface CertificateSectionProps {
  /**
   * The current certificate value (from the OAuth config).
   * null or undefined means no certificate is configured.
   */
  certificate?: {type?: string; value?: string} | null;
  /**
   * Called when the user changes the certificate type or value.
   * Passes null when the user selects "None".
   */
  onCertificateChange: (cert: {type: string; value: string} | null) => void;
  /**
   * When true, shows an inline error if no certificate is configured.
   * Use when tokenEndpointAuthMethod is private_key_jwt.
   */
  required?: boolean;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
}

/**
 * Section component for configuring application certificates.
 *
 * Allows selection of certificate type:
 * - None: No certificate configured
 * - JWKS: JSON Web Key Set as inline JSON
 * - JWKS URI: URL to fetch JWKS from
 *
 * When JWKS or JWKS URI is selected, displays a text field for entering the value.
 *
 * @param props - Component props
 * @returns Certificate configuration UI within a SettingsCard
 */
export default function CertificateSection({
  certificate = undefined,
  onCertificateChange,
  required = false,
  disabled = false,
}: CertificateSectionProps) {
  const {t} = useTranslation();

  const certificateTypeOptions = [
    {value: CertificateTypes.NONE, label: t('applications:edit.advanced.certificate.type.none')},
    {value: CertificateTypes.JWKS, label: t('applications:edit.advanced.certificate.type.jwks')},
    {value: CertificateTypes.JWKS_URI, label: t('applications:edit.advanced.certificate.type.jwksUri')},
  ];

  const currentCertType = certificate?.type ?? CertificateTypes.NONE;
  const currentCertValue = certificate?.value ?? '';

  return (
    <SettingsCard
      title={t('applications:edit.advanced.labels.certificate')}
      description={t('applications:edit.advanced.certificate.intro')}
    >
      <Stack spacing={2}>
        <FormControl fullWidth error={required && currentCertType === CertificateTypes.NONE}>
          <FormLabel htmlFor="certificate-type">{t('applications:edit.advanced.labels.certificateType')}</FormLabel>
          <Autocomplete
            id="certificate-type"
            value={certificateTypeOptions.find((opt) => opt.value === currentCertType) ?? certificateTypeOptions[0]}
            onChange={(_, newValue) => {
              const newType = newValue?.value ?? CertificateTypes.NONE;
              if (newType === CertificateTypes.NONE) {
                onCertificateChange(null);
              } else {
                onCertificateChange({type: newType, value: currentCertValue});
              }
            }}
            options={certificateTypeOptions}
            getOptionLabel={(option) => option.label}
            isOptionEqualToValue={(option, value) => option.value === value.value}
            renderInput={(params) => (
              <TextField {...params} fullWidth error={required && currentCertType === CertificateTypes.NONE} />
            )}
            disableClearable
            disabled={disabled}
          />
          {required && currentCertType === CertificateTypes.NONE && (
            <FormHelperText>
              {t(
                'applications:edit.advanced.certificate.error.required',
                'A certificate is required for private_key_jwt authentication.',
              )}
            </FormHelperText>
          )}
        </FormControl>

        {currentCertType !== CertificateTypes.NONE && (
          <TextField
            fullWidth
            multiline
            rows={3}
            value={currentCertValue}
            onChange={(e) => {
              onCertificateChange({type: currentCertType, value: e.target.value});
            }}
            disabled={disabled}
            error={required && !currentCertValue}
            placeholder={
              currentCertType === CertificateTypes.JWKS_URI
                ? t('applications:edit.advanced.certificate.placeholder.jwksUri')
                : t('applications:edit.advanced.certificate.placeholder.jwks')
            }
            helperText={
              required && !currentCertValue
                ? t(
                    'applications:edit.advanced.certificate.error.valueRequired',
                    'Please enter a value for the selected certificate type.',
                  )
                : currentCertType === CertificateTypes.JWKS_URI
                  ? t('applications:edit.advanced.certificate.hint.jwksUri')
                  : t('applications:edit.advanced.certificate.hint.jwks')
            }
          />
        )}
      </Stack>
    </SettingsCard>
  );
}
