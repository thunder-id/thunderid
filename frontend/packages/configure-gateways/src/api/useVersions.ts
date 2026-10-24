// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import useRequest from './request';
import useDeploySource from './source';
import DeployQueryKeys from '../constants/query-keys';
import type {Version} from '../models/deploy';

/**
 * The versions, newest first, without their content: one target's, when the source keeps versions per
 * target. The control plane keeps the newest one current as the configuration changes, so the list is
 * read again when the window comes back.
 */
export function useVersions(targetId?: string): UseQueryResult<Version[]> {
  const source = useDeploySource();
  return useQuery<Version[]>({
    queryKey: [DeployQueryKeys.VERSIONS, targetId],
    queryFn: (): Promise<Version[]> => source.versions(targetId),
  });
}

/**
 * One version by its hash, `latest`, or `current` for the configuration as it stands, with where it
 * refers to values. `current` is read afresh each time it is asked for: it is what a capture would take.
 */
export function useVersion(ref: string | undefined, enabled = true, targetId?: string): UseQueryResult<Version> {
  const source = useDeploySource();
  return useQuery<Version>({
    queryKey: [DeployQueryKeys.VERSION, ref, targetId],
    queryFn: (): Promise<Version> => source.version(ref ?? '', targetId),
    enabled: enabled && Boolean(ref),
    staleTime: ref === 'current' ? 0 : Infinity,
  });
}

export interface CaptureInput {
  name?: string;
  note?: string;
}

/** Captures the control plane's configuration as a new version. */
export function useCaptureVersion(): UseMutationResult<Version, Error, CaptureInput> {
  const request = useRequest();
  const queryClient = useQueryClient();
  return useMutation<Version, Error, CaptureInput>({
    mutationFn: (input: CaptureInput): Promise<Version> => request<Version>('POST', '/configuration-versions', input),
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.VERSIONS]});
      void queryClient.invalidateQueries({queryKey: [DeployQueryKeys.VERSION]});
    },
  });
}
