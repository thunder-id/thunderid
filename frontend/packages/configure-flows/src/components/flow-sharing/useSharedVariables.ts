// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetSharingPolicies} from '@thunderid/configure-resource-sharing';
import {useMemo} from 'react';
import type {SharedFlow} from './FlowSharingDialog';
import {sharedVariableIdentifiers} from './variableRules';

/**
 * The identifiers of the variables the sharing policies of a flow state a rule for.
 *
 * @param flow - The flow being edited, or `undefined` for one that is not saved yet and so is not shared
 * @returns The identifiers the policies name
 */
export default function useSharedVariables(flow: SharedFlow | undefined): Set<string> {
  const {data} = useGetSharingPolicies(`flows/${flow?.id ?? ''}`, flow !== undefined);

  return useMemo(() => sharedVariableIdentifiers(data), [data]);
}
