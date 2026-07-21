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

import {renderHook, act} from '@testing-library/react';
import type {Edge, Node} from '@xyflow/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import useFlowSimulation from '../useFlowSimulation';

const mockFitView = vi.fn(() => Promise.resolve(true));

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({
    fitView: mockFitView,
  }),
}));

const startNode = {id: 'start', type: 'START', position: {x: 0, y: 0}, data: {}} as Node;
const viewNode = {
  id: 'view-1',
  type: 'VIEW',
  position: {x: 0, y: 0},
  data: {
    components: [{id: 'action_001', type: 'ACTION', category: 'ACTION', label: 'Sign In'}],
  },
} as unknown as Node;
const executorNode = {id: 'executor-1', type: 'TASK_EXECUTION', position: {x: 0, y: 0}, data: {}} as Node;

const nodes: Node[] = [startNode, viewNode, executorNode];
const edges: Edge[] = [
  {id: 'e-start', source: 'start', target: 'view-1', sourceHandle: 'start_NEXT'},
  {id: 'e-action', source: 'view-1', target: 'executor-1', sourceHandle: 'action_001_NEXT'},
] as Edge[];

describe('useFlowSimulation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should be idle until started', () => {
    const {result} = renderHook(() => useFlowSimulation(nodes, edges));

    expect(result.current.isSimulating).toBe(false);
    expect(result.current.currentNodeId).toBeNull();
    expect(result.current.options).toEqual([]);
  });

  it('should start from the Start node and focus it', () => {
    const {result} = renderHook(() => useFlowSimulation(nodes, edges));

    act(() => result.current.start());

    expect(result.current.isSimulating).toBe(true);
    expect(result.current.pathNodeIds).toEqual(['start']);
    expect(result.current.options.map((option) => option.targetNodeId)).toEqual(['view-1']);
    expect(mockFitView).toHaveBeenCalledWith(expect.objectContaining({nodes: [{id: 'start'}]}));
  });

  it('should fall back to the first node when there is no Start node', () => {
    const {result} = renderHook(() => useFlowSimulation([viewNode, executorNode], edges));

    act(() => result.current.start());

    expect(result.current.pathNodeIds).toEqual(['view-1']);
  });

  it('should walk forward on choose and record the traversed edge kind', () => {
    const {result} = renderHook(() => useFlowSimulation(nodes, edges));

    act(() => result.current.start());
    act(() => result.current.choose(result.current.options[0]));
    act(() => result.current.choose(result.current.options[0]));

    expect(result.current.pathNodeIds).toEqual(['start', 'view-1', 'executor-1']);
    expect(result.current.pathEdges).toEqual([
      {edgeId: 'e-start', kind: 'success'},
      {edgeId: 'e-action', kind: 'action'},
    ]);
    expect(result.current.currentNodeId).toBe('executor-1');
  });

  it('should step back one node and drop the traversed edge', () => {
    const {result} = renderHook(() => useFlowSimulation(nodes, edges));

    act(() => result.current.start());
    act(() => result.current.choose(result.current.options[0]));
    act(() => result.current.back());

    expect(result.current.pathNodeIds).toEqual(['start']);
    expect(result.current.pathEdges).toEqual([]);
  });

  it('should ignore back on the first step', () => {
    const {result} = renderHook(() => useFlowSimulation(nodes, edges));

    act(() => result.current.start());
    act(() => result.current.back());

    expect(result.current.pathNodeIds).toEqual(['start']);
  });

  it('should clear all state on stop', () => {
    const {result} = renderHook(() => useFlowSimulation(nodes, edges));

    act(() => result.current.start());
    act(() => result.current.preview(result.current.options[0]));
    act(() => result.current.stop());

    expect(result.current.isSimulating).toBe(false);
    expect(result.current.pathNodeIds).toEqual([]);
    expect(result.current.pathEdges).toEqual([]);
    expect(result.current.previewedOption).toBeNull();
  });

  it('should not move the camera when follow camera is off', () => {
    const {result} = renderHook(() => useFlowSimulation(nodes, edges));

    act(() => result.current.toggleFollowCamera());
    mockFitView.mockClear();

    act(() => result.current.start());
    act(() => result.current.choose(result.current.options[0]));
    act(() => result.current.stop());

    expect(result.current.followCamera).toBe(false);
    expect(mockFitView).not.toHaveBeenCalled();
  });

  it('should keep the options array identity across unrelated node changes', () => {
    const {result, rerender} = renderHook(({n, e}: {n: Node[]; e: Edge[]}) => useFlowSimulation(n, e), {
      initialProps: {n: nodes, e: edges},
    });

    act(() => result.current.start());
    const initialOptions = result.current.options;

    // Simulates a drag tick: a new nodes array where only an off-path node moved.
    const draggedNodes = nodes.map((node) => (node.id === 'executor-1' ? {...node, position: {x: 100, y: 100}} : node));
    rerender({n: draggedNodes, e: edges});

    expect(result.current.options).toBe(initialOptions);
  });

  it('should exit the simulation when the current node is deleted', () => {
    const {result, rerender} = renderHook(({n, e}: {n: Node[]; e: Edge[]}) => useFlowSimulation(n, e), {
      initialProps: {n: nodes, e: edges},
    });

    act(() => result.current.start());
    act(() => result.current.choose(result.current.options[0]));
    expect(result.current.currentNodeId).toBe('view-1');

    rerender({n: [startNode, executorNode], e: [edges[0]]});

    expect(result.current.isSimulating).toBe(false);
    expect(result.current.pathNodeIds).toEqual([]);
  });
});
