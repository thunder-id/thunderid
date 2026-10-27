// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {GroupQueryKeys as SdkGroupQueryKeys, useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GroupQueryKeys from '../constants/group-query-keys';
import type {Group} from '../models/group';
import type {UpdateGroupRequest} from '../models/requests';

/**
 * Variables for the update group mutation.
 */
export interface UpdateGroupVariables {
  groupId: string;
  data: UpdateGroupRequest;
}

/**
 * Custom React hook to update an existing group.
 *
 * @returns TanStack Query mutation object for updating groups
 */
export default function useUpdateGroup(): UseMutationResult<Group, Error, UpdateGroupVariables> {
  const {http, invalidator} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('groups');
  const {showToast} = useToast();

  return useMutation<Group, Error, UpdateGroupVariables>({
    mutationFn: async ({groupId, data}: UpdateGroupVariables): Promise<Group> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: Group;
      } = await http.request({
        url: `${serverUrl}/groups/${groupId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, {groupId}) => {
      // Lists the SDK's admin components render keep their own state, so they refresh on this too.
      invalidator?.invalidate([SdkGroupQueryKeys.GROUPS]);
      queryClient.invalidateQueries({queryKey: [GroupQueryKeys.GROUP, groupId]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [GroupQueryKeys.GROUPS]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [GroupQueryKeys.GROUP_MEMBERS, groupId]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('update.success'), 'success');
    },
  });
}
