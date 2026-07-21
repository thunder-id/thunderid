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

import {Box} from '@wso2/oxygen-ui';
import {XIcon} from '@wso2/oxygen-ui-icons-react';
import {
  BaseEdge as XYFlowBaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
  getSmoothStepPath,
  useReactFlow,
  useStore,
  type EdgeProps,
  type ReactFlowState,
} from '@xyflow/react';
import {useMemo, useState, type ReactElement, type SyntheticEvent} from 'react';
import useFlowConfig from '../../hooks/useFlowConfig';
import {EdgeStyleTypes} from '../../models/steps';
import {calculateEdgePath, type EdgePathResult, type EdgeStyle} from '../../utils/calculateEdgePath';

/**
 * Props interface of {@link BaseEdge}
 */
export type BaseEdgePropsInterface = EdgeProps;

/**
 * Border radius for smooth step edges in pixels.
 */
const SMOOTH_STEP_BORDER_RADIUS = 20;

/**
 * Sentinel obstacle key returned while any node is being dragged, so edges skip
 * the expensive smart routing and stay stable across drag ticks.
 */
const DRAGGING_OBSTACLES_KEY = 'dragging';

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
function selectObstaclesKey(state: ReactFlowState): string {
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

/**
 * Enhanced edge component with custom routing algorithm to avoid nodes.
 * Includes custom delete button and label functionality with hover effects.
 * Supports multiple edge styles: Bezier, Smooth Step (with rounded corners), and Step.
 *
 * While a node is being dragged, the edge falls back to the cheap built-in path so
 * dragging stays smooth; the smart obstacle-avoiding route is recomputed on drop.
 */
function BaseEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  label,
  style,
  deletable,
  markerEnd,
  markerStart,
  selected,
}: BaseEdgePropsInterface): ReactElement {
  const {deleteElements, getNodes} = useReactFlow();
  const [isHovered, setIsHovered] = useState<boolean>(false);
  const {edgeStyle} = useFlowConfig();
  const obstaclesKey = useStore(selectObstaclesKey);

  const {
    path: edgePath,
    centerX: labelX,
    centerY: labelY,
  } = useMemo((): EdgePathResult => {
    if (obstaclesKey === DRAGGING_OBSTACLES_KEY) {
      const pathParams = {sourcePosition, sourceX, sourceY, targetPosition, targetX, targetY};
      const [path, centerX, centerY] =
        edgeStyle === EdgeStyleTypes.Bezier
          ? getBezierPath(pathParams)
          : getSmoothStepPath({
              ...pathParams,
              borderRadius: edgeStyle === EdgeStyleTypes.Step ? 0 : SMOOTH_STEP_BORDER_RADIUS,
            });
      return {centerX, centerY, path};
    }

    // Calculate smart path that routes around nodes with the selected edge style
    return calculateEdgePath(
      sourceX,
      sourceY,
      targetX,
      targetY,
      sourcePosition,
      targetPosition,
      getNodes(),
      edgeStyle as EdgeStyle,
      SMOOTH_STEP_BORDER_RADIUS,
    );
  }, [sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, obstaclesKey, edgeStyle, getNodes]);

  const handleDelete = (event: SyntheticEvent) => {
    event.stopPropagation();
    deleteElements({edges: [{id}]}).catch(() => null);
  };

  const handleDeleteKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      event.stopPropagation();
      deleteElements({edges: [{id}]}).catch(() => null);
    }
  };

  return (
    <g onMouseEnter={() => setIsHovered(true)} onMouseLeave={() => setIsHovered(false)}>
      {/* Invisible wider path for hover detection */}
      <path d={edgePath} fill="none" stroke="transparent" strokeWidth={20} style={{cursor: 'pointer'}} />
      <XYFlowBaseEdge
        id={id}
        path={edgePath}
        style={{
          ...style,
          strokeWidth: isHovered ? 3 : 2,
          transition: 'stroke-width 0.2s ease',
        }}
        interactionWidth={20}
        markerEnd={markerEnd}
        markerStart={markerStart}
      />
      <EdgeLabelRenderer>
        {label && (
          <Box
            className="nodrag nopan"
            onMouseEnter={() => setIsHovered(true)}
            onMouseLeave={() => setIsHovered(false)}
            sx={{
              pointerEvents: 'auto',
              position: 'absolute',
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              zIndex: 1000,
            }}
          >
            {label}
          </Box>
        )}
        {/* Clicking an edge selects it, keeping the delete button visible without
            hover precision; Delete/Backspace also removes the selected edge. */}
        {(isHovered || selected) && deletable !== false && (
          <Box
            className="nodrag nopan"
            onClick={handleDelete}
            onKeyDown={handleDeleteKeyDown}
            role="button"
            tabIndex={0}
            aria-label="Delete edge"
            onMouseEnter={() => setIsHovered(true)}
            onMouseLeave={() => setIsHovered(false)}
            sx={{
              position: 'absolute',
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
              pointerEvents: 'auto',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: '28px',
              height: '28px',
              backgroundColor: 'error.main',
              borderRadius: '50%',
              cursor: 'pointer',
              boxShadow: 2,
              transition: 'background-color 0.2s ease, transform 0.2s ease',
              zIndex: 10000,
              '&:hover, &:focus': {
                backgroundColor: 'error.dark',
                transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px) scale(1.1)`,
              },
            }}
          >
            <XIcon size={16} style={{color: 'white'}} />
          </Box>
        )}
      </EdgeLabelRenderer>
    </g>
  );
}

export default BaseEdge;
