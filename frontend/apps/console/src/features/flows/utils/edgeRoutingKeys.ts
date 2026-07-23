/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import type {ReactFlowState} from '@xyflow/react';

/**
 * Border radius for smooth step edges in pixels.
 */
export const SMOOTH_STEP_BORDER_RADIUS = 20;

/**
 * Sentinel obstacle key returned while any node is being dragged, so edges skip
 * the expensive smart routing and stay stable across drag ticks.
 */
export const DRAGGING_OBSTACLES_KEY = 'dragging';

/**
 * Cache of computed obstacle keys per store nodes snapshot. The selector runs for
 * every edge on every store update (including pan/zoom frames); React Flow only
 * replaces the nodes array when a node actually changes, so caching on the array
 * identity turns the O(edges × nodes) string building into a single computation.
 */
const obstaclesKeyCache = new WeakMap<object, string>();

/**
 * Derives a coarse key of all node bounds from the React Flow store. The key only
 * changes when a node settles at a new position or size — during a drag it returns
 * a constant, so edges neither re-render per drag tick nor re-route until drop.
 */
export function selectObstaclesKey(state: ReactFlowState): string {
  const {nodes} = state;

  if (nodes.some((node) => node.dragging)) {
    return DRAGGING_OBSTACLES_KEY;
  }

  const cached = obstaclesKeyCache.get(nodes);
  if (cached !== undefined) {
    return cached;
  }

  const key = nodes
    .map(
      (node) =>
        `${node.id}:${Math.round(node.position.x)},${Math.round(node.position.y)},` +
        `${Math.round(node.measured?.width ?? 0)}x${Math.round(node.measured?.height ?? 0)}`,
    )
    .join('|');
  obstaclesKeyCache.set(nodes, key);
  return key;
}
