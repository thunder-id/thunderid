// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {
  AdministrationFlowConfigKey,
  resolveAdministrationFlowHandles,
  type AdministrationFlowConfigKeyValue,
  type HttpLike,
} from '@thunderid/utils';
import UserQueryKeys from '../constants/user-query-keys';

/**
 * How long the resolved handles stay fresh. The flow section changes rarely.
 */
const STALE_TIME: number = 5 * 60 * 1000;

/**
 * Reads the handle of the flow the server configures for an action, or an empty string when none is set.
 * Every action shares one read of the `flow` section, so a page offering several actions fetches it once.
 * Reading it needs permission to read the server configuration.
 *
 * @param configKey - The `flow` server-config entry naming the flow
 * @param enabled - Whether to read it. Pass false where the action is not offered.
 * @returns TanStack Query result holding the handle
 */
export default function useGetAdministrationFlowHandle(
  configKey: AdministrationFlowConfigKeyValue,
  enabled: boolean,
): UseQueryResult<string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<Record<string, string>, Error, string>({
    enabled,
    queryFn: async (): Promise<Record<string, string>> =>
      resolveAdministrationFlowHandles(
        http as unknown as HttpLike,
        getServerUrl(),
        Object.values(AdministrationFlowConfigKey),
      ),
    queryKey: [UserQueryKeys.ADMINISTRATION_FLOW_HANDLES],
    select: (handles) => handles[configKey] ?? '',
    staleTime: STALE_TIME,
  });
}
