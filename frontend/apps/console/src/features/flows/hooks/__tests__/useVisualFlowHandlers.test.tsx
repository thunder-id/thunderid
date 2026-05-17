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

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access */

import {renderHook, act} from '@testing-library/react';
import type {Connection, Edge, Node} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowConfigContext, {type FlowConfigContextProps} from '../../context/FlowConfigContext';
import {EdgeStyleTypes} from '../../models/steps';
import useVisualFlowHandlers from '../useVisualFlowHandlers';

// Mock @xyflow/react
const mockGetNodes = vi.fn();
const mockGetEdges = vi.fn();

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({
    getNodes: mockGetNodes,
    getEdges: mockGetEdges,
  }),
  MarkerType: {
    ArrowClosed: 'arrowclosed',
  },
  addEdge: (edge: Edge, edges: Edge[]) => [...edges, edge],
  getConnectedEdges: (nodes: Node[], edges: Edge[]) =>
    edges.filter((e) => nodes.some((n) => n.id === e.source || n.id === e.target)),
  getIncomers: (node: Node, nodes: Node[], edges: Edge[]) => {
    const incomingEdges = edges.filter((e) => e.target === node.id);
    return nodes.filter((n) => incomingEdges.some((e) => e.source === n.id));
  },
  getOutgoers: (node: Node, nodes: Node[], edges: Edge[]) => {
    const outgoingEdges = edges.filter((e) => e.source === node.id);
    return nodes.filter((n) => outgoingEdges.some((e) => e.target === n.id));
  },
}));

// Mock useFlowPlugins
const {mockEmitEdgeDelete} = vi.hoisted(() => ({
  mockEmitEdgeDelete: vi.fn().mockReturnValue(true),
}));

vi.mock('../useFlowPlugins', () => ({
  default: () => ({
    onPropertyChange: vi.fn().mockReturnValue(vi.fn()),
    emitPropertyChange: vi.fn().mockReturnValue(true),
    onPropertyPanelOpen: vi.fn().mockReturnValue(vi.fn()),
    emitPropertyPanelOpen: vi.fn().mockReturnValue(true),
    onElementFilter: vi.fn().mockReturnValue(vi.fn()),
    emitElementFilter: vi.fn().mockReturnValue(true),
    onEdgeDelete: vi.fn().mockReturnValue(vi.fn()),
    emitEdgeDelete: mockEmitEdgeDelete,
    onNodeDelete: vi.fn().mockReturnValue(vi.fn()),
    emitNodeDelete: vi.fn().mockReturnValue(true),
    onNodeElementDelete: vi.fn().mockReturnValue(vi.fn()),
    emitNodeElementDelete: vi.fn().mockReturnValue(true),
    onTemplateLoad: vi.fn().mockReturnValue(vi.fn()),
    emitTemplateLoad: vi.fn().mockReturnValue(true),
  }),
}));

describe('useVisualFlowHandlers', () => {
  const mockSetEdges = vi.fn();

  const defaultFlowConfigValue: FlowConfigContextProps = {
    ElementFactory: () => null,
    ResourceProperties: () => null,
    flowCompletionConfigs: {},
    setFlowCompletionConfigs: vi.fn(),
    isVerboseMode: false,
    setIsVerboseMode: vi.fn(),
    edgeStyle: EdgeStyleTypes.SmoothStep,
    setEdgeStyle: vi.fn(),
    flowNodeTypes: {},
    flowEdgeTypes: {},
    setFlowNodeTypes: vi.fn(),
    setFlowEdgeTypes: vi.fn(),
    flowNodes: [],
    setFlowNodes: vi.fn(),
  };

  const createWrapper = (overrides: Partial<FlowConfigContextProps> = {}) => {
    const flowConfigValue = {...defaultFlowConfigValue, ...overrides};

    function Wrapper({children}: {children: ReactNode}) {
      return <FlowConfigContext.Provider value={flowConfigValue}>{children}</FlowConfigContext.Provider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetNodes.mockReturnValue([]);
    mockGetEdges.mockReturnValue([]);
  });

  describe('Hook Interface', () => {
    it('should return handleConnect function', () => {
      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      expect(typeof result.current.handleConnect).toBe('function');
    });

    it('should return handleNodesDelete function', () => {
      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      expect(typeof result.current.handleNodesDelete).toBe('function');
    });

    it('should return handleEdgesDelete function', () => {
      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      expect(typeof result.current.handleEdgesDelete).toBe('function');
    });

    it('should return stable function references across renders', () => {
      const {result, rerender} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      const initialConnect = result.current.handleConnect;
      const initialNodesDelete = result.current.handleNodesDelete;
      const initialEdgesDelete = result.current.handleEdgesDelete;

      rerender();

      expect(result.current.handleConnect).toBe(initialConnect);
      expect(result.current.handleNodesDelete).toBe(initialNodesDelete);
      expect(result.current.handleEdgesDelete).toBe(initialEdgesDelete);
    });
  });

  describe('handleConnect', () => {
    it('should add edge with default edge style when no onEdgeResolve provided', () => {
      mockGetNodes.mockReturnValue([{id: 'node-1'}, {id: 'node-2'}]);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      const connection: Connection = {
        source: 'node-1',
        target: 'node-2',
        sourceHandle: null,
        targetHandle: null,
      };

      act(() => {
        result.current.handleConnect(connection);
      });

      expect(mockSetEdges).toHaveBeenCalled();

      // Call the setter function to verify edge properties
      const setterFn = mockSetEdges.mock.calls[0][0];
      const newEdges = setterFn([]);

      expect(newEdges).toHaveLength(1);
      expect(newEdges[0]).toMatchObject({
        source: 'node-1',
        target: 'node-2',
        type: EdgeStyleTypes.SmoothStep,
        markerEnd: {type: 'arrowclosed'},
      });
    });

    it('should use onEdgeResolve when provided', () => {
      mockGetNodes.mockReturnValue([{id: 'node-1'}, {id: 'node-2'}]);

      const customEdge: Edge = {
        id: 'custom-edge',
        source: 'node-1',
        target: 'node-2',
        type: 'custom',
      };

      const onEdgeResolve = vi.fn().mockReturnValue(customEdge);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges, onEdgeResolve}), {
        wrapper: createWrapper(),
      });

      const connection: Connection = {
        source: 'node-1',
        target: 'node-2',
        sourceHandle: null,
        targetHandle: null,
      };

      act(() => {
        result.current.handleConnect(connection);
      });

      expect(onEdgeResolve).toHaveBeenCalledWith(connection, [{id: 'node-1'}, {id: 'node-2'}]);
    });

    it('should use current edgeStyle from context', () => {
      mockGetNodes.mockReturnValue([{id: 'node-1'}, {id: 'node-2'}]);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Bezier}),
      });

      const connection: Connection = {
        source: 'node-1',
        target: 'node-2',
        sourceHandle: null,
        targetHandle: null,
      };

      act(() => {
        result.current.handleConnect(connection);
      });

      const setterFn = mockSetEdges.mock.calls[0][0];
      const newEdges = setterFn([]);

      expect(newEdges[0].type).toBe(EdgeStyleTypes.Bezier);
    });
  });

  describe('handleNodesDelete', () => {
    it('should reconnect incomers to outgoers when node is deleted', () => {
      const nodes = [{id: 'node-1'}, {id: 'node-2'}, {id: 'node-3'}] as Node[];
      const edges = [
        {id: 'edge-1', source: 'node-1', target: 'node-2'},
        {id: 'edge-2', source: 'node-2', target: 'node-3'},
      ] as Edge[];

      mockGetNodes.mockReturnValue(nodes);
      mockGetEdges.mockReturnValue(edges);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current.handleNodesDelete([{id: 'node-2'} as Node]);
      });

      expect(mockSetEdges).toHaveBeenCalled();
    });

    it('should handle deleting multiple nodes', () => {
      const nodes = [{id: 'node-1'}, {id: 'node-2'}, {id: 'node-3'}] as Node[];
      const edges = [
        {id: 'edge-1', source: 'node-1', target: 'node-2'},
        {id: 'edge-2', source: 'node-2', target: 'node-3'},
      ] as Edge[];

      mockGetNodes.mockReturnValue(nodes);
      mockGetEdges.mockReturnValue(edges);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current.handleNodesDelete([{id: 'node-1'} as Node, {id: 'node-3'} as Node]);
      });

      expect(mockSetEdges).toHaveBeenCalled();
    });

    it('should use current edgeStyle for newly created edges', () => {
      const nodes = [{id: 'node-1'}, {id: 'node-2'}, {id: 'node-3'}] as Node[];
      const edges = [
        {id: 'edge-1', source: 'node-1', target: 'node-2'},
        {id: 'edge-2', source: 'node-2', target: 'node-3'},
      ] as Edge[];

      mockGetNodes.mockReturnValue(nodes);
      mockGetEdges.mockReturnValue(edges);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Step}),
      });

      act(() => {
        result.current.handleNodesDelete([{id: 'node-2'} as Node]);
      });

      expect(mockSetEdges).toHaveBeenCalled();
    });

    it('should reconnect nodes from the latest edges passed into the setter', () => {
      const nodes = [{id: 'node-1'}, {id: 'node-2'}, {id: 'node-3'}] as Node[];
      const latestEdges = [
        {id: 'edge-1', source: 'node-1', target: 'node-2'},
        {id: 'edge-2', source: 'node-2', target: 'node-3'},
      ] as Edge[];

      mockGetNodes.mockReturnValue(nodes);
      mockGetEdges.mockReturnValue([]);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Step}),
      });

      act(() => {
        result.current.handleNodesDelete([{id: 'node-2'} as Node]);
      });

      const setterFn = mockSetEdges.mock.calls[0][0] as (edges: Edge[]) => Edge[];
      const newEdges = setterFn(latestEdges);

      expect(mockGetEdges).not.toHaveBeenCalled();
      expect(newEdges).toEqual([
        {
          id: 'node-1-->node-3',
          source: 'node-1',
          target: 'node-3',
          type: EdgeStyleTypes.Step,
          markerEnd: {type: 'arrowclosed'},
        },
      ]);
    });
  });

  describe('handleEdgesDelete', () => {
    it('should call emitEdgeDelete with deleted edges', () => {
      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      const deletedEdges = [
        {id: 'edge-1', source: 'node-1', target: 'node-2'},
        {id: 'edge-2', source: 'node-2', target: 'node-3'},
      ] as Edge[];

      act(() => {
        result.current.handleEdgesDelete(deletedEdges);
      });

      expect(mockEmitEdgeDelete).toHaveBeenCalledWith(deletedEdges);
    });

    it('should handle empty deleted edges array', () => {
      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current.handleEdgesDelete([]);
      });

      expect(mockEmitEdgeDelete).toHaveBeenCalledWith([]);
    });
  });

  describe('Ref Updates', () => {
    it('should use latest props when handler is called', () => {
      mockGetNodes.mockReturnValue([{id: 'node-1'}, {id: 'node-2'}]);

      const mockSetEdges1 = vi.fn();
      const mockSetEdges2 = vi.fn();

      const {result, rerender} = renderHook(({setEdges}) => useVisualFlowHandlers({setEdges}), {
        wrapper: createWrapper(),
        initialProps: {setEdges: mockSetEdges1},
      });

      // Rerender with new setEdges
      rerender({setEdges: mockSetEdges2});

      const connection: Connection = {
        source: 'node-1',
        target: 'node-2',
        sourceHandle: null,
        targetHandle: null,
      };

      act(() => {
        result.current.handleConnect(connection);
      });

      // Should use the latest setEdges
      expect(mockSetEdges2).toHaveBeenCalled();
      expect(mockSetEdges1).not.toHaveBeenCalled();
    });
  });
});
