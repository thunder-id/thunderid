// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useMemo} from 'react';
import useDeploySource from './source';
import DeployQueryKeys from '../constants/query-keys';
import type {Master, MasterEntry, ResourceHistory} from '../models/deploy';

/** What a target holds, resource by resource: its master, with every entry. */
export function useMaster(targetId: string, enabled = true): UseQueryResult<Master> {
  const source = useDeploySource();
  return useQuery<Master>({
    queryKey: [DeployQueryKeys.MASTER, targetId],
    queryFn: (): Promise<Master> => source.master(targetId),
    enabled: enabled && Boolean(targetId),
    retry: false,
  });
}

/** A target's masters, newest first, without their entries. */
export function useMasterHistory(targetId: string, enabled = true): UseQueryResult<Master[]> {
  const source = useDeploySource();
  return useQuery<Master[]>({
    queryKey: [DeployQueryKeys.MASTER_HISTORY, targetId],
    queryFn: (): Promise<Master[]> => source.masterHistory(targetId),
    enabled: enabled && Boolean(targetId),
    retry: false,
  });
}

/** One resource's entry across a target's masters, newest first. */
export function useResourceHistory(targetId: string, key: string | null): UseQueryResult<ResourceHistory> {
  const source = useDeploySource();
  return useQuery<ResourceHistory>({
    queryKey: [DeployQueryKeys.RESOURCE_HISTORY, targetId, key],
    queryFn: (): Promise<ResourceHistory> => source.resourceHistory(targetId, key ?? ''),
    enabled: Boolean(targetId) && Boolean(key),
    retry: false,
  });
}

/**
 * The resources a target refused on its last deploy of each, as its master records them: the target
 * does not run them as the version has them, and every diff offers them again.
 */
export function useRefused(targetId: string): MasterEntry[] {
  const master = useMaster(targetId);
  return useMemo(
    () => (master.data?.entries ?? []).filter((entry: MasterEntry) => entry.state === 'failed'),
    [master.data],
  );
}
