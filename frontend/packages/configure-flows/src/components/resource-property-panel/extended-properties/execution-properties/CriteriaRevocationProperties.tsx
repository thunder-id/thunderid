// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {CRITERIA_REVOCATION_MODES} from './constants';
import ModeSelectorProperties from './ModeSelectorProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';

/**
 * Configures whether the criteria revocation node revokes or advances the cutoff after the change.
 */
function CriteriaRevocationProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  return (
    <ModeSelectorProperties
      resource={resource}
      onChange={onChange}
      modes={CRITERIA_REVOCATION_MODES}
      selectId="criteria-revocation-mode-select"
      fallbackLabel="Revoke Tokens"
      description={t(
        'flows:core.executions.criteriaRevocation.description',
        'Persist every token criterion from the trusted revocation plan.',
      )}
      label={t('flows:core.executions.criteriaRevocation.mode.label', 'Mode')}
      placeholder={t('flows:core.executions.criteriaRevocation.mode.placeholder', 'Select a mode')}
      hint={t(
        'flows:core.executions.criteriaRevocation.mode.hint',
        'Revoke runs before the change. Advance revocation cutoff runs after the change commits, closing the window a token could be minted in. A node without a mode revokes.',
      )}
    />
  );
}

export default CriteriaRevocationProperties;
