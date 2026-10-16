// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {LIFECYCLE_FLOW_PURPOSES} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {resolveAdministrationFlows, type AdministrationFlowConfigKeyValue, type HttpLike} from '@thunderid/utils';
import SettingsQueryKeys from '../constants/settings-query-keys';

const PURPOSES: AdministrationFlowConfigKeyValue[] = Object.values(LIFECYCLE_FLOW_PURPOSES).flatMap((operations) =>
  Object.values(operations).filter((purpose): purpose is AdministrationFlowConfigKeyValue => Boolean(purpose)),
);

/**
 * Resolves the administration flow behind each account lifecycle operation: purpose to handle through
 * the `flow` server-config section, then handle to flow id.
 *
 * @returns TanStack Query result holding the flow id for each purpose that names an existing flow.
 *
 * @public
 */
export default function useGetAccountLifecycleFlows(): UseQueryResult<Record<string, string>> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<Record<string, string>>({
    queryKey: [SettingsQueryKeys.ACCOUNT_LIFECYCLE_FLOWS],
    queryFn: async (): Promise<Record<string, string>> =>
      resolveAdministrationFlows(http as unknown as HttpLike, getServerUrl(), PURPOSES),
  });
}
