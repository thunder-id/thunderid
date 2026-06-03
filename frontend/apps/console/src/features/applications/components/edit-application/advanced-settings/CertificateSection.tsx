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
import {Stack, TextField, FormControl, FormLabel, Autocomplete} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import CertificateTypes from '../../../constants/certificate-types';
import type {Application} from '../../../models/application';

/**
 * Props for the {@link CertificateSection} component.
 */
interface CertificateSectionProps {
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
  application,
  editedApp,
  onFieldChange,
  disabled = false,
}: CertificateSectionProps) {
  const {t} = useTranslation();

  const certificateTypeOptions = [
    {value: CertificateTypes.NONE, label: t('applications:edit.advanced.certificate.type.none')},
    {value: CertificateTypes.JWKS, label: t('applications:edit.advanced.certificate.type.jwks')},
    {value: CertificateTypes.JWKS_URI, label: t('applications:edit.advanced.certificate.type.jwksUri')},
  ];

  const currentCertType =
    (editedApp.certificate as {type?: string})?.type ??
    (application.certificate as {type?: string})?.type ??
    CertificateTypes.NONE;

  return (
    <SettingsCard
      title={t('applications:edit.advanced.labels.certificate')}
      description={t('applications:edit.advanced.certificate.intro')}
    >
      <Stack spacing={2}>
        <FormControl fullWidth>
          <FormLabel htmlFor="certificate-type">{t('applications:edit.advanced.labels.certificateType')}</FormLabel>
          <Autocomplete
            id="certificate-type"
            value={certificateTypeOptions.find((opt) => opt.value === currentCertType) ?? certificateTypeOptions[0]}
            onChange={(_, newValue) => {
              const currentCert = (editedApp.certificate ??
                application.certificate ?? {
                  type: CertificateTypes.NONE,
                  value: '',
                }) as {
                type: string;
                value: string;
              };
              onFieldChange('certificate', {...currentCert, type: newValue?.value || CertificateTypes.NONE});
            }}
            options={certificateTypeOptions}
            getOptionLabel={(option) => option.label}
            isOptionEqualToValue={(option, value) => option.value === value.value}
            renderInput={(params) => <TextField {...params} fullWidth />}
            disableClearable
            disabled={disabled}
          />
        </FormControl>

        {((editedApp.certificate as {type?: string})?.type ?? (application.certificate as {type?: string})?.type) !==
          CertificateTypes.NONE && (
          <TextField
            fullWidth
            multiline
            rows={3}
            value={
              (editedApp.certificate as {value?: string})?.value ??
              (application.certificate as {value?: string})?.value ??
              ''
            }
            onChange={(e) => {
              const currentCert = (editedApp.certificate ??
                application.certificate ?? {
                  type: CertificateTypes.NONE,
                  value: '',
                }) as {
                type: string;
                value: string;
              };
              onFieldChange('certificate', {...currentCert, value: e.target.value});
            }}
            disabled={disabled}
            placeholder={
              ((editedApp.certificate as {type?: string})?.type ??
                (application.certificate as {type?: string})?.type) === CertificateTypes.JWKS_URI
                ? t('applications:edit.advanced.certificate.placeholder.jwksUri')
                : t('applications:edit.advanced.certificate.placeholder.jwks')
            }
            helperText={
              ((editedApp.certificate as {type?: string})?.type ??
                (application.certificate as {type?: string})?.type) === CertificateTypes.JWKS_URI
                ? t('applications:edit.advanced.certificate.hint.jwksUri')
                : t('applications:edit.advanced.certificate.hint.jwks')
            }
          />
        )}
      </Stack>
    </SettingsCard>
  );
}
