// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {ROLE_EXECUTOR_MODES} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

/**
 * Configures which role change the role executor node applies.
 */
function RoleExecutorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const currentMode = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {mode?: string})?.mode ?? '';
  }, [resource]);

  const handleModeChange = (selectedMode: string): void => {
    const modeConfig = ROLE_EXECUTOR_MODES.find((mode) => mode.value === selectedMode);

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
        label: modeConfig?.displayLabel ?? 'Role Executor',
      },
    };

    onChange('data', updatedData, resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t(
          'flows:core.executions.roleExecutor.description',
          'Apply the role change named by the trusted revocation plan.',
        )}
      </Typography>

      <div>
        <FormLabel htmlFor="role-executor-mode-select">
          {t('flows:core.executions.roleExecutor.mode.label', 'Mode')}
        </FormLabel>
        <Select
          id="role-executor-mode-select"
          value={currentMode}
          onChange={(e) => handleModeChange(e.target.value)}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {t('flows:core.executions.roleExecutor.mode.placeholder', 'Select a mode')}
          </MenuItem>
          {ROLE_EXECUTOR_MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>
              {t(mode.translationKey, mode.displayLabel)}
            </MenuItem>
          ))}
        </Select>
        <FormHelperText>
          {t(
            'flows:core.executions.roleExecutor.mode.hint',
            'Must match the mode of the access change validator earlier in the flow. A mismatched pair is refused at runtime.',
          )}
        </FormHelperText>
      </div>
    </Stack>
  );
}

export default RoleExecutorProperties;
