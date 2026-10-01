// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
