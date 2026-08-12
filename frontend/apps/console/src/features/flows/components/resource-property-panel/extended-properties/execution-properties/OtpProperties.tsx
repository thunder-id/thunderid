// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormHelperText, FormLabel, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
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

  // The executor reads these through a numeric conversion that rejects strings, so a
  // string would leave the configured value silently ignored at runtime.
  const handleNumberPropertyChange = (propertyName: string, value: string): void => {
    onChange(`data.properties.${propertyName}`, Number(value), resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.otp.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="otp-length">{t('flows:core.executions.otp.otpLength.label')}</FormLabel>
        <DraftTextField
          id="otp-length"
          type="number"
          value={String((properties.otpLength as number | string | undefined) ?? '')}
          onCommit={(value) => handleNumberPropertyChange('otpLength', value)}
          normalize={(raw) => clampToInteger(raw, 4, 10)}
          placeholder={t('flows:core.executions.otp.otpLength.placeholder')}
          fullWidth
          size="small"
          inputProps={{min: 4, max: 10}}
        />
        <FormHelperText>{t('flows:core.executions.otp.otpLength.hint')}</FormHelperText>
      </div>

      <div>
        <FormLabel htmlFor="otp-validity">{t('flows:core.executions.otp.otpValidityPeriodSeconds.label')}</FormLabel>
        <DraftTextField
          id="otp-validity"
          type="number"
          value={String((properties.otpValidityPeriodSeconds as number | string | undefined) ?? '')}
          onCommit={(value) => handleNumberPropertyChange('otpValidityPeriodSeconds', value)}
          normalize={(raw) => clampToInteger(raw, 30, 600)}
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
              checked={(properties.otpUseNumericOnly ?? true) as boolean}
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
        <DraftTextField
          id="otp-max-attempts"
          type="number"
          value={String((properties.maxAttempts as number | string | undefined) ?? '')}
          onCommit={(value) => handleNumberPropertyChange('maxAttempts', value)}
          normalize={(raw) => clampToInteger(raw, 1)}
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
