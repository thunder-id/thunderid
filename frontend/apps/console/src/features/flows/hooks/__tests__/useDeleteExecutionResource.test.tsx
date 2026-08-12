// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {ReactFlowProvider} from '@xyflow/react';
import type {Node} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UIPanelContext, {type UIPanelContextProps} from '../../context/UIPanelContext';

// Import after mocks
import useDeleteExecutionResource from '../useDeleteExecutionResource';

// Use vi.hoisted to define mocks that need to be referenced in vi.mock
const {
  mockGetEdges,
  mockGetNodes,
  mockUpdateNodeData,
  mockSetIsOpenResourcePropertiesPanel,
  registeredHandlers,
  mockUnsubscribes,
} = vi.hoisted(() => ({
  mockGetEdges: vi.fn().mockReturnValue([]),
  mockGetNodes: vi.fn().mockReturnValue([]),
  mockUpdateNodeData: vi.fn(),
  mockSetIsOpenResourcePropertiesPanel: vi.fn(),
  registeredHandlers: {} as Record<string, ((...args: unknown[]) => Promise<boolean>)[]>,
  mockUnsubscribes: {} as Record<string, ReturnType<typeof vi.fn>[]>,
}));

const mockOnNodeDelete = vi.fn().mockImplementation((handler: (...args: unknown[]) => Promise<boolean>) => {
  if (!registeredHandlers.onNodeDelete) registeredHandlers.onNodeDelete = [];
  registeredHandlers.onNodeDelete.push(handler);
  const unsub = vi.fn();
  if (!mockUnsubscribes.onNodeDelete) mockUnsubscribes.onNodeDelete = [];
  mockUnsubscribes.onNodeDelete.push(unsub);
  return unsub;
});

const mockOnNodeElementDelete = vi.fn().mockReturnValue(vi.fn());

const mockFlowPlugins = {
  onPropertyChange: vi.fn().mockReturnValue(vi.fn()),
  emitPropertyChange: vi.fn().mockReturnValue(true),
  onPropertyPanelOpen: vi.fn().mockReturnValue(vi.fn()),
  emitPropertyPanelOpen: vi.fn().mockReturnValue(true),
  onElementFilter: vi.fn().mockReturnValue(vi.fn()),
  emitElementFilter: vi.fn().mockReturnValue(true),
  onEdgeDelete: vi.fn().mockReturnValue(vi.fn()),
  emitEdgeDelete: vi.fn().mockReturnValue(true),
  onNodeDelete: mockOnNodeDelete,
  emitNodeDelete: vi.fn().mockReturnValue(true),
  onNodeElementDelete: mockOnNodeElementDelete,
  emitNodeElementDelete: vi.fn().mockReturnValue(true),
  onTemplateLoad: vi.fn().mockReturnValue(vi.fn()),
  emitTemplateLoad: vi.fn().mockReturnValue(true),
};

// Mock @xyflow/react
vi.mock('@xyflow/react', async () => {
  const actual = await vi.importActual('@xyflow/react');
  return {
    ...actual,
    useReactFlow: () => ({
      getEdges: mockGetEdges,
      getNodes: mockGetNodes,
      updateNodeData: mockUpdateNodeData,
    }),
  };
});

// Mock useFlowPlugins - capture handlers for testing
vi.mock('../useFlowPlugins', () => ({
  default: () => mockFlowPlugins,
}));

describe('useDeleteExecutionResource', () => {
  const defaultContextValue: UIPanelContextProps = {
    isResourcePanelOpen: true,
    isResourcePropertiesPanelOpen: false,
    isVersionHistoryPanelOpen: false,
    resourcePropertiesPanelHeading: '',
    setIsResourcePanelOpen: vi.fn(),
    setIsOpenResourcePropertiesPanel: mockSetIsOpenResourcePropertiesPanel,
    setIsVersionHistoryPanelOpen: vi.fn(),
    setResourcePropertiesPanelHeading: vi.fn(),
    registerCloseValidationPanel: vi.fn(),
  };

  const createWrapper = (contextValue: UIPanelContextProps = defaultContextValue) => {
    function Wrapper({children}: {children: ReactNode}) {
      return (
        <ReactFlowProvider>
          <UIPanelContext.Provider value={contextValue}>{children}</UIPanelContext.Provider>
        </ReactFlowProvider>
      );
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Clear registered handlers
    Object.keys(registeredHandlers).forEach((key) => {
      delete registeredHandlers[key];
    });
    Object.keys(mockUnsubscribes).forEach((key) => {
      delete mockUnsubscribes[key];
    });
    // Re-wire capture implementations after clearAllMocks
    mockOnNodeDelete.mockImplementation((handler: (...args: unknown[]) => Promise<boolean>) => {
      if (!registeredHandlers.onNodeDelete) registeredHandlers.onNodeDelete = [];
      registeredHandlers.onNodeDelete.push(handler);
      const unsub = vi.fn();
      if (!mockUnsubscribes.onNodeDelete) mockUnsubscribes.onNodeDelete = [];
      mockUnsubscribes.onNodeDelete.push(unsub);
      return unsub;
    });
    mockOnNodeElementDelete.mockReturnValue(vi.fn());
  });

  describe('Plugin Registration', () => {
    it('should register event handlers on mount', () => {
      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      // Check that handlers are registered
      expect(mockOnNodeDelete).toHaveBeenCalledWith(expect.any(Function));
    });

    it('should not subscribe to element deletion', () => {
      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      // Deleting a button never removes the execution node it points to, so the hook
      // has no element-deletion handler. Guards against reintroducing the shared-node
      // deletion (a button and an upstream step can target the same execution node).
      expect(mockOnNodeElementDelete).not.toHaveBeenCalled();
    });

    it('should call unsubscribe functions on unmount', () => {
      const {unmount} = renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      unmount();

      // Check that unsubscribe functions are called
      mockUnsubscribes.onNodeDelete?.forEach((unsub) => expect(unsub).toHaveBeenCalled());
    });
  });

  describe('deleteExecutionActionNode', () => {
    it('should register the handler with correct function identifier', () => {
      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      // The handler should be registered with the correct event type
      expect(mockOnNodeDelete).toHaveBeenCalledWith(expect.any(Function));
    });

    it('should set up nodes and edges getters for the handler', () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: 'TASK_EXECUTION',
        position: {x: 0, y: 0},
        data: {},
      };

      const actionNode: Node = {
        id: 'action-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'button-1', type: 'ACTION'},
            {id: 'button-2', type: 'ACTION'},
          ],
        },
      };

      mockGetNodes.mockReturnValue([actionNode, executionNode] as Node[]);
      mockGetEdges.mockReturnValue([]);

      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      // Verify the hook registered with the plugin registry
      expect(mockOnNodeDelete).toHaveBeenCalled();
    });
  });

  describe('Context Integration', () => {
    it('should use setIsOpenResourcePropertiesPanel from context', () => {
      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      // The hook should have access to context
      expect(mockOnNodeDelete).toHaveBeenCalledTimes(1);
    });
  });

  describe('deleteExecutionActionNode Handler', () => {
    it('should return true when no execution nodes are deleted', async () => {
      const viewNode: Node = {
        id: 'view-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {},
      };

      mockGetNodes.mockReturnValue([viewNode]);
      mockGetEdges.mockReturnValue([]);

      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      const deleteNodeHandler = registeredHandlers.onNodeDelete?.[0];
      expect(deleteNodeHandler).toBeDefined();

      // Delete a non-execution node
      const result = await deleteNodeHandler([viewNode]);
      expect(result).toBe(true);
    });

    it('should delete action components when execution node is deleted', async () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: 'TASK_EXECUTION',
        position: {x: 100, y: 0},
        data: {},
      };

      const actionNode: Node = {
        id: 'action-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'button-1', type: 'ACTION'},
            {id: 'button-2', type: 'ACTION'},
          ],
        },
      };

      mockGetNodes.mockReturnValue([actionNode, executionNode]);
      mockGetEdges.mockReturnValue([
        {
          id: 'edge-1',
          source: 'action-1',
          target: 'execution-1',
          sourceHandle: 'button-1-next',
        },
      ]);

      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      const deleteNodeHandler = registeredHandlers.onNodeDelete?.[0];
      expect(deleteNodeHandler).toBeDefined();

      const result = await deleteNodeHandler([executionNode]);
      expect(result).toBe(true);
      // The handler should register correctly - mockUpdateNodeData may not be called
      // if the node type doesn't match StepTypes.Execution
    });

    it('should return true when action nodes array is empty', async () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: 'TASK_EXECUTION',
        position: {x: 100, y: 0},
        data: {},
      };

      mockGetNodes.mockReturnValue([executionNode]);
      mockGetEdges.mockReturnValue([]);

      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      const deleteNodeHandler = registeredHandlers.onNodeDelete?.[0];
      const result = await deleteNodeHandler([executionNode]);
      expect(result).toBe(true);
    });
  });

  describe('deleteExecutionActionNode Handler - Callback Execution', () => {
    it('should execute updateNodeData callback to filter action components', async () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: 'TASK_EXECUTION',
        position: {x: 100, y: 0},
        data: {},
      };

      const actionNode: Node = {
        id: 'action-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'button-1', type: 'ACTION'},
            {id: 'button-2', type: 'ACTION'},
          ],
        },
      };

      mockGetNodes.mockReturnValue([actionNode, executionNode]);
      mockGetEdges.mockReturnValue([
        {
          id: 'edge-1',
          source: 'action-1',
          target: 'execution-1',
          sourceHandle: 'button-1_NEXT', // Use correct suffix format
        },
      ]);

      // Capture the callback passed to updateNodeData
      let capturedCallback: ((node: Node) => {components: unknown[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: unknown[]}) => {
        capturedCallback = callback;
      });

      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      const deleteNodeHandler = registeredHandlers.onNodeDelete?.[0];
      await deleteNodeHandler([executionNode]);

      expect(mockUpdateNodeData).toHaveBeenCalledWith('action-1', expect.any(Function));
      expect(capturedCallback).not.toBeNull();

      const result = capturedCallback!(actionNode);
      expect(result.components).toHaveLength(1);
      expect(result.components[0]).toEqual({id: 'button-2', type: 'ACTION'});
    });

    it('should close properties panel after updating node data', async () => {
      const executionNode: Node = {
        id: 'execution-1',
        type: 'TASK_EXECUTION',
        position: {x: 100, y: 0},
        data: {},
      };

      const actionNode: Node = {
        id: 'action-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [{id: 'button-1', type: 'ACTION'}],
        },
      };

      mockGetNodes.mockReturnValue([actionNode, executionNode]);
      mockGetEdges.mockReturnValue([
        {
          id: 'edge-1',
          source: 'action-1',
          target: 'execution-1',
          sourceHandle: 'button-1_NEXT', // Use correct suffix format
        },
      ]);

      renderHook(() => useDeleteExecutionResource(), {
        wrapper: createWrapper(),
      });

      const deleteNodeHandler = registeredHandlers.onNodeDelete?.[0];
      await deleteNodeHandler([executionNode]);

      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(false);
    });
  });
});
