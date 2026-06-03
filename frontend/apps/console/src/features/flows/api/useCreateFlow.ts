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
import type {CreateFlowRequest, FlowDefinitionResponse} from '../models/responses';

/**
 * Custom hook to create a new flow definition.
 *
 * @returns TanStack Query mutation object for creating flow definitions
 *
 * @example
 * ```tsx
 * function SaveFlowButton() {
 *   const createFlow = useCreateFlow();
 *
 *   const handleSave = (flowData: CreateFlowRequest) => {
 *     createFlow.mutate(flowData, {
 *       onSuccess: (flow) => {
 *         console.log('Flow created:', flow);
 *       },
 *       onError: (error) => {
 *         console.error('Failed to create flow:', error);
 *       }
 *     });
 *   };
 *
 *   return (
 *     <button onClick={() => handleSave(data)} disabled={createFlow.isPending}>
 *       {createFlow.isPending ? 'Saving...' : 'Save Flow'}
 *     </button>
 *   );
 * }
 * ```
 */
export default function useCreateFlow(): UseMutationResult<FlowDefinitionResponse, Error, CreateFlowRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();

  return useMutation<FlowDefinitionResponse, Error, CreateFlowRequest>({
    mutationFn: async (flowData: CreateFlowRequest): Promise<FlowDefinitionResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: FlowDefinitionResponse;
      } = await http.request({
        url: `${serverUrl}/flows`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: flowData,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      // Invalidate and refetch flows list after successful creation
      queryClient.invalidateQueries({queryKey: [FlowQueryKeys.FLOWS]}).catch(() => {
        // Ignore invalidation errors
      });
    },
  });
}
