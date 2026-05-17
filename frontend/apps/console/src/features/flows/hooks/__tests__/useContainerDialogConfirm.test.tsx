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

import {renderHook, act} from '@testing-library/react';
import {ReactFlowProvider} from '@xyflow/react';
import type {Node, Edge} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {DragSourceData, DragTargetData, DragEventWithNative} from '../../models/drag-drop';
import type {Element} from '../../models/elements';
import {ElementCategories, BlockTypes} from '../../models/elements';
import {ResourceTypes} from '../../models/resources';
import type {Resource} from '../../models/resources';
import type {Step} from '../../models/steps';
import {StepTypes} from '../../models/steps';
import useContainerDialogConfirm, {type UseContainerDialogConfirmProps} from '../useContainerDialogConfirm';
// Widget import removed - unused

// Use vi.hoisted to define mocks that need to be referenced in vi.mock
const {mockScreenToFlowPosition, mockUpdateNodeData, mockGetNodes, mockGetEdges} = vi.hoisted(() => ({
  mockScreenToFlowPosition: vi.fn((pos: {x: number; y: number}) => ({x: pos.x, y: pos.y})),
  mockUpdateNodeData: vi.fn(),
  mockGetNodes: vi.fn(() => []),
  mockGetEdges: vi.fn(() => []),
}));

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
  };
});

// Mock generateResourceId
vi.mock('../../utils/generateResourceId', () => ({
  default: vi.fn((prefix: string) => `${prefix}-generated-id`),
}));

vi.mock('../useFlowEvents', () => ({
  default: () => ({
    notifyElementAdded: vi.fn(),
  }),
}));

// Mock autoAssignConnections
vi.mock('../../utils/autoAssignConnections', () => ({
  default: vi.fn(),
}));

describe('useContainerDialogConfirm', () => {
  const mockHandleContainerDialogClose = vi.fn();
  const mockOnStepLoad = vi.fn((step: Step) => step);
  const mockSetNodes = vi.fn();
  const mockSetEdges = vi.fn();
  const mockOnResourceDropOnCanvas = vi.fn();
  const mockGenerateStepElement = vi.fn((element: Element) => ({
    ...element,
    id: `generated-${element.type || 'element'}`,
  }));
  const mockOnWidgetLoad = vi.fn((): [Node[], Edge[], Resource | null, string | null] => [[], [], null, null]);

  const createMockResource = (overrides: Partial<Resource> = {}): Resource =>
    ({
      id: 'resource-1',
      type: 'TEXT_INPUT',
      resourceType: ResourceTypes.Element,
      category: ElementCategories.Field,
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

  const createPendingDropRef = (
    clientX: number,
    clientY: number,
    dragged?: Resource,
    stepId?: string,
  ): React.MutableRefObject<{
    event: DragEventWithNative;
    sourceData: DragSourceData;
    targetData: DragTargetData;
  } | null> => ({
    current: {
      event: {
        nativeEvent: {clientX, clientY} as MouseEvent,
      },
      sourceData: {
        dragged,
      },
      targetData: {
        stepId,
      },
    },
  });

  const createDefaultProps = (
    dropScenario: UseContainerDialogConfirmProps['dropScenario'],
    pendingDropRef = createPendingDropRef(100, 200, createMockResource()),
  ): UseContainerDialogConfirmProps => ({
    dropScenario,
    handleContainerDialogClose: mockHandleContainerDialogClose,
    generateStepElement: mockGenerateStepElement,
    onStepLoad: mockOnStepLoad,
    setNodes: mockSetNodes,
    setEdges: mockSetEdges,
    onResourceDropOnCanvas: mockOnResourceDropOnCanvas,
    onWidgetLoad: mockOnWidgetLoad,
    pendingDropRef,
  });

  const createWrapper = () => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ReactFlowProvider>{children}</ReactFlowProvider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetNodes.mockReturnValue([]);
    mockGetEdges.mockReturnValue([]);
  });

  describe('Hook Initialization', () => {
    it('should return a stable handler function', () => {
      const props = createDefaultProps('form-on-canvas');
      const {result, rerender} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      const initialHandler = result.current;
      rerender();

      expect(result.current).toBe(initialHandler);
    });

    it('should return a function', () => {
      const props = createDefaultProps('form-on-canvas');
      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      expect(typeof result.current).toBe('function');
    });
  });

  describe('No Pending Data', () => {
    it('should close dialog when no pending data exists', () => {
      const props = createDefaultProps('form-on-canvas', {current: null});
      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockHandleContainerDialogClose).toHaveBeenCalled();
      expect(mockSetNodes).not.toHaveBeenCalled();
    });
  });

  describe('Invalid Event Data', () => {
    it('should close dialog when dropped resource is missing', () => {
      const pendingDropRef = createPendingDropRef(100, 200, undefined);
      const props = createDefaultProps('form-on-canvas', pendingDropRef);
      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockHandleContainerDialogClose).toHaveBeenCalled();
      expect(mockSetNodes).not.toHaveBeenCalled();
    });

    it('should close dialog when native event is missing', () => {
      const pendingDropRef: React.MutableRefObject<{
        event: DragEventWithNative;
        sourceData: DragSourceData;
        targetData: DragTargetData;
      } | null> = {
        current: {
          event: {nativeEvent: undefined},
          sourceData: {dragged: createMockResource()},
          targetData: {},
        },
      };
      const props = createDefaultProps('form-on-canvas', pendingDropRef);
      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockHandleContainerDialogClose).toHaveBeenCalled();
      expect(mockSetNodes).not.toHaveBeenCalled();
    });

    it('should close dialog when native event lacks clientX/clientY', () => {
      const pendingDropRef: React.MutableRefObject<{
        event: DragEventWithNative;
        sourceData: DragSourceData;
        targetData: DragTargetData;
      } | null> = {
        current: {
          event: {nativeEvent: new Event('custom')},
          sourceData: {dragged: createMockResource()},
          targetData: {},
        },
      };
      const props = createDefaultProps('form-on-canvas', pendingDropRef);
      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockHandleContainerDialogClose).toHaveBeenCalled();
      expect(mockSetNodes).not.toHaveBeenCalled();
    });
  });

  describe('form-on-canvas scenario', () => {
    it('should create a View step with the Form inside', () => {
      const formResource = createMockResource({
        type: BlockTypes.Form,
        category: ElementCategories.Block,
      });
      const pendingDropRef = createPendingDropRef(100, 200, formResource);
      const props = createDefaultProps('form-on-canvas', pendingDropRef);

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockOnStepLoad).toHaveBeenCalled();
      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
      expect(mockHandleContainerDialogClose).toHaveBeenCalled();

      // Verify that onStepLoad was called with a step containing the form component
      const stepArg = mockOnStepLoad.mock.calls[0][0];
      expect(stepArg.type).toBe(StepTypes.View);
      expect(stepArg.data.components).toHaveLength(1);
    });
  });

  describe('input-on-canvas scenario', () => {
    it('should create a View step with Form containing the Input', () => {
      const inputResource = createMockResource({
        type: 'TEXT_INPUT',
        category: ElementCategories.Field,
      });
      const pendingDropRef = createPendingDropRef(100, 200, inputResource);
      const props = createDefaultProps('input-on-canvas', pendingDropRef);

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockOnStepLoad).toHaveBeenCalled();
      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
      expect(mockHandleContainerDialogClose).toHaveBeenCalled();

      // Verify the step structure
      const stepArg = mockOnStepLoad.mock.calls[0][0];
      expect(stepArg.type).toBe(StepTypes.View);
      expect(stepArg.data.components).toHaveLength(1);
      // The first component should be a Form
      expect(stepArg.data.components?.[0].type).toBe(BlockTypes.Form);
    });
  });

  describe('input-on-view scenario', () => {
    it('should add a Form containing the Input to existing View', () => {
      const inputResource = createMockResource({
        type: 'TEXT_INPUT',
        category: ElementCategories.Field,
      });
      const pendingDropRef = createPendingDropRef(100, 200, inputResource, 'existing-step-id');
      const props = createDefaultProps('input-on-view', pendingDropRef);

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockUpdateNodeData).toHaveBeenCalledWith('existing-step-id', expect.any(Function));
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
      expect(mockHandleContainerDialogClose).toHaveBeenCalled();
    });

    it('should execute updateNodeData callback correctly to add form to existing components', () => {
      const inputResource = createMockResource({
        type: 'TEXT_INPUT',
        category: ElementCategories.Field,
      });
      const pendingDropRef = createPendingDropRef(100, 200, inputResource, 'existing-step-id');
      const props = createDefaultProps('input-on-view', pendingDropRef);

      // Capture the callback passed to updateNodeData
      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(capturedCallback).not.toBeNull();

      // Test the callback with a node that has existing components
      const mockNode: Node = {
        id: 'existing-step-id',
        type: StepTypes.View,
        position: {x: 0, y: 0},
        data: {
          components: [{id: 'existing-1', type: 'BUTTON'}],
        },
      };

      const callbackResult = capturedCallback!(mockNode);

      // Should add the new form to the existing components
      expect(callbackResult.components).toHaveLength(2);
      expect(callbackResult.components[0]).toEqual({id: 'existing-1', type: 'BUTTON'});
      expect(callbackResult.components[1].type).toBe(BlockTypes.Form);
    });

    it('should handle updateNodeData callback with node having no existing components', () => {
      const inputResource = createMockResource({
        type: 'TEXT_INPUT',
        category: ElementCategories.Field,
      });
      const pendingDropRef = createPendingDropRef(100, 200, inputResource, 'existing-step-id');
      const props = createDefaultProps('input-on-view', pendingDropRef);

      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      // Test with a node that has no components (undefined)
      const mockNodeNoComponents: Node = {
        id: 'existing-step-id',
        type: StepTypes.View,
        position: {x: 0, y: 0},
        data: {},
      };

      const callbackResult = capturedCallback!(mockNodeNoComponents);

      // Should create components array with just the form
      expect(callbackResult.components).toHaveLength(1);
      expect(callbackResult.components[0].type).toBe(BlockTypes.Form);
    });

    it('should not update node when target step id is missing', () => {
      const inputResource = createMockResource({
        type: 'TEXT_INPUT',
        category: ElementCategories.Field,
      });
      const pendingDropRef = createPendingDropRef(100, 200, inputResource, undefined);
      const props = createDefaultProps('input-on-view', pendingDropRef);

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockUpdateNodeData).not.toHaveBeenCalled();
      expect(mockHandleContainerDialogClose).toHaveBeenCalled();
    });
  });

  describe('widget-on-canvas scenario', () => {
    it('should create a View step and load widget', () => {
      const widgetResource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'IDENTIFIER_PASSWORD',
      });
      const mockNodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];
      mockGetNodes.mockReturnValue([]);
      mockGetEdges.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([mockNodes, [], widgetResource, 'step-id']);

      const pendingDropRef = createPendingDropRef(100, 200, widgetResource);
      const props = createDefaultProps('widget-on-canvas', pendingDropRef);

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockOnWidgetLoad).toHaveBeenCalledWith(
        expect.objectContaining({type: 'IDENTIFIER_PASSWORD'}),
        expect.objectContaining({type: StepTypes.View}),
        expect.any(Array),
        expect.any(Array),
        true,
      );
      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockSetEdges).toHaveBeenCalled();
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
      expect(mockHandleContainerDialogClose).toHaveBeenCalled();
    });

    it('should call autoAssignConnections when metadata has executorConnections', async () => {
      const autoAssignConnections = vi.mocked((await import('../../utils/autoAssignConnections')).default);

      const widgetResource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'IDENTIFIER_PASSWORD',
      });
      const mockNodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];
      mockGetNodes.mockReturnValue([]);
      mockGetEdges.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([mockNodes, [], null, null]);

      const pendingDropRef = createPendingDropRef(100, 200, widgetResource);
      const props: UseContainerDialogConfirmProps = {
        ...createDefaultProps('widget-on-canvas', pendingDropRef),
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

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(autoAssignConnections).toHaveBeenCalledWith(mockNodes, props.metadata?.executorConnections);
    });

    it('should use default property selector when onWidgetLoad returns null', () => {
      const widgetResource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'IDENTIFIER_PASSWORD',
      });
      mockGetNodes.mockReturnValue([]);
      mockGetEdges.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([[], [], null, null]);

      const pendingDropRef = createPendingDropRef(100, 200, widgetResource);
      const props = createDefaultProps('widget-on-canvas', pendingDropRef);

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockOnResourceDropOnCanvas).toHaveBeenCalledWith(
        expect.objectContaining({type: 'IDENTIFIER_PASSWORD'}),
        expect.any(String),
      );
    });

    it('should use defaultPropertySectorStepId from onWidgetLoad when available', () => {
      const widgetResource = createMockResource({
        resourceType: ResourceTypes.Widget,
        type: 'IDENTIFIER_PASSWORD',
      });
      const defaultPropertySelector = createMockResource({type: 'PASSWORD_INPUT'});
      const customStepId = 'custom-step-id-from-widget-load';

      mockGetNodes.mockReturnValue([]);
      mockGetEdges.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([
        [{id: 'node-1', position: {x: 0, y: 0}, data: {}}],
        [],
        defaultPropertySelector,
        customStepId,
      ]);

      const pendingDropRef = createPendingDropRef(100, 200, widgetResource);
      const props = createDefaultProps('widget-on-canvas', pendingDropRef);

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      // Should use the customStepId from onWidgetLoad, not generatedViewStep.id
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalledWith(
        expect.objectContaining({type: 'PASSWORD_INPUT'}),
        customStepId,
      );
    });
  });

  describe('Position Calculation', () => {
    it('should use screenToFlowPosition to calculate drop position', () => {
      const pendingDropRef = createPendingDropRef(150, 250, createMockResource());
      const props = createDefaultProps('form-on-canvas', pendingDropRef);

      const {result} = renderHook(() => useContainerDialogConfirm(props), {
        wrapper: createWrapper(),
      });

      act(() => {
        result.current();
      });

      expect(mockScreenToFlowPosition).toHaveBeenCalledWith({x: 150, y: 250});
    });
  });
});
