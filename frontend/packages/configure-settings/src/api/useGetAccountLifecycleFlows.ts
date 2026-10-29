// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {UseQueryResult} from '@tanstack/react-query';
import {useLifecycleFlows} from '@thunderid/components';
import type {AdministrationFlowConfigKeyValue, BasicFlowSummary} from '@thunderid/utils';
import {useCallback} from 'react';
import type {AccountLifecycleFlows} from '../models/accountLifecycleFlows';

/**
 * Lists the administration flows behind the given lifecycle purposes, from the resolution the record
 * pages read. Nothing is read when no purpose is given.
 *
 * @param purposes - The lifecycle purposes to list, in display order
 * @returns TanStack Query result holding each distinct flow, and the purposes no existing flow serves
 *
 * @public
 */
export default function useGetAccountLifecycleFlows(
  purposes: readonly AdministrationFlowConfigKeyValue[],
): UseQueryResult<AccountLifecycleFlows, Error> {
  const select = useCallback(
    (resolved: Record<string, BasicFlowSummary>): AccountLifecycleFlows => {
      // Keyed by id, so a flow several purposes share is listed once.
      const flows = new Map<string, BasicFlowSummary>();
      purposes.forEach((purpose) => {
        const flow: BasicFlowSummary | undefined = resolved[purpose];
        if (flow) {
          flows.set(flow.id, flow);
        }
      });
      return {flows: [...flows.values()], unconfigured: purposes.filter((purpose) => !resolved[purpose])};
    },
    [purposes],
  );

  return useLifecycleFlows({enabled: purposes.length > 0, select});
}
