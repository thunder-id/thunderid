// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {REVOCATION_MODES} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

/**
 * Configures the revocation breadth of the administrative pre-processing node.
 *
 * The breadth is the executor's mode rather than a runtime input on purpose: an administrator who
 * could choose it per request could weaken an operation to leave grants live. Fixing it here makes
 * it part of the flow's design, and flow-creation validation rejects any mode the executor does not
 * declare as supported.
 */
function PreDeleteProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const currentMode = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {mode?: string})?.mode ?? '';
  }, [resource]);

  const handleRevocationModeChange = (selectedMode: string): void => {
    const modeConfig = REVOCATION_MODES.find((mode) => mode.value === selectedMode);

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
        label: modeConfig?.displayLabel ?? 'Validate and Plan Full Revocation',
      },
    };

    onChange('data', updatedData, resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t(
          'flows:core.executions.preDelete.description',
          'Validate the target of an administrative operation and plan the revocation carried out by the executors that follow.',
        )}
      </Typography>

      <div>
        <FormLabel htmlFor="administrative-flow-pre-mode-select">
          {t('flows:core.executions.preDelete.mode.label', 'Revocation mode')}
        </FormLabel>
        <Select
          id="administrative-flow-pre-mode-select"
          value={currentMode}
          onChange={(e) => handleRevocationModeChange(e.target.value)}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {t('flows:core.executions.preDelete.mode.placeholder', 'Select a revocation mode')}
          </MenuItem>
          {REVOCATION_MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>
              {t(mode.translationKey, mode.displayLabel)}
            </MenuItem>
          ))}
        </Select>
        <FormHelperText>
          {t(
            'flows:core.executions.preDelete.mode.hint',
            'Applies to every grant the subject holds. The executors that follow act on this setting, so it cannot be changed per request.',
          )}
        </FormHelperText>
      </div>
    </Stack>
  );
}

export default PreDeleteProperties;
