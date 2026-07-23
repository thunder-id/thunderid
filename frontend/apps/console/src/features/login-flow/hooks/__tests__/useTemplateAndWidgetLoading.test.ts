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
import type {Edge, Node} from '@xyflow/react';
import type React from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import useTemplateAndWidgetLoading from '../useTemplateAndWidgetLoading';
import {BlockTypes, ElementCategories, ElementTypes, type Element} from '@/features/flows/models/elements';
import {ResourceTypes, type Resource, type Resources} from '@/features/flows/models/resources';
import {StepTypes, type Step} from '@/features/flows/models/steps';
import {TemplateTypes, type Template} from '@/features/flows/models/templates';
import type {Widget} from '@/features/flows/models/widget';

// Mock external dependencies
vi.mock('lodash-es/cloneDeep', () => ({
  default: <T>(obj: T): T => {
    if (obj === undefined || obj === null) {
      return obj;
    }
    return JSON.parse(JSON.stringify(obj)) as T;
  },
}));

vi.mock('lodash-es/isEmpty', () => ({
  default: (value: unknown): boolean => {
    if (value === null || value === undefined) return true;
    if (Array.isArray(value)) return value.length === 0;
    if (typeof value === 'object') return Object.keys(value).length === 0;
    return false;
  },
}));

vi.mock('lodash-es/mergeWith', () => ({
  default: (target: unknown, source: unknown, customizer: (a: unknown, b: unknown, key: string) => unknown) => {
    const result = {...(target as object)};
    const sourceObj = source as Record<string, unknown>;
    const targetObj = target as Record<string, unknown>;
    Object.keys(sourceObj).forEach((key) => {
      const customValue = customizer(targetObj[key], sourceObj[key], key);
      if (customValue !== undefined) {
        (result as Record<string, unknown>)[key] = customValue;
      } else {
        (result as Record<string, unknown>)[key] = sourceObj[key];
      }
    });
    return result;
  },
}));

vi.mock('@/features/flows/utils/generateIdsForResources', () => ({
  default: <T>(obj: T): T => obj,
}));

vi.mock('@/features/flows/utils/resolveComponentMetadata', () => ({
  default: (_resources: unknown, components: unknown) => components,
}));

vi.mock('@/features/flows/utils/resolveStepMetadata', () => ({
  default: (_resources: unknown, steps: unknown[]) => steps,
}));

vi.mock('@/features/flows/utils/updateTemplatePlaceholderReferences', () => ({
  default: (nodes: Node[]) => {
    const replacedPlaceholders = new Map<string, string>();
    // Simulate placeholder replacement
    nodes.forEach((node) => {
      if (node.id.includes('{{')) {
        const cleanId = node.id.replace(/[{}]/g, '');
        replacedPlaceholders.set(cleanId, `replaced-${cleanId}`);
      }
    });
    return [nodes, replacedPlaceholders];
  },
}));

const mockSetFlowCompletionConfigs = vi.fn();
const mockEdgeStyle = 'default';

vi.mock('@/features/flows/hooks/useFlowConfig', () => ({
  default: () => ({
    setFlowCompletionConfigs: mockSetFlowCompletionConfigs,
    edgeStyle: mockEdgeStyle,
  }),
}));

const mockGenerateStepElement = vi.fn((element: Element) => ({
  ...element,
  id: `generated-${element.id}`,
}));

vi.mock('@/features/flows/hooks/useGenerateStepElement', () => ({
  default: () => ({
    generateStepElement: mockGenerateStepElement,
  }),
}));

vi.mock('../../utils/edgeUtils', () => ({
  default: () => [],
}));

vi.mock('../../utils/componentMutations', () => ({
  mutateComponents: (components: Element[]) => components,
}));

const createMockResources = (): Resources =>
  ({
    templates: [],
    steps: [],
    elements: [],
    widgets: [],
  }) as unknown as Resources;

const createMockStep = (overrides: Partial<Step> = {}): Step =>
  ({
    id: 'step-1',
    type: StepTypes.View,
    position: {x: 0, y: 0},
    data: {},
    ...overrides,
  }) as Step;

const createMockElement = (overrides: Partial<Element> = {}): Element =>
  ({
    id: 'element-1',
    type: ElementTypes.Action,
    category: ElementCategories.Action,
    version: '1.0.0',
    deprecated: false,
    resourceType: ResourceTypes.Element,
    display: {label: 'Element', image: ''},
    config: {},
    ...overrides,
  }) as Element;

const createMockNode = (overrides: Partial<Node> = {}): Node => ({
  id: 'node-1',
  type: StepTypes.View,
  position: {x: 0, y: 0},
  data: {},
  ...overrides,
});

const createMockEdge = (overrides: Partial<Edge> = {}): Edge => ({
  id: 'edge-1',
  source: 'node-1',
  target: 'node-2',
  type: 'default',
  ...overrides,
});

type SetNodesFn = React.Dispatch<React.SetStateAction<Node[]>>;

describe('useTemplateAndWidgetLoading', () => {
  let mockSetNodes: SetNodesFn;
  let mockUpdateNodeInternals: ReturnType<typeof vi.fn>;
  let mockGenerateSteps: ReturnType<typeof vi.fn>;
  let mockGenerateEdges: ReturnType<typeof vi.fn>;
  let mockValidateEdges: ReturnType<typeof vi.fn>;
  let mockGetBlankTemplateComponents: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.clearAllMocks();
    mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
      if (typeof updater === 'function') {
        return updater([]);
      }
      return updater;
    }) as unknown as SetNodesFn;
    mockUpdateNodeInternals = vi.fn();
    mockGenerateSteps = vi.fn((steps: Node[]) => steps);
    mockGenerateEdges = vi.fn().mockReturnValue([]);
    mockValidateEdges = vi.fn((edges: Edge[]) => edges);
    mockGetBlankTemplateComponents = vi.fn().mockReturnValue([{id: 'blank-comp', type: 'TEXT'}]);
  });

  const renderUseTemplateAndWidgetLoading = (overrides = {}) => {
    const defaultProps = {
      resources: createMockResources(),
      generateSteps: mockGenerateSteps as unknown as (steps: Node[]) => Node[],
      generateEdges: mockGenerateEdges as unknown as () => Edge[],
      validateEdges: mockValidateEdges as unknown as (edges: Edge[]) => Edge[],
      getBlankTemplateComponents: mockGetBlankTemplateComponents as unknown as () => Element[],
      setNodes: mockSetNodes,
      updateNodeInternals: mockUpdateNodeInternals as unknown as (nodeId: string | string[]) => void,
      ...overrides,
    };

    return renderHook(() => useTemplateAndWidgetLoading(defaultProps));
  };

  describe('Hook Interface', () => {
    it('should return handleStepLoad function', () => {
      const {result} = renderUseTemplateAndWidgetLoading();
      expect(typeof result.current.handleStepLoad).toBe('function');
    });

    it('should return handleTemplateLoad function', () => {
      const {result} = renderUseTemplateAndWidgetLoading();
      expect(typeof result.current.handleTemplateLoad).toBe('function');
    });

    it('should return handleWidgetLoad function', () => {
      const {result} = renderUseTemplateAndWidgetLoading();
      expect(typeof result.current.handleWidgetLoad).toBe('function');
    });

    it('should return handleResourceAdd function', () => {
      const {result} = renderUseTemplateAndWidgetLoading();
      expect(typeof result.current.handleResourceAdd).toBe('function');
    });
  });

  describe('handleStepLoad', () => {
    it('should add blank template components to VIEW step without components', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const step = createMockStep({
        type: StepTypes.View,
        data: {components: []},
      });

      const loadedStep = result.current.handleStepLoad(step);

      expect(mockGetBlankTemplateComponents).toHaveBeenCalled();
      expect(loadedStep.data?.components).toBeDefined();
    });

    it('should preserve existing components in VIEW step', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const existingComponents = [createMockElement({id: 'existing-comp', type: ElementTypes.Text})];
      const step = createMockStep({
        type: StepTypes.View,
        data: {components: existingComponents},
      });

      const loadedStep = result.current.handleStepLoad(step);

      expect(mockGetBlankTemplateComponents).not.toHaveBeenCalled();
      expect(loadedStep.data?.components).toEqual(existingComponents);
    });

    it('should process non-VIEW steps without adding blank components', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const step = createMockStep({
        type: StepTypes.End,
        data: {},
      });

      const loadedStep = result.current.handleStepLoad(step);

      expect(mockGetBlankTemplateComponents).not.toHaveBeenCalled();
      expect(loadedStep.type).toBe(StepTypes.End);
    });
  });

  describe('handleTemplateLoad', () => {
    it('should return empty arrays when template has no steps', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const template = {
        id: 'template-1',
        type: TemplateTypes.Basic,
        config: {data: {}},
      } as Template;

      const [nodes, edges] = result.current.handleTemplateLoad(template);

      expect(nodes).toEqual([]);
      expect(edges).toEqual([]);
    });

    it('should call setFlowCompletionConfigs for End steps with config', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const template = {
        id: 'template-1',
        type: TemplateTypes.Basic,
        config: {
          data: {
            steps: [
              createMockStep({type: StepTypes.View}),
              createMockStep({
                type: StepTypes.End,
                config: {completionType: 'success'} as unknown as Step['config'],
              }),
            ],
          },
        },
      } as unknown as Template;

      result.current.handleTemplateLoad(template);

      expect(mockSetFlowCompletionConfigs).toHaveBeenCalledWith({completionType: 'success'});
    });

    it('should return execution step for BasicFederated template', () => {
      mockGenerateSteps.mockReturnValue([
        createMockNode({id: 'view-1', type: StepTypes.View}),
        createMockNode({id: 'execution-1', type: StepTypes.Execution}),
      ]);

      const {result} = renderUseTemplateAndWidgetLoading();

      const template = {
        id: 'template-1',
        type: TemplateTypes.BasicFederated,
        config: {
          data: {
            steps: [
              createMockStep({id: 'view-1', type: StepTypes.View}),
              createMockStep({id: 'execution-1', type: StepTypes.Execution}),
            ],
          },
        },
      } as unknown as Template;

      const [nodes, , resource, stepId] = result.current.handleTemplateLoad(template);

      expect(nodes).toHaveLength(2);
      expect(resource).toBeDefined();
      expect(stepId).toBe('execution-1');
    });

    it('should apply edge style to generated edges', () => {
      mockGenerateEdges.mockReturnValue([createMockEdge()]);
      mockValidateEdges.mockReturnValue([createMockEdge()]);

      const {result} = renderUseTemplateAndWidgetLoading();

      const template = {
        id: 'template-1',
        type: TemplateTypes.Basic,
        config: {
          data: {
            steps: [createMockStep()],
          },
        },
      } as unknown as Template;

      const [, edges] = result.current.handleTemplateLoad(template);

      expect(edges[0].type).toBe('default');
    });

    it('should process template with replacers', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const template = {
        id: 'template-1',
        type: TemplateTypes.Basic,
        config: {
          data: {
            steps: [createMockStep()],
            __generationMeta__: {
              replacers: [{placeholder: '{{ID}}', value: 'new-id'}],
            },
          },
        },
      } as unknown as Template;

      const [nodes] = result.current.handleTemplateLoad(template);

      expect(nodes).toBeDefined();
    });
  });

  describe('handleWidgetLoad', () => {
    it('should return unchanged nodes and edges when widget has no steps', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const widget = {
        id: 'widget-1',
        config: {data: {}},
      } as unknown as Widget;

      const currentNodes = [createMockNode()];
      const currentEdges = [createMockEdge()];

      const [nodes, edges, selector, stepId] = result.current.handleWidgetLoad(
        widget,
        {} as Resource,
        currentNodes,
        currentEdges,
      );

      expect(nodes).toEqual(currentNodes);
      expect(edges).toEqual(currentEdges);
      expect(selector).toBeNull();
      expect(stepId).toBeNull();
    });

    it('should merge widget step with target resource when strategy is MERGE_WITH_DROP_POINT', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const widget = {
        id: 'widget-1',
        config: {
          data: {
            steps: [
              {
                id: 'widget-step-1',
                type: StepTypes.View,
                __generationMeta__: {strategy: 'MERGE_WITH_DROP_POINT'},
                data: {components: [{id: 'new-comp', type: 'TEXT'}]},
              },
            ],
          },
        },
      } as unknown as Widget;

      const targetResource = {id: 'target-node', resourceType: ResourceTypes.Step} as Resource;
      const currentNodes = [createMockNode({id: 'target-node', data: {components: [{id: 'existing-comp'}]}})];
      const currentEdges: Edge[] = [];

      const [nodes] = result.current.handleWidgetLoad(widget, targetResource, currentNodes, currentEdges);

      expect(nodes).toBeDefined();
    });

    it('should add widget step as new node when no merge strategy', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const widget = {
        id: 'widget-1',
        config: {
          data: {
            steps: [
              {
                id: 'new-step',
                type: StepTypes.Execution,
                position: {x: 100, y: 100},
              },
            ],
          },
        },
      } as unknown as Widget;

      const currentNodes = [createMockNode({id: 'existing-node'})];
      const currentEdges: Edge[] = [];

      const [nodes] = result.current.handleWidgetLoad(widget, {} as Resource, currentNodes, currentEdges);

      expect(nodes.length).toBeGreaterThanOrEqual(1);
    });

    it('should find default property selector at node level', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const widget = {
        id: 'widget-1',
        config: {
          data: {
            steps: [
              {
                id: 'selector-node',
                type: StepTypes.View,
              },
            ],
            __generationMeta__: {
              defaultPropertySelectorId: 'selector-node',
            },
          },
        },
      } as unknown as Widget;

      const currentNodes: Node[] = [];
      const currentEdges: Edge[] = [];

      const [, , selector, stepId] = result.current.handleWidgetLoad(
        widget,
        {} as Resource,
        currentNodes,
        currentEdges,
      );

      expect(selector).toBeDefined();
      expect(stepId).toBe('selector-node');
    });

    it('should find default property selector in component', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const widget = {
        id: 'widget-1',
        config: {
          data: {
            steps: [
              {
                id: 'node-1',
                type: StepTypes.View,
                position: {x: 0, y: 0},
                data: {
                  components: [{id: 'selector-comp', type: 'TEXT'}],
                },
              },
            ],
            __generationMeta__: {
              defaultPropertySelectorId: 'selector-comp',
            },
          },
        },
      } as unknown as Widget;

      const currentNodes: Node[] = [];
      const currentEdges: Edge[] = [];

      const [, , selector, stepId] = result.current.handleWidgetLoad(
        widget,
        {} as Resource,
        currentNodes,
        currentEdges,
      );

      expect(selector).toBeDefined();
      expect(stepId).toBe('node-1');
    });

    it('should find default property selector in nested component', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const widget = {
        id: 'widget-1',
        config: {
          data: {
            steps: [
              {
                id: 'node-1',
                type: StepTypes.View,
                position: {x: 0, y: 0},
                data: {
                  components: [
                    {
                      id: 'form-1',
                      type: 'BLOCK',
                      components: [{id: 'nested-selector', type: 'TEXT_INPUT'}],
                    },
                  ],
                },
              },
            ],
            __generationMeta__: {
              defaultPropertySelectorId: 'form-1',
            },
          },
        },
      } as unknown as Widget;

      const currentNodes: Node[] = [];
      const currentEdges: Edge[] = [];

      const [, , selector] = result.current.handleWidgetLoad(widget, {} as Resource, currentNodes, currentEdges);

      expect(selector).toBeDefined();
    });

    it('should find default property selector when component has nested children and matches selector id', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      // Test the case where we search through component.components (lines 275-280)
      // The selector is a component that HAS nested components (not empty)
      const widget = {
        id: 'widget-1',
        config: {
          data: {
            steps: [
              {
                id: 'node-1',
                type: StepTypes.View,
                position: {x: 0, y: 0},
                data: {
                  components: [
                    {
                      id: 'form-with-children',
                      type: 'FORM',
                      // This component has nested components AND its id matches defaultPropertySelectorId
                      components: [{id: 'input-1', type: 'TEXT_INPUT'}],
                    },
                  ],
                },
              },
            ],
            __generationMeta__: {
              defaultPropertySelectorId: 'form-with-children',
            },
          },
        },
      } as unknown as Widget;

      const currentNodes: Node[] = [];
      const currentEdges: Edge[] = [];

      const [nodes, , selector, stepId] = result.current.handleWidgetLoad(
        widget,
        {} as Resource,
        currentNodes,
        currentEdges,
      );

      // The widget should process nodes correctly
      expect(nodes).toBeDefined();
      // Selector should be found at the component level
      expect(selector).toBeDefined();
      expect(stepId).toBe('node-1');
    });

    it('should handle widget with nested component structure', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      // Test processing a widget with nested component structure
      const widget = {
        id: 'widget-2',
        config: {
          data: {
            steps: [
              {
                id: 'view-node',
                type: StepTypes.View,
                position: {x: 0, y: 0},
                data: {
                  components: [
                    {
                      id: 'form-block',
                      type: 'FORM',
                      components: [
                        {id: 'input-1', type: 'TEXT_INPUT'},
                        {id: 'input-2', type: 'PASSWORD_INPUT'},
                      ],
                    },
                  ],
                },
              },
            ],
            __generationMeta__: {
              defaultPropertySelectorId: 'form-block',
            },
          },
        },
      } as unknown as Widget;

      const [nodes, edges] = result.current.handleWidgetLoad(widget, {} as Resource, [], []);

      // The widget should be processed
      expect(nodes).toBeDefined();
      expect(edges).toBeDefined();
    });

    it('should iterate through component.components when searching for selector', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      // This tests the branch at lines 275-280 where we check if component has nested components
      const widget = {
        id: 'widget-3',
        config: {
          data: {
            steps: [
              {
                id: 'step-1',
                type: StepTypes.View,
                position: {x: 0, y: 0},
                data: {
                  components: [
                    {
                      id: 'parent-block',
                      type: 'BLOCK',
                      // This component HAS nested components
                      components: [{id: 'nested-form', type: 'FORM'}],
                    },
                  ],
                },
              },
            ],
            __generationMeta__: {
              // Selector matches the parent which has components
              defaultPropertySelectorId: 'parent-block',
            },
          },
        },
      } as unknown as Widget;

      const [nodes, , selector] = result.current.handleWidgetLoad(widget, {} as Resource, [], []);

      // Widget should be processed successfully
      expect(nodes).toBeDefined();
      expect(selector).toBeDefined();
    });

    it('should handle placeholder replacement for selector id', () => {
      // Create a custom mock for updateTemplatePlaceholderReferences
      vi.doMock('@/features/flows/utils/updateTemplatePlaceholderReferences', () => ({
        default: (nodes: Node[]) => {
          const replacedPlaceholders = new Map<string, string>();
          replacedPlaceholders.set('SELECTOR_ID', 'replaced-selector-id');
          return [nodes, replacedPlaceholders];
        },
      }));

      const {result} = renderUseTemplateAndWidgetLoading();

      const widget = {
        id: 'widget-1',
        config: {
          data: {
            steps: [
              {
                id: '{{SELECTOR_ID}}',
                type: StepTypes.View,
                position: {x: 0, y: 0},
              },
            ],
            __generationMeta__: {
              replacers: [{placeholder: '{{SELECTOR_ID}}', value: 'new-id'}],
              defaultPropertySelectorId: '{{SELECTOR_ID}}',
            },
          },
        },
      } as unknown as Widget;

      const currentNodes: Node[] = [];
      const currentEdges: Edge[] = [];

      const [nodes] = result.current.handleWidgetLoad(widget, {} as Resource, currentNodes, currentEdges);

      expect(nodes).toBeDefined();
    });

    it('should not modify node when it does not match target resource', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const widget = {
        id: 'widget-1',
        config: {
          data: {
            steps: [
              {
                id: 'widget-step',
                type: StepTypes.View,
                __generationMeta__: {strategy: 'MERGE_WITH_DROP_POINT'},
              },
            ],
          },
        },
      } as unknown as Widget;

      const targetResource = {id: 'target-node', resourceType: ResourceTypes.Step} as Resource;
      const currentNodes = [
        createMockNode({id: 'other-node'}), // Different ID
      ];
      const currentEdges: Edge[] = [];

      const [nodes] = result.current.handleWidgetLoad(widget, targetResource, currentNodes, currentEdges);

      // The other-node should remain unchanged
      const otherNode = nodes.find((n) => n.id === 'other-node');
      expect(otherNode).toBeDefined();
    });

    it('auto-wires a consent-shaped widget between the authorization executor and the auth assert generator', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const widget = {
        id: 'consent-widget',
        config: {
          data: {
            __generationMeta__: {
              defaultPropertySelectorId: 'consent_check',
              autoWire: {
                entry: {stepRef: 'consent_check'},
                exit: {stepRef: 'consent_check', handle: 'success'},
                spliceAfter: [{executorName: 'AuthorizationExecutor'}],
                spliceBefore: [{executorName: 'AuthAssertExecutor'}],
              },
            },
            steps: [
              {
                id: 'consent_check',
                type: StepTypes.Execution,
                position: {x: 0, y: 0},
                data: {action: {type: 'EXECUTOR', executor: {name: 'ConsentExecutor'}, onSuccess: ''}},
              },
              {id: 'consent_view', type: StepTypes.View, position: {x: 0, y: 0}, data: {components: []}},
            ],
          },
        },
      } as unknown as Widget;

      const currentNodes: Node[] = [
        createMockNode({
          id: 'authz',
          type: StepTypes.Execution,
          data: {action: {executor: {name: 'AuthorizationExecutor'}}},
        }),
        createMockNode({
          id: 'auth_assert',
          type: StepTypes.Execution,
          data: {action: {executor: {name: 'AuthAssertExecutor'}}},
        }),
        createMockNode({id: 'end', type: StepTypes.End}),
      ];
      const currentEdges: Edge[] = [
        {id: 'authz->auth_assert', source: 'authz', target: 'auth_assert', sourceHandle: 'authz_NEXT', type: 'default'},
      ];

      const [nodes, edges] = result.current.handleWidgetLoad(widget, {} as Resource, currentNodes, currentEdges);

      expect(nodes.some((n) => n.id === 'consent_check')).toBe(true);
      expect(edges.some((e) => e.source === 'authz' && e.target === 'auth_assert')).toBe(false);
      expect(edges.some((e) => e.source === 'authz' && e.target === 'consent_check')).toBe(true);
      expect(edges.some((e) => e.source === 'consent_check' && e.target === 'auth_assert')).toBe(true);
    });
  });

  describe('handleResourceAdd', () => {
    it('should do nothing for non-Element resources', () => {
      const {result} = renderUseTemplateAndWidgetLoading();

      const resource = {
        id: 'step-1',
        resourceType: ResourceTypes.Step,
      } as Resource;

      act(() => {
        result.current.handleResourceAdd(resource);
      });

      expect(mockSetNodes).not.toHaveBeenCalled();
    });

    it('should add element to existing View step', () => {
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes = [createMockNode({id: 'view-1', type: StepTypes.View, data: {components: []}})];
          updater(nodes);
        }
      }) as unknown as SetNodesFn;

      const {result} = renderUseTemplateAndWidgetLoading({setNodes: mockSetNodes});

      const resource = createMockElement({
        id: 'button-1',
        type: ElementTypes.Action,
        resourceType: ResourceTypes.Element,
      });

      act(() => {
        result.current.handleResourceAdd(resource as Resource);
      });

      expect(mockSetNodes).toHaveBeenCalled();
      expect(mockGenerateStepElement).toHaveBeenCalled();
    });

    it('should replace existing Form when adding a Form element', () => {
      const existingForm = createMockElement({
        id: 'existing-form',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
      });

      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes = [
            createMockNode({
              id: 'view-1',
              type: StepTypes.View,
              data: {components: [existingForm]},
            }),
          ];
          capturedNodes = updater(nodes);
        }
      }) as unknown as SetNodesFn;

      mockGenerateStepElement.mockReturnValue(
        createMockElement({
          id: 'generated-new-form',
          type: BlockTypes.Form,
          category: ElementCategories.Block,
        }),
      );

      const {result} = renderUseTemplateAndWidgetLoading({setNodes: mockSetNodes});

      const newForm = createMockElement({
        id: 'new-form',
        type: BlockTypes.Form,
        category: ElementCategories.Block,
        resourceType: ResourceTypes.Element,
      });

      act(() => {
        result.current.handleResourceAdd(newForm as Resource);
      });

      expect(capturedNodes).toHaveLength(1);
      const viewNode = capturedNodes[0];
      const forms = (viewNode.data?.components as Element[])?.filter((c) => c.type === BlockTypes.Form);
      // Should only have one form (the new one replaced the old)
      expect(forms).toHaveLength(1);
      expect(forms[0].id).toBe('generated-new-form');
    });

    it('should do nothing when no View step exists', () => {
      let resultNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes = [createMockNode({id: 'end-1', type: StepTypes.End})]; // No View
          resultNodes = updater(nodes);
        }
      }) as unknown as SetNodesFn;

      const {result} = renderUseTemplateAndWidgetLoading({setNodes: mockSetNodes});

      const resource = createMockElement({
        id: 'button-1',
        type: ElementTypes.Action,
        resourceType: ResourceTypes.Element,
      });

      act(() => {
        result.current.handleResourceAdd(resource as Resource);
      });

      // Should return unchanged nodes
      expect(resultNodes).toHaveLength(1);
      expect(resultNodes[0].type).toBe(StepTypes.End);
    });

    it('should schedule updateNodeInternals after adding element', async () => {
      // Reset the mock to default behavior
      mockGenerateStepElement.mockImplementation((element: Element) => ({
        ...element,
        id: `generated-${element.id}`,
      }));

      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes = [createMockNode({id: 'view-1', type: StepTypes.View, data: {components: []}})];
          return updater(nodes);
        }
        return undefined;
      }) as unknown as SetNodesFn;

      const {result} = renderUseTemplateAndWidgetLoading({setNodes: mockSetNodes});

      const resource = createMockElement({
        id: 'button-1',
        type: ElementTypes.Action,
        resourceType: ResourceTypes.Element,
      });

      act(() => {
        result.current.handleResourceAdd(resource as Resource);
      });

      // Wait for queueMicrotask
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('view-1');
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('generated-button-1');
    });

    it('should not modify other nodes when adding to View', () => {
      let capturedNodes: Node[] = [];
      mockSetNodes = vi.fn((updater: React.SetStateAction<Node[]>) => {
        if (typeof updater === 'function') {
          const nodes = [
            createMockNode({id: 'view-1', type: StepTypes.View, data: {components: []}}),
            createMockNode({id: 'end-1', type: StepTypes.End, data: {}}),
          ];
          capturedNodes = updater(nodes);
        }
      }) as unknown as SetNodesFn;

      const {result} = renderUseTemplateAndWidgetLoading({setNodes: mockSetNodes});

      const resource = createMockElement({
        id: 'button-1',
        resourceType: ResourceTypes.Element,
      });

      act(() => {
        result.current.handleResourceAdd(resource as Resource);
      });

      expect(capturedNodes).toHaveLength(2);
      const endNode = capturedNodes.find((n) => n.id === 'end-1');
      expect(endNode?.type).toBe(StepTypes.End);
    });
  });
});
