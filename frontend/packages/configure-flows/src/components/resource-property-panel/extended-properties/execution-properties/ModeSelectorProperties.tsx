// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

/**
 * One mode an executor offers in its mode selector.
 */
export interface ExecutorModeOption {
  value: string;
  translationKey: string;
  displayLabel: string;
  /**
   * Node icon for the mode, when the mode changes what the node does.
   */
  image?: string;
}

/**
 * Props interface of {@link ModeSelectorProperties}
 */
export interface ModeSelectorPropertiesProps extends CommonResourcePropertiesPropsInterface {
  /**
   * The modes the executor supports, in display order.
   */
  modes: readonly ExecutorModeOption[];
  /**
   * Id of the select, which the label points at.
   */
  selectId: string;
  /**
   * Node label used when the selected mode is not in `modes`.
   */
  fallbackLabel: string;
  /**
   * Translated description shown above the selector.
   */
  description: string;
  /**
   * Translated label of the selector.
   */
  label: string;
  /**
   * Translated placeholder shown when the node carries no mode.
   */
  placeholder: string;
  /**
   * Translated caption shown under the selector.
   */
  hint?: string;
}

/**
 * Configures the mode of an executor node, and labels the node after the selected mode.
 *
 * @param props - Props injected to the component.
 * @returns The ModeSelectorProperties component.
 */
function ModeSelectorProperties({
  resource,
  onChange,
  modes,
  selectId,
  fallbackLabel,
  description,
  label,
  placeholder,
  hint = undefined,
}: ModeSelectorPropertiesProps): ReactNode {
  const {t} = useTranslation();

  const currentMode = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {mode?: string})?.mode ?? '';
  }, [resource]);

  const handleModeChange = (selectedMode: string): void => {
    const modeConfig = modes.find((mode) => mode.value === selectedMode);

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
        label: modeConfig?.displayLabel ?? fallbackLabel,
        ...(modeConfig?.image ? {image: modeConfig.image} : {}),
      },
    };

    onChange('data', updatedData, resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {description}
      </Typography>

      <div>
        <FormLabel htmlFor={selectId}>{label}</FormLabel>
        <Select
          id={selectId}
          value={currentMode}
          onChange={(e) => handleModeChange(e.target.value)}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {placeholder}
          </MenuItem>
          {modes.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>
              {t(mode.translationKey, mode.displayLabel)}
            </MenuItem>
          ))}
        </Select>
        {hint && <FormHelperText>{hint}</FormHelperText>}
      </div>
    </Stack>
  );
}

export default ModeSelectorProperties;
