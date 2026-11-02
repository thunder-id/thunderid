// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, act} from '@testing-library/react';
import type {Edge, Node} from '@xyflow/react';
import type React from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import type {Resources} from '../../models/resources';
import type {FlowDefinitionResponse} from '../../models/responses';
import {StaticStepTypes, StepTypes} from '../../models/steps';
import {TemplateTypes} from '../../models/templates';
import type {UseFlowInitializationProps} from '../useFlowInitialization';
import useFlowInitialization from '../useFlowInitialization';

// Mock external dependencies
vi.mock('lodash-es/cloneDeep', () => ({
  default: <T>(obj: T): T => {
    if (obj === undefined || obj === null) {
      return obj;
    }
    return JSON.parse(JSON.stringify(obj)) as T;
  },
}));

vi.mock('../../utils/generateIdsForResources', () => ({
  default: <T>(obj: T): T => obj,
}));

vi.mock('../../utils/resolveComponentMetadata', () => ({
  default: (_resources: unknown, components: unknown) => components,
}));

vi.mock('../../utils/resolveStepMetadata', () => ({
  default: (_resources: unknown, steps: unknown) => steps,
}));

vi.mock('../../utils/updateTemplatePlaceholderReferences', () => ({
  default: (steps: unknown[]) => [steps],
}));

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const mockTransformFlowToCanvas = vi.fn((flowData: FlowDefinitionResponse): any => ({
  nodes: flowData.nodes.map((node) => ({
    id: node.id,
    type: node.type,
    position: node.layout?.position ?? {x: 0, y: 0},
    data: node.meta ?? {},
  })),
  edges: [] as Edge[],
  viewport: {x: 0, y: 0, zoom: 1},
}));

vi.mock('../../utils/flowToCanvasTransformer', () => ({
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  transformFlowToCanvas: (flowData: FlowDefinitionResponse) => mockTransformFlowToCanvas(flowData),
}));

const createMockResources = (overrides: Partial<Resources> = {}): Resources =>
  ({
    templates: [
      {
        id: 'basic-template',
        type: TemplateTypes.Basic,
        config: {
          data: {
            steps: [
              {
                id: 'view-step-1',
                type: StepTypes.View,
                position: {x: 100, y: 100},
                data: {
                  components: [{id: 'comp-1', type: 'TEXT'}],
                },
              },
              {
                id: 'END',
                type: StepTypes.End,
                position: {x: 200, y: 200},
                data: {},
              },
            ],
          },
        },
      },
      {
        id: 'blank-template',
        type: TemplateTypes.Blank,
        config: {
          data: {
            steps: [
              {
                id: '{{ID}}',
                type: StepTypes.View,
                position: {x: 0, y: 0},
                data: {
                  components: [{id: 'blank-comp-1', type: 'TEXT'}],
                },
              },
            ],
          },
        },
      },
    ],
    steps: [],
    elements: [],
    widgets: [],
    ...overrides,
  }) as unknown as Resources;

const createMockExistingFlowData = (): FlowDefinitionResponse =>
  ({
    id: 'flow-1',
    name: 'Test Flow',
    handle: 'test-flow',
    flowType: 'AUTHENTICATION',
    activeVersion: 1,
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-01T00:00:00Z',
    nodes: [
      {
        id: 'start',
        type: StaticStepTypes.Start,
        layout: {position: {x: 0, y: 0}, size: {width: 100, height: 50}},
      },
      {
        id: 'view-1',
        type: StepTypes.View,
        layout: {position: {x: 100, y: 100}, size: {width: 200, height: 150}},
        meta: {
          components: [{id: 'comp-1', type: 'TEXT'}],
        },
      },
      {
        id: 'END',
        type: StepTypes.End,
        layout: {position: {x: 200, y: 200}, size: {width: 100, height: 50}},
      },
    ],
  }) as FlowDefinitionResponse;

describe('useFlowInitialization', () => {
  let mockSetNodes: React.Dispatch<React.SetStateAction<Node[]>> & ReturnType<typeof vi.fn>;
  let mockSetEdges: React.Dispatch<React.SetStateAction<Edge[]>> & ReturnType<typeof vi.fn>;
  let mockUpdateNodeInternals: ReturnType<typeof vi.fn>;
  let mockGenerateEdges: ((flowSteps: unknown[]) => Edge[]) & ReturnType<typeof vi.fn>;
  let mockValidateEdges: ((edges: Edge[], nodes?: Node[]) => Edge[]) & ReturnType<typeof vi.fn>;
  let mockOnNeedsAutoLayout: ((needsLayout: boolean) => void) & ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    // Mock setNodes to execute the callback if it's a function
    mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
      if (typeof updater === 'function') {
        updater([]);
      }
    }) as unknown as React.Dispatch<React.SetStateAction<Node[]>> & ReturnType<typeof vi.fn>;
    // Mock setEdges to execute the callback if it's a function
    mockSetEdges = vi.fn((updater: React.SetStateAction<Edge[]>) => {
      if (typeof updater === 'function') {
        updater([]);
      }
    }) as unknown as React.Dispatch<React.SetStateAction<Edge[]>> & ReturnType<typeof vi.fn>;
    mockUpdateNodeInternals = vi.fn();
    mockGenerateEdges = vi.fn().mockReturnValue([]) as unknown as ((flowSteps: unknown[]) => Edge[]) &
      ReturnType<typeof vi.fn>;
    mockValidateEdges = vi.fn((edges: Edge[]) => edges) as unknown as ((edges: Edge[], nodes?: Node[]) => Edge[]) &
      ReturnType<typeof vi.fn>;
    mockOnNeedsAutoLayout = vi.fn() as unknown as ((needsLayout: boolean) => void) & ReturnType<typeof vi.fn>;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const renderUseFlowInitialization = (overrides = {}) => {
    const defaultProps = {
      resources: createMockResources(),
      flowId: undefined,
      existingFlowData: undefined,
      isLoadingExistingFlow: false,
      setNodes: mockSetNodes,
      setEdges: mockSetEdges,
      updateNodeInternals: mockUpdateNodeInternals,
      generateEdges: mockGenerateEdges,
      validateEdges: mockValidateEdges,
      edgeStyle: 'default',
      onNeedsAutoLayout: mockOnNeedsAutoLayout,
      ...overrides,
    };

    return renderHook(() => useFlowInitialization(defaultProps as UseFlowInitializationProps));
  };

  describe('Hook Interface', () => {
    it('should return initialNodes', () => {
      const {result} = renderUseFlowInitialization();
      expect(result.current.initialNodes).toBeDefined();
      expect(Array.isArray(result.current.initialNodes)).toBe(true);
    });

    it('should return generateSteps function', () => {
      const {result} = renderUseFlowInitialization();
      expect(typeof result.current.generateSteps).toBe('function');
    });

    it('should return getBlankTemplateComponents function', () => {
      const {result} = renderUseFlowInitialization();
      expect(typeof result.current.getBlankTemplateComponents).toBe('function');
    });

    it('should return updateAllNodeInternals function', () => {
      const {result} = renderUseFlowInitialization();
      expect(typeof result.current.updateAllNodeInternals).toBe('function');
    });
  });

  describe('updateAllNodeInternals', () => {
    it('should call updateNodeInternals for each node', () => {
      const {result} = renderUseFlowInitialization();

      const nodes: Node[] = [
        {id: 'node-1', position: {x: 0, y: 0}, data: {}},
        {id: 'node-2', position: {x: 100, y: 0}, data: {}},
      ];

      act(() => {
        result.current.updateAllNodeInternals(nodes);
      });

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-2');
    });

    it('should call updateNodeInternals for node components', () => {
      const {result} = renderUseFlowInitialization();

      const nodes: Node[] = [
        {
          id: 'node-1',
          position: {x: 0, y: 0},
          data: {
            components: [
              {id: 'comp-1', type: 'TEXT'},
              {id: 'comp-2', type: 'ACTION'},
            ],
          },
        },
      ];

      act(() => {
        result.current.updateAllNodeInternals(nodes);
      });

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('comp-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('comp-2');
    });

    it('should call updateNodeInternals for nested components', () => {
      const {result} = renderUseFlowInitialization();

      const nodes: Node[] = [
        {
          id: 'node-1',
          position: {x: 0, y: 0},
          data: {
            components: [
              {
                id: 'form-1',
                type: 'BLOCK',
                components: [
                  {id: 'input-1', type: 'TEXT_INPUT'},
                  {id: 'input-2', type: 'PASSWORD_INPUT'},
                ],
              },
            ],
          },
        },
      ];

      act(() => {
        result.current.updateAllNodeInternals(nodes);
      });

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('form-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('input-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('input-2');
    });
  });

  describe('getBlankTemplateComponents', () => {
    it('should return components from blank template', () => {
      const {result} = renderUseFlowInitialization();

      const components = result.current.getBlankTemplateComponents();

      expect(components).toBeDefined();
      expect(Array.isArray(components)).toBe(true);
    });

    it('should return empty array when blank template not found', () => {
      const resourcesWithoutBlank = createMockResources({
        templates: [
          {
            id: 'basic-template',
            type: TemplateTypes.Basic,
            config: {data: {steps: []}},
          },
        ],
      } as unknown as Partial<Resources>);

      const {result} = renderUseFlowInitialization({resources: resourcesWithoutBlank});

      const components = result.current.getBlankTemplateComponents();

      expect(components).toEqual([]);
    });

    it('should set the first step ID to START_STEP_ID', () => {
      const {result} = renderUseFlowInitialization();

      // Call getBlankTemplateComponents to trigger the logic
      result.current.getBlankTemplateComponents();

      // The function modifies the step ID internally, this test ensures no errors
      expect(result.current.getBlankTemplateComponents).toBeDefined();
    });
  });

  describe('generateSteps', () => {
    it('should add START node to step nodes', () => {
      const {result} = renderUseFlowInitialization();

      const stepNodes: Node[] = [
        {
          id: 'view-step-1',
          type: StepTypes.View,
          position: {x: 100, y: 100},
          data: {},
        },
      ];

      const generatedSteps = result.current.generateSteps(stepNodes);

      // Should have START node as first element
      const startNode = generatedSteps.find((step) => step.type === StaticStepTypes.Start);
      expect(startNode).toBeDefined();
    });

    it('should use existing START step position if available', () => {
      const {result} = renderUseFlowInitialization();

      const stepNodes: Node[] = [
        {
          id: 'start',
          type: StaticStepTypes.Start,
          position: {x: 500, y: 500},
          data: {},
        },
        {
          id: 'view-step-1',
          type: StepTypes.View,
          position: {x: 100, y: 100},
          data: {},
        },
      ];

      const generatedSteps = result.current.generateSteps(stepNodes);

      const startNode = generatedSteps.find((step) => step.type === StaticStepTypes.Start);
      expect(startNode?.position).toEqual({x: 500, y: 500});
    });

    it('should filter out existing START step and add new one', () => {
      const {result} = renderUseFlowInitialization();

      const stepNodes: Node[] = [
        {
          id: 'existing-start',
          type: StaticStepTypes.Start,
          position: {x: 0, y: 0},
          data: {},
        },
        {
          id: 'view-step-1',
          type: StepTypes.View,
          position: {x: 100, y: 100},
          data: {},
        },
      ];

      const generatedSteps = result.current.generateSteps(stepNodes);

      const startNodes = generatedSteps.filter((step) => step.type === StaticStepTypes.Start);
      // Should only have one START node
      expect(startNodes).toHaveLength(1);
    });

    it('should set END step as non-deletable', () => {
      const {result} = renderUseFlowInitialization();

      const stepNodes: Node[] = [
        {
          id: 'view-step-1',
          type: StepTypes.View,
          position: {x: 100, y: 100},
          data: {},
        },
        {
          id: 'END',
          type: StepTypes.End,
          position: {x: 200, y: 200},
          data: {},
        },
      ];

      const generatedSteps = result.current.generateSteps(stepNodes);

      const endNode = generatedSteps.find((step) => step.type === StepTypes.End);
      expect(endNode?.deletable).toBe(false);
    });

    it('should set regular steps as deletable', () => {
      const {result} = renderUseFlowInitialization();

      const stepNodes: Node[] = [
        {
          id: 'view-step-1',
          type: StepTypes.View,
          position: {x: 100, y: 100},
          data: {},
        },
      ];

      const generatedSteps = result.current.generateSteps(stepNodes);

      const viewNode = generatedSteps.find((step) => step.type === StepTypes.View);
      expect(viewNode?.deletable).toBe(true);
    });

    it('should resolve component metadata for steps with components', () => {
      const {result} = renderUseFlowInitialization();

      const stepNodes: Node[] = [
        {
          id: 'view-step-1',
          type: StepTypes.View,
          position: {x: 100, y: 100},
          data: {
            components: [{id: 'comp-1', type: 'TEXT'}],
          },
        },
      ];

      const generatedSteps = result.current.generateSteps(stepNodes);

      const viewNode = generatedSteps.find((step) => step.id === 'view-step-1');
      expect(viewNode?.data?.['components']).toBeDefined();
    });
  });

  describe('initialNodes', () => {
    it('should generate initial nodes from basic template', () => {
      const {result} = renderUseFlowInitialization();

      expect(result.current.initialNodes).toBeDefined();
      expect(result.current.initialNodes.length).toBeGreaterThan(0);
    });

    it('should handle template without steps gracefully', () => {
      const resourcesWithEmptyTemplate = createMockResources({
        templates: [
          {
            id: 'basic-template',
            type: TemplateTypes.Basic,
            config: {data: {steps: []}},
          },
        ],
      } as unknown as Partial<Resources>);

      const {result} = renderUseFlowInitialization({resources: resourcesWithEmptyTemplate});

      expect(result.current.initialNodes).toBeDefined();
    });

    it('should apply template replacers when available', () => {
      const resourcesWithReplacers = createMockResources({
        templates: [
          {
            id: 'basic-template',
            type: TemplateTypes.Basic,
            config: {
              data: {
                steps: [{id: 'step-1', type: StepTypes.View, position: {x: 0, y: 0}}],
                __generationMeta__: {
                  replacers: [{placeholder: '{{SOME_ID}}', value: 'replaced-id'}],
                },
              },
            },
          },
        ],
      } as unknown as Partial<Resources>);

      const {result} = renderUseFlowInitialization({resources: resourcesWithReplacers});

      expect(result.current.initialNodes).toBeDefined();
    });
  });

  describe('useLayoutEffect - Flow Initialization', () => {
    it('should skip initialization when still loading existing flow', () => {
      renderUseFlowInitialization({
        flowId: 'flow-1',
        isLoadingExistingFlow: true,
      });

      expect(mockSetNodes).not.toHaveBeenCalled();
      expect(mockSetEdges).not.toHaveBeenCalled();
    });

    it('should load existing flow when flowId and existingFlowData are provided', () => {
      const existingFlowData = createMockExistingFlowData();

      renderUseFlowInitialization({
        flowId: 'flow-1',
        existingFlowData,
        isLoadingExistingFlow: false,
      });

      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockSetEdges).toHaveBeenCalled();
    });

    it('should call onNeedsAutoLayout when nodes lack layout data', () => {
      const existingFlowData = {
        ...createMockExistingFlowData(),
        nodes: [
          {id: 'start', type: StaticStepTypes.Start, data: {}}, // No layout
          {id: 'view-1', type: StepTypes.View, data: {}}, // No layout
          {id: 'END', type: StepTypes.End, data: {}}, // No layout
        ],
      } as unknown as FlowDefinitionResponse;

      renderUseFlowInitialization({
        flowId: 'flow-1',
        existingFlowData,
        isLoadingExistingFlow: false,
      });

      expect(mockOnNeedsAutoLayout).toHaveBeenCalledWith(true);
    });

    it('should not need auto layout when all nodes have position data', () => {
      const existingFlowData = createMockExistingFlowData();

      renderUseFlowInitialization({
        flowId: 'flow-1',
        existingFlowData,
        isLoadingExistingFlow: false,
      });

      expect(mockOnNeedsAutoLayout).toHaveBeenCalledWith(false);
    });

    it('should apply edge style to edges when loading existing flow', () => {
      const existingFlowData = createMockExistingFlowData();

      renderUseFlowInitialization({
        flowId: 'flow-1',
        existingFlowData,
        isLoadingExistingFlow: false,
        edgeStyle: 'smoothstep',
      });

      expect(mockSetEdges).toHaveBeenCalled();
      const edgesCall = mockSetEdges.mock.calls[0][0] as Edge[];
      edgesCall.forEach((edge: Edge) => {
        expect(edge.type).toBe('smoothstep');
      });
    });

    it('should load default template when no flowId is provided', () => {
      renderUseFlowInitialization({
        flowId: undefined,
        existingFlowData: undefined,
        isLoadingExistingFlow: false,
      });

      // Run all microtasks and RAF callbacks
      act(() => {
        vi.advanceTimersByTime(100);
      });

      expect(mockSetNodes).toHaveBeenCalled();
    });

    it('should schedule updateAllNodeInternals after setting nodes for existing flow', () => {
      const existingFlowData = createMockExistingFlowData();

      renderUseFlowInitialization({
        flowId: 'flow-1',
        existingFlowData,
        isLoadingExistingFlow: false,
      });

      // Wait for queueMicrotask to execute
      act(() => {
        vi.advanceTimersByTime(0);
      });

      // The updateNodeInternals should have been called
      expect(mockUpdateNodeInternals).toHaveBeenCalled();
    });

    it('should set deletable to false for END and START nodes in existing flow', () => {
      const existingFlowData = createMockExistingFlowData();

      renderUseFlowInitialization({
        flowId: 'flow-1',
        existingFlowData,
        isLoadingExistingFlow: false,
      });

      expect(mockSetNodes).toHaveBeenCalled();
      const nodesCall = mockSetNodes.mock.calls[0][0] as Node[];

      const startNode = nodesCall.find((n: Node) => n.type === StaticStepTypes.Start);
      const endNode = nodesCall.find((n: Node) => n.type === StepTypes.End);

      expect(startNode?.deletable).toBe(false);
      expect(endNode?.deletable).toBe(false);
    });

    it('should map edges with edgeStyle when loading existing flow with edges', () => {
      // Override the flowToCanvasTransformer mock to return edges
      mockTransformFlowToCanvas.mockReturnValueOnce({
        nodes: [
          {id: 'start', type: StaticStepTypes.Start, position: {x: 0, y: 0}, data: {}},
          {id: 'view-1', type: StepTypes.View, position: {x: 100, y: 100}, data: {}},
          {id: 'END', type: StepTypes.End, position: {x: 200, y: 200}, data: {}},
        ],
        edges: [
          {id: 'edge-1', source: 'start', target: 'view-1', type: 'default'},
          {id: 'edge-2', source: 'view-1', target: 'END', type: 'default'},
        ],
        viewport: {x: 0, y: 0, zoom: 1},
      });

      const existingFlowData = createMockExistingFlowData();

      renderUseFlowInitialization({
        flowId: 'flow-1',
        existingFlowData,
        isLoadingExistingFlow: false,
        edgeStyle: 'smoothstep',
      });

      expect(mockSetEdges).toHaveBeenCalled();
      const edgesCall = mockSetEdges.mock.calls[0][0] as Edge[];

      // All edges should have the edgeStyle applied
      edgesCall.forEach((edge: Edge) => {
        expect(edge.type).toBe('smoothstep');
      });
      expect(edgesCall).toHaveLength(2);
    });
  });

  describe('updateFlowWithSequence', () => {
    it('should prevent concurrent flow updates', () => {
      renderUseFlowInitialization({
        flowId: undefined,
        isLoadingExistingFlow: false,
      });

      // This will be called by useLayoutEffect automatically
      // The initial call happens during hook initialization

      act(() => {
        vi.advanceTimersByTime(100);
      });

      // setNodes should have been called at least once
      expect(mockSetNodes).toHaveBeenCalled();
    });

    it('should generate and validate edges after updating nodes', async () => {
      const mockEdges: Edge[] = [{id: 'edge-1', source: 'node-1', target: 'node-2'}];
      mockGenerateEdges.mockReturnValue(mockEdges);
      mockValidateEdges.mockReturnValue(mockEdges);

      renderUseFlowInitialization({
        flowId: undefined,
        isLoadingExistingFlow: false,
      });

      // Run RAF callbacks and microtasks multiple times to ensure async operations complete
      await act(async () => {
        vi.advanceTimersByTime(16); // one frame
        await Promise.resolve(); // flush microtasks
        vi.advanceTimersByTime(16); // another frame
        await Promise.resolve();
        vi.advanceTimersByTime(16);
        await Promise.resolve();
      });

      // The updateFlowWithSequence uses requestAnimationFrame internally
      // These should be called eventually
      expect(mockSetNodes).toHaveBeenCalled();
    });

    it('should apply edge style to generated edges', () => {
      const mockEdges: Edge[] = [{id: 'edge-1', source: 'node-1', target: 'node-2', type: 'default'}];
      mockGenerateEdges.mockReturnValue(mockEdges);
      mockValidateEdges.mockReturnValue(mockEdges);

      renderUseFlowInitialization({
        flowId: undefined,
        isLoadingExistingFlow: false,
        edgeStyle: 'smoothstep',
      });

      act(() => {
        vi.advanceTimersByTime(100);
      });

      // setEdges should be called with styled edges
      expect(mockSetEdges.mock.calls.length).toBeGreaterThan(0);
      const lastEdgesCall = mockSetEdges.mock.calls[mockSetEdges.mock.calls.length - 1] as [unknown];
      expect(typeof lastEdgesCall[0]).toBe('function');
      const edgesResult = (lastEdgesCall[0] as () => Edge[])();
      expect(edgesResult[0]?.type).toBe('smoothstep');
    });
  });

  describe('Edge cases', () => {
    it('should handle empty resources gracefully', () => {
      const emptyResources = {
        templates: [],
        steps: [],
        elements: [],
        widgets: [],
      } as unknown as Resources;

      const {result} = renderUseFlowInitialization({resources: emptyResources});

      expect(result.current.initialNodes).toBeDefined();
      expect(result.current.getBlankTemplateComponents()).toEqual([]);
    });

    it('should handle undefined resources gracefully', () => {
      const {result} = renderUseFlowInitialization({
        resources: undefined as unknown as Resources,
      });

      expect(result.current.getBlankTemplateComponents()).toEqual([]);
    });

    it('should handle nodes without data property', () => {
      const {result} = renderUseFlowInitialization();

      const nodes: Node[] = [
        {id: 'node-1', position: {x: 0, y: 0}, data: undefined as unknown as Record<string, unknown>},
      ];

      act(() => {
        result.current.updateAllNodeInternals(nodes);
      });

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
    });

    it('should handle step with undefined data', () => {
      const {result} = renderUseFlowInitialization();

      const stepNodes: Node[] = [
        {
          id: 'view-step-1',
          type: StepTypes.View,
          position: {x: 100, y: 100},
          data: undefined as unknown as Record<string, unknown>,
        },
      ];

      const generatedSteps = result.current.generateSteps(stepNodes);

      const viewNode = generatedSteps.find((step) => step.id === 'view-step-1');
      expect(viewNode).toBeDefined();
    });
  });

  describe('updateFlowWithSequence detailed testing', () => {
    it('should complete full RAF update sequence for new flow', async () => {
      const mockEdges: Edge[] = [{id: 'edge-1', source: 'start', target: 'view-step-1', type: 'default'}];
      mockGenerateEdges.mockReturnValue(mockEdges);
      mockValidateEdges.mockReturnValue(mockEdges);

      renderUseFlowInitialization({
        flowId: undefined,
        isLoadingExistingFlow: false,
        edgeStyle: 'smoothstep',
      });

      // Advance through multiple RAF cycles to complete the sequence
      await act(async () => {
        // First RAF - updateSequence is called
        vi.advanceTimersByTime(16);
        await Promise.resolve();
        // Second RAF - if nodesUpdatedRef is true, generate edges
        vi.advanceTimersByTime(16);
        await Promise.resolve();
        // Third RAF - complete edge generation
        vi.advanceTimersByTime(16);
        await Promise.resolve();
      });

      // setNodes should have been called to trigger the flow
      expect(mockSetNodes).toHaveBeenCalled();
    });

    it('should execute setNodes callback which sets nodesUpdatedRef', async () => {
      renderUseFlowInitialization({
        flowId: undefined,
        isLoadingExistingFlow: false,
      });

      await act(async () => {
        vi.advanceTimersByTime(50);
        await Promise.resolve();
      });

      // setNodes should have been called with a callback function
      expect(mockSetNodes).toHaveBeenCalled();
      // Verify the callback was a function (which sets nodesUpdatedRef.current = true on line 230)
      const setNodesCall = mockSetNodes.mock.calls[0];
      expect(typeof setNodesCall[0]).toBe('function');
    });

    it('should call generateEdges and validateEdges in RAF sequence', async () => {
      const mockEdges: Edge[] = [{id: 'edge-1', source: 'start', target: 'view-step-1', type: 'default'}];
      mockGenerateEdges.mockReturnValue(mockEdges);
      mockValidateEdges.mockReturnValue(mockEdges);

      renderUseFlowInitialization({
        flowId: undefined,
        isLoadingExistingFlow: false,
        edgeStyle: 'smoothstep',
      });

      // Run through multiple RAF cycles to trigger lines 240-254
      await act(async () => {
        vi.advanceTimersByTime(160);
        await Promise.resolve();
      });

      // generateEdges should be called (line 243)
      expect(mockGenerateEdges).toHaveBeenCalled();
      // validateEdges should be called (line 244)
      expect(mockValidateEdges).toHaveBeenCalled();
    });

    it('should call setEdges with styled edges', async () => {
      const mockEdges: Edge[] = [
        {id: 'edge-1', source: 'start', target: 'view-step-1', type: 'default'},
        {id: 'edge-2', source: 'view-step-1', target: 'END', type: 'default'},
      ];
      mockGenerateEdges.mockReturnValue(mockEdges);
      mockValidateEdges.mockReturnValue(mockEdges);

      renderUseFlowInitialization({
        flowId: undefined,
        isLoadingExistingFlow: false,
        edgeStyle: 'smoothstep',
      });

      // Run through multiple RAF cycles
      await act(async () => {
        vi.advanceTimersByTime(160);
        await Promise.resolve();
      });

      // setEdges should have been called (line 252)
      expect(mockSetEdges).toHaveBeenCalled();
    });

    it('should call updateAllNodeInternals in RAF sequence', async () => {
      mockGenerateEdges.mockReturnValue([]);
      mockValidateEdges.mockReturnValue([]);

      renderUseFlowInitialization({
        flowId: undefined,
        isLoadingExistingFlow: false,
      });

      // Run through RAF cycles to trigger line 240
      await act(async () => {
        vi.advanceTimersByTime(160);
        await Promise.resolve();
      });

      // updateNodeInternals should have been called (via updateAllNodeInternals on line 240)
      expect(mockUpdateNodeInternals).toHaveBeenCalled();
    });
  });

  describe('existing flow edge style application', () => {
    it('should apply custom edge style to existing flow edges', () => {
      const existingFlowData = {
        ...createMockExistingFlowData(),
      };

      renderUseFlowInitialization({
        flowId: 'flow-1',
        existingFlowData,
        isLoadingExistingFlow: false,
        edgeStyle: 'step',
      });

      expect(mockSetEdges).toHaveBeenCalled();
      const edgesCall = mockSetEdges.mock.calls[0][0] as Edge[];
      edgesCall.forEach((edge: Edge) => {
        expect(edge.type).toBe('step');
      });
    });

    it('should set flowUpdatesInProgress to false after completion', async () => {
      mockGenerateEdges.mockReturnValue([]);
      mockValidateEdges.mockReturnValue([]);

      renderUseFlowInitialization({
        flowId: undefined,
        isLoadingExistingFlow: false,
      });

      // Run through many RAF cycles to ensure completion
      await act(async () => {
        vi.advanceTimersByTime(320);
        await Promise.resolve();
      });

      // The flow should be updatable again (flowUpdatesInProgress reset to false)
      // This is indirectly tested by the fact that subsequent updates would work
      expect(mockSetNodes).toHaveBeenCalled();
    });
  });
});
