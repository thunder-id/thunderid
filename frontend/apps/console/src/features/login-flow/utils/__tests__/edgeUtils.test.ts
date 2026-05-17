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

import type {Edge, Node} from '@xyflow/react';
import {MarkerType} from '@xyflow/react';
import {describe, it, expect} from 'vitest';
import generateUnconnectedEdges from '../edgeUtils';
import {ElementTypes, type Element} from '@/features/flows/models/elements';
import {StepTypes} from '@/features/flows/models/steps';

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

    it('should handle action with falsy onIncomplete value', () => {
      const nodes: Node[] = [createMockNode({id: 'step-1', data: {action: {onIncomplete: ''}}})];
      expect(generateUnconnectedEdges([], nodes, 'default')).toEqual([]);
    });
  });

  describe('onIncomplete edges', () => {
    it('should generate onIncomplete edge for step-level action', () => {
      const nodes: Node[] = [
        createMockNode({id: 'executor-1', data: {action: {onIncomplete: 'prompt-1'}}}),
        createMockNode({id: 'prompt-1'}),
      ];

      const result = generateUnconnectedEdges([], nodes, 'smoothstep');

      const incompleteEdge = result.find((e) => e.id === 'executor-1_executor-1_INCOMPLETE_MISSING_EDGE');
      expect(incompleteEdge).toBeDefined();
      expect(incompleteEdge?.source).toBe('executor-1');
      expect(incompleteEdge?.sourceHandle).toBe('executor-1_INCOMPLETE');
      expect(incompleteEdge?.target).toBe('prompt-1');
      expect(incompleteEdge?.type).toBe('smoothstep');
    });

    it('should not generate onIncomplete edge when target node does not exist', () => {
      const nodes: Node[] = [createMockNode({id: 'executor-1', data: {action: {onIncomplete: 'non-existent'}}})];
      expect(generateUnconnectedEdges([], nodes, 'default')).toEqual([]);
    });

    it('should not generate onIncomplete edge when correct edge already exists', () => {
      const nodes: Node[] = [
        createMockNode({id: 'executor-1', data: {action: {onIncomplete: 'prompt-1'}}}),
        createMockNode({id: 'prompt-1'}),
      ];
      const existingEdges: Edge[] = [
        {id: 'e1', source: 'executor-1', sourceHandle: 'executor-1_INCOMPLETE', target: 'prompt-1'} as Edge,
      ];

      expect(generateUnconnectedEdges(existingEdges, nodes, 'default')).toEqual([]);
    });

    it('should generate onIncomplete edge when existing edge points to wrong target', () => {
      const nodes: Node[] = [
        createMockNode({id: 'executor-1', data: {action: {onIncomplete: 'prompt-1'}}}),
        createMockNode({id: 'prompt-1'}),
        createMockNode({id: 'wrong-prompt'}),
      ];
      const existingEdges: Edge[] = [
        {
          id: 'e1',
          source: 'executor-1',
          sourceHandle: 'executor-1_INCOMPLETE',
          target: 'wrong-prompt',
        } as Edge,
      ];

      const result = generateUnconnectedEdges(existingEdges, nodes, 'default');
      const incompleteEdge = result.find((e) => e.id === 'executor-1_executor-1_INCOMPLETE_MISSING_EDGE');
      expect(incompleteEdge).toBeDefined();
      expect(incompleteEdge?.target).toBe('prompt-1');
    });
  });

  describe('Edge cases (continued)', () => {
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
