// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries} from '@tanstack/react-query';
import {useMemo, type JSX, type ReactNode} from 'react';
import useDeploySource from '../api/source';
import DeployQueryKeys from '../constants/query-keys';
import ResourceNamesContext from '../contexts/ResourceNames';
import {resourceNames} from '../explorer/model';
import type {AppliedConfiguration} from '../models/deploy';

/**
 * Names the resources the given gateways run, as their maps name them, for the rows below it whose
 * changes carry no name of their own.
 */
export default function ResourceNamesProvider({
  targetIds,
  children,
}: {
  targetIds: string[];
  children: ReactNode;
}): JSX.Element {
  const source = useDeploySource();
  const results = useQueries({
    queries: targetIds.filter(Boolean).map((targetId: string) => ({
      queryKey: [DeployQueryKeys.APPLIED_CONFIGURATION, targetId],
      queryFn: (): Promise<AppliedConfiguration> => source.appliedConfiguration(targetId),
      staleTime: 30_000,
      retry: false,
    })),
  });
  const stamp = results.map((result) => result.dataUpdatedAt).join();
  const names = useMemo(
    () => new Map(results.flatMap((result) => [...resourceNames(result.data?.resources ?? []).entries()]).reverse()),
    // The results array is new on every render; when each was last read is what changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [stamp],
  );
  return <ResourceNamesContext.Provider value={names}>{children}</ResourceNamesContext.Provider>;
}
