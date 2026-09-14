// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Checkbox,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {OU_RESOLVE_FROM_OPTIONS} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '@/features/flows/models/steps';

function OUResolverProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const currentResolveFrom = (properties.resolveFrom as string) || 'caller';

  const handleBooleanPropertyChange = useCallback(
    (propertyName: string, value: boolean): void => {
      onChange(`data.properties.${propertyName}`, value, resource);
    },
    [resource, onChange],
  );

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.ouResolver.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="resolve-from-select">{t('flows:core.executions.ouResolver.resolveFrom.label')}</FormLabel>
        <Select
          id="resolve-from-select"
          value={currentResolveFrom}
          onChange={(e) => onChange('data.properties.resolveFrom', e.target.value, resource)}
          fullWidth
        >
          {OU_RESOLVE_FROM_OPTIONS.map((option) => (
            <MenuItem key={option.value} value={option.value}>
              {t(option.translationKey)}
            </MenuItem>
          ))}
        </Select>
      </div>

      {currentResolveFrom === 'prompt' && (
        <div>
          <FormControlLabel
            control={
              <Checkbox
                checked={!!properties.promptUseHandle}
                onChange={(e) => handleBooleanPropertyChange('promptUseHandle', e.target.checked)}
                size="small"
              />
            }
            label={t('flows:core.executions.ouResolver.promptUseHandle.label')}
          />
          <FormHelperText>{t('flows:core.executions.ouResolver.promptUseHandle.hint')}</FormHelperText>
        </div>
      )}
    </Stack>
  );
}

export default OUResolverProperties;
