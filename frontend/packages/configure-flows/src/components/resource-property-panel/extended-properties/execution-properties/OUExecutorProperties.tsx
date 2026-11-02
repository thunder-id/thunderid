// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

function OUExecutorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.ouExecutor.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="parent-ou-id">{t('flows:core.executions.ouExecutor.parentOuId.label')}</FormLabel>
        <DraftTextField
          id="parent-ou-id"
          value={(properties['parentOuId'] as string) || ''}
          onCommit={(value) => onChange('data.properties.parentOuId', value, resource)}
          placeholder={t('flows:core.executions.ouExecutor.parentOuId.placeholder')}
          fullWidth
          size="small"
        />
        <FormHelperText>{t('flows:core.executions.ouExecutor.parentOuId.hint')}</FormHelperText>
      </div>
    </Stack>
  );
}

export default OUExecutorProperties;
