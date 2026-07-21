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

import type {Node, Edge} from '@xyflow/react';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import applyAutoLayout from '../applyAutoLayout';

interface ElkTestPort {
  id: string;
  layoutOptions: Record<string, string>;
}

interface ElkTestNode {
  id: string;
  width: number;
  height: number;
  layoutOptions: Record<string, string>;
  ports?: ElkTestPort[];
}

interface ElkTestEdge {
  id: string;
  sources: string[];
  targets: string[];
  layoutOptions?: Record<string, string>;
}

interface ElkTestGraph {
  id: string;
  layoutOptions: Record<string, string>;
  children: ElkTestNode[];
  edges: ElkTestEdge[];
}

// Capture the graph handed to ELK so the tests can assert on the layout
// semantics (ports, constraints, priorities) rather than on ELK's output.
let lastGraph: ElkTestGraph | null = null;
let failLayout = false;

vi.mock('elkjs/lib/elk.bundled.js', () => ({
  default: class MockELK {
    layout(graph: ElkTestGraph) {
      lastGraph = graph;
      if (failLayout) {
        return Promise.reject(new Error('layout failed'));
      }
      const layoutedChildren = graph.children.map((child, index) => ({
        ...child,
        x: index * 200,
        y: 100,
      }));

      return Promise.resolve({
        ...graph,
        children: layoutedChildren,
      });
    }
  },
}));

describe('applyAutoLayout', () => {
  const createNode = (
    id: string,
    type: string,
    position = {x: 0, y: 0},
    measured?: {width: number; height: number},
  ): Node => ({
    id,
    type,
    position,
    data: {},
    ...(measured && {measured}),
  });

  const createEdge = (id: string, source: string, target: string, sourceHandle?: string): Edge => ({
    id,
    source,
    target,
    ...(sourceHandle && {sourceHandle}),
  });

  beforeEach(() => {
    lastGraph = null;
    failLayout = false;
  });

  describe('Empty and Single Node Cases', () => {
    it('should return empty array when no nodes provided', async () => {
      const result = await applyAutoLayout([], []);

      expect(result).toEqual([]);
    });

    it('should position a single node', async () => {
      const nodes: Node[] = [createNode('node1', 'VIEW', {x: 50, y: 50})];

      const result = await applyAutoLayout(nodes, []);

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('node1');
    });
  });

  describe('Positioning', () => {
    it('should apply ELK positions plus offsets to every node type uniformly', async () => {
      const nodes: Node[] = [
        createNode('start', 'START'),
        createNode('view1', 'VIEW'),
        createNode('exec1', 'TASK_EXECUTION'),
        createNode('call1', 'CALL'),
        createNode('end', 'END'),
      ];

      const result = await applyAutoLayout(nodes, [], {offsetX: 10, offsetY: 20});

      // Mock ELK returns x = index * 200, y = 100 for all nodes; no node type
      // gets special post-processing.
      result.forEach((node, index) => {
        expect(node.position).toEqual({x: index * 200 + 10, y: 120});
      });
    });

    it('should apply default offsets when none provided', async () => {
      const nodes: Node[] = [createNode('a', 'VIEW'), createNode('b', 'TASK_EXECUTION')];

      const result = await applyAutoLayout(nodes, []);

      expect(result[0].position).toEqual({x: 50, y: 150});
      expect(result[1].position).toEqual({x: 250, y: 150});
    });

    it('should preserve node data after layout', async () => {
      const nodes: Node[] = [
        {
          ...createNode('view1', 'VIEW'),
          data: {label: 'My View', components: [{id: 'comp1'}]},
        },
      ];

      const result = await applyAutoLayout(nodes, []);

      expect(result[0].data).toEqual({label: 'My View', components: [{id: 'comp1'}]});
    });
  });

  describe('Layout Options', () => {
    it('should pass node spacing and rank spacing to ELK and lay out left to right', async () => {
      await applyAutoLayout([createNode('a', 'VIEW')], [], {
        nodeSpacing: 80,
        rankSpacing: 220,
      });

      expect(lastGraph?.layoutOptions['elk.direction']).toBe('RIGHT');
      expect(lastGraph?.layoutOptions['elk.spacing.nodeNode']).toBe('80');
      expect(lastGraph?.layoutOptions['elk.layered.spacing.nodeNodeBetweenLayers']).toBe('220');
    });
  });

  describe('Layer Constraints', () => {
    it('should constrain START to the first layer and END to the last', async () => {
      const nodes: Node[] = [createNode('start', 'START'), createNode('view1', 'VIEW'), createNode('end', 'END')];

      await applyAutoLayout(nodes, []);

      const byId = new Map(lastGraph?.children.map((child) => [child.id, child]));
      expect(byId.get('start')?.layoutOptions['elk.layered.layering.layerConstraint']).toBe('FIRST');
      expect(byId.get('end')?.layoutOptions['elk.layered.layering.layerConstraint']).toBe('LAST');
      expect(byId.get('view1')?.layoutOptions['elk.layered.layering.layerConstraint']).toBeUndefined();
    });

    it('should handle mixed case node types', async () => {
      await applyAutoLayout([createNode('start', 'start'), createNode('end', 'End')], []);

      const byId = new Map(lastGraph?.children.map((child) => [child.id, child]));
      expect(byId.get('start')?.layoutOptions['elk.layered.layering.layerConstraint']).toBe('FIRST');
      expect(byId.get('end')?.layoutOptions['elk.layered.layering.layerConstraint']).toBe('LAST');
    });
  });

  describe('Handle Ports', () => {
    it('should model success handles as east ports and targets as west ports', async () => {
      const nodes: Node[] = [createNode('a', 'TASK_EXECUTION'), createNode('b', 'TASK_EXECUTION')];
      const edges: Edge[] = [createEdge('e1', 'a', 'b', 'a_NEXT')];

      await applyAutoLayout(nodes, edges);

      const byId = new Map(lastGraph?.children.map((child) => [child.id, child]));
      const sourcePorts = byId.get('a')?.ports ?? [];
      const targetPorts = byId.get('b')?.ports ?? [];

      expect(sourcePorts).toHaveLength(1);
      expect(sourcePorts[0].layoutOptions['elk.port.side']).toBe('EAST');
      expect(targetPorts[0].layoutOptions['elk.port.side']).toBe('WEST');
      expect(lastGraph?.edges[0].sources[0]).toBe(sourcePorts[0].id);
      expect(lastGraph?.edges[0].targets[0]).toBe(targetPorts[0].id);
      expect(byId.get('a')?.layoutOptions['elk.portConstraints']).toBe('FIXED_SIDE');
    });

    it('should model failure handles as south ports and incomplete handles as north ports', async () => {
      const nodes: Node[] = [
        createNode('exec', 'TASK_EXECUTION'),
        createNode('fail', 'TASK_EXECUTION'),
        createNode('retry', 'VIEW'),
      ];
      const edges: Edge[] = [
        createEdge('e1', 'exec', 'fail', 'failure'),
        createEdge('e2', 'exec', 'retry', 'exec_INCOMPLETE'),
      ];

      await applyAutoLayout(nodes, edges);

      const byId = new Map(lastGraph?.children.map((child) => [child.id, child]));
      const ports = byId.get('exec')?.ports ?? [];
      const sides = ports.map((port) => port.layoutOptions['elk.port.side']).sort();

      expect(sides).toEqual(['NORTH', 'SOUTH']);
    });

    it('should not add ports or port constraints to unconnected nodes', async () => {
      await applyAutoLayout([createNode('lonely', 'VIEW')], []);

      const child = lastGraph?.children[0];
      expect(child?.ports).toBeUndefined();
      expect(child?.layoutOptions['elk.portConstraints']).toBeUndefined();
    });
  });

  describe('Happy Path Straightening', () => {
    it('should assign straightness priority to the success chain from START', async () => {
      const nodes: Node[] = [
        createNode('start', 'START'),
        createNode('view1', 'VIEW'),
        createNode('exec1', 'TASK_EXECUTION'),
        createNode('end', 'END'),
        createNode('recovery', 'CALL'),
      ];
      const edges: Edge[] = [
        createEdge('e1', 'start', 'view1', 'start_NEXT'),
        createEdge('e2', 'view1', 'exec1', 'btn_NEXT'),
        createEdge('e3', 'exec1', 'end', 'exec1_NEXT'),
        createEdge('e4', 'exec1', 'recovery', 'failure'),
      ];

      await applyAutoLayout(nodes, edges);

      const byEdgeId = new Map(lastGraph?.edges.map((edge) => [edge.id, edge]));
      expect(byEdgeId.get('e1')?.layoutOptions?.['elk.layered.priority.straightness']).toBe('10');
      expect(byEdgeId.get('e2')?.layoutOptions?.['elk.layered.priority.straightness']).toBe('10');
      expect(byEdgeId.get('e3')?.layoutOptions?.['elk.layered.priority.straightness']).toBe('10');
      expect(byEdgeId.get('e4')?.layoutOptions).toBeUndefined();
    });

    it('should not follow failure branches when collecting the happy path', async () => {
      const nodes: Node[] = [createNode('start', 'START'), createNode('a', 'TASK_EXECUTION'), createNode('end', 'END')];
      const edges: Edge[] = [createEdge('e1', 'start', 'a', 'failure'), createEdge('e2', 'a', 'end', 'a_NEXT')];

      await applyAutoLayout(nodes, edges);

      const byEdgeId = new Map(lastGraph?.edges.map((edge) => [edge.id, edge]));
      expect(byEdgeId.get('e1')?.layoutOptions).toBeUndefined();
      expect(byEdgeId.get('e2')?.layoutOptions).toBeUndefined();
    });

    it('should terminate on cyclic success chains', async () => {
      const nodes: Node[] = [createNode('start', 'START'), createNode('a', 'VIEW'), createNode('b', 'VIEW')];
      const edges: Edge[] = [
        createEdge('e1', 'start', 'a', 'start_NEXT'),
        createEdge('e2', 'a', 'b', 'a_NEXT'),
        createEdge('e3', 'b', 'a', 'b_NEXT'),
      ];

      const result = await applyAutoLayout(nodes, edges);

      expect(result).toHaveLength(3);
    });
  });

  describe('Edge Handling', () => {
    it('should deduplicate edges with the same source handle and target', async () => {
      const nodes: Node[] = [createNode('a', 'VIEW'), createNode('b', 'VIEW')];
      const edges: Edge[] = [createEdge('e1', 'a', 'b', 'a_NEXT'), createEdge('e2', 'a', 'b', 'a_NEXT')];

      await applyAutoLayout(nodes, edges);

      expect(lastGraph?.edges).toHaveLength(1);
    });

    it('should keep parallel edges that leave different handles', async () => {
      const nodes: Node[] = [createNode('a', 'TASK_EXECUTION'), createNode('b', 'VIEW')];
      const edges: Edge[] = [createEdge('e1', 'a', 'b', 'a_NEXT'), createEdge('e2', 'a', 'b', 'failure')];

      await applyAutoLayout(nodes, edges);

      expect(lastGraph?.edges).toHaveLength(2);
    });

    it('should filter out edges with non-existent source nodes', async () => {
      const nodes: Node[] = [createNode('a', 'VIEW')];
      const edges: Edge[] = [createEdge('e1', 'ghost', 'a')];

      await applyAutoLayout(nodes, edges);

      expect(lastGraph?.edges).toHaveLength(0);
    });

    it('should filter out edges with non-existent target nodes', async () => {
      const nodes: Node[] = [createNode('a', 'VIEW')];
      const edges: Edge[] = [createEdge('e1', 'a', 'ghost')];

      await applyAutoLayout(nodes, edges);

      expect(lastGraph?.edges).toHaveLength(0);
    });
  });

  describe('Node Dimensions', () => {
    it('should use measured dimensions when available', async () => {
      const nodes: Node[] = [createNode('a', 'VIEW', {x: 0, y: 0}, {width: 350, height: 500})];

      await applyAutoLayout(nodes, []);

      expect(lastGraph?.children[0].width).toBe(350);
      expect(lastGraph?.children[0].height).toBe(500);
    });

    it('should use width/height properties when measured is not available', async () => {
      const nodes: Node[] = [{...createNode('a', 'VIEW'), width: 320, height: 240}];

      await applyAutoLayout(nodes, []);

      expect(lastGraph?.children[0].width).toBe(320);
      expect(lastGraph?.children[0].height).toBe(240);
    });

    it('should use default dimensions when neither measured nor width/height available', async () => {
      await applyAutoLayout([createNode('a', 'VIEW')], []);

      expect(lastGraph?.children[0].width).toBe(200);
      expect(lastGraph?.children[0].height).toBe(100);
    });
  });

  describe('Error Handling', () => {
    it('should return original nodes when ELK layout fails', async () => {
      failLayout = true;
      const nodes: Node[] = [createNode('a', 'VIEW', {x: 123, y: 456})];

      const result = await applyAutoLayout(nodes, []);

      expect(result[0].position).toEqual({x: 123, y: 456});
    });

    it('should handle nodes without type', async () => {
      const nodes: Node[] = [{id: 'untyped', position: {x: 0, y: 0}, data: {}}];

      const result = await applyAutoLayout(nodes, []);

      expect(result).toHaveLength(1);
    });
  });
});
