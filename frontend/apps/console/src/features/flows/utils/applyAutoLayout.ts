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
   * Direction of the layout.
   * @default 'RIGHT' (left to right)
   */
  direction?: 'RIGHT' | 'LEFT' | 'DOWN' | 'UP';
  /**
   * Spacing between nodes (perpendicular to flow direction).
   * @default 100
   */
  nodeSpacing?: number;
  /**
   * Spacing between ranks/layers (parallel to flow direction).
   * @default 200
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
  return elkPromise;
};

/**
 * Applies automatic layout to nodes using ELK (Eclipse Layout Kernel).
 * ELK provides sophisticated graph layout algorithms that position nodes
 * to minimize edge crossings.
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
  const {direction = 'RIGHT', nodeSpacing = 150, rankSpacing = 300, offsetX = 50, offsetY = 50} = options;

  if (nodes.length === 0) {
    return nodes;
  }

  // Node type constants
  const NODE_TYPES = {
    START: 'START',
    VIEW: 'VIEW',
    EXECUTION: 'EXECUTION',
    END: 'END',
  };

  // Build ELK graph structure with layer constraints based on node type
  const elkNodes = nodes.map((node) => {
    const width = node.measured?.width ?? node.width ?? 200;
    const height = node.measured?.height ?? node.height ?? 100;
    const nodeType = node.type?.toUpperCase() ?? '';

    // Assign layer constraints for START and END nodes only
    const layoutOptions: Record<string, string> = {};

    if (nodeType === NODE_TYPES.START) {
      // START nodes go first (leftmost)
      layoutOptions['elk.layered.layering.layerConstraint'] = 'FIRST';
    } else if (nodeType === NODE_TYPES.END) {
      // END nodes go last (rightmost)
      layoutOptions['elk.layered.layering.layerConstraint'] = 'LAST';
    }

    return {
      id: node.id,
      width,
      height,
      layoutOptions,
    };
  });

  // Deduplicate edges and build ELK edge structure
  const addedEdges = new Set<string>();
  const elkEdges: {id: string; sources: string[]; targets: string[]}[] = [];
  const nodeIds = new Set(nodes.map((n) => n.id));

  edges.forEach((edge) => {
    const edgeKey = `${edge.source}->${edge.target}`;

    // Only add edges where both source and target exist
    if (nodeIds.has(edge.source) && nodeIds.has(edge.target) && !addedEdges.has(edgeKey)) {
      elkEdges.push({
        id: edge.id,
        sources: [edge.source],
        targets: [edge.target],
      });
      addedEdges.add(edgeKey);
    }
  });

  // ELK graph with layout options optimized to reduce edge-node collisions
  const elkGraph = {
    id: 'root',
    layoutOptions: {
      // Use layered algorithm - best for directed graphs
      'elk.algorithm': 'layered',
      // Direction of the layout
      'elk.direction': direction,
      // Spacing between nodes in the same layer - increased significantly
      'elk.spacing.nodeNode': String(nodeSpacing),
      // Spacing between layers - increased for better horizontal separation
      'elk.layered.spacing.nodeNodeBetweenLayers': String(rankSpacing),
      // Edge routing strategy - POLYLINE gives more flexibility for routing
      'elk.edgeRouting': 'POLYLINE',
      // Large spacing between edges and nodes to prevent overlaps
      'elk.spacing.edgeNode': '200',
      'elk.spacing.edgeEdge': '80',
      // Additional edge-node spacing in layered layout
      'elk.layered.spacing.edgeNodeBetweenLayers': '150',
      'elk.layered.spacing.edgeEdgeBetweenLayers': '80',
      // Node placement strategy - NETWORK_SIMPLEX gives better vertical distribution
      'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
      // Crossing minimization - more thorough strategy
      'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
      'elk.layered.crossingMinimization.greedySwitch.type': 'TWO_SIDED',
      // Cycle breaking strategy
      'elk.layered.cycleBreaking.strategy': 'DEPTH_FIRST',
      // Don't merge edges - keep them separate for cleaner routing
      'elk.layered.mergeEdges': 'false',
      // Higher thoroughness for better layout quality
      'elk.layered.thoroughness': '50',
      // Consider node labels for spacing
      'elk.layered.considerModelOrder.strategy': 'NODES_AND_EDGES',
      // Wrapping strategy for long edges
      'elk.layered.wrapping.strategy': 'OFF',
      // Spacing for ports (connection points)
      'elk.spacing.portPort': '20',
      // Edge label placement
      'elk.layered.edgeLabels.sideSelection': 'SMART_UP',
    },
    children: elkNodes,
    edges: elkEdges,
  };

  try {
    // Run ELK layout algorithm
    const elk = await getElk();
    const layoutedGraph = await elk.layout(elkGraph);

    // Map the calculated positions back to React Flow nodes
    let layoutedNodes = nodes.map((node) => {
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

    // Post-process: Align all VIEW nodes to the same Y position (horizontal row)
    const viewNodes = layoutedNodes.filter((n) => n.type?.toUpperCase() === NODE_TYPES.VIEW);
    if (viewNodes.length > 0) {
      // First, collect ALL node types and calculate their center positions
      const startNodes = layoutedNodes.filter((n) => n.type?.toUpperCase() === NODE_TYPES.START);
      const endNodes = layoutedNodes.filter((n) => n.type?.toUpperCase() === NODE_TYPES.END);

      // Calculate the overall bounding box center Y for all nodes (START, END, VIEWs)
      const allRelevantNodes = [...startNodes, ...endNodes, ...viewNodes];

      let minCenterY = Infinity;
      let maxCenterY = -Infinity;

      allRelevantNodes.forEach((node) => {
        const nodeHeight = node.measured?.height ?? node.height ?? 100;
        const centerY = node.position.y + nodeHeight / 2;
        minCenterY = Math.min(minCenterY, centerY);
        maxCenterY = Math.max(maxCenterY, centerY);
      });

      // Use the middle point as the target center Y
      const targetCenterY = (minCenterY + maxCenterY) / 2;

      // Align all VIEW nodes so their centers align with targetCenterY
      // Each view may have different height, so calculate Y individually
      layoutedNodes = layoutedNodes.map((node) => {
        if (node.type?.toUpperCase() === NODE_TYPES.VIEW) {
          const nodeHeight = node.measured?.height ?? node.height ?? 100;
          const nodeY = targetCenterY - nodeHeight / 2;
          return {
            ...node,
            position: {
              ...node.position,
              y: nodeY,
            },
          };
        }
        return node;
      });

      const viewCenterY = targetCenterY;

      // Align START and END nodes to the same horizontal level (centered with Views)
      layoutedNodes = layoutedNodes.map((node) => {
        const nodeType = node.type?.toUpperCase() ?? '';
        if (nodeType === NODE_TYPES.START || nodeType === NODE_TYPES.END) {
          const nodeHeight = node.measured?.height ?? node.height ?? 50;
          return {
            ...node,
            position: {
              ...node.position,
              y: viewCenterY - nodeHeight / 2,
            },
          };
        }
        return node;
      });

      // Position EXECUTION nodes - try to keep them on the same horizontal line as views
      // Only move them below if they would overlap with other nodes
      const executionNodes = layoutedNodes.filter((n) => n.type?.toUpperCase() === NODE_TYPES.EXECUTION);

      if (executionNodes.length > 0) {
        // Sort execution nodes by their X position to maintain left-to-right order
        const sortedExecutionNodes = [...executionNodes].sort((a, b) => a.position.x - b.position.x);

        // Try to center execution nodes vertically with the views first
        layoutedNodes = layoutedNodes.map((node) => {
          if (node.type?.toUpperCase() === NODE_TYPES.EXECUTION) {
            const nodeHeight = node.measured?.height ?? node.height ?? 100;
            return {
              ...node,
              position: {
                ...node.position,
                y: viewCenterY - nodeHeight / 2,
              },
            };
          }
          return node;
        });

        // Track placed execution nodes with their final positions for overlap detection
        const placedExecutionNodes: {
          id: string;
          x: number;
          y: number;
          width: number;
          height: number;
        }[] = [];

        // Now check for overlaps and move conflicting execution nodes below
        // Find the maximum bottom Y of all view nodes
        const viewBottoms = layoutedNodes
          .filter((n) => n.type?.toUpperCase() === NODE_TYPES.VIEW)
          .map((n) => {
            const height = n.measured?.height ?? n.height ?? 100;
            return n.position.y + height;
          });
        const maxViewBottom = viewBottoms.length > 0 ? Math.max(...viewBottoms) : 0;
        const viewBottomY = maxViewBottom + nodeSpacing;
        const horizontalPadding = 50;
        const verticalPadding = 30;

        // Check each execution node for overlap with views AND other execution nodes
        sortedExecutionNodes.forEach((execNode) => {
          const execX = execNode.position.x;
          const execWidth = execNode.measured?.width ?? execNode.width ?? 200;
          const execHeight = execNode.measured?.height ?? execNode.height ?? 100;
          const execRight = execX + execWidth;
          let execY = viewCenterY - execHeight / 2; // Start at view center

          // Check if this execution node overlaps horizontally with any view
          const overlapsWithView = viewNodes.some((viewNode) => {
            const viewX = viewNode.position.x;
            const viewWidth = viewNode.measured?.width ?? viewNode.width ?? 350;
            const viewRight = viewX + viewWidth;

            // Check horizontal overlap (with some padding)
            return execRight + horizontalPadding > viewX && execX - horizontalPadding < viewRight;
          });

          if (overlapsWithView) {
            // Need to move below views
            execY = viewBottomY;
          }

          // Check for overlap with already placed execution nodes
          // Keep moving down until no overlap
          // Helper to check if a Y position overlaps with any placed node
          const checkOverlapAtY = (testY: number): {overlaps: boolean; nextY: number} => {
            let result = {overlaps: false, nextY: testY};

            placedExecutionNodes.forEach((placed) => {
              if (result.overlaps) return; // Already found overlap

              // Check if rectangles overlap (with padding)
              const horizontalOverlap =
                execRight + horizontalPadding > placed.x && execX - horizontalPadding < placed.x + placed.width;

              const verticalOverlap =
                testY + execHeight + verticalPadding > placed.y && testY - verticalPadding < placed.y + placed.height;

              if (horizontalOverlap && verticalOverlap) {
                result = {
                  overlaps: true,
                  nextY: placed.y + placed.height + verticalPadding + nodeSpacing,
                };
              }
            });

            return result;
          };

          // Find non-overlapping Y position
          let iterations = 0;
          const maxIterations = 20;
          let overlapCheck = checkOverlapAtY(execY);

          while (overlapCheck.overlaps && iterations < maxIterations) {
            iterations += 1;
            execY = overlapCheck.nextY;
            overlapCheck = checkOverlapAtY(execY);
          }

          // Update the node position
          layoutedNodes = layoutedNodes.map((node) => {
            if (node.id === execNode.id) {
              return {
                ...node,
                position: {
                  ...node.position,
                  y: execY,
                },
              };
            }
            return node;
          });

          // Add to placed nodes
          placedExecutionNodes.push({
            id: execNode.id,
            x: execX,
            y: execY,
            width: execWidth,
            height: execHeight,
          });
        });
      }
    }

    return layoutedNodes;
  } catch {
    // Return original nodes if layout fails
    return nodes;
  }
}
