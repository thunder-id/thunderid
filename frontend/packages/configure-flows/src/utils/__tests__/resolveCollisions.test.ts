// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import {resolveCollisions} from '../resolveCollisions';

describe('resolveCollisions', () => {
  const createNode = (id: string, x: number, y: number, width = 100, height = 50): Node => ({
    id,
    position: {x, y},
    width,
    height,
    data: {},
  });

  const defaultOptions = {
    maxIterations: 50,
    overlapThreshold: 0.5,
    margin: 0,
  };

  describe('No Collision Cases', () => {
    it('should return nodes unchanged when no collisions exist', () => {
      const nodes = [createNode('node-1', 0, 0, 100, 50), createNode('node-2', 200, 0, 100, 50)];

      const result = resolveCollisions(nodes, defaultOptions);

      expect(result[0].position).toEqual({x: 0, y: 0});
      expect(result[1].position).toEqual({x: 200, y: 0});
    });

    it('should handle empty nodes array', () => {
      const result = resolveCollisions([], defaultOptions);

      expect(result).toEqual([]);
    });

    it('should handle single node', () => {
      const nodes = [createNode('node-1', 0, 0)];

      const result = resolveCollisions(nodes, defaultOptions);

      expect(result).toHaveLength(1);
      expect(result[0].position).toEqual({x: 0, y: 0});
    });

    it('should handle nodes that are just touching', () => {
      const nodes = [createNode('node-1', 0, 0, 100, 50), createNode('node-2', 100, 0, 100, 50)];

      const result = resolveCollisions(nodes, defaultOptions);

      // Nodes touching but not overlapping significantly should not be moved
      expect(result[0].position.x).toBeLessThanOrEqual(result[1].position.x - 50);
    });
  });

  describe('Horizontal Collisions', () => {
    it('should resolve horizontal overlap by pushing nodes apart on x-axis', () => {
      // Use nodes where x-overlap is clearly smaller than y-overlap
      // so the algorithm will choose to resolve along x-axis
      const nodes = [createNode('node-1', 0, 0, 100, 200), createNode('node-2', 80, 0, 100, 200)];

      const result = resolveCollisions(nodes, defaultOptions);

      // After resolution, the x-overlap should be reduced
      // Original overlap: node1 ends at x=100, node2 starts at x=80, so 20px overlap
      const originalXOverlap = 20;
      const newXOverlap = Math.max(0, result[0].position.x + 100 - result[1].position.x);

      expect(newXOverlap).toBeLessThan(originalXOverlap);
    });

    it('should move nodes when they have significant overlap', () => {
      // Use nodes where x-overlap is smaller than y-overlap to force x-axis resolution
      const nodes = [createNode('node-1', 0, 0, 100, 200), createNode('node-2', 80, 0, 100, 200)];

      const result = resolveCollisions(nodes, defaultOptions);

      // At least one node should have moved from its original position (x or y)
      const node1Moved = result[0].position.x !== 0 || result[0].position.y !== 0;
      const node2Moved = result[1].position.x !== 80 || result[1].position.y !== 0;

      expect(node1Moved || node2Moved).toBe(true);
    });
  });

  describe('Vertical Collisions', () => {
    it('should resolve vertical overlap by pushing nodes apart', () => {
      const nodes = [createNode('node-1', 0, 0, 100, 100), createNode('node-2', 0, 50, 100, 100)];

      const result = resolveCollisions(nodes, defaultOptions);

      // Nodes should be pushed apart vertically
      const node1Bottom = result[0].position.y + 100;
      const node2Top = result[1].position.y;

      expect(node1Bottom).toBeLessThanOrEqual(node2Top + 1);
    });
  });

  describe('Multiple Collisions', () => {
    it('should resolve collisions between multiple overlapping nodes', () => {
      const nodes = [
        createNode('node-1', 0, 0, 100, 50),
        createNode('node-2', 50, 0, 100, 50),
        createNode('node-3', 100, 0, 100, 50),
      ];

      const result = resolveCollisions(nodes, defaultOptions);

      // All nodes should end up without significant overlaps
      for (let i = 0; i < result.length; i += 1) {
        for (let j = i + 1; j < result.length; j += 1) {
          const nodeA = result[i];
          const nodeB = result[j];

          const overlapX = Math.max(
            0,
            Math.min(nodeA.position.x + 100, nodeB.position.x + 100) - Math.max(nodeA.position.x, nodeB.position.x),
          );
          const overlapY = Math.max(
            0,
            Math.min(nodeA.position.y + 50, nodeB.position.y + 50) - Math.max(nodeA.position.y, nodeB.position.y),
          );

          // Overlap should be minimal after resolution
          expect(overlapX * overlapY).toBeLessThan(100);
        }
      }
    });

    it('should handle grid of overlapping nodes', () => {
      const nodes = [
        createNode('node-1', 0, 0, 100, 100),
        createNode('node-2', 50, 0, 100, 100),
        createNode('node-3', 0, 50, 100, 100),
        createNode('node-4', 50, 50, 100, 100),
      ];

      const result = resolveCollisions(nodes, defaultOptions);

      expect(result).toHaveLength(4);
      // All nodes should have updated positions
      result.forEach((node) => {
        expect(node.position).toBeDefined();
        expect(typeof node.position.x).toBe('number');
        expect(typeof node.position.y).toBe('number');
      });
    });
  });

  describe('Options', () => {
    it('should respect maxIterations option', () => {
      const nodes = [createNode('node-1', 0, 0, 100, 50), createNode('node-2', 10, 0, 100, 50)];

      const result = resolveCollisions(nodes, {
        maxIterations: 1,
        overlapThreshold: 0.5,
        margin: 0,
      });

      // Should complete without error even with limited iterations
      expect(result).toHaveLength(2);
    });

    it('should respect overlapThreshold option', () => {
      const nodes = [createNode('node-1', 0, 0, 100, 50), createNode('node-2', 99, 0, 100, 50)];

      // With high threshold, small overlaps should not trigger movement
      const result = resolveCollisions(nodes, {
        maxIterations: 50,
        overlapThreshold: 10,
        margin: 0,
      });

      // Nodes with small overlap (1px) should not be moved with high threshold
      expect(result[0].position.x).toBe(0);
      expect(result[1].position.x).toBe(99);
    });

    it('should respect margin option', () => {
      const nodes = [createNode('node-1', 0, 0, 100, 50), createNode('node-2', 110, 0, 100, 50)];

      // Without margin, nodes are not overlapping
      const resultNoMargin = resolveCollisions(nodes, {
        maxIterations: 50,
        overlapThreshold: 0.5,
        margin: 0,
      });

      expect(resultNoMargin[0].position.x).toBe(0);
      expect(resultNoMargin[1].position.x).toBe(110);

      // With margin, nodes should be considered overlapping
      const resultWithMargin = resolveCollisions(nodes, {
        maxIterations: 50,
        overlapThreshold: 0.5,
        margin: 10,
      });

      // Nodes should be pushed apart due to margin
      const gap = resultWithMargin[1].position.x - (resultWithMargin[0].position.x + 100);

      expect(gap).toBeGreaterThan(0);
    });
  });

  describe('Node Properties', () => {
    it('should preserve node properties other than position', () => {
      const nodes: Node[] = [
        {
          id: 'node-1',
          position: {x: 0, y: 0},
          width: 100,
          height: 50,
          data: {label: 'Node 1'},
          type: 'custom',
        },
        {
          id: 'node-2',
          position: {x: 50, y: 0},
          width: 100,
          height: 50,
          data: {label: 'Node 2'},
          type: 'custom',
        },
      ];

      const result = resolveCollisions(nodes, defaultOptions);

      expect(result[0].id).toBe('node-1');
      expect(result[0].data).toEqual({label: 'Node 1'});
      expect(result[0].type).toBe('custom');
      expect(result[1].id).toBe('node-2');
      expect(result[1].data).toEqual({label: 'Node 2'});
      expect(result[1].type).toBe('custom');
    });

    it('should use measured dimensions as fallback', () => {
      const nodes: Node[] = [
        {
          id: 'node-1',
          position: {x: 0, y: 0},
          measured: {width: 100, height: 50},
          data: {},
        },
        {
          id: 'node-2',
          position: {x: 50, y: 0},
          measured: {width: 100, height: 50},
          data: {},
        },
      ];

      const result = resolveCollisions(nodes, defaultOptions);

      // Collision should be detected using measured dimensions
      expect(result[0].position.x).not.toBe(result[1].position.x);
    });

    it('should handle nodes with zero dimensions', () => {
      const nodes = [createNode('node-1', 0, 0, 0, 0), createNode('node-2', 0, 0, 0, 0)];

      const result = resolveCollisions(nodes, defaultOptions);

      expect(result).toHaveLength(2);
    });
  });

  describe('Early Exit', () => {
    it('should exit early when no overlaps are found', () => {
      const nodes = [createNode('node-1', 0, 0, 100, 50), createNode('node-2', 200, 200, 100, 50)];

      const result = resolveCollisions(nodes, defaultOptions);

      // Nodes should remain unchanged
      expect(result[0].position).toEqual({x: 0, y: 0});
      expect(result[1].position).toEqual({x: 200, y: 200});
    });
  });
});
