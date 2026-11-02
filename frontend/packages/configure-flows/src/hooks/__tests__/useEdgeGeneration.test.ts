// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {MarkerType, type Edge, type Node} from '@xyflow/react';
import {describe, it, expect} from 'vitest';

import type {Element} from '../../models/elements';
import {ElementTypes} from '../../models/elements';
import {StepTypes, type Step} from '../../models/steps';
import useEdgeGeneration from '../useEdgeGeneration';

const createMockElement = (overrides: Partial<Element> = {}): Element =>
  ({
    id: 'element-1',
    type: ElementTypes.Action,
    category: 'ACTION',
    version: '1.0.0',
    deprecated: false,
    deletable: true,
    resourceType: 'ELEMENT',
    display: {
      label: 'Element Label',
      image: '',
      showOnResourcePanel: true,
    },
    config: {
      field: {name: '', type: 'ACTION'},
      styles: {},
    },
    ...overrides,
  }) as Element;

const createMockStep = (overrides: Partial<Step> = {}): Step =>
  ({
    id: 'step-1',
    type: StepTypes.View,
    category: 'INTERFACE',
    version: '1.0.0',
    deprecated: false,
    deletable: true,
    resourceType: 'STEP',
    display: {
      label: 'Step Label',
      image: '',
      showOnResourcePanel: true,
    },
    config: {
      field: {name: '', type: 'TEXT_INPUT'},
      styles: {},
    },
    size: {width: 200, height: 100},
    position: {x: 0, y: 0},
    __generationMeta__: null,
    data: {},
    ...overrides,
  }) as Step;

describe('useEdgeGeneration', () => {
  describe('generateEdges', () => {
    it('should return empty array for empty steps', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const edges = result.current.generateEdges([]);

      expect(edges).toEqual([]);
    });

    it('should create edge from START to first step', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({id: 'view-step-1', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should have edge from START to first step
      const startEdge = edges.find((e) => e.source === 'start');
      expect(startEdge).toBeDefined();
      expect(startEdge?.target).toBe('view-step-1');
    });

    it('should skip START step when creating edge to first step', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({id: 'start', type: 'START' as typeof StepTypes.View}),
        createMockStep({id: 'view-step-1', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      const startEdge = edges.find((e) => e.source === 'start');
      expect(startEdge?.target).toBe('view-step-1');
    });

    it('should create edges for buttons with action.onSuccess', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'button-1',
                type: ElementTypes.Action,
                action: {onSuccess: 'END'},
              }),
            ],
          },
        }),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      const buttonEdge = edges.find((e) => e.id === 'button-1');
      expect(buttonEdge).toBeDefined();
      expect(buttonEdge?.target).toBe('END');
    });

    it('should handle nested buttons in BLOCK containers', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'block-1',
                type: 'BLOCK',
                components: [
                  createMockElement({
                    id: 'nested-button',
                    type: ElementTypes.Action,
                    action: {onSuccess: 'END'},
                  }),
                ],
              }),
            ],
          },
        }),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      const nestedButtonEdge = edges.find((e) => e.id === 'nested-button');
      expect(nestedButtonEdge).toBeDefined();
    });

    it('should create edges for RESEND buttons', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'resend-button',
                type: ElementTypes.Resend,
                action: {onSuccess: 'view-step-2'},
              }),
            ],
          },
        }),
        createMockStep({id: 'view-step-2', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      const resendEdge = edges.find((e) => e.id === 'resend-button');
      expect(resendEdge).toBeDefined();
      expect(resendEdge?.target).toBe('view-step-2');
    });

    it('should create edges from step-level actions', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            action: {onSuccess: 'view-step-2'},
          },
        }),
        createMockStep({id: 'view-step-2', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      const stepActionEdge = edges.find((e) => e.id === 'view-step-1-to-view-step-2');
      expect(stepActionEdge).toBeDefined();
    });

    it('should connect to END step when action.onSuccess is StepTypes.End', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            action: {onSuccess: StepTypes.End},
          },
        }),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      const endEdge = edges.find((e) => e.target === 'END');
      expect(endEdge).toBeDefined();
    });

    it('should connect button to END step when button action.onSuccess is StepTypes.End', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'button-1',
                type: ElementTypes.Action,
                action: {onSuccess: StepTypes.End},
              }),
            ],
          },
        }),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should have edge from button to END step
      const buttonEdge = edges.find((e) => e.id === 'button-1');
      expect(buttonEdge).toBeDefined();
      expect(buttonEdge?.target).toBe('END');
    });

    it('should create fallback edge to END when no button has explicit action', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'text-1',
                type: ElementTypes.Text,
                // No action property - not a button
              }),
            ],
          },
        }),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should have a fallback edge from view-step-1 to END
      const fallbackEdge = edges.find((e) => e.source === 'view-step-1' && e.target === 'END');
      expect(fallbackEdge).toBeDefined();
    });

    it('should find first action button in nested structures for fallback edge', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'block-1',
                type: 'BLOCK',
                components: [
                  createMockElement({
                    id: 'nested-button',
                    type: ElementTypes.Action,
                    // No explicit next - should still create edge
                  }),
                ],
              }),
            ],
          },
        }),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should have an edge from nested-button to END
      const buttonEdge = edges.find((e) => e.id === 'nested-button');
      expect(buttonEdge).toBeDefined();
      expect(buttonEdge?.target).toBe('END');
    });

    it('should create step-level edge to END when step.data.action.onSuccess is StepTypes.End', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [],
            action: {onSuccess: StepTypes.End},
          },
        }),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should have step-level edge from view-step-1 to END
      const stepEdge = edges.find((e) => e.id === 'view-step-1-to-END');
      expect(stepEdge).toBeDefined();
      expect(stepEdge?.target).toBe('END');
    });

    it('should use custom startStepId from props', () => {
      const {result} = renderHook(() => useEdgeGeneration({startStepId: 'custom-start'}));

      const steps: Step[] = [
        createMockStep({id: 'view-step-1', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      const startEdge = edges.find((e) => e.source === 'custom-start');
      expect(startEdge).toBeDefined();
    });

    it('should use custom endStepId from props', () => {
      const {result} = renderHook(() => useEdgeGeneration({endStepId: 'custom-end'}));

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'button-1',
                type: ElementTypes.Action,
                // No explicit next - should default to end
              }),
            ],
          },
        }),
        createMockStep({id: 'custom-end', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      const buttonEdge = edges.find((e) => e.id === 'button-1');
      expect(buttonEdge?.target).toBe('custom-end');
    });

    it('should create edges with correct marker type', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({id: 'view-step-1', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      edges.forEach((edge) => {
        expect(edge.markerEnd).toEqual({type: MarkerType.Arrow});
      });
    });
  });

  describe('validateEdges', () => {
    it('should return edges with valid targets', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const edges: Edge[] = [
        {
          id: 'edge-1',
          source: 'node-1',
          target: 'node-2',
          type: 'default',
        },
      ];

      const nodes: Node[] = [
        {id: 'node-1', position: {x: 0, y: 0}, data: {}},
        {id: 'node-2', position: {x: 100, y: 0}, data: {}},
      ];

      const validEdges = result.current.validateEdges(edges, nodes);

      expect(validEdges).toHaveLength(1);
    });

    it('should filter out edges with invalid targets', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const edges: Edge[] = [
        {id: 'edge-1', source: 'node-1', target: 'node-2', type: 'default'},
        {id: 'edge-2', source: 'node-1', target: 'invalid-node', type: 'default'},
      ];

      const nodes: Node[] = [
        {id: 'node-1', position: {x: 0, y: 0}, data: {}},
        {id: 'node-2', position: {x: 100, y: 0}, data: {}},
      ];

      const validEdges = result.current.validateEdges(edges, nodes);

      expect(validEdges).toHaveLength(1);
      expect(validEdges[0].id).toBe('edge-1');
    });

    it('should filter out edges with invalid sources', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const edges: Edge[] = [{id: 'edge-1', source: 'invalid-source', target: 'node-2', type: 'default'}];

      const nodes: Node[] = [{id: 'node-2', position: {x: 0, y: 0}, data: {}}];

      const validEdges = result.current.validateEdges(edges, nodes);

      expect(validEdges).toHaveLength(0);
    });

    it('should always include START as valid target', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const edges: Edge[] = [{id: 'edge-1', source: 'node-1', target: 'start', type: 'default'}];

      const nodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];

      const validEdges = result.current.validateEdges(edges, nodes);

      expect(validEdges).toHaveLength(1);
    });

    it('should always include END as valid target', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const edges: Edge[] = [{id: 'edge-1', source: 'node-1', target: 'END', type: 'default'}];

      const nodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];

      const validEdges = result.current.validateEdges(edges, nodes);

      expect(validEdges).toHaveLength(1);
    });

    it('should return empty array when no edges are valid', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const edges: Edge[] = [
        {id: 'edge-1', source: 'invalid-1', target: 'invalid-2', type: 'default'},
        {id: 'edge-2', source: 'invalid-3', target: 'invalid-4', type: 'default'},
      ];

      const nodes: Node[] = [{id: 'node-1', position: {x: 0, y: 0}, data: {}}];

      const validEdges = result.current.validateEdges(edges, nodes);

      expect(validEdges).toHaveLength(0);
    });

    it('should handle empty edges array', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const validEdges = result.current.validateEdges([], []);

      expect(validEdges).toEqual([]);
    });
  });

  describe('Hook Interface', () => {
    it('should return generateEdges function', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      expect(typeof result.current.generateEdges).toBe('function');
    });

    it('should return validateEdges function', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      expect(typeof result.current.validateEdges).toBe('function');
    });

    it('should maintain stable function references', () => {
      const {result, rerender} = renderHook(() => useEdgeGeneration());

      const initialGenerateEdges = result.current.generateEdges;
      const initialValidateEdges = result.current.validateEdges;

      rerender();

      expect(result.current.generateEdges).toBe(initialGenerateEdges);
      expect(result.current.validateEdges).toBe(initialValidateEdges);
    });
  });

  describe('onFailure edge generation', () => {
    it('should create edge for step-level onFailure action', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'execution-step-1',
          type: StepTypes.View,
          data: {
            action: {onSuccess: 'view-step-2', onFailure: 'view-step-3'},
          },
        }),
        createMockStep({id: 'view-step-2', type: StepTypes.View}),
        createMockStep({id: 'view-step-3', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should have both success and failure edges
      const successEdge = edges.find((e) => e.id === 'execution-step-1-to-view-step-2');
      const failureEdge = edges.find((e) => e.id === 'execution-step-1-failure-to-view-step-3');

      expect(successEdge).toBeDefined();
      expect(successEdge?.sourceHandle).toBe('execution-step-1_NEXT');

      expect(failureEdge).toBeDefined();
      expect(failureEdge?.source).toBe('execution-step-1');
      expect(failureEdge?.sourceHandle).toBe('failure');
      expect(failureEdge?.target).toBe('view-step-3');
    });

    it('should not create failure edge when onFailure target does not exist', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'execution-step-1',
          type: StepTypes.View,
          data: {
            action: {onSuccess: 'view-step-2', onFailure: 'non-existent-step'},
          },
        }),
        createMockStep({id: 'view-step-2', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should only have success edge, not failure edge
      const successEdge = edges.find((e) => e.id === 'execution-step-1-to-view-step-2');
      const failureEdge = edges.find((e) => e.sourceHandle === 'failure');

      expect(successEdge).toBeDefined();
      expect(failureEdge).toBeUndefined();
    });

    it('should create failure edge without success edge when only onFailure is defined', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'execution-step-1',
          type: StepTypes.View,
          data: {
            action: {onFailure: 'view-step-2'},
          },
        }),
        createMockStep({id: 'view-step-2', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      const failureEdge = edges.find((e) => e.id === 'execution-step-1-failure-to-view-step-2');
      expect(failureEdge).toBeDefined();
      expect(failureEdge?.sourceHandle).toBe('failure');
      expect(failureEdge?.target).toBe('view-step-2');
    });

    it('should handle step with both component actions and step-level onFailure', () => {
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'button-1',
                type: ElementTypes.Action,
                action: {onSuccess: 'view-step-2'},
              }),
            ],
            action: {onSuccess: 'view-step-3', onFailure: 'error-step'},
          },
        }),
        createMockStep({id: 'view-step-2', type: StepTypes.View}),
        createMockStep({id: 'view-step-3', type: StepTypes.View}),
        createMockStep({id: 'error-step', type: StepTypes.View}),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should have button edge, step-level success edge, and step-level failure edge
      const buttonEdge = edges.find((e) => e.id === 'button-1');
      const stepSuccessEdge = edges.find((e) => e.id === 'view-step-1-to-view-step-3');
      const stepFailureEdge = edges.find((e) => e.id === 'view-step-1-failure-to-error-step');

      expect(buttonEdge).toBeDefined();
      expect(stepSuccessEdge).toBeDefined();
      expect(stepFailureEdge).toBeDefined();
      expect(stepFailureEdge?.sourceHandle).toBe('failure');
    });
  });

  describe('Edge cases for branch coverage', () => {
    it('should connect button to actual END step when button action.onSuccess equals StepTypes.End string and END step has different id', () => {
      // This test covers lines 147-150: when button.action.onSuccess === StepTypes.End (the string 'END')
      // but there's no step with id 'END' in the flow (the end step has a different id like 'user-onboard')
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'submit-button',
                type: ElementTypes.Action,
                // action.onSuccess is 'END' string (StepTypes.End value)
                // but there's no step with id 'END' - the end step has id 'user-onboard'
                action: {onSuccess: 'END'},
              }),
            ],
          },
        }),
        // End step has a DIFFERENT id than 'END'
        createMockStep({id: 'user-onboard', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should have edge from button to the actual end step (user-onboard)
      const buttonEdge = edges.find((e) => e.id === 'submit-button');
      expect(buttonEdge).toBeDefined();
      expect(buttonEdge?.target).toBe('user-onboard');
    });

    it('should connect step-level action to actual END step when action.onSuccess equals StepTypes.End string and END step has different id', () => {
      // This test covers lines 204-208: when step.data.action.onSuccess === StepTypes.End (the string 'END')
      // but there's no step with id 'END' in the flow
      const {result} = renderHook(() => useEdgeGeneration());

      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [],
            // action.onSuccess is 'END' string but there's no step with that id
            action: {onSuccess: 'END'},
          },
        }),
        // End step has a DIFFERENT id than 'END'
        createMockStep({id: 'user-onboard', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // Should have step-level edge to the actual end step (user-onboard)
      const stepEdge = edges.find((e) => e.id === 'view-step-1-to-user-onboard');
      expect(stepEdge).toBeDefined();
      expect(stepEdge?.target).toBe('user-onboard');
    });

    it('should use button ID from nested component when creating fallback edge with no prior edges to END', () => {
      // This test specifically covers line 227: using buttonId for fallback edge
      const {result} = renderHook(() => useEdgeGeneration());

      // We need a scenario where:
      // 1. A view step has components but none create edges to END
      // 2. userOnboardEdgeCreated remains false
      // 3. Fallback logic kicks in and finds a button
      const steps: Step[] = [
        createMockStep({
          id: 'view-step-1',
          type: StepTypes.View,
          data: {
            components: [
              createMockElement({
                id: 'continue-btn',
                type: ElementTypes.Action,
                // This points to view-step-2, NOT to END
                action: {onSuccess: 'view-step-2'},
              }),
            ],
          },
        }),
        createMockStep({
          id: 'view-step-2',
          type: StepTypes.View,
          data: {
            components: [
              // This step has a button that will be found in fallback
              createMockElement({
                id: 'final-btn',
                type: ElementTypes.Action,
                // Also points away from END
                action: {onSuccess: 'view-step-1'},
              }),
            ],
          },
        }),
        createMockStep({id: 'END', type: StepTypes.End}),
      ];

      const edges = result.current.generateEdges(steps);

      // No edges to END were created in the main loop
      // The fallback should create an edge using the last view step's button
      // which is 'final-btn' from view-step-2
      const fallbackEdge = edges.find((e) => e.source === 'view-step-2' && e.target === 'END');
      expect(fallbackEdge).toBeDefined();
      // The edge ID should be the button ID (line 231)
      expect(fallbackEdge?.id).toBe('final-btn');
    });
  });
});
