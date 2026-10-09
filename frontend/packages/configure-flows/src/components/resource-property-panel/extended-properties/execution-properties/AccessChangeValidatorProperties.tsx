// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {ACCESS_CHANGE_VALIDATOR_MODES} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

/**
 * Configures which access change the validator node checks and plans revocation for.
 */
function AccessChangeValidatorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const currentMode = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {mode?: string})?.mode ?? '';
  }, [resource]);

  const handleModeChange = (selectedMode: string): void => {
    const modeConfig = ACCESS_CHANGE_VALIDATOR_MODES.find((mode) => mode.value === selectedMode);

    const updatedData = {
      ...((resource?.data as StepData) ?? {}),
      action: {
        ...((resource?.data as StepData)?.action ?? {}),
        executor: {
          ...((resource?.data as StepData)?.action?.executor ?? {}),
          mode: selectedMode,
        },
      },
      display: {
        ...((resource?.data as StepData)?.['display'] ?? {}),
        label: modeConfig?.displayLabel ?? 'Access Change Validator',
      },
    };

    onChange('data', updatedData, resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t(
          'flows:core.executions.accessChangeValidator.description',
          'Validate an access change and plan the revocation of the scopes it takes away, carried out by the executors that follow.',
        )}
      </Typography>

      <div>
        <FormLabel htmlFor="access-change-validator-mode-select">
          {t('flows:core.executions.accessChangeValidator.mode.label', 'Mode')}
        </FormLabel>
        <Select
          id="access-change-validator-mode-select"
          value={currentMode}
          onChange={(e) => handleModeChange(e.target.value)}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {t('flows:core.executions.accessChangeValidator.mode.placeholder', 'Select a mode')}
          </MenuItem>
          {ACCESS_CHANGE_VALIDATOR_MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>
              {t(mode.translationKey, mode.displayLabel)}
            </MenuItem>
          ))}
        </Select>
      </div>
    </Stack>
  );
}

export default AccessChangeValidatorProperties;
