// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, Stack} from '@wso2/oxygen-ui';
import {useContext, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {SHARING_EXECUTOR_MODES} from './constants';
import DraftTextField from './DraftTextField';
import ModeSelectorProperties from './ModeSelectorProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';
import FlowTypeContext from '../../../../context/FlowTypeContext';
import {FlowType} from '../../../../models/flows';
import type {StepData} from '../../../../models/steps';

/**
 * Configures the sharing node: whether it authorizes the share or applies it, and what is shared where. Outside an
 * invitation flow it only shares, and only what a sign-up created.
 */
function SharingExecutorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  // From the builder, which knows the type of an unsaved draft too, where there is no saved flow to read.
  const flowType = useContext(FlowTypeContext);
  const signUp = flowType === FlowType.AUTHENTICATION || flowType === FlowType.REGISTRATION;

  const stepData = resource?.data as StepData | undefined;
  const properties = useMemo(() => stepData?.properties ?? {}, [stepData]);
  const mode = (stepData?.action?.executor as {mode?: string} | undefined)?.mode;

  // A sign-up only shares; a mode already on the node stays listed so it can be changed.
  const modes = useMemo(
    () => SHARING_EXECUTOR_MODES.filter((option) => !signUp || option.value === 'share' || option.value === mode),
    [mode, signUp],
  );

  const fields = [
    {
      key: 'resourceType',
      placeholder: 'user',
      show: true,
      label: 'Resource type',
      hint: 'The type of resource to share. Only user can be shared from a flow.',
    },
    {
      key: 'resourceId',
      placeholder: '{{ctx(userID)}}',
      show: true,
      label: 'Resource ID',
      hint: 'The resource to share. Leave it empty in Validate Share for a user the flow creates on acceptance.',
    },
    // Only validate authorizes a share for a resource created later, which names its owning unit instead.
    {
      key: 'owningOuId',
      placeholder: '{{ctx(defaultOUID)}}',
      show: mode === 'validate',
      label: 'Owning organization unit',
      hint: 'The organization unit that owns a resource the flow creates on acceptance.',
    },
    {
      key: 'targetOuId',
      placeholder: '{{ctx(ouId)}}',
      show: true,
      label: 'Target organization unit',
      hint: 'The child organization unit to share into. Values resolve from the flow runtime data only.',
    },
  ];

  return (
    <Stack gap={2}>
      <ModeSelectorProperties
        resource={resource}
        onChange={onChange}
        modes={modes}
        selectId="sharing-executor-mode-select"
        fallbackLabel="Sharing"
        description={
          signUp
            ? t(
                'flows:core.executions.sharing.signUpDescription',
                "Share the resource the properties name into a child organization unit of its owner, on the owner's authority, without checking who asked. Place it right after provisioning and name the user the flow just created.",
              )
            : t(
                'flows:core.executions.sharing.description',
                "Validate Share authorizes sharing a resource into a child organization unit as the admin who started the flow. Share applies it, on the owner's authority, once the invitee accepts.",
              )
        }
        label={t('flows:core.executions.sharing.mode.label', 'Mode')}
        placeholder={t('flows:core.executions.sharing.mode.placeholder', 'Select a mode')}
      />
      {fields
        .filter((field) => field.show)
        .map((field) => (
          <div key={field.key}>
            <FormLabel htmlFor={`sharing-${field.key}`}>
              {t(`flows:core.executions.sharing.${field.key}.label`, field.label)}
            </FormLabel>
            <DraftTextField
              id={`sharing-${field.key}`}
              value={typeof properties[field.key] === 'string' ? (properties[field.key] as string) : ''}
              onCommit={(value) => onChange(`data.properties.${field.key}`, value.trim(), resource)}
              placeholder={field.placeholder}
              fullWidth
              size="small"
            />
            <FormHelperText>{t(`flows:core.executions.sharing.${field.key}.hint`, field.hint)}</FormHelperText>
          </div>
        ))}
    </Stack>
  );
}

export default SharingExecutorProperties;
