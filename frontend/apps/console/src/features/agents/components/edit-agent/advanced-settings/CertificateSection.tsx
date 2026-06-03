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
import {Stack, TextField, FormControl, FormLabel, Autocomplete} from '@wso2/oxygen-ui';
import {useTranslation} from 'react-i18next';
import CertificateTypes from '../../../../applications/constants/certificate-types';
import type {Agent} from '../../../models/agent';

interface CertificateSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  disabled?: boolean;
}

export default function CertificateSection({
  agent,
  editedAgent,
  onFieldChange,
  disabled = false,
}: CertificateSectionProps) {
  const {t} = useTranslation();

  const certificateTypeOptions = [
    {value: CertificateTypes.NONE, label: t('applications:edit.advanced.certificate.type.none')},
    {value: CertificateTypes.JWKS, label: t('applications:edit.advanced.certificate.type.jwks')},
    {value: CertificateTypes.JWKS_URI, label: t('applications:edit.advanced.certificate.type.jwksUri')},
  ];

  const currentCertType = editedAgent.certificate?.type ?? agent.certificate?.type ?? CertificateTypes.NONE;

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
              const currentCert = editedAgent.certificate ??
                agent.certificate ?? {type: CertificateTypes.NONE, value: ''};
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

        {currentCertType !== CertificateTypes.NONE && (
          <TextField
            fullWidth
            multiline
            rows={3}
            value={editedAgent.certificate?.value ?? agent.certificate?.value ?? ''}
            onChange={(e) => {
              const currentCert = editedAgent.certificate ??
                agent.certificate ?? {type: CertificateTypes.NONE, value: ''};
              onFieldChange('certificate', {...currentCert, value: e.target.value});
            }}
            disabled={disabled}
            placeholder={
              currentCertType === CertificateTypes.JWKS_URI
                ? t('applications:edit.advanced.certificate.placeholder.jwksUri')
                : t('applications:edit.advanced.certificate.placeholder.jwks')
            }
            helperText={
              currentCertType === CertificateTypes.JWKS_URI
                ? t('applications:edit.advanced.certificate.hint.jwksUri')
                : t('applications:edit.advanced.certificate.hint.jwks')
            }
          />
        )}
      </Stack>
    </SettingsCard>
  );
}
