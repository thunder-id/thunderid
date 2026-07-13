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
import {Stack, TextField, FormControl, FormLabel, Autocomplete, FormHelperText} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import CertificateTypes from '../../../../applications/constants/certificate-types';

interface CertificateSectionProps {
  certificate?: {type?: string; value?: string} | null;
  onCertificateChange: (cert: {type: string; value: string} | null) => void;
  required?: boolean;
  disabled?: boolean;
}

export default function CertificateSection({
  certificate = undefined,
  onCertificateChange,
  required = false,
  disabled = false,
}: CertificateSectionProps) {
  const {t} = useTranslation();

  const certificateTypeOptions = [
    {value: CertificateTypes.NONE, label: t('agents:edit.credentials.certificate.type.none', 'None')},
    {value: CertificateTypes.JWKS, label: t('agents:edit.credentials.certificate.type.jwks', 'JWKS (JSON)')},
    {value: CertificateTypes.JWKS_URI, label: t('agents:edit.credentials.certificate.type.jwksUri', 'JWKS URI')},
  ];

  const currentCertType = certificate?.type ?? CertificateTypes.NONE;
  const currentCertValue = certificate?.value ?? '';

  return (
    <SettingsCard
      title={t('agents:edit.credentials.certificate.title', 'Certificate')}
      description={t(
        'agents:edit.credentials.certificate.description',
        'Used to verify signed requests from this agent when it authenticates with private_key_jwt.',
      )}
    >
      <Stack spacing={2}>
        <FormControl fullWidth error={required && currentCertType === CertificateTypes.NONE}>
          <FormLabel htmlFor="certificate-type">
            {t('agents:edit.credentials.certificate.sourceLabel', 'Public key source')}
          </FormLabel>
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
                'agents:edit.credentials.certificate.error.required',
                'This agent needs a certificate before it can use private_key_jwt authentication.',
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
                ? t(
                    'agents:edit.credentials.certificate.placeholder.jwksUri',
                    'https://example.com/.well-known/jwks.json',
                  )
                : t('agents:edit.credentials.certificate.placeholder.jwks', '{ "keys": [ ... ] }')
            }
            helperText={
              required && !currentCertValue
                ? t('agents:edit.credentials.certificate.error.valueRequired', 'This field cannot be empty.')
                : currentCertType === CertificateTypes.JWKS_URI
                  ? t(
                      'agents:edit.credentials.certificate.hint.jwksUri',
                      'The URL to verify signed requests from this agent against.',
                    )
                  : t(
                      'agents:edit.credentials.certificate.hint.jwks',
                      'The JSON Web Key Set to verify signed requests from this agent against.',
                    )
            }
          />
        )}
      </Stack>
    </SettingsCard>
  );
}
