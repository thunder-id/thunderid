/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useThunderID} from '@thunderid/react';
import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import FlowQueryKeys from '../constants/flow-query-keys';

/**
 * Custom React hook to delete a flow from the server.
 *
 * This hook uses TanStack Query mutations to handle the flow deletion process,
 * providing loading states and error handling. Upon successful deletion, it automatically
 * removes the flow from cache and invalidates the flows list query to trigger a refetch.
 *
 * @returns TanStack Query mutation object for deleting flows with mutate function, loading state, and error information
 *
 * @example
 * ```tsx
 * function DeleteFlowButton({ flowId }: { flowId: string }) {
 *   const deleteFlow = useDeleteFlow();
 *
 *   const handleDelete = () => {
 *     if (confirm('Are you sure you want to delete this flow?')) {
 *       deleteFlow.mutate(flowId, {
 *         onSuccess: () => {
 *           console.log('Flow deleted successfully');
 *         },
 *         onError: (error) => {
 *           console.error('Failed to delete flow:', error);
 *         }
 *       });
 *     }
 *   };
 *
 *   return (
 *     <button onClick={handleDelete} disabled={deleteFlow.isPending}>
 *       {deleteFlow.isPending ? 'Deleting...' : 'Delete Flow'}
 *     </button>
 *   );
 * }
 * ```
 */
export default function useDeleteFlow(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: async (flowId: string): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/flows/${flowId}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, flowId) => {
      // Remove the specific flow from cache
      queryClient.removeQueries({queryKey: [FlowQueryKeys.FLOW, flowId]});
      // Invalidate and refetch flows list
      queryClient.invalidateQueries({queryKey: [FlowQueryKeys.FLOWS]}).catch(() => {
        // Ignore invalidation errors
      });
    },
  });
}
