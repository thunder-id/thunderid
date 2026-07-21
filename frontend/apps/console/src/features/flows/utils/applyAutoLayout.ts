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

import type {Edge, Node} from '@xyflow/react';
import type {ELK} from 'elkjs/lib/elk-api';

/**
 * Configuration options for auto-layout.
 */
export interface AutoLayoutOptions {
  /**
   * Spacing between nodes (perpendicular to flow direction).
   * @default 100
   */
  nodeSpacing?: number;
  /**
   * Spacing between ranks/layers (parallel to flow direction).
   * @default 160
   */
  rankSpacing?: number;
  /**
   * Horizontal offset to shift all nodes.
   * @default 50
   */
  offsetX?: number;
  /**
   * Vertical offset to shift all nodes.
   * @default 50
   */
  offsetY?: number;
}

// ELK (the layout engine) is ~1.5MB. It is only needed when a layout is actually
// requested (toolbar button, or opening a flow with no stored positions), so it is
// dynamically imported and cached rather than bundled into the builder's entry chunk.
let elkPromise: Promise<ELK> | undefined;

const getElk = async (): Promise<ELK> => {
  elkPromise ??= import('elkjs/lib/elk.bundled.js').then((module) => new module.default());
  try {
    return await elkPromise;
  } catch (error) {
    // A failed chunk load (e.g. a transient network error) must not stay
    // cached, or every later layout attempt would fail until a full reload.
    elkPromise = undefined;
    throw error;
  }
};

const FAILURE_HANDLE = 'failure';
const INCOMPLETE_HANDLE_SUFFIX = '_INCOMPLETE';
const PREVIOUS_HANDLE_SUFFIX = '_PREVIOUS';

/**
 * Which side of the node an edge leaves from, derived from the canvas handle
 * conventions: success/next handles sit on the right, failure on the bottom,
 * incomplete on the top, previous on the left.
 */
const sourcePortSide = (sourceHandle: string | null | undefined): 'EAST' | 'SOUTH' | 'NORTH' | 'WEST' => {
  if (sourceHandle === FAILURE_HANDLE) {
    return 'SOUTH';
  }
  if (sourceHandle?.endsWith(INCOMPLETE_HANDLE_SUFFIX)) {
    return 'NORTH';
  }
  if (sourceHandle?.endsWith(PREVIOUS_HANDLE_SUFFIX)) {
    return 'WEST';
  }
  return 'EAST';
};

interface ElkPort {
  id: string;
  layoutOptions: Record<string, string>;
}

/**
 * The happy path is the success chain from the start node to the end node.
 * Its edges get a straightening priority so ELK aligns the main flow on one
 * row; failure and incomplete branches hang off it.
 */
const collectHappyPathEdgeIds = (nodes: Node[], edges: Edge[]): Set<string> => {
  const startNode = nodes.find((node) => node.type?.toUpperCase() === 'START');
  const happyEdgeIds = new Set<string>();
  if (!startNode) {
    return happyEdgeIds;
  }

  const visited = new Set<string>([startNode.id]);
  let current: string | undefined = startNode.id;

  while (current) {
    const nextEdge: Edge | undefined = edges.find(
      (edge) => edge.source === current && sourcePortSide(edge.sourceHandle) === 'EAST' && !visited.has(edge.target),
    );
    if (!nextEdge) {
      break;
    }
    happyEdgeIds.add(nextEdge.id);
    visited.add(nextEdge.target);
    current = nextEdge.target;
  }

  return happyEdgeIds;
};

/**
 * Applies automatic layout to nodes using ELK (Eclipse Layout Kernel).
 *
 * The graph handed to ELK carries the canvas semantics so the result matches
 * how edges actually attach on screen:
 * - Handles are modeled as fixed-side ports (success → right, failure →
 *   bottom, incomplete → top), so branch targets land on the matching side.
 * - The success chain from START to END gets a straightening priority, so the
 *   main flow reads as one left-to-right row with branches hanging off it.
 * - START and END are constrained to the first and last layer.
 *
 * ELK's positions are used as-is for every node type; there is deliberately no
 * per-type post-processing (repositioning some node types but not others tears
 * the layout apart).
 *
 * @param nodes - Array of nodes to layout.
 * @param edges - Array of edges connecting the nodes (used for layout calculation).
 * @param options - Layout configuration options.
 * @returns Promise resolving to positioned nodes.
 */
export default async function applyAutoLayout(
  nodes: Node[],
  edges: Edge[],
  options: AutoLayoutOptions = {},
): Promise<Node[]> {
  const {nodeSpacing = 100, rankSpacing = 160, offsetX = 50, offsetY = 50} = options;

  if (nodes.length === 0) {
    return nodes;
  }

  const nodeIds = new Set(nodes.map((n) => n.id));
  const happyPathEdgeIds = collectHappyPathEdgeIds(nodes, edges);

  // Deduplicate edges, register a fixed-side port per distinct handle, and
  // build the ELK edge list referencing those ports.
  const portsByNode = new Map<string, Map<string, ElkPort>>();
  const registerPort = (nodeId: string, handleKey: string, side: string): string => {
    const portId = `${nodeId}__${handleKey}`;
    const ports = portsByNode.get(nodeId) ?? new Map<string, ElkPort>();
    if (!ports.has(portId)) {
      ports.set(portId, {id: portId, layoutOptions: {'elk.port.side': side}});
    }
    portsByNode.set(nodeId, ports);
    return portId;
  };

  const addedEdges = new Set<string>();
  const elkEdges: {id: string; sources: string[]; targets: string[]; layoutOptions?: Record<string, string>}[] = [];

  edges.forEach((edge) => {
    const edgeKey = `${edge.source}#${edge.sourceHandle ?? ''}->${edge.target}`;

    if (!nodeIds.has(edge.source) || !nodeIds.has(edge.target) || addedEdges.has(edgeKey)) {
      return;
    }
    addedEdges.add(edgeKey);

    const side = sourcePortSide(edge.sourceHandle);
    const sourcePortId = registerPort(edge.source, edge.sourceHandle ?? 'out', side);
    const targetPortId = registerPort(edge.target, 'in', 'WEST');

    elkEdges.push({
      id: edge.id,
      sources: [sourcePortId],
      targets: [targetPortId],
      // Straighten the happy path so the main flow forms a single row.
      ...(happyPathEdgeIds.has(edge.id) && {layoutOptions: {'elk.layered.priority.straightness': '10'}}),
    });
  });

  const elkNodes = nodes.map((node) => {
    const width = node.measured?.width ?? node.width ?? 200;
    const height = node.measured?.height ?? node.height ?? 100;
    const nodeType = node.type?.toUpperCase() ?? '';

    const layoutOptions: Record<string, string> = {};

    if (nodeType === 'START') {
      // START nodes go first (leftmost)
      layoutOptions['elk.layered.layering.layerConstraint'] = 'FIRST';
    } else if (nodeType === 'END') {
      // END nodes go last (rightmost)
      layoutOptions['elk.layered.layering.layerConstraint'] = 'LAST';
    }

    const ports = portsByNode.get(node.id);
    if (ports) {
      layoutOptions['elk.portConstraints'] = 'FIXED_SIDE';
    }

    return {
      id: node.id,
      width,
      height,
      layoutOptions,
      ...(ports && {ports: [...ports.values()]}),
    };
  });

  const elkGraph = {
    id: 'root',
    layoutOptions: {
      // Use layered algorithm - best for directed graphs
      'elk.algorithm': 'layered',
      // The layout direction is fixed: the port sides mirror the canvas's
      // physical handle geometry (success right, failure bottom, incomplete
      // top), which only composes with a left-to-right flow.
      'elk.direction': 'RIGHT',
      // Spacing between nodes in the same layer
      'elk.spacing.nodeNode': String(nodeSpacing),
      // Spacing between layers
      'elk.layered.spacing.nodeNodeBetweenLayers': String(rankSpacing),
      'elk.edgeRouting': 'POLYLINE',
      // Moderate edge clearances: the canvas draws its own smart edges, so
      // ELK's routing only needs to influence placement, not look good itself.
      // Oversized clearances inflate the layout's footprint.
      'elk.spacing.edgeNode': '60',
      'elk.spacing.edgeEdge': '40',
      'elk.layered.spacing.edgeNodeBetweenLayers': '60',
      'elk.layered.spacing.edgeEdgeBetweenLayers': '40',
      // NETWORK_SIMPLEX honors the per-edge straightness priorities set on the
      // happy path, keeping the main flow on one row.
      'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
      'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
      'elk.layered.crossingMinimization.greedySwitch.type': 'TWO_SIDED',
      'elk.layered.cycleBreaking.strategy': 'DEPTH_FIRST',
      'elk.layered.mergeEdges': 'false',
      'elk.layered.thoroughness': '50',
      // Keep the authored node/edge order as a tiebreaker so equivalent
      // layouts stay stable across runs.
      'elk.layered.considerModelOrder.strategy': 'NODES_AND_EDGES',
      'elk.spacing.portPort': '20',
    },
    children: elkNodes,
    edges: elkEdges,
  };

  try {
    // Run ELK layout algorithm
    const elk = await getElk();
    const layoutedGraph = await elk.layout(elkGraph);

    // Map the calculated positions back to React Flow nodes
    return nodes.map((node) => {
      const elkNode = layoutedGraph.children?.find((n) => n.id === node.id);

      if (elkNode?.x === undefined || elkNode.y === undefined) {
        return node;
      }

      return {
        ...node,
        position: {
          x: elkNode.x + offsetX,
          y: elkNode.y + offsetY,
        },
      };
    });
  } catch {
    // Return original nodes if layout fails
    return nodes;
  }
}
