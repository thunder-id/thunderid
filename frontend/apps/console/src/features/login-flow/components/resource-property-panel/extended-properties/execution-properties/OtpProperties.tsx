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

import {Checkbox, FormControlLabel, FormHelperText, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '@/features/flows/models/steps';

function OtpProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const handleBooleanPropertyChange = (propertyName: string, value: boolean): void => {
    onChange(`data.properties.${propertyName}`, value, resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.otp.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="otp-length">{t('flows:core.executions.otp.otpLength.label')}</FormLabel>
        <TextField
          id="otp-length"
          type="number"
          value={(properties.otpLength as string) ?? ''}
          onChange={(e) => onChange('data.properties.otpLength', e.target.value, resource, true)}
          placeholder={t('flows:core.executions.otp.otpLength.placeholder')}
          fullWidth
          size="small"
          inputProps={{min: 4, max: 10}}
        />
        <FormHelperText>{t('flows:core.executions.otp.otpLength.hint')}</FormHelperText>
      </div>

      <div>
        <FormLabel htmlFor="otp-validity">{t('flows:core.executions.otp.otpValidityPeriodSeconds.label')}</FormLabel>
        <TextField
          id="otp-validity"
          type="number"
          value={(properties.otpValidityPeriodSeconds as string) ?? ''}
          onChange={(e) => onChange('data.properties.otpValidityPeriodSeconds', e.target.value, resource, true)}
          placeholder={t('flows:core.executions.otp.otpValidityPeriodSeconds.placeholder')}
          fullWidth
          size="small"
          inputProps={{min: 30, max: 600}}
        />
        <FormHelperText>{t('flows:core.executions.otp.otpValidityPeriodSeconds.hint')}</FormHelperText>
      </div>

      <div>
        <FormControlLabel
          control={
            <Checkbox
              checked={!!properties.otpUseNumericOnly}
              onChange={(e) => handleBooleanPropertyChange('otpUseNumericOnly', e.target.checked)}
              size="small"
            />
          }
          label={t('flows:core.executions.otp.otpUseNumericOnly.label')}
        />
        <FormHelperText>{t('flows:core.executions.otp.otpUseNumericOnly.hint')}</FormHelperText>
      </div>

      <div>
        <FormLabel htmlFor="otp-max-attempts">{t('flows:core.executions.otp.maxAttempts.label')}</FormLabel>
        <TextField
          id="otp-max-attempts"
          type="number"
          value={(properties.maxAttempts as string) || ''}
          onChange={(e) => onChange('data.properties.maxAttempts', e.target.value, resource, true)}
          placeholder={t('flows:core.executions.otp.maxAttempts.placeholder')}
          fullWidth
          size="small"
          inputProps={{min: 1}}
        />
        <FormHelperText>{t('flows:core.executions.otp.maxAttempts.hint')}</FormHelperText>
      </div>
    </Stack>
  );
}

export default OtpProperties;
