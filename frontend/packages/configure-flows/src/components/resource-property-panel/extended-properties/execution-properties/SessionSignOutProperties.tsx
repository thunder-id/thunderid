// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormHelperText, Stack, Typography} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

function SessionSignOutProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const handleBooleanPropertyChange = useCallback(
    (propertyName: string, value: boolean): void => {
      onChange(`data.properties.${propertyName}`, value, resource);
    },
    [resource, onChange],
  );

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.sessionSignOut.description')}
      </Typography>

      <FormControlLabel
        control={
          <Checkbox
            checked={!!properties['promptOnSignOut']}
            onChange={(e) => handleBooleanPropertyChange('promptOnSignOut', e.target.checked)}
            size="small"
          />
        }
        label={t('flows:core.executions.sessionSignOut.promptOnSignOut.label')}
      />
      <FormHelperText>{t('flows:core.executions.sessionSignOut.promptOnSignOut.hint')}</FormHelperText>
    </Stack>
  );
}

export default SessionSignOutProperties;
