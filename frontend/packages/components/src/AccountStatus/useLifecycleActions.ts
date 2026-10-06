// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {LifecycleCategory, LifecycleOperation} from '@thunderid/types';
import {
  AdministrationFlowConfigKey,
  resolveAdministrationFlows,
  type AdministrationFlowConfigKeyValue,
  type HttpLike,
} from '@thunderid/utils';

/**
 * Server-config purpose naming the administration flow for each category and operation. Applications
 * have no unlock, since they are never locked.
 *
 * @public
 */
export const LIFECYCLE_FLOW_PURPOSES: Record<
  LifecycleCategory,
  Partial<Record<LifecycleOperation, AdministrationFlowConfigKeyValue>>
> = {
  agent: {
    suspend: AdministrationFlowConfigKey.AGENT_SUSPEND,
    unlock: AdministrationFlowConfigKey.AGENT_UNLOCK,
    unsuspend: AdministrationFlowConfigKey.AGENT_UNSUSPEND,
  },
  application: {
    suspend: AdministrationFlowConfigKey.APPLICATION_SUSPEND,
    unsuspend: AdministrationFlowConfigKey.APPLICATION_UNSUSPEND,
  },
  user: {
    suspend: AdministrationFlowConfigKey.USER_SUSPEND,
    unlock: AdministrationFlowConfigKey.USER_UNLOCK,
    unsuspend: AdministrationFlowConfigKey.USER_UNSUSPEND,
  },
};

const ALL_PURPOSES: AdministrationFlowConfigKeyValue[] = Object.values(LIFECYCLE_FLOW_PURPOSES).flatMap((operations) =>
  Object.values(operations).filter((purpose): purpose is AdministrationFlowConfigKeyValue => Boolean(purpose)),
);

/**
 * Query key for the resolved lifecycle flows, one entry for every category.
 */
export const LIFECYCLE_FLOWS_QUERY_KEY = ['lifecycle-flows'] as const;

export interface LifecycleActions {
  /**
   * Flow id for each operation this category can currently perform. An operation is absent when no
   * flow is configured for it or the configured handle names no existing flow.
   */
  available: Partial<Record<LifecycleOperation, string>>;
  /**
   * True while availability is being resolved; nothing renders until it settles.
   */
  isPending: boolean;
}

/**
 * Resolves which lifecycle operations a deployment offers for a category. An operation is offered
 * only when its administration flow resolves; a failed resolution offers none.
 *
 * @param category - Identity category the caller is rendering.
 * @returns The operations available for that category, and the resolution state.
 */
export default function useLifecycleActions(category: LifecycleCategory): LifecycleActions {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  const query: UseQueryResult<Record<string, string>, Error> = useQuery<Record<string, string>, Error>({
    queryFn: async (): Promise<Record<string, string>> =>
      resolveAdministrationFlows(http as unknown as HttpLike, getServerUrl(), ALL_PURPOSES),
    queryKey: LIFECYCLE_FLOWS_QUERY_KEY,
  });

  const resolved = query.data ?? {};
  const available: Partial<Record<LifecycleOperation, string>> = {};

  Object.entries(LIFECYCLE_FLOW_PURPOSES[category]).forEach(([operation, purpose]) => {
    const flowId = purpose ? resolved[purpose] : undefined;

    if (flowId) {
      available[operation as LifecycleOperation] = flowId;
    }
  });

  return {
    available,
    isPending: query.isPending,
  };
}
