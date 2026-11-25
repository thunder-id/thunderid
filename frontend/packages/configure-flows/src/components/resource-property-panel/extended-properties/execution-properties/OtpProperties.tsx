// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormHelperText, Stack, Typography} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
import VariableBinding from './VariableBinding';
import {BOOLEAN_VARIABLES, NUMBER_VARIABLES} from './variableBindingTypes';

function OtpProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.otp.description')}
      </Typography>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="otpLength"
        accepts={NUMBER_VARIABLES}
        label={t('flows:core.executions.otp.otpLength.label')}
      >
        {({commit, label, value: stored}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            helperText={t('flows:core.executions.otp.otpLength.hint')}
            id="otp-length"
            type="number"
            value={String((stored as number | string | undefined) ?? '')}
            onCommit={(value) => commit(Number(value))}
            normalize={(raw) => clampToInteger(raw, 4, 10)}
            placeholder={t('flows:core.executions.otp.otpLength.placeholder')}
            fullWidth
            size="small"
            inputProps={{min: 4, max: 10}}
          />
        )}
      </VariableBinding>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="otpValidityPeriodSeconds"
        accepts={NUMBER_VARIABLES}
        label={t('flows:core.executions.otp.otpValidityPeriodSeconds.label')}
      >
        {({commit, label, value: stored}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            helperText={t('flows:core.executions.otp.otpValidityPeriodSeconds.hint')}
            id="otp-validity"
            type="number"
            value={String((stored as number | string | undefined) ?? '')}
            onCommit={(value) => commit(Number(value))}
            normalize={(raw) => clampToInteger(raw, 30, 600)}
            placeholder={t('flows:core.executions.otp.otpValidityPeriodSeconds.placeholder')}
            fullWidth
            size="small"
            inputProps={{min: 30, max: 600}}
          />
        )}
      </VariableBinding>

      <div>
        <VariableBinding
          inline
          resource={resource}
          onChange={onChange}
          propertyKey="otpUseNumericOnly"
          accepts={BOOLEAN_VARIABLES}
          label={t('flows:core.executions.otp.otpUseNumericOnly.label')}
        >
          {({commit, label, value: stored}) => (
            <div>
              <FormControlLabel
                control={
                  <Checkbox
                    checked={(stored ?? true) as boolean}
                    onChange={(e) => commit(e.target.checked)}
                    size="small"
                  />
                }
                label={label}
              />
              <FormHelperText>{t('flows:core.executions.otp.otpUseNumericOnly.hint')}</FormHelperText>
            </div>
          )}
        </VariableBinding>
      </div>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="maxAttempts"
        accepts={NUMBER_VARIABLES}
        label={t('flows:core.executions.otp.maxAttempts.label')}
      >
        {({commit, label, value: stored}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            helperText={t('flows:core.executions.otp.maxAttempts.hint')}
            id="otp-max-attempts"
            type="number"
            value={String((stored as number | string | undefined) ?? '')}
            onCommit={(value) => commit(Number(value))}
            normalize={(raw) => clampToInteger(raw, 1)}
            placeholder={t('flows:core.executions.otp.maxAttempts.placeholder')}
            fullWidth
            size="small"
            inputProps={{min: 1}}
          />
        )}
      </VariableBinding>
    </Stack>
  );
}

export default OtpProperties;
