// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {ENTITY_MODE_OPTIONS} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '@/features/flows/models/steps';

function AttributeUniquenessValidatorProperties({
  resource,
  onChange,
}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const currentMode = (properties.mode as string) || 'user';

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.attributeUniquenessValidator.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="attribute-uniqueness-mode-select">
          {t('flows:core.executions.attributeUniquenessValidator.mode.label')}
        </FormLabel>
        <Select
          id="attribute-uniqueness-mode-select"
          value={currentMode}
          onChange={(e) => onChange('data.properties.mode', e.target.value, resource)}
          fullWidth
        >
          {ENTITY_MODE_OPTIONS.map((option) => (
            <MenuItem key={option.value} value={option.value}>
              {t(option.translationKey)}
            </MenuItem>
          ))}
        </Select>
        <FormHelperText>{t('flows:core.executions.attributeUniquenessValidator.mode.hint')}</FormHelperText>
      </div>
    </Stack>
  );
}

export default AttributeUniquenessValidatorProperties;
