// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {ROLE_EXECUTOR_MODES} from './constants';
import ModeSelectorProperties from './ModeSelectorProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';

/**
 * Configures which role change the role executor node applies.
 */
function RoleExecutorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  return (
    <ModeSelectorProperties
      resource={resource}
      onChange={onChange}
      modes={ROLE_EXECUTOR_MODES}
      selectId="role-executor-mode-select"
      fallbackLabel="Role Executor"
      description={t(
        'flows:core.executions.roleExecutor.description',
        'Apply the role change named by the trusted revocation plan.',
      )}
      label={t('flows:core.executions.roleExecutor.mode.label', 'Mode')}
      placeholder={t('flows:core.executions.roleExecutor.mode.placeholder', 'Select a mode')}
      hint={t(
        'flows:core.executions.roleExecutor.mode.hint',
        'Must match the mode of the access change validator earlier in the flow. A mismatched pair is refused at runtime.',
      )}
    />
  );
}

export default RoleExecutorProperties;
