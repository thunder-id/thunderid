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
import type {Element} from '../../models/elements';
import {ResourceTypes} from '../../models/resources';
import {StepTypes} from '../../models/steps';
import type {Step} from '../../models/steps';
import type {Template} from '../../models/templates';
import type {Widget} from '../../models/widget';
import useResourceAdd, {type UseResourceAddProps} from '../useResourceAdd';

// Use vi.hoisted to define mocks
const {
  mockScreenToFlowPosition,
  mockGetNodes,
  mockGetEdges,
  mockUpdateNodeData,
  mockFitView,
  mockUpdateNodeInternals,
  mockExecuteSync,
} = vi.hoisted(() => {
  // Create a fitView mock that always returns a promise
  // Using a wrapper function ensures the Promise is always returned even after clearAllMocks
  const fitViewMock = vi.fn(() => Promise.resolve(undefined));

  return {
    mockScreenToFlowPosition: vi.fn().mockReturnValue({x: 100, y: 100}),
    mockGetNodes: vi.fn().mockReturnValue([]),
    mockGetEdges: vi.fn().mockReturnValue([]),
    mockUpdateNodeData: vi.fn(),
    mockFitView: fitViewMock,
    mockUpdateNodeInternals: vi.fn(),
    mockExecuteSync: vi.fn(),
  };
});

// Mock @xyflow/react
vi.mock('@xyflow/react', async () => {
  const actual = await vi.importActual('@xyflow/react');
  return {
    ...actual,
    useReactFlow: () => ({
      screenToFlowPosition: mockScreenToFlowPosition,
      getNodes: mockGetNodes,
      getEdges: mockGetEdges,
      updateNodeData: mockUpdateNodeData,
      fitView: mockFitView,
    }),
    useUpdateNodeInternals: () => mockUpdateNodeInternals,
  };
});

// Mock useFlowPlugins
vi.mock('../useFlowPlugins', () => ({
  default: () => ({
    onPropertyChange: vi.fn().mockReturnValue(vi.fn()),
    emitPropertyChange: vi.fn().mockReturnValue(true),
    onPropertyPanelOpen: vi.fn().mockReturnValue(vi.fn()),
    emitPropertyPanelOpen: vi.fn().mockReturnValue(true),
    onElementFilter: vi.fn().mockReturnValue(vi.fn()),
    emitElementFilter: vi.fn().mockReturnValue(true),
    onEdgeDelete: vi.fn().mockReturnValue(vi.fn()),
    emitEdgeDelete: vi.fn().mockReturnValue(true),
    onNodeDelete: vi.fn().mockReturnValue(vi.fn()),
    emitNodeDelete: vi.fn().mockReturnValue(true),
    onNodeElementDelete: vi.fn().mockReturnValue(vi.fn()),
    emitNodeElementDelete: vi.fn().mockReturnValue(true),
    onTemplateLoad: vi.fn().mockReturnValue(vi.fn()),
    emitTemplateLoad: mockExecuteSync,
  }),
}));

// Mock generateResourceId
vi.mock('../../utils/generateResourceId', () => ({
  default: vi.fn().mockImplementation((type: string) => `${type}-generated-id`),
}));

// Mock autoAssignConnections
vi.mock('../../utils/autoAssignConnections', () => ({
  default: vi.fn(),
}));

vi.mock('../useFlowEvents', () => ({
  default: () => ({
    notifyElementAdded: vi.fn(),
  }),
}));

describe('useResourceAdd', () => {
  const mockOnTemplateLoad = vi.fn();
  const mockOnWidgetLoad = vi.fn();
  const mockOnStepLoad = vi.fn();
  const mockSetNodes = vi.fn();
  const mockSetEdges = vi.fn();
  const mockGenerateStepElement = vi.fn();
  const mockOnResourceDropOnCanvas = vi.fn();

  const defaultProps: UseResourceAddProps = {
    onTemplateLoad: mockOnTemplateLoad,
    onWidgetLoad: mockOnWidgetLoad,
    onStepLoad: mockOnStepLoad,
    setNodes: mockSetNodes,
    setEdges: mockSetEdges,
    generateStepElement: mockGenerateStepElement,
    onResourceDropOnCanvas: mockOnResourceDropOnCanvas,
  };

  const createWrapper = () => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ReactFlowProvider>{children}</ReactFlowProvider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockOnTemplateLoad.mockReturnValue([[], []]);
    mockOnWidgetLoad.mockReturnValue([[], [], null, null]);
    mockOnStepLoad.mockImplementation((step: Step) => step);
    mockGenerateStepElement.mockImplementation((element: Element) => ({...element, id: 'generated-element-id'}));
    mockGetNodes.mockReturnValue([]);
    mockGetEdges.mockReturnValue([]);
    mockScreenToFlowPosition.mockReturnValue({x: 100, y: 100});
    // Reset fitView to return a resolved promise by default after clearAllMocks
    // Use mockReturnValue to ensure it always returns a promise
    mockFitView.mockReturnValue(Promise.resolve(undefined));
  });

  afterEach(async () => {
    // Clean up any pending timers/requestAnimationFrame callbacks to prevent test pollution
    // First, ensure fitView mock returns a promise before running pending callbacks
    mockFitView.mockReturnValue(Promise.resolve(undefined));
    // Switch to fake timers if not already using them, then flush all pending callbacks
    vi.useFakeTimers();
    await vi.runAllTimersAsync();
    vi.useRealTimers();
    cleanup();
  });

  describe('Hook Initialization', () => {
    it('should return a stable function reference', () => {
      const {result, rerender} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const firstRef = result.current;
      rerender();
      const secondRef = result.current;

      expect(firstRef).toBe(secondRef);
    });

    it('should return a function', () => {
      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      expect(typeof result.current).toBe('function');
    });
  });

  describe('Template Handling', () => {
    it('should handle template resource type', () => {
      const newNodes: Node[] = [{id: 'node-1', type: 'VIEW', position: {x: 0, y: 0}, data: {}}];
      const newEdges: Edge[] = [{id: 'edge-1', source: 'node-1', target: 'node-2'}];

      mockOnTemplateLoad.mockReturnValue([newNodes, newEdges]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const template = {
        id: 'template-1',
        type: 'BASIC',
        resourceType: ResourceTypes.Template,
      } as Template;

      act(() => {
        result.current(template);
      });

      expect(mockExecuteSync).toHaveBeenCalled();
      expect(mockOnTemplateLoad).toHaveBeenCalledWith(expect.objectContaining({id: 'template-1'}));
      expect(mockSetNodes).toHaveBeenCalledWith(newNodes);
      expect(mockSetEdges).toHaveBeenCalledWith([...newEdges]);
    });

    it('should apply auto-assign connections when metadata has executorConnections', () => {
      const newNodes: Node[] = [{id: 'node-1', type: 'VIEW', position: {x: 0, y: 0}, data: {}}];

      mockOnTemplateLoad.mockReturnValue([newNodes, []]);

      const propsWithMetadata: UseResourceAddProps = {
        ...defaultProps,
        metadata: {
          executorConnections: [{source: 'a', target: 'b'}],
        } as unknown as UseResourceAddProps['metadata'],
      };

      const {result} = renderHook(() => useResourceAdd(propsWithMetadata), {
        wrapper: createWrapper(),
      });

      const template = {
        id: 'template-1',
        type: 'BASIC',
        resourceType: ResourceTypes.Template,
      } as Template;

      act(() => {
        result.current(template);
      });

      expect(mockOnTemplateLoad).toHaveBeenCalled();
    });

    it('should update node internals for nodes with components', () => {
      const newNodes: Node[] = [
        {
          id: 'node-1',
          type: 'VIEW',
          position: {x: 0, y: 0},
          data: {
            components: [
              {
                id: 'component-1',
                type: 'FORM',
                components: [{id: 'nested-1', type: 'INPUT'}],
              },
            ],
          },
        },
      ];

      mockOnTemplateLoad.mockReturnValue([newNodes, []]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const template = {
        id: 'template-1',
        type: 'BASIC',
        resourceType: ResourceTypes.Template,
      } as Template;

      act(() => {
        result.current(template);
      });

      expect(mockSetNodes).toHaveBeenCalledWith(newNodes);
    });

    it('should call updateNodeInternals for nodes, components and nested components via requestAnimationFrame', () => {
      vi.useFakeTimers();

      const newNodes: Node[] = [
        {
          id: 'node-1',
          type: 'VIEW',
          position: {x: 0, y: 0},
          data: {
            components: [
              {
                id: 'component-1',
                type: 'FORM',
                components: [{id: 'nested-1', type: 'INPUT'}],
              },
            ],
          },
        },
      ];

      mockOnTemplateLoad.mockReturnValue([newNodes, []]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const template = {
        id: 'template-1',
        type: 'BASIC',
        resourceType: ResourceTypes.Template,
      } as Template;

      act(() => {
        result.current(template);
      });

      // Trigger the first requestAnimationFrame callback (updateAllNodeInternals)
      act(() => {
        vi.runAllTimers();
      });

      // updateNodeInternals should be called for node, component, and nested component
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('component-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('nested-1');

      vi.useRealTimers();
    });

    it('should call fitView after updating node internals', () => {
      vi.useFakeTimers();

      const newNodes: Node[] = [{id: 'node-1', type: 'VIEW', position: {x: 0, y: 0}, data: {}}];

      mockOnTemplateLoad.mockReturnValue([newNodes, []]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const template = {
        id: 'template-1',
        type: 'BASIC',
        resourceType: ResourceTypes.Template,
      } as Template;

      act(() => {
        result.current(template);
      });

      // Trigger both requestAnimationFrame callbacks
      act(() => {
        vi.runAllTimers();
      });

      expect(mockFitView).toHaveBeenCalledWith({padding: 0.2, duration: 300});

      vi.useRealTimers();
    });

    it('should handle fitView rejection gracefully', () => {
      vi.useFakeTimers();

      mockFitView.mockRejectedValueOnce(new Error('fitView failed'));

      const newNodes: Node[] = [{id: 'node-1', type: 'VIEW', position: {x: 0, y: 0}, data: {}}];

      mockOnTemplateLoad.mockReturnValue([newNodes, []]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const template = {
        id: 'template-1',
        type: 'BASIC',
        resourceType: ResourceTypes.Template,
      } as Template;

      // Should not throw
      act(() => {
        result.current(template);
      });

      act(() => {
        vi.runAllTimers();
      });

      expect(mockFitView).toHaveBeenCalled();

      vi.useRealTimers();
    });

    it('should handle nodes without components in updateAllNodeInternals', () => {
      vi.useFakeTimers();

      const newNodes: Node[] = [
        {id: 'node-1', type: 'VIEW', position: {x: 0, y: 0}, data: {}},
        {id: 'node-2', type: 'VIEW', position: {x: 100, y: 0}, data: {components: []}},
      ];

      mockOnTemplateLoad.mockReturnValue([newNodes, []]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const template = {
        id: 'template-1',
        type: 'BASIC',
        resourceType: ResourceTypes.Template,
      } as Template;

      act(() => {
        result.current(template);
      });

      act(() => {
        vi.runAllTimers();
      });

      // Should only call updateNodeInternals for node ids, not for undefined components
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-2');

      vi.useRealTimers();
    });

    it('should handle components without nested components', () => {
      vi.useFakeTimers();

      const newNodes: Node[] = [
        {
          id: 'node-1',
          type: 'VIEW',
          position: {x: 0, y: 0},
          data: {
            components: [
              {id: 'component-1', type: 'BUTTON'}, // No nested components
            ],
          },
        },
      ];

      mockOnTemplateLoad.mockReturnValue([newNodes, []]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const template = {
        id: 'template-1',
        type: 'BASIC',
        resourceType: ResourceTypes.Template,
      } as Template;

      act(() => {
        result.current(template);
      });

      act(() => {
        vi.runAllTimers();
      });

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('component-1');

      vi.useRealTimers();
    });
  });

  describe('Widget Handling', () => {
    it('should handle widget resource type with existing view step', () => {
      const existingViewStep: Node = {
        id: 'view-1',
        type: StepTypes.View,
        position: {x: 0, y: 0},
        data: {components: []},
      };

      mockGetNodes.mockReturnValue([existingViewStep]);

      const newNodes: Node[] = [existingViewStep];
      mockOnWidgetLoad.mockReturnValue([newNodes, [], null, null]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const widget = {
        id: 'widget-1',
        type: 'SIGNIN',
        resourceType: ResourceTypes.Widget,
      } as Widget;

      act(() => {
        result.current(widget);
      });

      expect(mockOnWidgetLoad).toHaveBeenCalledWith(
        expect.objectContaining({id: 'widget-1'}),
        {},
        [expect.objectContaining({id: 'view-1'})],
        [],
        false,
      );
      expect(mockSetNodes).toHaveBeenCalledWith(newNodes);
    });

    it('should create new view step when no existing view step', () => {
      mockGetNodes.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([[], [], null, null]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const widget = {
        id: 'widget-1',
        type: 'SIGNIN',
        resourceType: ResourceTypes.Widget,
      } as Widget;

      act(() => {
        result.current(widget);
      });

      expect(mockOnWidgetLoad).toHaveBeenCalledWith(expect.objectContaining({id: 'widget-1'}), {}, [], [], false);
    });

    it('should apply auto-assign connections for widgets when metadata exists', () => {
      mockGetNodes.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([[], [], null, null]);

      const propsWithMetadata: UseResourceAddProps = {
        ...defaultProps,
        metadata: {
          executorConnections: [{source: 'a', target: 'b'}],
        } as unknown as UseResourceAddProps['metadata'],
      };

      const {result} = renderHook(() => useResourceAdd(propsWithMetadata), {
        wrapper: createWrapper(),
      });

      const widget = {
        id: 'widget-1',
        type: 'SIGNIN',
        resourceType: ResourceTypes.Widget,
      } as Widget;

      act(() => {
        result.current(widget);
      });

      expect(mockOnWidgetLoad).toHaveBeenCalled();
    });

    it('should call updateNodeInternals for widgets with nested components via requestAnimationFrame', () => {
      vi.useFakeTimers();

      const newNodes: Node[] = [
        {
          id: 'view-1',
          type: StepTypes.View,
          position: {x: 0, y: 0},
          data: {
            components: [
              {
                id: 'form-1',
                type: 'FORM',
                components: [{id: 'input-1', type: 'INPUT'}],
              },
            ],
          },
        },
      ];

      mockGetNodes.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([newNodes, [], null, null]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const widget = {
        id: 'widget-1',
        type: 'SIGNIN',
        resourceType: ResourceTypes.Widget,
      } as Widget;

      act(() => {
        result.current(widget);
      });

      act(() => {
        vi.runAllTimers();
      });

      // updateNodeInternals should be called for node, component, and nested component
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('view-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('form-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('input-1');

      vi.useRealTimers();
    });

    it('should call fitView after updating widget node internals', () => {
      vi.useFakeTimers();

      const newNodes: Node[] = [{id: 'view-1', type: StepTypes.View, position: {x: 0, y: 0}, data: {}}];

      mockGetNodes.mockReturnValue([]);
      mockOnWidgetLoad.mockReturnValue([newNodes, [], null, null]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const widget = {
        id: 'widget-1',
        type: 'SIGNIN',
        resourceType: ResourceTypes.Widget,
      } as Widget;

      act(() => {
        result.current(widget);
      });

      act(() => {
        vi.runAllTimers();
      });

      expect(mockFitView).toHaveBeenCalledWith({padding: 0.2, duration: 300});

      vi.useRealTimers();
    });
  });

  describe('Step Handling', () => {
    it('should handle step resource type', () => {
      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const step = {
        id: 'step-template',
        type: StepTypes.View,
        resourceType: ResourceTypes.Step,
      } as unknown as Step;

      act(() => {
        result.current(step);
      });

      expect(mockScreenToFlowPosition).toHaveBeenCalled();
      expect(mockOnStepLoad).toHaveBeenCalled();
      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
    });

    it('should set deletable to true for generated steps', () => {
      let capturedStep: Step | null = null;
      mockOnStepLoad.mockImplementation((step: Step) => {
        capturedStep = step;
        return step;
      });

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const step = {
        id: 'step-template',
        type: StepTypes.View,
        resourceType: ResourceTypes.Step,
        data: {someData: 'value'},
      } as unknown as Step;

      act(() => {
        result.current(step);
      });

      expect(capturedStep).not.toBeNull();
      expect(capturedStep!.deletable).toBe(true);
    });
  });

  describe('Element Handling', () => {
    it('should handle element resource type with existing view step', () => {
      const existingViewStep: Node = {
        id: 'view-1',
        type: StepTypes.View,
        position: {x: 0, y: 0},
        data: {components: []},
      };

      mockGetNodes.mockReturnValue([existingViewStep]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const element = {
        id: 'element-template',
        type: 'INPUT',
        resourceType: ResourceTypes.Element,
      } as unknown as Element;

      act(() => {
        result.current(element);
      });

      expect(mockGenerateStepElement).toHaveBeenCalledWith(expect.objectContaining({type: 'INPUT'}));
      expect(mockUpdateNodeData).toHaveBeenCalledWith('view-1', expect.any(Function));
      expect(mockOnResourceDropOnCanvas).toHaveBeenCalled();
    });

    it('should not add element when no view step exists', () => {
      mockGetNodes.mockReturnValue([]);

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const element = {
        id: 'element-template',
        type: 'INPUT',
        resourceType: ResourceTypes.Element,
      } as unknown as Element;

      act(() => {
        result.current(element);
      });

      expect(mockUpdateNodeData).not.toHaveBeenCalled();
    });

    it('should execute updateNodeData callback correctly for elements', () => {
      const existingViewStep: Node = {
        id: 'view-1',
        type: StepTypes.View,
        position: {x: 0, y: 0},
        data: {components: [{id: 'existing-1', type: 'BUTTON'}]},
      };

      mockGetNodes.mockReturnValue([existingViewStep]);

      let capturedCallback: ((node: Node) => {components: Element[]}) | null = null;
      mockUpdateNodeData.mockImplementation((_id: string, callback: (node: Node) => {components: Element[]}) => {
        capturedCallback = callback;
      });

      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const element = {
        id: 'element-template',
        type: 'INPUT',
        resourceType: ResourceTypes.Element,
      } as unknown as Element;

      act(() => {
        result.current(element);
      });

      expect(capturedCallback).not.toBeNull();

      const nodeData: Node = {
        id: 'view-1',
        type: StepTypes.View,
        position: {x: 0, y: 0},
        data: {components: [{id: 'existing-1', type: 'BUTTON'}]},
      };

      const result2 = capturedCallback!(nodeData);
      expect(result2.components.length).toBe(2);
    });
  });

  describe('Unknown Resource Type', () => {
    it('should not perform any action for unknown resource types', () => {
      const {result} = renderHook(() => useResourceAdd(defaultProps), {
        wrapper: createWrapper(),
      });

      const unknownResource = {
        id: 'unknown-1',
        type: 'UNKNOWN',
        resourceType: 'UNKNOWN_TYPE',
      };

      act(() => {
        result.current(unknownResource as never);
      });

      expect(mockOnTemplateLoad).not.toHaveBeenCalled();
      expect(mockOnWidgetLoad).not.toHaveBeenCalled();
      expect(mockOnStepLoad).not.toHaveBeenCalled();
      expect(mockSetNodes).not.toHaveBeenCalled();
    });
  });

  describe('Ref Updates', () => {
    it('should use latest props values when handler is called', () => {
      const {result, rerender} = renderHook(({props}: {props: UseResourceAddProps}) => useResourceAdd(props), {
        wrapper: createWrapper(),
        initialProps: {props: defaultProps},
      });

      const newOnTemplateLoad = vi.fn().mockReturnValue([[], []]);
      const newProps: UseResourceAddProps = {
        ...defaultProps,
        onTemplateLoad: newOnTemplateLoad,
      };

      rerender({props: newProps});

      const template = {
        id: 'template-1',
        type: 'BASIC',
        resourceType: ResourceTypes.Template,
      } as Template;

      act(() => {
        result.current(template);
      });

      expect(newOnTemplateLoad).toHaveBeenCalled();
      expect(mockOnTemplateLoad).not.toHaveBeenCalled();
    });
  });
});
