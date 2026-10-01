// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Query key constants for flows feature cache management.
 *
 * @public
 * @remarks
 * These constants are used with TanStack Query to manage caching,
 * invalidation, and refetching of flow-related data. Each key
 * represents a different type of flow query.
 *
 * @example
 * ```typescript
 * // Using in a query
 * useQuery({
 *   queryKey: [FlowQueryKeys.FLOWS, { limit: 10, offset: 0 }],
 *   queryFn: fetchFlows
 * });
 *
 * // Invalidating cache
 * queryClient.invalidateQueries({
 *   queryKey: [FlowQueryKeys.FLOWS]
 * });
 * ```
 */
const FlowQueryKeys = {
  /**
   * Base key for all flow-related queries
   */
  FLOWS: 'flows',
  /**
   * Key for a single flow query
   */
  FLOW: 'flow',
  /**
   * Key for a flow usages query
   */
  FLOW_USAGES: 'flow-usages',
} as const;

export default FlowQueryKeys;
