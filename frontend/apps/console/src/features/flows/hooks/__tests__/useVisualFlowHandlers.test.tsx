// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
const mockUpdateNodeData = vi.fn();

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({
    getNodes: mockGetNodes,
    getEdges: mockGetEdges,
    updateNodeData: mockUpdateNodeData,
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
    isSnapToGridEnabled: false,
    setIsSnapToGridEnabled: vi.fn(),
    isMiniMapVisible: true,
    setIsMiniMapVisible: vi.fn(),
    flowNodeTypes: {},
    flowEdgeTypes: {},
    setFlowNodeTypes: vi.fn(),
    setFlowEdgeTypes: vi.fn(),
    flowNodes: [],
    setFlowNodes: vi.fn(),
    flowEdges: [],
    setFlowEdges: vi.fn(),
    graphValidationRules: [],
    setGraphValidationRules: vi.fn(),
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

    describe('RichText action ref auto-population', () => {
      const richTextComponentId = 'rt-1';
      const nextSuffix = '_NEXT';

      const buildRichTextNode = (actionRef: string | undefined = '') => ({
        id: 'view-1',
        data: {
          components: [
            {
              id: richTextComponentId,
              type: 'RICH_TEXT',
              action: actionRef === undefined ? undefined : {ref: actionRef},
            },
          ],
        },
      });

      it('should populate RichText action.ref when edge is drawn from its next handle', () => {
        mockGetNodes.mockReturnValue([buildRichTextNode(''), {id: 'target-1', data: {}}]);

        const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
          wrapper: createWrapper(),
        });

        const connection: Connection = {
          source: 'view-1',
          target: 'target-1',
          sourceHandle: `${richTextComponentId}${nextSuffix}`,
          targetHandle: null,
        };

        act(() => {
          result.current.handleConnect(connection);
        });

        expect(mockUpdateNodeData).toHaveBeenCalledTimes(1);
        const [nodeId, updater] = mockUpdateNodeData.mock.calls[0];
        expect(nodeId).toBe('view-1');
        // Invoke the updater with the current node to inspect the produced data
        const updated = updater({data: buildRichTextNode('').data});
        expect(updated.components[0]).toEqual({
          id: richTextComponentId,
          type: 'RICH_TEXT',
          action: {ref: 'target-1'},
        });
      });

      it('should update RichText action.ref nested inside a container component', () => {
        const nested = {
          id: 'view-1',
          data: {
            components: [
              {
                id: 'block-1',
                type: 'BLOCK',
                components: [
                  {
                    id: richTextComponentId,
                    type: 'RICH_TEXT',
                    action: {ref: ''},
                  },
                ],
              },
            ],
          },
        };
        mockGetNodes.mockReturnValue([nested, {id: 'target-2'}]);

        const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
          wrapper: createWrapper(),
        });

        act(() => {
          result.current.handleConnect({
            source: 'view-1',
            target: 'target-2',
            sourceHandle: `${richTextComponentId}${nextSuffix}`,
            targetHandle: null,
          });
        });

        expect(mockUpdateNodeData).toHaveBeenCalledTimes(1);
        const [, updater] = mockUpdateNodeData.mock.calls[0];
        const updated = updater({data: nested.data});
        expect(updated.components[0].components[0].action).toEqual({ref: 'target-2'});
      });

      it('should not call updateNodeData when RichText already has the target ref', () => {
        mockGetNodes.mockReturnValue([buildRichTextNode('target-1'), {id: 'target-1'}]);

        const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
          wrapper: createWrapper(),
        });

        act(() => {
          result.current.handleConnect({
            source: 'view-1',
            target: 'target-1',
            sourceHandle: `${richTextComponentId}${nextSuffix}`,
            targetHandle: null,
          });
        });

        expect(mockUpdateNodeData).not.toHaveBeenCalled();
      });

      it('should not touch RichText when the component has no action defined', () => {
        // action === undefined means the rich-text is display-only; the hook should skip it
        mockGetNodes.mockReturnValue([
          {
            id: 'view-1',
            data: {components: [{id: richTextComponentId, type: 'RICH_TEXT'}]},
          },
          {id: 'target-1'},
        ]);

        const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
          wrapper: createWrapper(),
        });

        act(() => {
          result.current.handleConnect({
            source: 'view-1',
            target: 'target-1',
            sourceHandle: `${richTextComponentId}${nextSuffix}`,
            targetHandle: null,
          });
        });

        expect(mockUpdateNodeData).not.toHaveBeenCalled();
      });

      it('should ignore connections whose sourceHandle does not end with _NEXT', () => {
        mockGetNodes.mockReturnValue([buildRichTextNode(''), {id: 'target-1'}]);

        const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
          wrapper: createWrapper(),
        });

        act(() => {
          result.current.handleConnect({
            source: 'view-1',
            target: 'target-1',
            sourceHandle: `${richTextComponentId}_OTHER`,
            targetHandle: null,
          });
        });

        expect(mockUpdateNodeData).not.toHaveBeenCalled();
      });

      it('should skip auto-population when source node has no components (non-step data)', () => {
        mockGetNodes.mockReturnValue([{id: 'view-1', data: {}}, {id: 'target-1'}]);

        const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
          wrapper: createWrapper(),
        });

        act(() => {
          result.current.handleConnect({
            source: 'view-1',
            target: 'target-1',
            sourceHandle: `${richTextComponentId}${nextSuffix}`,
            targetHandle: null,
          });
        });

        expect(mockUpdateNodeData).not.toHaveBeenCalled();
      });

      it('should leave non-matching components untouched when updating', () => {
        const sourceNode = {
          id: 'view-1',
          data: {
            components: [
              {id: 'other', type: 'RICH_TEXT', action: {ref: 'existing'}},
              {id: richTextComponentId, type: 'RICH_TEXT', action: {ref: ''}},
            ],
          },
        };
        mockGetNodes.mockReturnValue([sourceNode, {id: 'target-1'}]);

        const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
          wrapper: createWrapper(),
        });

        act(() => {
          result.current.handleConnect({
            source: 'view-1',
            target: 'target-1',
            sourceHandle: `${richTextComponentId}${nextSuffix}`,
            targetHandle: null,
          });
        });

        const [, updater] = mockUpdateNodeData.mock.calls[0];
        const updated = updater({data: sourceNode.data});
        expect(updated.components[0].action).toEqual({ref: 'existing'});
        expect(updated.components[1].action).toEqual({ref: 'target-1'});
      });

      describe('label sentinel sync', () => {
        interface SyncedComponent {
          action: {ref: string};
          label?: string;
        }

        const connectTo = (target: string, node: {id: string; data: unknown}): SyncedComponent => {
          mockGetNodes.mockReturnValue([node, {id: target}]);

          const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
            wrapper: createWrapper(),
          });

          act(() => {
            result.current.handleConnect({
              source: 'view-1',
              target,
              sourceHandle: `${richTextComponentId}${nextSuffix}`,
              targetHandle: null,
            });
          });

          const [, updater] = mockUpdateNodeData.mock.calls[0];
          const updated = updater({data: node.data}) as {components: SyncedComponent[]};

          return updated.components[0];
        };

        const buildLabelledNode = (actionRef: string, label: string) => ({
          id: 'view-1',
          data: {
            components: [{id: richTextComponentId, type: 'RICH_TEXT', action: {ref: actionRef}, label}],
          },
        });

        it('rewrites the sentinel alongside the ref', () => {
          const component = connectTo(
            'target-1',
            buildLabelledNode('action_recovery', '<p><a href="#" data-action-ref="action_recovery">Reset</a></p>'),
          );

          expect(component.action).toEqual({ref: 'target-1'});
          expect(component.label).toBe('<p><a href="#" data-action-ref="target-1">Reset</a></p>');
        });

        it('rewrites a lone sentinel that has drifted from the ref', () => {
          // Flows saved before the two moved together carry a target node id in `action.ref`
          // while the label kept the authored name. Redrawing the edge has to heal that.
          const component = connectTo(
            'target-1',
            buildLabelledNode('recovery_call_g0v6', '<p><a href="#" data-action-ref="action_recovery">Reset</a></p>'),
          );

          expect(component.label).toBe('<p><a href="#" data-action-ref="target-1">Reset</a></p>');
        });

        it('leaves a foreign sentinel alone when the label has several', () => {
          const component = connectTo(
            'target-1',
            buildLabelledNode(
              'action_recovery',
              '<p><a href="#" data-action-ref="action_recovery">Reset</a>' +
                '<a href="#" data-action-ref="action_signup">Sign up</a></p>',
            ),
          );

          expect(component.label).toBe(
            '<p><a href="#" data-action-ref="target-1">Reset</a>' +
              '<a href="#" data-action-ref="action_signup">Sign up</a></p>',
          );
        });

        it('rewrites only the first sentinel when the ref is empty and the label has several', () => {
          // With no ref to match on, the anchor the properties panel derives the ref from is the
          // first. Rewriting them all would point the unrelated link at this action too.
          const component = connectTo(
            'target-1',
            buildLabelledNode(
              '',
              '<p><a href="#" data-action-ref="action_recovery">Reset</a>' +
                '<a href="#" data-action-ref="action_signup">Sign up</a></p>',
            ),
          );

          expect(component.action).toEqual({ref: 'target-1'});
          expect(component.label).toBe(
            '<p><a href="#" data-action-ref="target-1">Reset</a>' +
              '<a href="#" data-action-ref="action_signup">Sign up</a></p>',
          );
        });

        it('leaves a label with no sentinel untouched', () => {
          const label = '<p><a href="#">Reset</a></p>';
          const component = connectTo('target-1', buildLabelledNode('action_recovery', label));

          expect(component.action).toEqual({ref: 'target-1'});
          expect(component.label).toBe(label);
        });

        it('heals a drifted sentinel when reconnected to the step the ref already names', () => {
          // Redrawing the edge to the same step is the most direct repair gesture. Keying the
          // update off the ref alone would make it a no-op and leave the link dead.
          const component = connectTo(
            'target-1',
            buildLabelledNode('target-1', '<p><a href="#" data-action-ref="action_recovery">Reset</a></p>'),
          );

          expect(component.label).toBe('<p><a href="#" data-action-ref="target-1">Reset</a></p>');
        });

        it('does not touch the node when the ref and the sentinel already match the target', () => {
          mockGetNodes.mockReturnValue([
            buildLabelledNode('target-1', '<p><a href="#" data-action-ref="target-1">Reset</a></p>'),
            {id: 'target-1'},
          ]);

          const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
            wrapper: createWrapper(),
          });

          act(() => {
            result.current.handleConnect({
              source: 'view-1',
              target: 'target-1',
              sourceHandle: `${richTextComponentId}${nextSuffix}`,
              targetHandle: null,
            });
          });

          expect(mockUpdateNodeData).not.toHaveBeenCalled();
        });
      });
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

    it('should produce exactly one reconnect edge A->C when middle node B is deleted from A->B->C', () => {
      const nodes = [{id: 'node-1'}, {id: 'node-2'}, {id: 'node-3'}] as Node[];
      const latestEdges = [
        {id: 'edge-1', source: 'node-1', target: 'node-2'},
        {id: 'edge-2', source: 'node-2', target: 'node-3'},
      ] as Edge[];

      mockGetNodes.mockReturnValue(nodes);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current.handleNodesDelete([{id: 'node-2'} as Node]);
      });

      // Capture and invoke the functional setter with the live edge state
      const setterFn = mockSetEdges.mock.calls[0][0];
      const resultEdges: Edge[] = setterFn(latestEdges);

      expect(resultEdges).toHaveLength(1);
      expect(resultEdges[0]).toMatchObject({
        id: 'node-1-->node-3',
        source: 'node-1',
        target: 'node-3',
        markerEnd: {type: 'arrowclosed'},
      });
    });

    it('should use latestEdges from setter callback, not a stale getEdges() snapshot', () => {
      const nodes = [{id: 'node-1'}, {id: 'node-2'}, {id: 'node-3'}] as Node[];

      // getEdges returns a stale empty list — the correct edges come via the setter callback
      mockGetNodes.mockReturnValue(nodes);
      mockGetEdges.mockReturnValue([]);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current.handleNodesDelete([{id: 'node-2'} as Node]);
      });

      const setterFn = mockSetEdges.mock.calls[0][0];

      // Provide the live edge state through the setter callback
      const liveEdges = [
        {id: 'edge-1', source: 'node-1', target: 'node-2'},
        {id: 'edge-2', source: 'node-2', target: 'node-3'},
      ] as Edge[];

      const resultEdges: Edge[] = setterFn(liveEdges);

      // Reconnect must be computed from liveEdges, not the stale empty snapshot
      expect(resultEdges).toHaveLength(1);
      expect(resultEdges[0]).toMatchObject({source: 'node-1', target: 'node-3'});
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

    it('should apply edgeStyle to reconnect edges', () => {
      const nodes = [{id: 'node-1'}, {id: 'node-2'}, {id: 'node-3'}] as Node[];
      const latestEdges = [
        {id: 'edge-1', source: 'node-1', target: 'node-2'},
        {id: 'edge-2', source: 'node-2', target: 'node-3'},
      ] as Edge[];

      mockGetNodes.mockReturnValue(nodes);

      const {result} = renderHook(() => useVisualFlowHandlers({setEdges: mockSetEdges}), {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Step}),
      });

      act(() => {
        result.current.handleNodesDelete([{id: 'node-2'} as Node]);
      });

      const setterFn = mockSetEdges.mock.calls[0][0];
      const resultEdges: Edge[] = setterFn(latestEdges);

      expect(resultEdges[0].type).toBe(EdgeStyleTypes.Step);
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
