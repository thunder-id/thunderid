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

import {renderHook, act, cleanup} from '@testing-library/react';
import {ReactFlowProvider} from '@xyflow/react';
import type {Node, Edge} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import type {DragSourceData, DragTargetData} from '../../models/drag-drop';
import type {Element} from '../../models/elements';
import {ResourceTypes} from '../../models/resources';
import type {Resource} from '../../models/resources';
import type {Step} from '../../models/steps';
import type {Widget} from '../../models/widget';

// Import the mocked module
import autoAssignConnections from '../../utils/autoAssignConnections';
import useDragDropHandlers, {type UseDragDropHandlersProps} from '../useDragDropHandlers';

// Use vi.hoisted to define mocks that need to be referenced in vi.mock
const {mockScreenToFlowPosition, mockUpdateNodeData, mockGetNodes, mockGetEdges, mockUpdateNodeInternals} = vi.hoisted(
  () => ({
    mockScreenToFlowPosition: vi.fn((pos: {x: number; y: number}) => ({x: pos.x, y: pos.y})),
    mockUpdateNodeData: vi.fn(),
    mockGetNodes: vi.fn().mockReturnValue([]),
    mockGetEdges: vi.fn().mockReturnValue([]),
    mockUpdateNodeInternals: vi.fn(),
  }),
);

// Mock @xyflow/react
vi.mock('@xyflow/react', async () => {
  const actual = await vi.importActual('@xyflow/react');
  return {
    ...actual,
    useReactFlow: () => ({
      screenToFlowPosition: mockScreenToFlowPosition,
      updateNodeData: mockUpdateNodeData,
      getNodes: mockGetNodes,
      getEdges: mockGetEdges,
    }),
    useUpdateNodeInternals: () => mockUpdateNodeInternals,
  };
});

// Mock @dnd-kit/helpers
vi.mock('@dnd-kit/helpers', () => ({
  move: vi.fn((items: unknown[]) => items),
}));

// Mock generateResourceId
vi.mock('../../utils/generateResourceId', () => ({
  default: vi.fn((prefix: string) => `${prefix}-generated-id`),
}));

// Mock autoAssignConnections
vi.mock('../../utils/autoAssignConnections', () => ({
  default: vi.fn(),
}));

// Mock widgetUtils
const {mockWidgetNeedsViewContainer} = vi.hoisted(() => ({
  mockWidgetNeedsViewContainer: vi.fn().mockReturnValue(false),
}));

vi.mock('../../utils/widgetUtils', () => ({
  widgetNeedsViewContainer: mockWidgetNeedsViewContainer,
}));

describe('useDragDropHandlers', () => {
  const mockOnStepLoad = vi.fn((step: Step) => step);
  const mockSetNodes = vi.fn();
  const mockSetEdges = vi.fn();
  const mockOnResourceDropOnCanvas = vi.fn();
  const mockGenerateStepElement = vi.fn((element: Element) => ({
    ...element,
    id: `generated-${element.type}`,
  }));
  const mockMutateComponents = vi.fn((components: Element[]) => components);
  const mockOnWidgetLoad = vi.fn((): [Node[], Edge[], Resource | null, string | null] => [[], [], null, null]);

  const defaultProps: UseDragDropHandlersProps = {
    onStepLoad: mockOnStepLoad,
    setNodes: mockSetNodes,
    setEdges: mockSetEdges,
    onResourceDropOnCanvas: mockOnResourceDropOnCanvas,
    generateStepElement: mockGenerateStepElement,
    mutateComponents: mockMutateComponents,
    onWidgetLoad: mockOnWidgetLoad,
  };

  const createWrapper = () => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ReactFlowProvider>{children}</ReactFlowProvider>;
    }
    return Wrapper;
  };

  const createMockResource = (overrides: Partial<Resource> = {}): Resource =>
    ({
      id: 'resource-1',
      type: 'VIEW',
      resourceType: ResourceTypes.Step,
      category: 'STEP',
      version: '1.0.0',
      deprecated: false,
      deletable: true,
      display: {
        label: 'Test Resource',
        image: '',
        showOnResourcePanel: true,
      },
      config: {
        field: {name: '', type: {}},
        styles: {},
      },
      ...overrides,
    }) as Resource;

  const createMockDragEvent = (clientX: number, clientY: number) => ({
    nativeEvent: {
      clientX,
      clientY,
    } as MouseEvent,
  });

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetNodes.mockReturnValue([]);
    mockGetEdges.mockReturnValue([]);
    mockWidgetNeedsViewContainer.mockReturnValue(false);
  });

  afterEach(async () => {
    // Clean up any pending timers/requestAnimationFrame callbacks to prevent test pollution
    vi.useFakeTimers();
    await vi.runAllTimersAsync();
    vi.useRealTimers();
    cleanup();
  });

  describe('Hook Initialization', () => {
    it('should return stable handler functions', () => {
      const {result, rerender} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const initialHandlers = result.current;

      rerender();

      // Handlers should remain the same reference
      expect(result.current.addCanvasNode).toBe(initialHandlers.addCanvasNode);
      expect(result.current.addToView).toBe(initialHandlers.addToView);
      expect(result.current.addToForm).toBe(initialHandlers.addToForm);
      expect(result.current.addToViewAtIndex).toBe(initialHandlers.addToViewAtIndex);
      expect(result.current.addToFormAtIndex).toBe(initialHandlers.addToFormAtIndex);
    });

    it('should return all required handler functions', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      expect(result.current).toHaveProperty('addCanvasNode');
      expect(result.current).toHaveProperty('addToView');
      expect(result.current).toHaveProperty('addToForm');
      expect(result.current).toHaveProperty('addToViewAtIndex');
      expect(result.current).toHaveProperty('addToFormAtIndex');
    });
  });

  describe('addCanvasNode', () => {
    it('should add a new node to the canvas', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource(),
      };
      const targetData: DragTargetData = {};
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addCanvasNode(
          event as unknown as Parameters<typeof result.current.addCanvasNode>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
      expect(mockOnStepLoad).toHaveBeenCalled();
    });

    it('should not add node when source resource is missing', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {};
      const targetData: DragTargetData = {};
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addCanvasNode(
          event as unknown as Parameters<typeof result.current.addCanvasNode>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockSetNodes).not.toHaveBeenCalled();
    });

    it('should not add node when native event is missing', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource(),
      };
      const targetData: DragTargetData = {};
      const event = {nativeEvent: undefined};

      act(() => {
        result.current.addCanvasNode(
          event as unknown as Parameters<typeof result.current.addCanvasNode>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockSetNodes).not.toHaveBeenCalled();
    });

    it('should return early when widget needs a view container', () => {
      mockWidgetNeedsViewContainer.mockReturnValue(true);

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Widget, type: 'SIGNIN'}),
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addCanvasNode(
          event as unknown as Parameters<typeof result.current.addCanvasNode>[0],
          sourceData,
          {},
        );
      });

      expect(mockSetNodes).not.toHaveBeenCalled();
      expect(mockSetEdges).not.toHaveBeenCalled();
      expect(mockOnResourceDropOnCanvas).not.toHaveBeenCalled();
    });

    it('should handle widget drop on canvas when widget does not need a view container', () => {
      mockWidgetNeedsViewContainer.mockReturnValue(false);

      const mockNodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];
      const mockEdges: Edge[] = [];
      mockGetNodes.mockReturnValue(mockNodes);
      mockGetEdges.mockReturnValue(mockEdges);
      mockOnWidgetLoad.mockReturnValue([mockNodes, mockEdges, null, null]);

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const widgetResource: Resource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'SIGNIN',
      });
      const sourceData: DragSourceData = {dragged: widgetResource};
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addCanvasNode(
          event as unknown as Parameters<typeof result.current.addCanvasNode>[0],
          sourceData,
          {},
        );
      });

      expect(mockOnWidgetLoad).toHaveBeenCalledWith(
        widgetResource as Widget,
        widgetResource as Widget,
        mockNodes,
        mockEdges,
      );
      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockSetEdges).toHaveBeenCalled();
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
    });

    it('should call autoAssignConnections when dropping standalone widget on canvas with metadata', () => {
      const mockAutoAssignConnections = vi.mocked(autoAssignConnections);
      mockWidgetNeedsViewContainer.mockReturnValue(false);

      const mockNodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];
      mockGetNodes.mockReturnValue(mockNodes);
      mockGetEdges.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([mockNodes, [], null, null]);

      const propsWithMetadata: UseDragDropHandlersProps = {
        ...defaultProps,
        metadata: {
          flowType: 'LOGIN',
          supportedExecutors: [],
          connectorConfigs: {
            multiAttributeLoginEnabled: false,
            accountVerificationEnabled: false,
          },
          attributeProfile: 'default',
          attributeMetadata: [],
          executorConnections: [{executorName: 'executor1', connections: ['step-2']}],
        },
      };

      const {result} = renderHook(() => useDragDropHandlers(propsWithMetadata), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Widget, type: 'SIGNIN'}),
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addCanvasNode(
          event as unknown as Parameters<typeof result.current.addCanvasNode>[0],
          sourceData,
          {},
        );
      });

      expect(mockAutoAssignConnections).toHaveBeenCalledWith(
        mockNodes,
        propsWithMetadata.metadata?.executorConnections,
      );
    });

    it('should not add node when native event lacks clientX/clientY', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource(),
      };
      const targetData: DragTargetData = {};
      const event = {nativeEvent: new Event('custom')};

      act(() => {
        result.current.addCanvasNode(
          event as unknown as Parameters<typeof result.current.addCanvasNode>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockSetNodes).not.toHaveBeenCalled();
    });
  });

  describe('addToView', () => {
    it('should add element to view step', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element}),
      };
      const targetData: DragTargetData = {
        stepId: 'step-1',
        droppedOn: createMockResource(),
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToView(
          event as unknown as Parameters<typeof result.current.addToView>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
      expect(mockGenerateStepElement).toHaveBeenCalled();
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
    });

    it('should handle widget drop on view', () => {
      const mockNodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];
      const mockEdges: Edge[] = [];
      mockGetNodes.mockReturnValue(mockNodes);
      mockGetEdges.mockReturnValue(mockEdges);
      mockOnWidgetLoad.mockReturnValue([mockNodes, mockEdges, null, null]);

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const widgetResource: Resource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'IDENTIFIER_PASSWORD',
      });

      const sourceData: DragSourceData = {
        dragged: widgetResource,
      };
      const targetData: DragTargetData = {
        stepId: 'step-1',
        droppedOn: createMockResource(),
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToView(
          event as unknown as Parameters<typeof result.current.addToView>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockOnWidgetLoad).toHaveBeenCalledWith(widgetResource as Widget, expect.any(Object), mockNodes, mockEdges);
      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockSetEdges).toHaveBeenCalled();
    });

    it('should not add element when source resource is missing', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {};
      const targetData: DragTargetData = {
        stepId: 'step-1',
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToView(
          event as unknown as Parameters<typeof result.current.addToView>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockUpdateNodeData).not.toHaveBeenCalled();
    });
  });

  describe('addToForm', () => {
    it('should add element to form within a step', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element}),
      };
      const targetData: DragTargetData = {
        stepId: 'step-1',
        droppedOn: createMockResource({id: 'form-1'}),
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToForm(
          event as unknown as Parameters<typeof result.current.addToForm>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
      expect(mockGenerateStepElement).toHaveBeenCalled();
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
    });

    it('should not add element when target step is missing', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element}),
      };
      const targetData: DragTargetData = {
        droppedOn: createMockResource({id: 'form-1'}),
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToForm(
          event as unknown as Parameters<typeof result.current.addToForm>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockUpdateNodeData).not.toHaveBeenCalled();
    });
  });

  describe('addToViewAtIndex', () => {
    it('should add element at specific index in view', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element}),
      };

      act(() => {
        result.current.addToViewAtIndex(sourceData, 'step-1', 'element-2');
      });

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
      expect(mockGenerateStepElement).toHaveBeenCalled();
    });

    it('should handle widget drop at index', () => {
      const mockTargetNode: Node = {
        id: 'step-1',
        position: {x: 0, y: 0},
        data: {components: [{id: 'element-1'}, {id: 'element-2'}]},
      };
      mockGetNodes.mockReturnValue([mockTargetNode]);
      mockGetEdges.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([
        [{...mockTargetNode, data: {components: [{id: 'element-1'}, {id: 'element-2'}, {id: 'widget-button'}]}}],
        [],
        null,
        null,
      ]);

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const widgetResource: Resource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'IDENTIFIER_PASSWORD',
      });

      const sourceData: DragSourceData = {
        dragged: widgetResource,
      };

      act(() => {
        result.current.addToViewAtIndex(sourceData, 'step-1', 'element-2');
      });

      expect(mockOnWidgetLoad).toHaveBeenCalled();
      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockSetEdges).toHaveBeenCalled();
    });

    it('should not add when source resource is missing', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {};

      act(() => {
        result.current.addToViewAtIndex(sourceData, 'step-1', 'element-2');
      });

      expect(mockUpdateNodeData).not.toHaveBeenCalled();
    });

    it('should not add when target step is missing', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element}),
      };

      act(() => {
        result.current.addToViewAtIndex(sourceData, '', 'element-2');
      });

      expect(mockUpdateNodeData).not.toHaveBeenCalled();
    });
  });

  describe('addToFormAtIndex', () => {
    it('should add element at specific index in form', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element}),
      };

      act(() => {
        result.current.addToFormAtIndex(sourceData, 'step-1', 'form-1', 'element-2');
      });

      expect(mockUpdateNodeData).toHaveBeenCalledWith('step-1', expect.any(Function));
      expect(mockGenerateStepElement).toHaveBeenCalled();
    });

    it('should not add when source resource is missing', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {};

      act(() => {
        result.current.addToFormAtIndex(sourceData, 'step-1', 'form-1', 'element-2');
      });

      expect(mockUpdateNodeData).not.toHaveBeenCalled();
    });

    it('should not add when target step is missing', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element}),
      };

      act(() => {
        result.current.addToFormAtIndex(sourceData, '', 'form-1', 'element-2');
      });

      expect(mockUpdateNodeData).not.toHaveBeenCalled();
    });

    it('should not add when form id is missing', () => {
      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element}),
      };

      act(() => {
        result.current.addToFormAtIndex(sourceData, 'step-1', '', 'element-2');
      });

      expect(mockUpdateNodeData).not.toHaveBeenCalled();
    });
  });

  describe('Metadata Handling', () => {
    it('should call autoAssignConnections when metadata has executorConnections', () => {
      const mockAutoAssignConnections = vi.mocked(autoAssignConnections);

      const propsWithMetadata: UseDragDropHandlersProps = {
        ...defaultProps,
        metadata: {
          flowType: 'LOGIN',
          supportedExecutors: [],
          connectorConfigs: {
            multiAttributeLoginEnabled: false,
            accountVerificationEnabled: false,
          },
          attributeProfile: 'default',
          attributeMetadata: [],
          executorConnections: [{executorName: 'executor1', connections: ['step-2']}],
        },
      };

      const mockNodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];
      mockGetNodes.mockReturnValue(mockNodes);
      mockGetEdges.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([mockNodes, [], null, null]);

      const {result} = renderHook(() => useDragDropHandlers(propsWithMetadata), {
        wrapper: createWrapper(),
      });

      const widgetResource: Resource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'IDENTIFIER_PASSWORD',
      });

      const sourceData: DragSourceData = {
        dragged: widgetResource,
      };
      const targetData: DragTargetData = {
        stepId: 'step-1',
        droppedOn: createMockResource(),
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToView(
          event as unknown as Parameters<typeof result.current.addToView>[0],
          sourceData,
          targetData,
        );
      });

      expect(mockAutoAssignConnections).toHaveBeenCalledWith(
        mockNodes,
        propsWithMetadata.metadata?.executorConnections,
      );
    });
  });

  describe('addToView - Callback Execution', () => {
    it('should execute updateNodeData callback to add element to components', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };
      const targetData: DragTargetData = {
        stepId: 'step-1',
        droppedOn: createMockResource(),
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToView(
          event as unknown as Parameters<typeof result.current.addToView>[0],
          sourceData,
          targetData,
        );
      });

      expect(capturedCallback).not.toBeNull();

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [{id: 'existing-1', type: 'BUTTON'}],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      expect(callbackResult.components).toBeDefined();
      expect(callbackResult.components.length).toBeGreaterThan(0);
    });
  });

  describe('addToForm - Callback Execution', () => {
    it('should execute updateNodeData callback to add element to form components', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const targetFormResource = createMockResource({id: 'form-1', type: 'FORM'});
      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };
      const targetData: DragTargetData = {
        stepId: 'step-1',
        droppedOn: targetFormResource,
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToForm(
          event as unknown as Parameters<typeof result.current.addToForm>[0],
          sourceData,
          targetData,
        );
      });

      expect(capturedCallback).not.toBeNull();

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'form-1', type: 'FORM', components: [{id: 'input-1', type: 'INPUT'}]},
            {id: 'button-1', type: 'BUTTON'},
          ],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      expect(callbackResult.components).toBeDefined();
      expect(callbackResult.components.length).toBe(2);
      // The form should have the new element added
      const form = callbackResult.components.find((c: Element) => c.id === 'form-1') as Element & {
        components?: Element[];
      };
      expect(form?.components?.length).toBeGreaterThan(1);
    });

    it('should add element to a stack nested inside a form', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const targetStackResource = createMockResource({id: 'stack-1', type: 'STACK'});
      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'IMAGE'}),
      };
      const targetData: DragTargetData = {
        stepId: 'step-1',
        droppedOn: targetStackResource,
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToForm(
          event as unknown as Parameters<typeof result.current.addToForm>[0],
          sourceData,
          targetData,
        );
      });

      expect(capturedCallback).not.toBeNull();

      // Form contains a nested Stack - the Stack is not a top-level component of the view.
      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: 'FORM',
              components: [{id: 'stack-1', type: 'STACK', components: [{id: 'button-1', type: 'BUTTON'}]}],
            },
          ],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      const form = callbackResult.components.find((c: Element) => c.id === 'form-1') as Element & {
        components?: Element[];
      };
      const stack = form?.components?.find((c: Element) => c.id === 'stack-1') as Element & {
        components?: Element[];
      };
      // The nested stack should receive the new element.
      expect(stack?.components?.length).toBe(2);
    });

    it('should handle node with no existing components', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const targetFormResource = createMockResource({id: 'form-1', type: 'FORM'});
      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };
      const targetData: DragTargetData = {
        stepId: 'step-1',
        droppedOn: targetFormResource,
      };
      const event = createMockDragEvent(100, 200);

      act(() => {
        result.current.addToForm(
          event as unknown as Parameters<typeof result.current.addToForm>[0],
          sourceData,
          targetData,
        );
      });

      // Execute the callback with a node that has no components
      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {},
      };

      const callbackResult = capturedCallback!(mockNode);
      expect(callbackResult.components).toEqual([]);
    });
  });

  describe('addToViewAtIndex - Callback Execution', () => {
    it('should execute updateNodeData callback to insert element at target index', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };

      act(() => {
        result.current.addToViewAtIndex(sourceData, 'step-1', 'element-2');
      });

      expect(capturedCallback).not.toBeNull();

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'element-1', type: 'BUTTON'},
            {id: 'element-2', type: 'INPUT'},
            {id: 'element-3', type: 'BUTTON'},
          ],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      expect(callbackResult.components).toBeDefined();
      // The new element should be inserted at index 1 (before element-2)
      expect(callbackResult.components.length).toBe(4);
    });

    it('should append element when target index is not found', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };

      act(() => {
        result.current.addToViewAtIndex(sourceData, 'step-1', 'non-existent-element');
      });

      // Execute the callback with a node where target element doesn't exist
      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [{id: 'element-1', type: 'BUTTON'}],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      // Element should be appended at the end
      expect(callbackResult.components.length).toBe(2);
    });

    it('should handle node with empty components array', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };

      act(() => {
        result.current.addToViewAtIndex(sourceData, 'step-1', 'element-2');
      });

      // Execute the callback with a node that has no components
      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {},
      };

      const callbackResult = capturedCallback!(mockNode);
      expect(callbackResult.components.length).toBe(1);
    });

    it('should handle widget drop at index with empty components', () => {
      const mockTargetNode: Node = {
        id: 'step-1',
        position: {x: 0, y: 0},
        data: {components: []},
      };
      mockGetNodes.mockReturnValue([mockTargetNode]);
      mockGetEdges.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([[{...mockTargetNode, data: {components: []}}], [], null, null]);

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const widgetResource: Resource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'IDENTIFIER_PASSWORD',
      });

      const sourceData: DragSourceData = {
        dragged: widgetResource,
      };

      act(() => {
        result.current.addToViewAtIndex(sourceData, 'step-1', 'element-2');
      });

      expect(mockOnWidgetLoad).toHaveBeenCalled();
      expect(mockSetNodes).toHaveBeenCalled();
    });
  });

  describe('addToFormAtIndex - Callback Execution', () => {
    it('should execute updateNodeData callback to insert element at target index in form', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };

      act(() => {
        result.current.addToFormAtIndex(sourceData, 'step-1', 'form-1', 'input-2');
      });

      expect(capturedCallback).not.toBeNull();

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: 'FORM',
              components: [
                {id: 'input-1', type: 'INPUT'},
                {id: 'input-2', type: 'INPUT'},
                {id: 'input-3', type: 'INPUT'},
              ],
            },
            {id: 'button-1', type: 'BUTTON'},
          ],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      expect(callbackResult.components).toBeDefined();
      expect(callbackResult.components.length).toBe(2);
      // The form should have the new element inserted before input-2
      const form = callbackResult.components.find((c: Element) => c.id === 'form-1') as Element & {
        components?: Element[];
      };
      expect(form?.components?.length).toBe(4);
    });

    it('should append element when target index is not found in form', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };

      act(() => {
        result.current.addToFormAtIndex(sourceData, 'step-1', 'form-1', 'non-existent');
      });

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [{id: 'form-1', type: 'FORM', components: [{id: 'input-1', type: 'INPUT'}]}],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      const form = callbackResult.components.find((c: Element) => c.id === 'form-1') as Element & {
        components?: Element[];
      };
      // Should append at the end
      expect(form?.components?.length).toBe(2);
    });

    it('should handle form with no existing components', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };

      act(() => {
        result.current.addToFormAtIndex(sourceData, 'step-1', 'form-1', 'input-2');
      });

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'form-1', type: 'FORM'}, // Form without components
          ],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      const form = callbackResult.components.find((c: Element) => c.id === 'form-1') as Element & {
        components?: Element[];
      };
      expect(form?.components?.length).toBe(1);
    });

    it('should not modify components that do not match the form id', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'TEXT_INPUT'}),
      };

      act(() => {
        result.current.addToFormAtIndex(sourceData, 'step-1', 'form-1', 'input-2');
      });

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'form-1', type: 'FORM', components: [{id: 'input-1', type: 'INPUT'}]},
            {id: 'other-component', type: 'BUTTON', components: []},
          ],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      // The other-component should remain unchanged
      const otherComponent = callbackResult.components.find((c: Element) => c.id === 'other-component') as Element & {
        components?: Element[];
      };
      expect(otherComponent?.components?.length).toBe(0);
    });

    it('should insert element at target index in a stack nested inside a form', () => {
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useDragDropHandlers(defaultProps), {
        wrapper: createWrapper(),
      });

      const sourceData: DragSourceData = {
        dragged: createMockResource({resourceType: ResourceTypes.Element, type: 'IMAGE'}),
      };

      act(() => {
        result.current.addToFormAtIndex(sourceData, 'step-1', 'stack-1', 'button-2');
      });

      const mockNode: Node = {
        id: 'step-1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {
              id: 'form-1',
              type: 'FORM',
              components: [
                {
                  id: 'stack-1',
                  type: 'STACK',
                  components: [
                    {id: 'button-1', type: 'BUTTON'},
                    {id: 'button-2', type: 'BUTTON'},
                  ],
                },
              ],
            },
          ],
        },
      };

      const callbackResult = capturedCallback!(mockNode);
      const form = callbackResult.components.find((c: Element) => c.id === 'form-1') as Element & {
        components?: Element[];
      };
      const stack = form?.components?.find((c: Element) => c.id === 'stack-1') as Element & {
        components?: Element[];
      };
      // The new element should be inserted before button-2 within the nested stack.
      expect(stack?.components?.length).toBe(3);
      expect(stack?.components?.[2].id).toBe('button-2');
    });
  });

  describe('addToViewAtIndex - Widget handling with metadata', () => {
    it('should call autoAssignConnections when dropping widget at index with metadata', () => {
      const mockAutoAssignConnections = vi.mocked(autoAssignConnections);

      const propsWithMetadata: UseDragDropHandlersProps = {
        ...defaultProps,
        metadata: {
          flowType: 'LOGIN',
          supportedExecutors: [],
          connectorConfigs: {
            multiAttributeLoginEnabled: false,
            accountVerificationEnabled: false,
          },
          attributeProfile: 'default',
          attributeMetadata: [],
          executorConnections: [{executorName: 'executor1', connections: ['step-2']}],
        },
      };

      const mockTargetNode: Node = {
        id: 'step-1',
        position: {x: 0, y: 0},
        data: {components: [{id: 'element-1'}, {id: 'element-2'}]},
      };
      mockGetNodes.mockReturnValue([mockTargetNode]);
      mockGetEdges.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([
        [{...mockTargetNode, data: {components: [{id: 'element-1'}, {id: 'element-2'}, {id: 'widget-button'}]}}],
        [],
        null,
        null,
      ]);

      const {result} = renderHook(() => useDragDropHandlers(propsWithMetadata), {
        wrapper: createWrapper(),
      });

      const widgetResource: Resource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'IDENTIFIER_PASSWORD',
      });

      const sourceData: DragSourceData = {
        dragged: widgetResource,
      };

      act(() => {
        result.current.addToViewAtIndex(sourceData, 'step-1', 'element-2');
      });

      expect(mockAutoAssignConnections).toHaveBeenCalledWith(
        expect.any(Array),
        propsWithMetadata.metadata?.executorConnections,
      );
    });
  });
});
