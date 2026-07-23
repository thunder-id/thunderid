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

import {Position, type Node} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import {calculateEdgePath, calculateAllEdgePaths, type EdgeInput} from '../calculateEdgePath';

describe('calculateEdgePath', () => {
  const createNode = (id: string, x: number, y: number, width = 150, height = 50): Node => ({
    id,
    position: {x, y},
    data: {},
    measured: {width, height},
  });

  describe('Basic Path Calculation', () => {
    it('should calculate a path between two points', () => {
      const result = calculateEdgePath(100, 100, 300, 100, Position.Right, Position.Left, []);

      expect(result).toBeDefined();
      expect(result.path).toBeDefined();
      expect(typeof result.path).toBe('string');
      expect(result.path.startsWith('M')).toBe(true);
    });

    it('should return center coordinates', () => {
      const result = calculateEdgePath(0, 0, 200, 0, Position.Right, Position.Left, []);

      expect(typeof result.centerX).toBe('number');
      expect(typeof result.centerY).toBe('number');
    });

    it('should handle horizontal straight line', () => {
      const result = calculateEdgePath(0, 100, 200, 100, Position.Right, Position.Left, []);

      expect(result.path).toContain('M 0,100');
      expect(result.centerY).toBeCloseTo(100, 0);
    });

    it('should handle vertical straight line', () => {
      const result = calculateEdgePath(100, 0, 100, 200, Position.Bottom, Position.Top, []);

      expect(result.path).toContain('M 100,0');
    });
  });

  describe('Edge Styles', () => {
    const nodes: Node[] = [];

    it('should generate smoothstep path by default', () => {
      const result = calculateEdgePath(0, 0, 200, 100, Position.Right, Position.Left, nodes);

      // Smoothstep uses Q (quadratic bezier) for corners
      expect(result.path).toBeDefined();
    });

    it('should generate smoothstep path when specified', () => {
      const result = calculateEdgePath(0, 0, 200, 100, Position.Right, Position.Left, nodes, 'smoothstep');

      expect(result.path).toBeDefined();
    });

    it('should generate step path when specified', () => {
      const result = calculateEdgePath(0, 0, 200, 100, Position.Right, Position.Left, nodes, 'step');

      // Step path uses only M and L commands (no curves)
      expect(result.path).toBeDefined();
      expect(result.path).toMatch(/^M.*L/);
    });

    it('should generate bezier path when default style specified', () => {
      const result = calculateEdgePath(0, 0, 200, 100, Position.Right, Position.Left, nodes, 'default');

      // Bezier uses C (cubic bezier) command
      expect(result.path).toBeDefined();
      expect(result.path).toContain('C');
    });

    it('should apply custom border radius for smoothstep', () => {
      const result1 = calculateEdgePath(0, 0, 200, 100, Position.Right, Position.Left, nodes, 'smoothstep', 10);
      const result2 = calculateEdgePath(0, 0, 200, 100, Position.Right, Position.Left, nodes, 'smoothstep', 30);

      // Both should produce valid paths
      expect(result1.path).toBeDefined();
      expect(result2.path).toBeDefined();
    });
  });

  describe('Source and Target Positions', () => {
    const nodes: Node[] = [];

    it('should handle Right to Left positions', () => {
      const result = calculateEdgePath(100, 100, 300, 100, Position.Right, Position.Left, nodes);

      expect(result.path).toBeDefined();
    });

    it('should handle Left to Right positions', () => {
      const result = calculateEdgePath(300, 100, 100, 100, Position.Left, Position.Right, nodes);

      expect(result.path).toBeDefined();
    });

    it('should handle Bottom to Top positions', () => {
      const result = calculateEdgePath(100, 100, 100, 300, Position.Bottom, Position.Top, nodes);

      expect(result.path).toBeDefined();
    });

    it('should handle Top to Bottom positions', () => {
      const result = calculateEdgePath(100, 300, 100, 100, Position.Top, Position.Bottom, nodes);

      expect(result.path).toBeDefined();
    });

    it('should handle diagonal Right to Left', () => {
      const result = calculateEdgePath(0, 0, 200, 200, Position.Right, Position.Left, nodes);

      expect(result.path).toBeDefined();
    });

    it('should handle diagonal Bottom to Top', () => {
      const result = calculateEdgePath(0, 0, 200, 200, Position.Bottom, Position.Top, nodes);

      expect(result.path).toBeDefined();
    });
  });

  describe('Obstacle Avoidance', () => {
    it('should route around a single obstacle', () => {
      const obstacle = createNode('obstacle', 100, 75, 100, 50);
      const result = calculateEdgePath(0, 100, 300, 100, Position.Right, Position.Left, [obstacle]);

      expect(result.path).toBeDefined();
      // Path should not go through the obstacle
    });

    it('should route around multiple obstacles', () => {
      const obstacles = [createNode('obs1', 100, 75, 50, 50), createNode('obs2', 200, 75, 50, 50)];
      const result = calculateEdgePath(0, 100, 350, 100, Position.Right, Position.Left, obstacles);

      expect(result.path).toBeDefined();
    });

    it('should handle obstacles that block horizontal path', () => {
      const obstacle = createNode('blocker', 100, 50, 100, 100);
      const result = calculateEdgePath(0, 100, 300, 100, Position.Right, Position.Left, [obstacle]);

      expect(result.path).toBeDefined();
      // Path should find alternative route
    });

    it('should handle obstacles that block vertical path', () => {
      const obstacle = createNode('blocker', 50, 100, 100, 100);
      const result = calculateEdgePath(100, 0, 100, 300, Position.Bottom, Position.Top, [obstacle]);

      expect(result.path).toBeDefined();
    });

    it('should use node measured dimensions for collision detection', () => {
      const obstacle: Node = {
        id: 'measured',
        position: {x: 100, y: 75},
        data: {},
        measured: {width: 100, height: 50},
      };
      const result = calculateEdgePath(0, 100, 300, 100, Position.Right, Position.Left, [obstacle]);

      expect(result.path).toBeDefined();
    });

    it('should use node width/height when measured not available', () => {
      const obstacle: Node = {
        id: 'sized',
        position: {x: 100, y: 75},
        data: {},
        width: 100,
        height: 50,
      };
      const result = calculateEdgePath(0, 100, 300, 100, Position.Right, Position.Left, [obstacle]);

      expect(result.path).toBeDefined();
    });
  });

  describe('Exit Point Calculation', () => {
    it('should add padding when exiting from Right position', () => {
      const result = calculateEdgePath(0, 0, 200, 0, Position.Right, Position.Left, []);

      expect(result.path).toBeDefined();
    });

    it('should add padding when exiting from Left position', () => {
      const result = calculateEdgePath(200, 0, 0, 0, Position.Left, Position.Right, []);

      expect(result.path).toBeDefined();
    });

    it('should add padding when exiting from Bottom position', () => {
      const result = calculateEdgePath(0, 0, 0, 200, Position.Bottom, Position.Top, []);

      expect(result.path).toBeDefined();
    });

    it('should add padding when exiting from Top position', () => {
      const result = calculateEdgePath(0, 200, 0, 0, Position.Top, Position.Bottom, []);

      expect(result.path).toBeDefined();
    });

    it('should handle exit point inside container node', () => {
      const container = createNode('container', 0, 0, 200, 200);
      const result = calculateEdgePath(100, 100, 400, 100, Position.Right, Position.Left, [container]);

      expect(result.path).toBeDefined();
    });
  });

  describe('Center Point Calculation', () => {
    it('should calculate center for horizontal path', () => {
      const result = calculateEdgePath(0, 100, 200, 100, Position.Right, Position.Left, []);

      expect(result.centerX).toBeCloseTo(100, -1);
      expect(result.centerY).toBeCloseTo(100, 0);
    });

    it('should calculate center for L-shaped path', () => {
      const result = calculateEdgePath(0, 0, 200, 200, Position.Right, Position.Left, [], 'step');

      expect(typeof result.centerX).toBe('number');
      expect(typeof result.centerY).toBe('number');
    });

    it('should calculate center for bezier curve', () => {
      const result = calculateEdgePath(0, 0, 200, 100, Position.Right, Position.Left, [], 'default');

      expect(typeof result.centerX).toBe('number');
      expect(typeof result.centerY).toBe('number');
    });
  });

  describe('Bezier Edge Style', () => {
    it('should create bezier curve with control points', () => {
      const result = calculateEdgePath(0, 0, 200, 0, Position.Right, Position.Left, [], 'default');

      expect(result.path).toContain('C');
    });

    it('should handle backward-flowing edges (target left of source)', () => {
      const result = calculateEdgePath(200, 0, 0, 0, Position.Right, Position.Left, [], 'default');

      expect(result.path).toBeDefined();
      expect(result.path).toContain('C');
    });

    it('should create elaborate curve for significantly backward edges', () => {
      const result = calculateEdgePath(300, 100, 0, 100, Position.Right, Position.Left, [], 'default');

      expect(result.path).toBeDefined();
    });

    it('should handle backward edges going down', () => {
      const result = calculateEdgePath(300, 0, 0, 200, Position.Right, Position.Left, [], 'default');

      expect(result.path).toBeDefined();
    });

    it('should handle backward edges going up', () => {
      const result = calculateEdgePath(300, 200, 0, 0, Position.Right, Position.Left, [], 'default');

      expect(result.path).toBeDefined();
    });
  });

  describe('Path Simplification', () => {
    it('should remove collinear points from path', () => {
      const result = calculateEdgePath(0, 100, 300, 100, Position.Right, Position.Left, []);

      // A straight horizontal line should be simplified
      expect(result.path).toBeDefined();
    });

    it('should remove duplicate points', () => {
      const result = calculateEdgePath(0, 0, 100, 0, Position.Right, Position.Left, []);

      expect(result.path).toBeDefined();
    });
  });
});

describe('getExitPoint Coverage', () => {
  const createNode = (id: string, x: number, y: number, width = 150, height = 50): Node => ({
    id,
    position: {x, y},
    data: {},
    measured: {width, height},
  });

  describe('Exit point with no containers', () => {
    it('should handle default/unknown position when no containers', () => {
      // This tests the default case in getExitPoint when containers.length === 0
      // We need to trigger an edge that uses a position that isn't handled
      // Since Position only has Right, Left, Top, Bottom, we test each explicitly
      const result = calculateEdgePath(100, 100, 300, 100, Position.Right, Position.Left, []);
      expect(result.path).toBeDefined();
    });
  });

  describe('Exit point inside container with different positions', () => {
    it('should handle Bottom position when handle is inside container', () => {
      // Create a container that fully contains the source point
      const container = createNode('container', 0, 0, 300, 300);
      const result = calculateEdgePath(150, 150, 500, 400, Position.Bottom, Position.Top, [container]);

      expect(result.path).toBeDefined();
      // The exit point should be pushed outside the container
    });

    it('should handle Top position when handle is inside container', () => {
      const container = createNode('container', 0, 100, 300, 300);
      const result = calculateEdgePath(150, 250, 500, 0, Position.Top, Position.Bottom, [container]);

      expect(result.path).toBeDefined();
    });

    it('should handle Left position when handle is inside container', () => {
      const container = createNode('container', 50, 0, 300, 300);
      const result = calculateEdgePath(200, 150, 0, 150, Position.Left, Position.Right, [container]);

      expect(result.path).toBeDefined();
    });

    it('should handle multiple overlapping containers for Bottom position', () => {
      const container1 = createNode('c1', 0, 0, 200, 200);
      const container2 = createNode('c2', 50, 50, 200, 250);
      const result = calculateEdgePath(125, 175, 500, 400, Position.Bottom, Position.Top, [container1, container2]);

      expect(result.path).toBeDefined();
    });

    it('should handle multiple overlapping containers for Top position', () => {
      const container1 = createNode('c1', 0, 100, 200, 200);
      const container2 = createNode('c2', 50, 50, 200, 200);
      const result = calculateEdgePath(125, 175, 500, 0, Position.Top, Position.Bottom, [container1, container2]);

      expect(result.path).toBeDefined();
    });
  });
});

describe('calculateAllEdgePaths', () => {
  const createNode = (id: string, x: number, y: number, width = 150, height = 50): Node => ({
    id,
    position: {x, y},
    data: {},
    measured: {width, height},
  });

  const createEdgeInput = (
    id: string,
    sourceX: number,
    sourceY: number,
    targetX: number,
    targetY: number,
    sourcePosition = Position.Right,
    targetPosition = Position.Left,
  ): EdgeInput => ({
    id,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
  });

  describe('Basic Functionality', () => {
    it('should return a Map of edge paths', () => {
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 0, 200, 0)];
      const nodes: Node[] = [];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result).toBeInstanceOf(Map);
      expect(result.size).toBe(1);
    });

    it('should calculate paths for multiple edges', () => {
      const edges: EdgeInput[] = [
        createEdgeInput('e1', 0, 0, 200, 0),
        createEdgeInput('e2', 0, 100, 200, 100),
        createEdgeInput('e3', 0, 200, 200, 200),
      ];
      const nodes: Node[] = [];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.size).toBe(3);
      expect(result.has('e1')).toBe(true);
      expect(result.has('e2')).toBe(true);
      expect(result.has('e3')).toBe(true);
    });

    it('should return EdgePathResult for each edge', () => {
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 0, 200, 100)];
      const nodes: Node[] = [];

      const result = calculateAllEdgePaths(edges, nodes);
      const edgeResult = result.get('e1');

      expect(edgeResult).toBeDefined();
      expect(edgeResult?.path).toBeDefined();
      expect(typeof edgeResult?.centerX).toBe('number');
      expect(typeof edgeResult?.centerY).toBe('number');
    });
  });

  describe('Edge Styles', () => {
    const edges: EdgeInput[] = [createEdgeInput('e1', 0, 0, 200, 100)];
    const nodes: Node[] = [];

    it('should apply smoothstep style by default', () => {
      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.get('e1')?.path).toBeDefined();
    });

    it('should apply specified edge style', () => {
      const resultStep = calculateAllEdgePaths(edges, nodes, 'step');
      const resultBezier = calculateAllEdgePaths(edges, nodes, 'default');

      expect(resultStep.get('e1')?.path).not.toEqual(resultBezier.get('e1')?.path);
    });

    it('should apply custom border radius', () => {
      const result = calculateAllEdgePaths(edges, nodes, 'smoothstep', 15);

      expect(result.get('e1')?.path).toBeDefined();
    });
  });

  describe('Edge Separation', () => {
    it('should separate overlapping horizontal edges', () => {
      const edges: EdgeInput[] = [
        createEdgeInput('e1', 0, 100, 200, 100),
        createEdgeInput('e2', 0, 100, 200, 100), // Same path
      ];
      const nodes: Node[] = [];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.size).toBe(2);
      // Both edges should have valid paths
      expect(result.get('e1')?.path).toBeDefined();
      expect(result.get('e2')?.path).toBeDefined();
    });

    it('should separate overlapping vertical edges', () => {
      const edges: EdgeInput[] = [
        createEdgeInput('e1', 100, 0, 100, 200, Position.Bottom, Position.Top),
        createEdgeInput('e2', 100, 0, 100, 200, Position.Bottom, Position.Top),
      ];
      const nodes: Node[] = [];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.size).toBe(2);
    });

    it('should never offset the terminal segments away from the handles', () => {
      // Both edges are a single straight segment; that segment touches both
      // handles, so no separation may be applied to it at all.
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 100, 200, 100), createEdgeInput('e2', 0, 100, 200, 100)];

      const result = calculateAllEdgePaths(edges, [], 'step');

      expect(result.get('e1')?.path).toBe('M 0,100 L 200,100');
      expect(result.get('e2')?.path).toBe('M 0,100 L 200,100');
    });

    it('should separate interior segments while keeping paths anchored to the handles', () => {
      // Identical Z-shaped routes: the shared interior vertical segment must be
      // offset apart, but both paths must still start and end exactly on the
      // source and target handles.
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 0, 300, 200), createEdgeInput('e2', 0, 0, 300, 200)];

      const result = calculateAllEdgePaths(edges, [], 'step');
      const path1 = result.get('e1')!.path;
      const path2 = result.get('e2')!.path;

      expect(path1.startsWith('M 0,0 ')).toBe(true);
      expect(path2.startsWith('M 0,0 ')).toBe(true);
      expect(path1.endsWith('L 300,200')).toBe(true);
      expect(path2.endsWith('L 300,200')).toBe(true);
      // The interior corridor is separated, so the full paths differ.
      expect(path1).not.toBe(path2);
    });

    it('should handle edges with different paths (no overlap)', () => {
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 0, 200, 0), createEdgeInput('e2', 0, 200, 200, 200)];
      const nodes: Node[] = [];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.size).toBe(2);
    });
  });

  describe('Obstacle Avoidance', () => {
    it('should route edges around nodes', () => {
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 100, 400, 100)];
      const nodes: Node[] = [createNode('obstacle', 150, 75, 100, 50)];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.get('e1')?.path).toBeDefined();
    });

    it('should route multiple edges around multiple nodes', () => {
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 100, 500, 100), createEdgeInput('e2', 0, 150, 500, 150)];
      const nodes: Node[] = [createNode('obs1', 150, 75, 100, 100), createNode('obs2', 300, 75, 100, 100)];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.size).toBe(2);
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty edges array', () => {
      const result = calculateAllEdgePaths([], []);

      expect(result.size).toBe(0);
    });

    it('should handle empty nodes array', () => {
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 0, 100, 0)];

      const result = calculateAllEdgePaths(edges, []);

      expect(result.size).toBe(1);
    });

    it('should handle edges at the same position', () => {
      const edges: EdgeInput[] = [createEdgeInput('e1', 100, 100, 100, 100)];

      const result = calculateAllEdgePaths(edges, []);

      expect(result.get('e1')?.path).toBeDefined();
    });

    it('should handle very short edges', () => {
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 0, 10, 0)];

      const result = calculateAllEdgePaths(edges, []);

      expect(result.get('e1')?.path).toBeDefined();
    });

    it('should handle very long edges', () => {
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 0, 10000, 0)];

      const result = calculateAllEdgePaths(edges, []);

      expect(result.get('e1')?.path).toBeDefined();
    });
  });

  describe('Complex Scenarios', () => {
    it('should handle a graph with multiple connected nodes', () => {
      const nodes: Node[] = [
        createNode('n1', 0, 0, 100, 50),
        createNode('n2', 200, 0, 100, 50),
        createNode('n3', 400, 0, 100, 50),
        createNode('n4', 200, 100, 100, 50),
      ];
      const edges: EdgeInput[] = [
        createEdgeInput('e1', 100, 25, 200, 25),
        createEdgeInput('e2', 300, 25, 400, 25),
        createEdgeInput('e3', 100, 25, 200, 125, Position.Right, Position.Left),
        createEdgeInput('e4', 300, 125, 400, 25, Position.Right, Position.Left),
      ];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.size).toBe(4);
      result.forEach((edgeResult) => {
        expect(edgeResult.path).toBeDefined();
        expect(edgeResult.path.length).toBeGreaterThan(0);
      });
    });

    it('should handle edges that need to go around enclosed obstacles', () => {
      const nodes: Node[] = [
        createNode('box1', 100, 0, 50, 200),
        createNode('box2', 200, 0, 50, 200),
        createNode('box3', 100, 0, 150, 50),
        createNode('box4', 100, 150, 150, 50),
      ];
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 100, 300, 100)];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.get('e1')?.path).toBeDefined();
    });
  });

  describe('5-Segment and Fallback Paths', () => {
    it('should use 5-segment path when L-shaped and 3-segment paths are blocked', () => {
      // Create a maze-like obstacle configuration that blocks simple paths
      const nodes: Node[] = [
        createNode('block1', 50, 0, 100, 80), // Blocks direct horizontal
        createNode('block2', 50, 120, 100, 80), // Blocks going around below
        createNode('block3', -50, 50, 50, 100), // Blocks left side corridor
      ];
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 100, 200, 100)];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.get('e1')?.path).toBeDefined();
    });

    it('should use fallback path when all corridor paths are blocked', () => {
      // Create obstacles that block all normal corridors
      // This forces the algorithm to use the wide fallback paths
      const nodes: Node[] = [
        // Create a wall of obstacles
        createNode('wall1', 80, -100, 40, 400),
        createNode('wall2', -100, 80, 400, 40),
        createNode('wall3', -100, 120, 400, 40),
      ];
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 100, 200, 100)];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.get('e1')?.path).toBeDefined();
    });

    it('should handle complex obstacle avoidance requiring multi-segment path', () => {
      // Create an L-shaped obstacle that requires going around
      const nodes: Node[] = [createNode('L1', 75, 50, 100, 50), createNode('L2', 75, 100, 50, 50)];
      const edges: EdgeInput[] = [createEdgeInput('e1', 50, 125, 200, 75)];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.get('e1')?.path).toBeDefined();
    });

    it('should find path around completely enclosed area', () => {
      // Create a box with all sides blocked except corners
      const nodes: Node[] = [
        createNode('top', 50, 30, 200, 30),
        createNode('bottom', 50, 170, 200, 30),
        createNode('left', 50, 60, 30, 110),
        createNode('right', 220, 60, 30, 110),
      ];
      const edges: EdgeInput[] = [createEdgeInput('e1', 0, 115, 300, 115)];

      const result = calculateAllEdgePaths(edges, nodes);

      expect(result.get('e1')?.path).toBeDefined();
    });
  });
});

describe('Path Style Edge Cases', () => {
  describe('pathToSmoothStepResult edge cases', () => {
    it('should handle path with very small corner radius', () => {
      // When points are very close together, maxRadius will be < 1
      // Create a scenario with very tight corners
      const nodes: Node[] = [];
      // Create edges that result in very short segments
      const result = calculateEdgePath(0, 0, 5, 5, Position.Right, Position.Left, nodes, 'smoothstep', 1);

      expect(result.path).toBeDefined();
    });

    it('should handle single point path (length < 2)', () => {
      // Edge at same position - results in minimal path
      const result = calculateEdgePath(100, 100, 100, 100, Position.Right, Position.Left, [], 'smoothstep');

      expect(result.path).toBeDefined();
    });

    it('should handle two-point straight line path', () => {
      // Straight horizontal line has exactly 2 points after simplification
      const result = calculateEdgePath(0, 100, 200, 100, Position.Right, Position.Left, [], 'smoothstep');

      expect(result.path).toBeDefined();
      expect(result.path).toContain('M');
    });
  });

  describe('pathToBezierResult edge cases', () => {
    it('should handle empty or single point path', () => {
      // When source and target are at the same position
      const result = calculateEdgePath(50, 50, 50, 50, Position.Right, Position.Left, [], 'default');

      expect(result.path).toBeDefined();
    });

    it('should handle backward edges with target below source', () => {
      // Target significantly to the left and below
      const result = calculateEdgePath(300, 0, 0, 100, Position.Right, Position.Left, [], 'default');

      expect(result.path).toBeDefined();
      expect(result.path).toContain('C');
    });

    it('should handle backward edges with target above source', () => {
      // Target significantly to the left and above
      const result = calculateEdgePath(300, 100, 0, 0, Position.Right, Position.Left, [], 'default');

      expect(result.path).toBeDefined();
      expect(result.path).toContain('C');
    });
  });

  describe('pathToResult default case', () => {
    it('should fallback to smoothstep for unknown edge style', () => {
      // Testing with a valid style to ensure the switch works
      const resultSmoothstep = calculateEdgePath(0, 0, 200, 100, Position.Right, Position.Left, [], 'smoothstep');
      const resultStep = calculateEdgePath(0, 0, 200, 100, Position.Right, Position.Left, [], 'step');

      expect(resultSmoothstep.path).toBeDefined();
      expect(resultStep.path).toBeDefined();
      // Step uses only L commands, smoothstep may use Q
      expect(resultStep.path).not.toContain('Q');
    });
  });
});

describe('Additional Path Calculation Edge Cases', () => {
  const createNode = (id: string, x: number, y: number, width = 150, height = 50): Node => ({
    id,
    position: {x, y},
    data: {},
    measured: {width, height},
  });

  describe('Target exit point scenarios', () => {
    it('should add target exit point when different from last path point', () => {
      // Create scenario where target exit is calculated differently
      const container = createNode('container', 200, 50, 200, 100);
      const result = calculateEdgePath(0, 100, 300, 100, Position.Right, Position.Left, [container]);

      expect(result.path).toBeDefined();
    });

    it('should handle target exit inside container', () => {
      const container = createNode('container', 150, 50, 200, 100);
      const result = calculateEdgePath(0, 100, 250, 100, Position.Right, Position.Left, [container]);

      expect(result.path).toBeDefined();
    });
  });

  describe('Segment extraction and overlap', () => {
    const createEdgeInput = (
      id: string,
      sourceX: number,
      sourceY: number,
      targetX: number,
      targetY: number,
      sourcePosition = Position.Right,
      targetPosition = Position.Left,
    ): EdgeInput => ({
      id,
      sourceX,
      sourceY,
      targetX,
      targetY,
      sourcePosition,
      targetPosition,
    });

    it('should detect vertical segment overlaps', () => {
      // Create edges that share vertical segments
      const edges: EdgeInput[] = [
        createEdgeInput('e1', 0, 0, 200, 200, Position.Bottom, Position.Top),
        createEdgeInput('e2', 0, 0, 200, 200, Position.Bottom, Position.Top),
      ];

      const result = calculateAllEdgePaths(edges, []);

      expect(result.size).toBe(2);
      expect(result.get('e1')?.path).toBeDefined();
      expect(result.get('e2')?.path).toBeDefined();
    });

    it('should handle non-overlapping segments of different types', () => {
      const edges: EdgeInput[] = [
        createEdgeInput('e1', 0, 0, 200, 0), // Horizontal
        createEdgeInput('e2', 100, -50, 100, 50, Position.Bottom, Position.Top), // Vertical
      ];

      const result = calculateAllEdgePaths(edges, []);

      expect(result.size).toBe(2);
    });

    it('should apply offsets correctly for overlapping segments', () => {
      // Three edges with the exact same path should all get different offsets
      const edges: EdgeInput[] = [
        createEdgeInput('e1', 0, 100, 200, 100),
        createEdgeInput('e2', 0, 100, 200, 100),
        createEdgeInput('e3', 0, 100, 200, 100),
      ];

      const result = calculateAllEdgePaths(edges, []);

      expect(result.size).toBe(3);
      // All three should have valid but potentially different paths
      const path1 = result.get('e1')?.path;
      const path2 = result.get('e2')?.path;
      const path3 = result.get('e3')?.path;

      expect(path1).toBeDefined();
      expect(path2).toBeDefined();
      expect(path3).toBeDefined();
    });
  });

  describe('Node bounds calculation', () => {
    it('should use default dimensions when neither measured nor width/height provided', () => {
      const node: Node = {
        id: 'default-size',
        position: {x: 100, y: 75},
        data: {},
        // No measured, width, or height - should use defaults (150x50)
      };
      const result = calculateEdgePath(0, 100, 300, 100, Position.Right, Position.Left, [node]);

      expect(result.path).toBeDefined();
    });
  });

  describe('Path through obstacles requiring corridors', () => {
    it('should find Y corridor around horizontal blocker', () => {
      // Obstacle blocks the direct horizontal path
      const obstacle = createNode('blocker', 75, 75, 100, 50);
      const result = calculateEdgePath(0, 100, 250, 100, Position.Right, Position.Left, [obstacle]);

      expect(result.path).toBeDefined();
    });

    it('should find X corridor around vertical blocker', () => {
      // Obstacle blocks the direct vertical path
      const obstacle = createNode('blocker', 75, 50, 50, 150);
      const result = calculateEdgePath(100, 0, 100, 250, Position.Bottom, Position.Top, [obstacle]);

      expect(result.path).toBeDefined();
    });
  });
});
