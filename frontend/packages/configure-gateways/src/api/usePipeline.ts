// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import useDeploySource from './source';
import DeployQueryKeys from '../constants/query-keys';
import type {AutoApplySetting, Pipeline} from '../models/pipeline';

/** The environments a version is deployed and promoted through, in order. */
export function usePipeline(): UseQueryResult<Pipeline> {
  const source = useDeploySource();
  return useQuery<Pipeline>({
    queryKey: [DeployQueryKeys.PIPELINE],
    queryFn: (): Promise<Pipeline> => source.pipeline(),
    staleTime: 60_000,
  });
}

/** Whether a change saved here deploys to the first environment on its own. */
export function useAutoApply(): UseQueryResult<AutoApplySetting> {
  const source = useDeploySource();
  return useQuery<AutoApplySetting>({
    queryKey: [DeployQueryKeys.AUTO_APPLY],
    queryFn: (): Promise<AutoApplySetting> => source.autoApply(),
    retry: false,
  });
}

/** Turns deploying a saved change to the first environment on or off. */
export function useSetAutoApply(): UseMutationResult<void, Error, boolean> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  return useMutation<void, Error, boolean>({
    mutationFn: (enabled: boolean): Promise<void> => source.setAutoApply(enabled),
    onSuccess: async (): Promise<void> => {
      await queryClient.invalidateQueries({queryKey: [DeployQueryKeys.AUTO_APPLY]});
    },
  });
}
