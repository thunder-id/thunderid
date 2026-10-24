// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {REVOCATION_MODES} from './constants';
import ModeSelectorProperties from './ModeSelectorProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';

/**
 * Configures the revocation breadth of the administrative pre-processing node.
 *
 * The breadth is the executor's mode rather than a runtime input on purpose: an administrator who
 * could choose it per request could weaken an operation to leave grants live. Fixing it here makes
 * it part of the flow's design, and flow-creation validation rejects any mode the executor does not
 * declare as supported.
 */
function PreDeleteProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  return (
    <ModeSelectorProperties
      resource={resource}
      onChange={onChange}
      modes={REVOCATION_MODES}
      selectId="administrative-flow-pre-mode-select"
      fallbackLabel="Validate and Plan Full Revocation"
      description={t(
        'flows:core.executions.preDelete.description',
        'Validate the target of an administrative operation and plan the revocation carried out by the executors that follow.',
      )}
      label={t('flows:core.executions.preDelete.mode.label', 'Revocation mode')}
      placeholder={t('flows:core.executions.preDelete.mode.placeholder', 'Select a revocation mode')}
      hint={t(
        'flows:core.executions.preDelete.mode.hint',
        'Applies to every grant the subject holds. The executors that follow act on this setting, so it cannot be changed per request.',
      )}
    />
  );
}

export default PreDeleteProperties;
