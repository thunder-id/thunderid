// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import {IDENTITY_GOVERNANCE_MODES} from './constants';
import ModeSelectorProperties from './ModeSelectorProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';
import useGetFlowById from '../../../../api/useGetFlowById';
import {FlowType} from '../../../../models/flows';
import type {StepData} from '../../../../models/steps';

/**
 * Operations that run outside an administration flow.
 */
const NON_ADMINISTRATION_MODES: readonly string[] = ['unlock'];

/**
 * Selects the lifecycle operation an Identity Governance node performs, fixed in the flow design.
 * Suspend and unsuspend are offered only in an administration flow.
 */
function IdentityGovernanceProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {flowId} = useParams<{flowId: string}>();
  const {data: flow} = useGetFlowById(flowId);
  const flowType = flow?.flowType;
  const currentMode = ((resource?.data as StepData | undefined)?.action?.executor as {mode?: string} | undefined)?.mode;

  // Every operation until the flow loads; a mode already on the node stays listed so it can be changed.
  const modes = useMemo(
    () =>
      !flowType || flowType === FlowType.ADMINISTRATION
        ? IDENTITY_GOVERNANCE_MODES
        : IDENTITY_GOVERNANCE_MODES.filter(
            (mode) => NON_ADMINISTRATION_MODES.includes(mode.value) || mode.value === currentMode,
          ),
    [currentMode, flowType],
  );

  return (
    <ModeSelectorProperties
      resource={resource}
      onChange={onChange}
      modes={modes}
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
        'Suspend and unlock act on different holds, so releasing one leaves the other in place. Suspend acts on the identity a Pre Suspend Executor planned.',
      )}
    />
  );
}

export default IdentityGovernanceProperties;
