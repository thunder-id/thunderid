// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {MAGIC_LINK_MODES} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

function MagicLinkProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const currentMode = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {mode?: string})?.mode ?? '';
  }, [resource]);

  const handleModeChange = useCallback(
    (selectedMode: string): void => {
      const modeConfig = MAGIC_LINK_MODES.find((mode) => mode.value === selectedMode);

      const updatedData = {
        ...((resource?.data as StepData) ?? {}),
        action: {
          ...((resource?.data as StepData)?.action ?? {}),
          executor: {
            ...((resource?.data as StepData)?.action?.executor ?? {}),
            mode: selectedMode,
            inputs: [],
          },
        },
        display: {
          ...((resource?.data as StepData)?.['display'] ?? {}),
          label: modeConfig?.displayLabel ?? 'Magic Link',
        },
      };

      onChange('data', updatedData, resource);
    },
    [resource, onChange],
  );

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.magicLink.description', 'Configure the Magic Link step behavior.')}
      </Typography>

      <div>
        <FormLabel htmlFor="magiclink-mode-select">{t('flows:core.executions.magicLink.mode.label', 'Mode')}</FormLabel>
        <Select
          id="magiclink-mode-select"
          value={currentMode}
          onChange={(e) => handleModeChange(e.target.value)}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {t('flows:core.executions.magicLink.mode.placeholder', 'Select an action mode')}
          </MenuItem>
          {MAGIC_LINK_MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>
              {t(mode.translationKey, mode.displayLabel)}
            </MenuItem>
          ))}
        </Select>
      </div>
    </Stack>
  );
}

export default MagicLinkProperties;
