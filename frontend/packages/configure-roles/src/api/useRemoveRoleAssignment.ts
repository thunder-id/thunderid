// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {AdministrationModes, useAdministrationOperation, useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import RoleQueryKeys from '../constants/role-query-keys';
import type {RoleAssignment} from '../models/role';
import {
  removeRoleAssignmentNatively,
  removeRoleAssignmentViaFlow,
  type HttpLike,
} from '../utils/roleAdministrationFlow';

/**
 * One assignment per call: a loop of flow executions could not be rolled back if one failed.
 */
export interface RemoveRoleAssignmentVariables {
  roleId: string;
  assignment: RoleAssignment;
}

/**
 * Custom React hook to remove a user or group assignment from a role.
 *
 * @returns TanStack Query mutation object for removing a role assignment
 */
export default function useRemoveRoleAssignment(): UseMutationResult<void, Error, RemoveRoleAssignmentVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('roles');
  const {showToast} = useToast();
  const mode = useAdministrationOperation<[RemoveRoleAssignmentVariables], void>('roles', 'removeAssignment');

  return useMutation<void, Error, RemoveRoleAssignmentVariables>({
    mutationFn: async (variables: RemoveRoleAssignmentVariables): Promise<void> => {
      const client = http as unknown as HttpLike;
      const {roleId, assignment} = variables;

      if (typeof mode === 'function') {
        await mode(variables);
        return;
      }
      if (mode === AdministrationModes.NATIVE) {
        await removeRoleAssignmentNatively(client, getServerUrl(), roleId, assignment);
        return;
      }
      await removeRoleAssignmentViaFlow(client, getServerUrl(), roleId, assignment);
    },
    onSuccess: (_data, {roleId}) => {
      queryClient.invalidateQueries({queryKey: [RoleQueryKeys.ROLE_ASSIGNMENTS, roleId]}).catch(() => {
        /* noop */
      });
      showToast(t('assignments.remove.success'), 'success');
    },
  });
}
