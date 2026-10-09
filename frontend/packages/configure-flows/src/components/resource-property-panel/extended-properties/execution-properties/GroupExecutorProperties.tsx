// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {GROUP_EXECUTOR_MODES} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

/**
 * Configures which group change the group executor node applies.
 */
function GroupExecutorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const currentMode = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {mode?: string})?.mode ?? '';
  }, [resource]);

  const handleModeChange = (selectedMode: string): void => {
    const modeConfig = GROUP_EXECUTOR_MODES.find((mode) => mode.value === selectedMode);

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
        label: modeConfig?.displayLabel ?? 'Group Executor',
      },
    };

    onChange('data', updatedData, resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t(
          'flows:core.executions.groupExecutor.description',
          'Apply the group change named by the trusted revocation plan.',
        )}
      </Typography>

      <div>
        <FormLabel htmlFor="group-executor-mode-select">
          {t('flows:core.executions.groupExecutor.mode.label', 'Mode')}
        </FormLabel>
        <Select
          id="group-executor-mode-select"
          value={currentMode}
          onChange={(e) => handleModeChange(e.target.value)}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {t('flows:core.executions.groupExecutor.mode.placeholder', 'Select a mode')}
          </MenuItem>
          {GROUP_EXECUTOR_MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>
              {t(mode.translationKey, mode.displayLabel)}
            </MenuItem>
          ))}
        </Select>
        <FormHelperText>
          {t(
            'flows:core.executions.groupExecutor.mode.hint',
            'Must match the mode of the access change validator earlier in the flow. A mismatched pair is refused at runtime.',
          )}
        </FormHelperText>
      </div>
    </Stack>
  );
}

export default GroupExecutorProperties;
