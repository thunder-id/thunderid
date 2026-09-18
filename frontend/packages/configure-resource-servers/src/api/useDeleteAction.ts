// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {HttpLike} from '@thunderid/utils';
import ResourceServerQueryKeys from '../constants/resource-server-query-keys';
import {deleteActionViaFlow} from '../utils/scopeAdministrationFlow';

export default function useDeleteAction(
  resourceServerId: string,
  resourceId?: string,
): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: async (actionId: string): Promise<void> => {
      const serverUrl = getServerUrl();

      // Deleting an action retires the scope it defines. The flow denies that scope deployment-wide
      // before the action goes; the native endpoint below revokes nothing.
      if (await deleteActionViaFlow(http as unknown as HttpLike, serverUrl, resourceServerId, actionId, resourceId)) {
        return;
      }
      const url = resourceId
        ? `${serverUrl}/resource-servers/${resourceServerId}/resources/${resourceId}/actions/${actionId}`
        : `${serverUrl}/resource-servers/${resourceServerId}/actions/${actionId}`;

      await http.request({
        url,
        method: 'DELETE',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: () => {
      if (resourceId) {
        void queryClient.invalidateQueries({
          queryKey: [ResourceServerQueryKeys.RESOURCE_ACTIONS, resourceServerId, resourceId],
        });
      } else {
        void queryClient.invalidateQueries({
          queryKey: [ResourceServerQueryKeys.SERVER_ACTIONS, resourceServerId],
        });
      }
    },
  });
}
