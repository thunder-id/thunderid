// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import {MarkerType} from '@xyflow/react';
import {describe, it, expect} from 'vitest';
import {ElementTypes, type Element} from '../../models/elements';
import {StepTypes} from '../../models/steps';
import generateUnconnectedEdges from '../edgeUtils';

const createMockNode = (overrides: Partial<Node> = {}): Node => ({
  id: 'node-1',
  type: StepTypes.View,
  position: {x: 0, y: 0},
  data: {},
  ...overrides,
});

const createMockElement = (overrides: Partial<Element> = {}): Element =>
  ({
    id: 'element-1',
    type: ElementTypes.Action,
    category: 'ACTION',
    version: '1.0.0',
    deprecated: false,
    resourceType: 'ELEMENT',
    display: {label: 'Element', image: ''},
    config: {},
    ...overrides,
  }) as Element;

const createMockEdge = (overrides: Partial<Edge> = {}): Edge => ({
  id: 'edge-1',
  source: 'node-1',
  target: 'node-2',
  type: 'default',
  ...overrides,
});

describe('generateUnconnectedEdges', () => {
  describe('Basic functionality', () => {
    it('should return empty array for empty nodes', () => {
      const result = generateUnconnectedEdges([], [], 'default');
      expect(result).toEqual([]);
    });

    it('should return empty array when no actions have onSuccess property', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'node-1',
          data: {
            components: [createMockElement({id: 'button-1', action: {}})],
          },
        }),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');
      expect(result).toEqual([]);
    });

    it('should skip nodes without data', () => {
      const nodes: Node[] = [createMockNode({id: 'node-1', data: undefined as unknown as Record<string, unknown>})];

      const result = generateUnconnectedEdges([], nodes, 'default');
      expect(result).toEqual([]);
    });
  });

  describe('Component actions', () => {
    it('should generate edge for component with action.onSuccess', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [
              createMockElement({
                id: 'button-1',
                action: {onSuccess: 'step-2'},
              }),
            ],
          },
        }),
        createMockNode({id: 'step-2'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'smoothstep');

      expect(result).toHaveLength(1);
      expect(result[0]).toEqual({
        animated: false,
        id: 'button-1_MISSING_EDGE',
        markerEnd: {type: MarkerType.Arrow},
        source: 'step-1',
        sourceHandle: 'button-1_NEXT',
        target: 'step-2',
        type: 'smoothstep',
      });
    });

    it('should not generate edge when target node does not exist', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [
              createMockElement({
                id: 'button-1',
                action: {onSuccess: 'non-existent-step'},
              }),
            ],
          },
        }),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');
      expect(result).toEqual([]);
    });

    it('should not generate edge when edge already exists with correct target', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [
              createMockElement({
                id: 'button-1',
                action: {onSuccess: 'step-2'},
              }),
            ],
          },
        }),
        createMockNode({id: 'step-2'}),
      ];

      const existingEdges: Edge[] = [
        createMockEdge({
          id: 'existing-edge',
          source: 'step-1',
          sourceHandle: 'button-1_NEXT',
          target: 'step-2',
        }),
      ];

      const result = generateUnconnectedEdges(existingEdges, nodes, 'default');
      expect(result).toEqual([]);
    });

    it('should generate edge when existing edge has wrong target', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [
              createMockElement({
                id: 'button-1',
                action: {onSuccess: 'step-2'},
              }),
            ],
          },
        }),
        createMockNode({id: 'step-2'}),
        createMockNode({id: 'step-3'}),
      ];

      const existingEdges: Edge[] = [
        createMockEdge({
          id: 'existing-edge',
          source: 'step-1',
          sourceHandle: 'button-1_NEXT',
          target: 'step-3', // Wrong target
        }),
      ];

      const result = generateUnconnectedEdges(existingEdges, nodes, 'default');

      expect(result).toHaveLength(1);
      expect(result[0].target).toBe('step-2');
    });
  });

  describe('Nested components (Form)', () => {
    it('should process nested components in forms', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [
              createMockElement({
                id: 'form-1',
                type: 'BLOCK',
                components: [
                  createMockElement({
                    id: 'nested-button-1',
                    action: {onSuccess: 'step-2'},
                  }),
                ],
              }),
            ],
          },
        }),
        createMockNode({id: 'step-2'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('nested-button-1_MISSING_EDGE');
      expect(result[0].sourceHandle).toBe('nested-button-1_NEXT');
    });

    it('should handle multiple nested components with actions', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [
              createMockElement({
                id: 'form-1',
                type: 'BLOCK',
                components: [
                  createMockElement({
                    id: 'nested-button-1',
                    action: {onSuccess: 'step-2'},
                  }),
                  createMockElement({
                    id: 'nested-button-2',
                    action: {onSuccess: 'step-3'},
                  }),
                ],
              }),
            ],
          },
        }),
        createMockNode({id: 'step-2'}),
        createMockNode({id: 'step-3'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');

      expect(result).toHaveLength(2);
      expect(result.map((e) => e.id)).toContain('nested-button-1_MISSING_EDGE');
      expect(result.map((e) => e.id)).toContain('nested-button-2_MISSING_EDGE');
    });
  });

  describe('Step-level actions', () => {
    it('should process step-level action', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            action: {onSuccess: 'step-2'},
          },
        }),
        createMockNode({id: 'step-2'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('step-1_MISSING_EDGE');
      expect(result[0].source).toBe('step-1');
      expect(result[0].sourceHandle).toBe('step-1_NEXT');
      expect(result[0].target).toBe('step-2');
    });

    it('should not generate step-level edge when edge already exists', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            action: {onSuccess: 'step-2'},
          },
        }),
        createMockNode({id: 'step-2'}),
      ];

      const existingEdges: Edge[] = [
        createMockEdge({
          id: 'existing-edge',
          source: 'step-1',
          sourceHandle: 'step-1_NEXT',
          target: 'step-2',
        }),
      ];

      const result = generateUnconnectedEdges(existingEdges, nodes, 'default');
      expect(result).toEqual([]);
    });
  });

  describe('Mixed scenarios', () => {
    it('should handle both component and step-level actions', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [
              createMockElement({
                id: 'button-1',
                action: {onSuccess: 'step-2'},
              }),
            ],
            action: {onSuccess: 'step-3'},
          },
        }),
        createMockNode({id: 'step-2'}),
        createMockNode({id: 'step-3'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');

      expect(result).toHaveLength(2);
      expect(result.map((e) => e.id)).toContain('button-1_MISSING_EDGE');
      expect(result.map((e) => e.id)).toContain('step-1_MISSING_EDGE');
    });

    it('should handle multiple nodes with multiple components', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [
              createMockElement({id: 'button-1', action: {onSuccess: 'step-2'}}),
              createMockElement({id: 'button-2', action: {onSuccess: 'step-3'}}),
            ],
          },
        }),
        createMockNode({
          id: 'step-2',
          data: {
            components: [createMockElement({id: 'button-3', action: {onSuccess: 'step-3'}})],
          },
        }),
        createMockNode({id: 'step-3'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'step');

      expect(result).toHaveLength(3);
      expect(result.every((e) => e.type === 'step')).toBe(true);
    });

    it('should apply correct edge style to all generated edges', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [createMockElement({id: 'button-1', action: {onSuccess: 'step-2'}})],
          },
        }),
        createMockNode({id: 'step-2'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'smoothstep');

      expect(result).toHaveLength(1);
      expect(result[0].type).toBe('smoothstep');
    });
  });

  describe('onFailure edges', () => {
    it('should generate edge for step-level action with onFailure', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            action: {onSuccess: 'step-2', onFailure: 'step-3'},
          },
        }),
        createMockNode({id: 'step-2'}),
        createMockNode({id: 'step-3'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'smoothstep');

      expect(result).toHaveLength(2);
      // Check success edge
      expect(result.find((e) => e.id === 'step-1_MISSING_EDGE')).toBeDefined();
      // Check failure edge
      const failureEdge = result.find((e) => e.id === 'step-1_FAILURE_MISSING_EDGE');
      expect(failureEdge).toBeDefined();
      expect(failureEdge?.sourceHandle).toBe('failure');
      expect(failureEdge?.target).toBe('step-3');
    });

    it('should not generate failure edge when onFailure target does not exist', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            action: {onSuccess: 'step-2', onFailure: 'non-existent'},
          },
        }),
        createMockNode({id: 'step-2'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('step-1_MISSING_EDGE');
    });

    it('should not generate failure edge when edge already exists with correct target', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            action: {onSuccess: 'step-2', onFailure: 'step-3'},
          },
        }),
        createMockNode({id: 'step-2'}),
        createMockNode({id: 'step-3'}),
      ];

      const existingEdges: Edge[] = [
        createMockEdge({
          id: 'existing-success',
          source: 'step-1',
          sourceHandle: 'step-1_NEXT',
          target: 'step-2',
        }),
        createMockEdge({
          id: 'existing-failure',
          source: 'step-1',
          sourceHandle: 'failure',
          target: 'step-3',
        }),
      ];

      const result = generateUnconnectedEdges(existingEdges, nodes, 'default');
      expect(result).toEqual([]);
    });

    it('should generate failure edge when existing failure edge has wrong target', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            action: {onSuccess: 'step-2', onFailure: 'step-3'},
          },
        }),
        createMockNode({id: 'step-2'}),
        createMockNode({id: 'step-3'}),
        createMockNode({id: 'step-4'}),
      ];

      const existingEdges: Edge[] = [
        createMockEdge({
          id: 'existing-failure',
          source: 'step-1',
          sourceHandle: 'failure',
          target: 'step-4', // Wrong target
        }),
      ];

      const result = generateUnconnectedEdges(existingEdges, nodes, 'default');

      const failureEdge = result.find((e) => e.id === 'step-1_FAILURE_MISSING_EDGE');
      expect(failureEdge).toBeDefined();
      expect(failureEdge?.target).toBe('step-3');
    });

    it('should handle falsy onFailure value', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            action: {onSuccess: 'step-2', onFailure: ''},
          },
        }),
        createMockNode({id: 'step-2'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');

      // Should only generate success edge, not failure edge
      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('step-1_MISSING_EDGE');
    });
  });

  describe('onIncomplete edges', () => {
    it('should not generate incomplete edge when edge already exists with correct target', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {action: {onSuccess: 'step-2', onIncomplete: 'step-3'}},
        }),
        createMockNode({id: 'step-2'}),
        createMockNode({id: 'step-3'}),
      ];

      const existingEdges: Edge[] = [
        createMockEdge({
          id: 'existing-success',
          source: 'step-1',
          sourceHandle: 'step-1_NEXT',
          target: 'step-2',
        }),
        createMockEdge({
          id: 'existing-incomplete',
          source: 'step-1',
          sourceHandle: 'step-1_INCOMPLETE',
          target: 'step-3',
        }),
      ];

      const result = generateUnconnectedEdges(existingEdges, nodes, 'default');
      expect(result).toEqual([]);
    });

    it('should generate edge for step-level action with onIncomplete', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            action: {onSuccess: 'step-2', onIncomplete: 'step-3'},
          },
        }),
        createMockNode({id: 'step-2'}),
        createMockNode({id: 'step-3'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'smoothstep');

      expect(result).toHaveLength(2);
      const incompleteEdge = result.find((edge) => edge.id === 'step-1_INCOMPLETE_MISSING_EDGE');
      expect(incompleteEdge).toBeDefined();
      expect(incompleteEdge?.sourceHandle).toBe('step-1_INCOMPLETE');
      expect(incompleteEdge?.target).toBe('step-3');
    });
  });

  describe('Edge cases', () => {
    it('should handle action with falsy onSuccess value', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [
              createMockElement({id: 'button-1', action: {onSuccess: ''}}),
              createMockElement({id: 'button-2', action: {onSuccess: undefined}}),
            ],
          },
        }),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');
      expect(result).toEqual([]);
    });

    it('should handle action that is not an object', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [createMockElement({id: 'button-1', action: 'string-action' as unknown as Element['action']})],
          },
        }),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');
      expect(result).toEqual([]);
    });

    it('should handle components without action property', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [createMockElement({id: 'text-1', type: 'TEXT', action: undefined})],
          },
        }),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');
      expect(result).toEqual([]);
    });

    it('should handle components array being empty', () => {
      const nodes: Node[] = [
        createMockNode({
          id: 'step-1',
          data: {
            components: [],
          },
        }),
      ];

      const result = generateUnconnectedEdges([], nodes, 'default');
      expect(result).toEqual([]);
    });
  });
});
