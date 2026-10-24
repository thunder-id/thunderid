// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {GROUP_EXECUTOR_MODES} from './constants';
import ModeSelectorProperties from './ModeSelectorProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';

/**
 * Configures which group change the group executor node applies.
 */
function GroupExecutorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  return (
    <ModeSelectorProperties
      resource={resource}
      onChange={onChange}
      modes={GROUP_EXECUTOR_MODES}
      selectId="group-executor-mode-select"
      fallbackLabel="Group Executor"
      description={t(
        'flows:core.executions.groupExecutor.description',
        'Apply the group change named by the trusted revocation plan.',
      )}
      label={t('flows:core.executions.groupExecutor.mode.label', 'Mode')}
      placeholder={t('flows:core.executions.groupExecutor.mode.placeholder', 'Select a mode')}
      hint={t(
        'flows:core.executions.groupExecutor.mode.hint',
        'Must match the mode of the access change validator earlier in the flow. A mismatched pair is refused at runtime.',
      )}
    />
  );
}

export default GroupExecutorProperties;
