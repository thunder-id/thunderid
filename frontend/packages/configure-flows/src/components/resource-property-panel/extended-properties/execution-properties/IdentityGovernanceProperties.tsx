// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {IDENTITY_GOVERNANCE_MODES} from './constants';
import ModeSelectorProperties from './ModeSelectorProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';

/**
 * Selects the lifecycle operation an Identity Governance node performs.
 *
 * The operation is the executor's mode rather than a runtime input: the operations are not
 * variations of one thing and one of them is destructive, so fixing it here makes it part of the
 * flow's design and validation rejects a mode the executor does not support.
 *
 * Suspend and unsuspend share one palette entry as two directions of one hold. Unlock has its own,
 * because it acts on a different dimension and is the one operation a recovery flow can also run.
 */
function IdentityGovernanceProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  return (
    <ModeSelectorProperties
      resource={resource}
      onChange={onChange}
      modes={IDENTITY_GOVERNANCE_MODES}
      selectId="identity-governance-mode-select"
      fallbackLabel="Identity Governance"
      description={t(
        'flows:core.executions.identityGovernance.description',
        'Run one lifecycle operation on the identity the flow resolved.',
      )}
      label={t('flows:core.executions.identityGovernance.mode.label', 'Operation')}
      placeholder={t('flows:core.executions.identityGovernance.mode.placeholder', 'Select an operation')}
      hint={t(
        'flows:core.executions.identityGovernance.mode.hint',
        'Suspend and unlock act on different holds, so releasing one leaves the other in place. In a recovery flow only Unlock runs, on the identity that proved control earlier in the flow.',
      )}
    />
  );
}

export default IdentityGovernanceProperties;
