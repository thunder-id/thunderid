// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {ACCESS_CHANGE_VALIDATOR_MODES} from './constants';
import ModeSelectorProperties from './ModeSelectorProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';

/**
 * Configures which access change the validator node checks and plans revocation for.
 */
function AccessChangeValidatorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  return (
    <ModeSelectorProperties
      resource={resource}
      onChange={onChange}
      modes={ACCESS_CHANGE_VALIDATOR_MODES}
      selectId="access-change-validator-mode-select"
      fallbackLabel="Access Change Validator"
      description={t(
        'flows:core.executions.accessChangeValidator.description',
        'Validate an access change and plan the revocation of the scopes it takes away, carried out by the executors that follow.',
      )}
      label={t('flows:core.executions.accessChangeValidator.mode.label', 'Mode')}
      placeholder={t('flows:core.executions.accessChangeValidator.mode.placeholder', 'Select a mode')}
    />
  );
}

export default AccessChangeValidatorProperties;
