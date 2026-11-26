// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {CREDENTIAL_SETTER_MODES} from './constants';
import ModeSelectorProperties from './ModeSelectorProperties';
import NoConfigProperties from './NoConfigProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';
import FlowTypeContext from '../../../../context/FlowTypeContext';
import {FlowType} from '../../../../models/flows';
import type {StepData} from '../../../../models/steps';

/**
 * The mode a Credential Setter node without one runs.
 */
const DEFAULT_MODE = 'set';

/**
 * Selects what a Credential Setter node does. Outside an administrator-initiated flow it only sets a
 * credential, so there is nothing to configure, unless the node already names another mode, which
 * stays listed so it can be changed back.
 */
function CredentialSetterProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  // From the builder, which knows the type of an unsaved draft too, where there is no saved flow to read.
  const flowType = useContext(FlowTypeContext);
  const currentMode =
    ((resource?.data as StepData | undefined)?.action?.executor as {mode?: string} | undefined)?.mode ?? DEFAULT_MODE;

  // Every operation outside the builder; a mode already on the node stays listed so it can be changed.
  const modes = useMemo(
    () =>
      !flowType || flowType === FlowType.ADMIN_INITIATED
        ? CREDENTIAL_SETTER_MODES
        : CREDENTIAL_SETTER_MODES.filter((mode) => mode.value === DEFAULT_MODE || mode.value === currentMode),
    [currentMode, flowType],
  );

  if (modes.length === 1) {
    return <NoConfigProperties />;
  }

  return (
    <ModeSelectorProperties
      resource={resource}
      onChange={onChange}
      modes={modes}
      defaultMode={DEFAULT_MODE}
      selectId="credential-setter-mode-select"
      fallbackLabel="Set Credentials"
      description={t(
        'flows:core.executions.credentialSetter.description',
        "Sets, validates or removes a user's credential.",
      )}
      label={t('flows:core.executions.credentialSetter.mode.label', 'Operation')}
      placeholder={t('flows:core.executions.credentialSetter.mode.placeholder', 'Select an operation')}
      hint={t(
        'flows:core.executions.credentialSetter.mode.hint',
        'In an administrator-initiated flow, Validate credential reset runs before the link is sent, Remove credential after it is emailed, and Set credentials when the user submits the new value.',
      )}
    />
  );
}

export default CredentialSetterProperties;
