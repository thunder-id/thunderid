// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
