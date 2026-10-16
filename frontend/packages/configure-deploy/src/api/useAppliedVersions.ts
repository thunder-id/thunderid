// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries} from '@tanstack/react-query';
import useDeploySource from './source';
import DeployQueryKeys from '../constants/query-keys';
import type {AppliedVersion} from '../models/deploy';

export interface AppliedVersions {
  /** What each target runs, by target id; absent while unread or unreadable. */
  byGateway: Record<string, AppliedVersion | undefined>;
  isLoading: boolean;
}

/** Which version each of several targets runs, read together. */
export default function useAppliedVersions(gatewayIds: string[]): AppliedVersions {
  const source = useDeploySource();
  return useQueries({
    queries: gatewayIds.map((gatewayId: string) => ({
      queryKey: [DeployQueryKeys.APPLIED_VERSION, gatewayId],
      queryFn: (): Promise<AppliedVersion> => source.appliedVersion(gatewayId),
      retry: false,
    })),
    combine: (results) => ({
      byGateway: Object.fromEntries(
        gatewayIds.map((gatewayId: string, index: number): [string, AppliedVersion | undefined] => [
          gatewayId,
          results[index]?.data,
        ]),
      ),
      isLoading: results.some((result) => result.isLoading),
    }),
  });
}
