// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, Stack} from '@wso2/oxygen-ui';
import {useContext, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {ROLE_EXECUTOR_MODES} from './constants';
import DraftTextField from './DraftTextField';
import ModeSelectorProperties from './ModeSelectorProperties';
import type {CommonResourcePropertiesPropsInterface} from './types';
import FlowTypeContext from '../../../../context/FlowTypeContext';
import {FlowType} from '../../../../models/flows';
import type {StepData} from '../../../../models/steps';

/**
 * Modes that run in an invitation flow; the others apply a revocation plan in an administration flow.
 */
const INVITATION_MODES: readonly string[] = ['role_selection', 'role_assignment'];

/**
 * Modes that run in a sign-up flow, which has no admin to pick roles: assigning the roles the node names.
 */
const SIGN_UP_MODES: readonly string[] = ['role_assignment'];

/**
 * Selects the role change a Role Executor node makes. An invitation flow is offered selecting and assigning
 * roles, a sign-up flow assigning the roles the node names, and any other flow the revocation plan modes.
 */
function RoleExecutorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  // From the builder, which knows the type of an unsaved draft too, where there is no saved flow to read.
  const flowType = useContext(FlowTypeContext);
  const stepData = resource?.data as StepData | undefined;
  const currentMode = (stepData?.action?.executor as {mode?: string} | undefined)?.mode;
  const configuredRoles = stepData?.properties?.['roles'];
  const invitation = flowType === FlowType.INVITATION;
  const signUp = flowType === FlowType.REGISTRATION;
  const namesRoles = currentMode === 'role_selection' || (signUp && currentMode === 'role_assignment');

  // Every mode until the flow loads; a mode already on the node stays listed so it can be changed.
  const modes = useMemo(() => {
    if (!flowType) {
      return ROLE_EXECUTOR_MODES;
    }
    const isOffered = (mode: string): boolean =>
      signUp ? SIGN_UP_MODES.includes(mode) : INVITATION_MODES.includes(mode) === invitation;

    return ROLE_EXECUTOR_MODES.filter((mode) => isOffered(mode.value) || mode.value === currentMode);
  }, [currentMode, flowType, invitation, signUp]);

  return (
    <Stack gap={2}>
      <ModeSelectorProperties
        resource={resource}
        onChange={onChange}
        modes={modes}
        selectId="role-executor-mode-select"
        fallbackLabel="Role Executor"
        description={t(
          'flows:core.executions.roleExecutor.description',
          'In an administration flow, apply the role change named by the trusted revocation plan. In an invitation flow, Select roles offers the roles the admin can assign in the organization unit and records the choice, and Assign roles assigns them, as that admin, once the invitee accepts. In a sign-up flow, Assign roles assigns the roles you name to the new user in the organization unit the flow created or resolved.',
        )}
        label={t('flows:core.executions.roleExecutor.mode.label', 'Mode')}
        placeholder={t('flows:core.executions.roleExecutor.mode.placeholder', 'Select a mode')}
        hint={
          invitation || signUp
            ? undefined
            : t(
                'flows:core.executions.roleExecutor.mode.hint',
                'Must match the mode of the access change validator earlier in the flow. A mismatched pair is refused at runtime.',
              )
        }
      />
      {namesRoles && (
        <div>
          <FormLabel htmlFor="role-executor-roles">
            {t('flows:core.executions.roleExecutor.roles.label', 'Roles')}
          </FormLabel>
          <DraftTextField
            id="role-executor-roles"
            value={typeof configuredRoles === 'string' ? configuredRoles : ''}
            onCommit={(value) => onChange('data.properties.roles', value.trim(), resource)}
            placeholder="role-id-1 role-id-2"
            fullWidth
            size="small"
          />
          <FormHelperText>
            {signUp
              ? t(
                  'flows:core.executions.roleExecutor.roles.signUpHint',
                  'IDs of the roles assigned to the new user, separated by spaces. Each role must be assignable in the organization unit: owned by it, or shared with it with assignments editable.',
                )
              : t(
                  'flows:core.executions.roleExecutor.roles.hint',
                  'IDs of the roles to assign, separated by spaces. When set, the admin is not asked to pick roles, and the step fails if the admin may not assign one of them.',
                )}
          </FormHelperText>
        </div>
      )}
    </Stack>
  );
}

export default RoleExecutorProperties;
