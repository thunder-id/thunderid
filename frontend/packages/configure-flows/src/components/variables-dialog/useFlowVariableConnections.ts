// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ConnectionInstanceCategories, useConnections} from '@thunderid/configure-connections';
import type {ConnectionInstance} from '@thunderid/react';

/**
 * The most connections one request returns, which is the most the server allows a page to hold.
 */
const CONNECTIONS_LIMIT = 100;

/**
 * The connections a connection variable can take, with the state of loading them.
 */
export interface FlowVariableConnections {
  /**
   * The connections, in the form the variable components read them. Empty while they load and when they failed to.
   */
  connections: Pick<ConnectionInstance, 'categories' | 'id' | 'name' | 'type'>[];
  /**
   * Why the connections could not be loaded, or `null`.
   */
  error: Error | null;
  /**
   * Loads the connections again.
   */
  refetch: () => void;
}

/**
 * The connections a connection variable can take. Only identity providers qualify, which is also the one category a
 * variable can name. When the organization unit that owns the flow is given, only the connections it owns or was
 * shared are listed, which are the ones the server accepts for the flow's variables.
 *
 * @param ownerOuId - The organization unit that owns the flow, or `undefined` while the flow is not saved yet
 * @returns The connections and the state of loading them.
 */
export default function useFlowVariableConnections(ownerOuId: string | undefined = undefined): FlowVariableConnections {
  const {data, error, refetch} = useConnections({
    category: ConnectionInstanceCategories.IDENTITY_PROVIDER,
    filter: ownerOuId ? `ouId eq "${ownerOuId}"` : undefined,
    limit: CONNECTIONS_LIMIT,
  });

  return {connections: data?.connections ?? [], error, refetch: () => void refetch()};
}
