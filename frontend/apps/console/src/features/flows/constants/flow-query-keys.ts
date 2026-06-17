/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
