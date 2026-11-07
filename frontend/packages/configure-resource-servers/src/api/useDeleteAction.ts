// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useAdministrationOperation, useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ResourceServerQueryKeys from '../constants/resource-server-query-keys';
import {deleteActionNatively, type HttpLike} from '../utils/actionAdministrationFlow';

/**
 * Custom React hook to delete an action of a resource server or one of its resources.
 *
 * The deletion calls the action's `DELETE` endpoint; the server revokes every token carrying the
 * action's scope before the action is removed. A console that declares its own operation runs that
 * instead.
 *
 * @returns TanStack Query mutation object for deleting actions
 */
export default function useDeleteAction(
  resourceServerId: string,
  resourceId?: string,
): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const mode = useAdministrationOperation<[string, string, string | undefined], void>(
    'resourceServers',
    'deleteAction',
  );

  return useMutation<void, Error, string>({
    mutationFn: async (actionId: string): Promise<void> => {
      const client = http as unknown as HttpLike;

      if (typeof mode === 'function') {
        await mode(resourceServerId, actionId, resourceId);
        return;
      }
      await deleteActionNatively(client, getServerUrl(), resourceServerId, actionId, resourceId);
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
