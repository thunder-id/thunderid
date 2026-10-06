// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {LIFECYCLE_FLOW_PURPOSES} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {
  resolveAdministrationFlowSummaries,
  type AdministrationFlowConfigKeyValue,
  type BasicFlowSummary,
  type HttpLike,
} from '@thunderid/utils';
import SettingsQueryKeys from '../constants/settings-query-keys';
import type {AccountLifecycleFlows} from '../models/accountLifecycleFlows';

const PURPOSES: AdministrationFlowConfigKeyValue[] = Object.values(LIFECYCLE_FLOW_PURPOSES).flatMap((operations) =>
  Object.values(operations).filter((purpose): purpose is AdministrationFlowConfigKeyValue => Boolean(purpose)),
);

/**
 * Resolves the administration flows behind the account lifecycle operations: purpose to handle
 * through the `flow` server-config section, then handle to the listed flow.
 *
 * @returns TanStack Query result holding each distinct flow, and the purposes no existing flow serves
 *
 * @public
 */
export default function useGetAccountLifecycleFlows(): UseQueryResult<AccountLifecycleFlows> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<AccountLifecycleFlows>({
    queryKey: [SettingsQueryKeys.ACCOUNT_LIFECYCLE_FLOWS],
    queryFn: async (): Promise<AccountLifecycleFlows> => {
      const resolved: Record<string, BasicFlowSummary> = await resolveAdministrationFlowSummaries(
        http as unknown as HttpLike,
        getServerUrl(),
        PURPOSES,
      );
      // Keyed by id, so a flow several purposes share is listed once.
      const flows = new Map<string, BasicFlowSummary>();
      PURPOSES.forEach((purpose) => {
        const flow: BasicFlowSummary | undefined = resolved[purpose];
        if (flow) {
          flows.set(flow.id, flow);
        }
      });
      return {flows: [...flows.values()], unconfigured: PURPOSES.filter((purpose) => !resolved[purpose])};
    },
  });
}
