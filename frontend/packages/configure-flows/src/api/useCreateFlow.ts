// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
