// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormHelperText, Stack} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '@/features/flows/models/steps';

function AttributeUniquenessProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const properties = (resource?.data as StepData | undefined)?.properties ?? {};

  return (
    <Stack gap={2}>
      <FormControlLabel
        control={
          <Checkbox
            checked={!!properties.allowCrossOUProvisioning}
            onChange={(e) => onChange('data.properties.allowCrossOUProvisioning', e.target.checked, resource)}
            size="small"
          />
        }
        label={t('flows:core.executions.federation.allowCrossOUProvisioning.label')}
      />
      <FormHelperText>{t('flows:core.executions.federation.allowCrossOUProvisioning.hint')}</FormHelperText>
    </Stack>
  );
}

export default AttributeUniquenessProperties;
