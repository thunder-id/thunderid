// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {HttpLike} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';
import RoleQueryKeys from '../constants/role-query-keys';
import type {RoleAssignment} from '../models/role';
import {removeRoleAssignmentViaFlow} from '../utils/roleAdministrationFlow';

export interface RemoveRoleAssignmentsVariables {
  roleId: string;
  assignments: RoleAssignment[];
}

/**
 * Custom React hook to remove user or group assignments from a role.
 *
 * @returns TanStack Query mutation object for removing role assignments
 */
export default function useRemoveRoleAssignments(): UseMutationResult<void, Error, RemoveRoleAssignmentsVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('roles');
  const {showToast} = useToast();

  return useMutation<void, Error, RemoveRoleAssignmentsVariables>({
    mutationFn: async ({roleId, assignments}: RemoveRoleAssignmentsVariables): Promise<void> => {
      const serverUrl: string = getServerUrl();

      // One assignee per execution, because a criterion names one principal and the flow revokes for the
      // principal that actually lost the grant.
      //
      // This is not atomic, and the native endpoint it replaces was: a refusal partway through leaves
      // the earlier assignees removed and revoked while the mutation reports failure. Continuing past a
      // refusal would be worse, since the caller would be told everything succeeded. Whoever surfaces
      // the error should re-read the list rather than assume nothing changed.
      let ranViaFlow = false;

      for (const assignment of assignments) {
        ranViaFlow = await removeRoleAssignmentViaFlow(http as unknown as HttpLike, serverUrl, roleId, assignment.id);
        if (!ranViaFlow) {
          break;
        }
      }
      if (ranViaFlow) {
        return;
      }
      await http.request({
        url: `${serverUrl}/roles/${roleId}/assignments/remove`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data: {assignments},
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, {roleId}) => {
      queryClient.invalidateQueries({queryKey: [RoleQueryKeys.ROLE_ASSIGNMENTS, roleId]}).catch(() => {
        /* noop */
      });
      showToast(t('assignments.remove.success'), 'success');
    },
  });
}
