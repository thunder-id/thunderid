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

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import FlowQueryKeys from '../constants/flow-query-keys';
import type {UpdateFlowRequest, FlowDefinitionResponse} from '../models/responses';

/**
 * Variables for the update flow mutation
 */
interface UpdateFlowVariables {
  flowId: string;
  flowData: UpdateFlowRequest;
}

/**
 * Custom hook to update an existing flow definition.
 *
 * @returns TanStack Query mutation object for updating flow definitions
 *
 * @example
 * ```tsx
 * function SaveFlowButton({ flowId }: { flowId: string }) {
 *   const updateFlow = useUpdateFlow();
 *
 *   const handleSave = (flowData: UpdateFlowRequest) => {
 *     updateFlow.mutate({ flowId, flowData }, {
 *       onSuccess: (flow) => {
 *         console.log('Flow updated:', flow);
 *       },
 *       onError: (error) => {
 *         console.error('Failed to update flow:', error);
 *       }
 *     });
 *   };
 *
 *   return (
 *     <button onClick={() => handleSave(data)} disabled={updateFlow.isPending}>
 *       {updateFlow.isPending ? 'Saving...' : 'Save Flow'}
 *     </button>
 *   );
 * }
 * ```
 */
export default function useUpdateFlow(): UseMutationResult<FlowDefinitionResponse, Error, UpdateFlowVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();

  return useMutation<FlowDefinitionResponse, Error, UpdateFlowVariables>({
    mutationFn: async ({flowId, flowData}: UpdateFlowVariables): Promise<FlowDefinitionResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: FlowDefinitionResponse;
      } = await http.request({
        url: `${serverUrl}/flows/${flowId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: flowData,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, variables) => {
      // Invalidate and refetch flows list and the specific flow after successful update
      queryClient.invalidateQueries({queryKey: [FlowQueryKeys.FLOWS]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [FlowQueryKeys.FLOW, variables.flowId]}).catch(() => {
        // Ignore invalidation errors
      });
    },
  });
}
