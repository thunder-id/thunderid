// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxWithHint from './CheckboxWithHint';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

/**
 * Properties panel for the OU Delete and User Rollback executors: whether the node ends the flow with the
 * original failure once the rollback has finished.
 */
function RollbackExecutorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.rollbackExecutor.description')}
      </Typography>

      <CheckboxWithHint
        checked={properties['forwardFailure'] === true}
        label={t('flows:core.executions.rollbackExecutor.forwardFailure.label')}
        hint={t('flows:core.executions.rollbackExecutor.forwardFailure.hint')}
        onChange={(checked) => onChange('data.properties.forwardFailure', checked, resource)}
      />
    </Stack>
  );
}

export default RollbackExecutorProperties;
