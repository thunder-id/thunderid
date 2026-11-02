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
import type {AppliedConfiguration, AppliedVersion, ApplyRequest, ApplyResult, ValueReference} from '../models/deploy';

/** Which version a gateway runs. */
export function useAppliedVersion(gatewayId: string, enabled = true): UseQueryResult<AppliedVersion> {
  const source = useDeploySource();
  return useQuery<AppliedVersion>({
    queryKey: [DeployQueryKeys.APPLIED_VERSION, gatewayId],
    queryFn: (): Promise<AppliedVersion> => source.appliedVersion(gatewayId),
    enabled: enabled && Boolean(gatewayId),
  });
}

/** Everything a gateway runs, each resource as its own read returns it. */
export function useAppliedConfiguration(gatewayId: string, enabled = true): UseQueryResult<AppliedConfiguration> {
  const source = useDeploySource();
  return useQuery<AppliedConfiguration>({
    queryKey: [DeployQueryKeys.APPLIED_CONFIGURATION, gatewayId],
    queryFn: (): Promise<AppliedConfiguration> => source.appliedConfiguration(gatewayId),
    enabled: enabled && Boolean(gatewayId),
    staleTime: 30_000,
    retry: false,
  });
}

/**
 * What applying a version to a gateway would do: the changes, with their keys for a selection, and
 * the values the gateway lacks. Nothing is written.
 */
export function useApplyCheck(
  gatewayId: string,
  version: string | undefined,
  enabled = true,
): UseQueryResult<ApplyResult> {
  const source = useDeploySource();
  return useQuery<ApplyResult>({
    queryKey: [DeployQueryKeys.APPLY_CHECK, gatewayId, version],
    queryFn: (): Promise<ApplyResult> => source.applyCheck(gatewayId, version ?? ''),
    enabled: enabled && Boolean(gatewayId) && Boolean(version),
    staleTime: 0,
    // The check is read when the deploy panel opens, and the user then works through it: refetching
    // when the window regains focus would move the list under them.
    refetchOnWindowFocus: false,
    retry: false,
  });
}

export interface ApplyInput {
  gatewayId: string;
  data: ApplyRequest;
}

/** Writes a version to a gateway, so the gateway runs it. */
export function useApplyVersion(): UseMutationResult<ApplyResult, Error, ApplyInput> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  return useMutation<ApplyResult, Error, ApplyInput>({
    mutationFn: ({gatewayId, data}: ApplyInput): Promise<ApplyResult> => source.apply(gatewayId, data),
    onSuccess: (result: ApplyResult, {gatewayId}: ApplyInput) => {
      if (result.dryRun) return;
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.APPLIED_VERSION, gatewayId]});
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.APPLIED_CONFIGURATION, gatewayId]});
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.APPLY_CHECK, gatewayId]});
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.RUNNING_REFERENCES, gatewayId]});
    },
  });
}

export interface RevertInput {
  gatewayId: string;
  dryRun?: boolean;
  confirmRemovals?: boolean;
}

/** Puts a gateway back on the version it ran before the one it runs now. */
export function useRevert(): UseMutationResult<ApplyResult, Error, RevertInput> {
  const source = useDeploySource();
  const queryClient = useQueryClient();
  return useMutation<ApplyResult, Error, RevertInput>({
    mutationFn: ({gatewayId, dryRun = false, confirmRemovals = false}: RevertInput): Promise<ApplyResult> =>
      source.revert(gatewayId, dryRun, confirmRemovals),
    onSuccess: (result: ApplyResult, {gatewayId}: RevertInput) => {
      if (result.dryRun) return;
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.APPLIED_VERSION, gatewayId]});
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.APPLIED_CONFIGURATION, gatewayId]});
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.APPLY_CHECK, gatewayId]});
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.RUNNING_REFERENCES, gatewayId]});
    },
  });
}

/** Where the version a target runs refers to values. */
export function useRunningReferences(gatewayId: string, enabled = true): UseQueryResult<ValueReference[]> {
  const source = useDeploySource();
  return useQuery<ValueReference[]>({
    queryKey: [DeployQueryKeys.RUNNING_REFERENCES, gatewayId],
    queryFn: (): Promise<ValueReference[]> => source.runningReferences(gatewayId),
    enabled: enabled && Boolean(gatewayId),
    retry: false,
  });
}
