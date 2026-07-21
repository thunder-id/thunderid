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

import type {Node, Edge} from '@xyflow/react';
import {describe, it, expect, vi} from 'vitest';
import VisualFlowConstants from '../../constants/VisualFlowConstants';
import {ElementTypes, ElementCategories, ActionEventTypes, ButtonTypes} from '../../models/elements';
import type {Element} from '../../models/elements';
import type {StepData} from '../../models/steps';
import {StepTypes, StaticStepTypes} from '../../models/steps';
import {
  transformReactFlow,
  validateFlowGraph,
  createFlowConfiguration,
  type ReactFlowCanvasData,
  type FlowGraph,
} from '../reactFlowTransformer';

// Mock generateResourceId
vi.mock('../generateResourceId', () => ({
  default: vi.fn((prefix: string) => `${prefix}-generated-id`),
}));

describe('reactFlowTransformer', () => {
  const createNode = (
    id: string,
    type: string,
    position: {x: number; y: number} = {x: 0, y: 0},
    data: StepData = {},
  ): Node<StepData> => ({
    id,
    type,
    position,
    data,
  });

  const createEdge = (id: string, source: string, target: string, sourceHandle?: string): Edge => ({
    id,
    source,
    target,
    ...(sourceHandle && {sourceHandle}),
  });

  describe('transformReactFlow', () => {
    describe('Basic Node Transformation', () => {
      it('should transform START node correctly', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('start-1', StaticStepTypes.Start, {x: 100, y: 100})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes).toHaveLength(1);
        expect(result.nodes[0].id).toBe('start-1');
        expect(result.nodes[0].type).toBe('START');
        expect(result.nodes[0].layout.position).toEqual({x: 100, y: 100});
      });

      it('should transform END node correctly', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('end-1', StepTypes.End, {x: 200, y: 200})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes).toHaveLength(1);
        expect(result.nodes[0].type).toBe('END');
      });

      it('should transform VIEW node to PROMPT', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].type).toBe('PROMPT');
      });

      it('should transform EXECUTION node to TASK_EXECUTION', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('exec-1', StepTypes.Execution, {x: 0, y: 0})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].type).toBe('TASK_EXECUTION');
      });

      it('should transform RULE node to DECISION', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('rule-1', StepTypes.Rule, {x: 0, y: 0})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].type).toBe('DECISION');
      });

      it('should use measured dimensions when available', () => {
        const node: Node<StepData> = {
          id: 'node-1',
          type: StepTypes.View,
          position: {x: 0, y: 0},
          data: {},
          measured: {width: 300, height: 200},
        };

        const canvasData: ReactFlowCanvasData = {
          nodes: [node],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].layout.size).toEqual({width: 300, height: 200});
      });

      it('should use default dimensions when measured is not available', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('node-1', StepTypes.View)],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].layout.size).toEqual({width: 200, height: 100});
      });
    });

    describe('Component Processing', () => {
      it('should clean and include components in meta for VIEW nodes', () => {
        const components: Element[] = [
          {
            id: 'text-input-1',
            type: ElementTypes.TextInput,
            category: ElementCategories.Field,
            resourceType: 'ELEMENT',
            version: '1.0.0',
            deprecated: false,
            deletable: true,
            display: {label: 'Username', image: '', showOnResourcePanel: true},
            config: {field: {name: 'username', type: {}}, styles: {}},
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].meta).toBeDefined();
        expect(result.nodes[0].meta?.components).toHaveLength(1);
        // Verify internal properties are removed
        expect(result.nodes[0].meta?.components?.[0]).not.toHaveProperty('display');
        expect(result.nodes[0].meta?.components?.[0]).not.toHaveProperty('config');
      });

      it('should extract inputs from VIEW components', () => {
        const components: Element[] = [
          {
            id: 'block-1',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              {
                id: 'text-input-1',
                type: ElementTypes.TextInput,
                category: ElementCategories.Field,
                name: 'username',
                required: true,
              } as unknown as Element,
              {
                id: 'password-input-1',
                type: ElementTypes.PasswordInput,
                category: ElementCategories.Field,
                name: 'password',
                required: true,
              } as unknown as Element,
              {
                id: 'button-1',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                action: {onSuccess: 'next-node'},
              } as Element,
            ],
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [createEdge('edge-1', 'view-1', 'next-node', 'button-1_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        // Inputs should be in the prompts array (associated with the action in the same BLOCK)
        expect(result.nodes[0].prompts).toHaveLength(1);
        expect(result.nodes[0].prompts?.[0].inputs).toHaveLength(2);
        expect(result.nodes[0].prompts?.[0].inputs?.[0]).toEqual({
          ref: 'text-input-1',
          type: ElementTypes.TextInput,
          identifier: 'username',
          required: true,
        });
        expect(result.nodes[0].prompts?.[0].action).toEqual({
          ref: 'button-1',
          nextNode: 'next-node',
        });
      });

      it('should extract actions from buttons', () => {
        const components: Element[] = [
          {
            id: 'button-1',
            type: ElementTypes.Action,
            category: ElementCategories.Action,
            action: {onSuccess: 'next-node'},
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [createEdge('edge-1', 'view-1', 'next-node', 'button-1_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        // Actions should be in the prompts array
        expect(result.nodes[0].prompts).toHaveLength(1);
        expect(result.nodes[0].prompts?.[0].action).toEqual({
          ref: 'button-1',
          nextNode: 'next-node',
        });
      });

      it('should handle nested components in forms', () => {
        const formComponent: Element = {
          id: 'form-1',
          type: 'BLOCK',
          category: ElementCategories.Block,
          components: [
            {
              id: 'input-1',
              type: ElementTypes.TextInput,
              category: ElementCategories.Field,
              name: 'email',
            } as unknown as Element,
            {
              id: 'button-1',
              type: ElementTypes.Action,
              category: ElementCategories.Action,
              action: {onSuccess: 'next-node'},
            } as Element,
          ],
        } as unknown as Element;

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components: [formComponent]})],
          edges: [createEdge('edge-1', 'view-1', 'next-node', 'button-1_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        // Inputs from nested components appear in prompts (associated with the action in the same BLOCK)
        expect(result.nodes[0].prompts).toHaveLength(1);
        expect(result.nodes[0].prompts?.[0].inputs).toHaveLength(1);
        expect(result.nodes[0].prompts?.[0].inputs?.[0].identifier).toBe('email');
        expect(result.nodes[0].prompts?.[0].action?.ref).toBe('button-1');
      });

      it('should extract inputs from deeply nested block structures (Block A -> Block B -> Action)', () => {
        // Structure:
        // Outer Block (contains email input)
        //   -> Inner Block (contains password input and submit button)
        // Expected: Submit action should have BOTH email and password inputs
        const complexComponent: Element = {
          id: 'outer-block',
          type: 'BLOCK',
          category: ElementCategories.Block,
          components: [
            {
              id: 'input-email',
              type: ElementTypes.TextInput,
              category: ElementCategories.Field,
              name: 'email',
            } as unknown as Element,
            {
              id: 'inner-block',
              type: 'BLOCK',
              category: ElementCategories.Block,
              components: [
                {
                  id: 'input-password',
                  type: ElementTypes.PasswordInput,
                  category: ElementCategories.Field,
                  name: 'password',
                } as unknown as Element,
                {
                  id: 'submit-btn',
                  type: ElementTypes.Action,
                  category: ElementCategories.Action,
                  action: {onSuccess: 'next-node'},
                } as Element,
              ],
            } as unknown as Element,
          ],
        } as unknown as Element;

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components: [complexComponent]})],
          edges: [createEdge('edge-1', 'view-1', 'next-node', 'submit-btn_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        // Verification
        expect(result.nodes[0].prompts).toHaveLength(1);
        const prompt = result.nodes[0].prompts?.[0];

        // Should have inputs from both blocks
        expect(prompt?.inputs).toHaveLength(2);

        const identifiers = prompt?.inputs?.map((i) => i.identifier).sort();
        expect(identifiers).toEqual(['email', 'password'].sort());

        expect(prompt?.action?.ref).toBe('submit-btn');
      });

      it('should handle components in END nodes', () => {
        const components: Element[] = [
          {
            id: 'text-1',
            type: ElementTypes.Text,
            category: ElementCategories.Display,
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('end-1', StepTypes.End, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].meta?.components).toHaveLength(1);
      });

      it('should recursively extract inputs from deeply nested components without action', () => {
        // This test covers the recursive processing in extractInputs
        const components: Element[] = [
          {
            id: 'outer-container',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              {
                id: 'inner-container',
                type: 'BLOCK',
                category: ElementCategories.Block,
                components: [
                  {
                    id: 'deep-input',
                    type: ElementTypes.TextInput,
                    category: ElementCategories.Field,
                    name: 'deepField',
                  } as unknown as Element,
                ],
              } as unknown as Element,
            ],
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components}),
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 100, y: 0},
              {
                action: {executor: {name: 'TestExecutor'}},
              },
            ),
          ],
          edges: [createEdge('edge-1', 'view-1', 'exec-1')],
        };

        const result = transformReactFlow(canvasData);

        // The execution node should have collected the deeply nested input
        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        expect(execNode?.executor?.inputs).toHaveLength(1);
        expect(execNode?.executor?.inputs?.[0].identifier).toBe('deepField');
      });

      it('should not use stale action.onSuccess as fallback when no edge exists for action', () => {
        const components: Element[] = [
          {
            id: 'button-1',
            type: ElementTypes.Action,
            category: ElementCategories.Action,
            action: {onSuccess: 'fallback-target'},
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [], // No edge — stale action.onSuccess should be ignored
        };

        const result = transformReactFlow(canvasData);

        // Without an edge, the action should not be included (no nextNode)
        expect(result.nodes[0].prompts).toBeUndefined();
      });

      it('should handle RESEND element type in extractPrompts with edge', () => {
        const components: Element[] = [
          {
            id: 'resend-1',
            type: ElementTypes.Resend,
            category: ElementCategories.Action,
            action: {onSuccess: 'resend-target'},
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [createEdge('edge-1', 'view-1', 'resend-target', 'resend-1_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].prompts?.[0].action?.ref).toBe('resend-1');
        expect(result.nodes[0].prompts?.[0].action?.nextNode).toBe('resend-target');
      });
    });

    describe('Edge Connections', () => {
      it('should set onSuccess for START node from edges', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('start-1', StaticStepTypes.Start), createNode('view-1', StepTypes.View)],
          edges: [createEdge('edge-1', 'start-1', 'view-1')],
        };

        const result = transformReactFlow(canvasData);

        const startNode = result.nodes.find((n) => n.type === 'START');
        expect(startNode?.onSuccess).toBe('view-1');
      });

      it('should set onSuccess for EXECUTION node from edges', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 0, y: 0},
              {
                action: {executor: {name: 'TestExecutor'}},
              },
            ),
            createNode('end-1', StepTypes.End),
          ],
          edges: [createEdge('edge-1', 'exec-1', 'end-1')],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        expect(execNode?.onSuccess).toBe('end-1');
      });

      it('should set onFailure for EXECUTION node when failure handle exists', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('exec-1', StepTypes.Execution),
            createNode('success-1', StepTypes.End),
            createNode('failure-1', StepTypes.End),
          ],
          edges: [createEdge('edge-1', 'exec-1', 'success-1'), createEdge('edge-2', 'exec-1', 'failure-1', 'failure')],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        expect(execNode?.onSuccess).toBe('success-1');
        expect(execNode?.onFailure).toBe('failure-1');
      });

      it('should set onIncomplete for EXECUTION node when incomplete handle exists', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('exec-1', StepTypes.Execution),
            createNode('success-1', StepTypes.End),
            createNode('incomplete-1', StepTypes.End),
          ],
          edges: [
            createEdge('edge-1', 'exec-1', 'success-1'),
            createEdge(
              'edge-2',
              'exec-1',
              'incomplete-1',
              `exec-1${VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX}`,
            ),
          ],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        expect(execNode?.onSuccess).toBe('success-1');
        expect(execNode?.onIncomplete).toBe('incomplete-1');
      });

      it('should set onSuccess for DECISION node from edges', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('rule-1', StepTypes.Rule), createNode('view-1', StepTypes.View)],
          edges: [createEdge('edge-1', 'rule-1', 'view-1')],
        };

        const result = transformReactFlow(canvasData);

        const ruleNode = result.nodes.find((n) => n.type === 'DECISION');
        expect(ruleNode?.onSuccess).toBe('view-1');
      });

      it('should include properties for DECISION nodes', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode(
              'rule-1',
              StepTypes.Rule,
              {x: 0, y: 0},
              {
                properties: {
                  condition: 'user.role === "admin"',
                  operator: 'equals',
                },
              },
            ),
          ],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        const ruleNode = result.nodes.find((n) => n.type === 'DECISION');
        expect(ruleNode?.properties).toEqual({
          condition: 'user.role === "admin"',
          operator: 'equals',
        });
      });

      it('should persist properties for PROMPT nodes (e.g. the user-set display name)', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode(
              'view-1',
              StepTypes.View,
              {x: 0, y: 0},
              {
                components: [],
                properties: {displayName: 'Collect credentials'},
              },
            ),
          ],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        const promptNode = result.nodes.find((n) => n.id === 'view-1');
        expect(promptNode?.properties).toEqual({displayName: 'Collect credentials'});
      });

      it('should use first edge target when no default edge exists (all edges have sourceHandle)', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('start-1', StaticStepTypes.Start),
            createNode('view-1', StepTypes.View),
            createNode('view-2', StepTypes.View),
          ],
          // Both edges have sourceHandle, so no "default" edge exists
          edges: [
            createEdge('edge-1', 'start-1', 'view-1', 'handle-a'),
            createEdge('edge-2', 'start-1', 'view-2', 'handle-b'),
          ],
        };

        const result = transformReactFlow(canvasData);

        const startNode = result.nodes.find((n) => n.type === 'START');
        // Should use the first edge's target when no default edge found
        expect(startNode?.onSuccess).toBe('view-1');
      });

      it('should not use stale action.onSuccess for START node when no edges exist', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode(
              'start-1',
              StaticStepTypes.Start,
              {x: 0, y: 0},
              {
                action: {onSuccess: 'fallback-view'},
              },
            ),
            createNode('fallback-view', StepTypes.View),
          ],
          edges: [], // No edges — stale action.onSuccess should be ignored
        };

        const result = transformReactFlow(canvasData);

        const startNode = result.nodes.find((n) => n.type === 'START');
        expect(startNode?.onSuccess).toBeUndefined();
      });

      it('should prefer edges over action.onSuccess for button connections', () => {
        const components: Element[] = [
          {
            id: 'button-1',
            type: ElementTypes.Action,
            category: ElementCategories.Action,
            action: {onSuccess: 'stale-node'}, // This is stale
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [createEdge('edge-1', 'view-1', 'current-node', 'button-1_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        // Should use edge target, not action.onSuccess
        expect(result.nodes[0].prompts?.[0].action?.nextNode).toBe('current-node');
      });
    });

    describe('Execution Node Processing', () => {
      it('should include executor configuration', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 0, y: 0},
              {
                action: {
                  executor: {
                    name: 'TestExecutor',
                    config: {key: 'value'},
                  },
                },
              },
            ),
          ],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].executor).toEqual({
          name: 'TestExecutor',
          config: {key: 'value'},
        });
      });

      it('should include properties for EXECUTION nodes', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 0, y: 0},
              {
                properties: {timeout: 5000, retries: 3},
              },
            ),
          ],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].properties).toEqual({timeout: 5000, retries: 3});
      });

      it('should collect inputs from preceding PROMPT node', () => {
        const promptComponents: Element[] = [
          {
            id: 'input-1',
            type: ElementTypes.TextInput,
            category: ElementCategories.Field,
            name: 'username',
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components: promptComponents}),
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 100, y: 0},
              {
                action: {executor: {name: 'PasswordValidator'}},
              },
            ),
          ],
          edges: [createEdge('edge-1', 'view-1', 'exec-1')],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        // Inputs are now inside executor
        expect(execNode?.executor?.inputs).toHaveLength(1);
        expect(execNode?.executor?.inputs?.[0].identifier).toBe('username');
      });

      it('should use ref as execution input identifier when name is missing', () => {
        const promptComponents: Element[] = [
          {
            id: 'input-ref-1',
            type: ElementTypes.TextInput,
            category: ElementCategories.Field,
            ref: 'usernameRef',
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components: promptComponents}),
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 100, y: 0},
              {
                action: {executor: {name: 'PasswordValidator'}},
              },
            ),
          ],
          edges: [createEdge('edge-1', 'view-1', 'exec-1')],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.id === 'exec-1');
        expect(execNode?.executor?.inputs?.[0].identifier).toBe('usernameRef');
      });

      it('should use id as execution input identifier when both name and ref are missing', () => {
        const promptComponents: Element[] = [
          {
            id: 'input-id-fallback',
            type: ElementTypes.TextInput,
            category: ElementCategories.Field,
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components: promptComponents}),
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 100, y: 0},
              {
                action: {executor: {name: 'PasswordValidator'}},
              },
            ),
          ],
          edges: [createEdge('edge-1', 'view-1', 'exec-1')],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.id === 'exec-1');
        expect(execNode?.executor?.inputs?.[0].identifier).toBe('input-id-fallback');
      });

      it('should use code input for OAuth executors', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 0, y: 0},
              {
                action: {executor: {name: 'GoogleOIDCAuthExecutor'}},
              },
            ),
          ],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        // Inputs are now inside executor
        expect(execNode?.executor?.inputs).toHaveLength(1);
        expect(execNode?.executor?.inputs?.[0].identifier).toBe('code');
        expect(execNode?.executor?.inputs?.[0].type).toBe('TEXT_INPUT');
      });

      it('should use code input for GitHub OAuth executor', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 0, y: 0},
              {
                action: {executor: {name: 'GithubOAuthExecutor'}},
              },
            ),
          ],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        // Inputs are now inside executor
        expect(execNode?.executor?.inputs?.[0].identifier).toBe('code');
      });

      it('should use consent_decisions input for ConsentExecutor', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode(
              'exec-consent-1',
              StepTypes.Execution,
              {x: 0, y: 0},
              {
                action: {executor: {name: 'ConsentExecutor'}},
              },
            ),
          ],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.id === 'exec-consent-1');
        expect(execNode?.executor?.inputs).toEqual([
          {
            ref: 'input-generated-id',
            type: 'CONSENT_INPUT',
            identifier: 'consent_decisions',
            required: true,
          },
        ]);
      });

      it('should collect inputs from PROMPT node connected via START node', () => {
        // Scenario: START -> PROMPT -> EXECUTION
        // The EXECUTION node should collect inputs from the PROMPT node
        const promptComponents: Element[] = [
          {
            id: 'input-1',
            type: ElementTypes.TextInput,
            category: ElementCategories.Field,
            name: 'username',
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('start-1', StaticStepTypes.Start),
            createNode('view-1', StepTypes.View, {x: 100, y: 0}, {components: promptComponents}),
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 200, y: 0},
              {
                action: {executor: {name: 'TestExecutor'}},
              },
            ),
          ],
          edges: [createEdge('edge-1', 'start-1', 'view-1'), createEdge('edge-2', 'view-1', 'exec-1')],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        expect(execNode?.executor?.inputs).toHaveLength(1);
        expect(execNode?.executor?.inputs?.[0].identifier).toBe('username');
      });

      it('should find PROMPT node through START node indirection', () => {
        // Scenario: START connects to PROMPT, and EXECUTION connects from START
        // This tests the findPrecedingPromptNode logic for START -> PROMPT path
        const promptComponents: Element[] = [
          {
            id: 'email-input',
            type: ElementTypes.EmailInput,
            category: ElementCategories.Field,
            name: 'email',
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('start-1', StaticStepTypes.Start),
            createNode('view-1', StepTypes.View, {x: 100, y: 0}, {components: promptComponents}),
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 200, y: 0},
              {
                action: {executor: {name: 'EmailValidator'}},
              },
            ),
          ],
          // START -> PROMPT and START -> EXECUTION (execution coming from start)
          edges: [createEdge('edge-1', 'start-1', 'view-1'), createEdge('edge-2', 'start-1', 'exec-1')],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        // Should find the PROMPT node via the START node path
        expect(execNode?.executor?.inputs).toHaveLength(1);
        expect(execNode?.executor?.inputs?.[0].identifier).toBe('email');
      });

      it('should not add inputs when execution node has no executor name', () => {
        const promptComponents: Element[] = [
          {
            id: 'input-1',
            type: ElementTypes.TextInput,
            category: ElementCategories.Field,
            name: 'username',
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components: promptComponents}),
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 100, y: 0},
              {
                // No executor defined, or executor without name
                action: {},
              },
            ),
          ],
          edges: [createEdge('edge-1', 'view-1', 'exec-1')],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        // Should not have inputs since there's no executor name
        expect(execNode?.executor).toBeUndefined();
      });

      it('should not add inputs when preceding PROMPT node has no components', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('view-1', StepTypes.View, {x: 0, y: 0}, {}), // No components
            createNode(
              'exec-1',
              StepTypes.Execution,
              {x: 100, y: 0},
              {
                action: {executor: {name: 'TestExecutor'}},
              },
            ),
          ],
          edges: [createEdge('edge-1', 'view-1', 'exec-1')],
        };

        const result = transformReactFlow(canvasData);

        const execNode = result.nodes.find((n) => n.type === 'TASK_EXECUTION');
        // Should have executor but no inputs
        expect(execNode?.executor?.name).toBe('TestExecutor');
        expect(execNode?.executor?.inputs).toBeUndefined();
      });
    });

    describe('Event Type Derivation', () => {
      it('should derive SUBMIT eventType for submit button', () => {
        const components: Element[] = [
          {
            id: 'button-1',
            type: ElementTypes.Action,
            category: ElementCategories.Action,
            buttonType: ButtonTypes.Submit,
          } as Element & {buttonType: string},
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].meta?.components?.[0].eventType).toBe(ActionEventTypes.Submit);
      });

      it('should derive TRIGGER eventType for regular button', () => {
        const components: Element[] = [
          {
            id: 'button-1',
            type: ElementTypes.Action,
            category: ElementCategories.Action,
            buttonType: ButtonTypes.Button,
          } as Element & {buttonType: string},
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].meta?.components?.[0].eventType).toBe(ActionEventTypes.Trigger);
      });

      it('should default to TRIGGER eventType when buttonType is missing', () => {
        const components: Element[] = [
          {
            id: 'button-1',
            type: ElementTypes.Action,
            category: ElementCategories.Action,
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].meta?.components?.[0].eventType).toBe(ActionEventTypes.Trigger);
      });

      it('should set SUBMIT eventType for buttons inside a block with input fields', () => {
        const components: Element[] = [
          {
            id: 'block-1',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              {
                id: 'field-phone',
                type: ElementTypes.PhoneInput,
                category: ElementCategories.Field,
                inputType: 'tel',
                label: 'Mobile Number',
              },
              {
                id: 'action-1',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                eventType: ActionEventTypes.Trigger,
                label: 'Continue',
                variant: 'PRIMARY',
              },
            ],
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        const block = result.nodes[0].meta?.components?.[0] ?? {};
        const blockChildren = block.components as Record<string, unknown>[];
        const actionButton = blockChildren.find((c) => c.type === ElementTypes.Action);
        expect(actionButton?.eventType).toBe(ActionEventTypes.Submit);
      });

      it('should keep TRIGGER eventType for buttons inside a block without input fields', () => {
        const components: Element[] = [
          {
            id: 'block-1',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              {
                id: 'text-1',
                type: ElementTypes.Text,
                category: ElementCategories.Display,
                label: 'Hello',
              },
              {
                id: 'action-1',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                eventType: ActionEventTypes.Trigger,
                label: 'Click me',
                variant: 'PRIMARY',
              },
            ],
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        const block = result.nodes[0].meta?.components?.[0] ?? {};
        const blockChildren = block.components as Record<string, unknown>[];
        const actionButton = blockChildren.find((c) => c.type === ElementTypes.Action);
        expect(actionButton?.eventType).toBe(ActionEventTypes.Trigger);
      });

      it('should not promote eventType when a block has inputs and multiple buttons', () => {
        const components: Element[] = [
          {
            id: 'block-1',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              {
                id: 'field-phone',
                type: ElementTypes.PhoneInput,
                category: ElementCategories.Field,
                inputType: 'tel',
                label: 'Mobile Number',
              },
              {
                id: 'action-submit',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                eventType: ActionEventTypes.Trigger,
                label: 'Submit',
                variant: 'PRIMARY',
              },
              {
                id: 'action-cancel',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                eventType: ActionEventTypes.Trigger,
                label: 'Cancel',
                variant: 'SECONDARY',
              },
            ],
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        const block = result.nodes[0].meta?.components?.[0] ?? {};
        const blockChildren = block.components as Record<string, unknown>[];
        const actionButtons = blockChildren.filter((c) => c.type === ElementTypes.Action);
        expect(actionButtons).toHaveLength(2);
        expect(actionButtons[0].eventType).toBe(ActionEventTypes.Trigger);
        expect(actionButtons[1].eventType).toBe(ActionEventTypes.Trigger);
      });
    });

    describe('Input Field Processing', () => {
      it('should set ref for input fields', () => {
        const components: Element[] = [
          {
            id: 'input-1',
            type: ElementTypes.EmailInput,
            category: ElementCategories.Field,
            name: 'email',
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].meta?.components?.[0].ref).toBe('email');
      });

      it('should use id as ref fallback when name is missing', () => {
        const components: Element[] = [
          {
            id: 'input-1',
            type: ElementTypes.TextInput,
            category: ElementCategories.Field,
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        expect(result.nodes[0].meta?.components?.[0].ref).toBe('input-1');
      });

      it('should use ref property as identifier fallback when name is not a string', () => {
        const components: Element[] = [
          {
            id: 'block-1',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              {
                id: 'input-1',
                type: ElementTypes.TextInput,
                category: ElementCategories.Field,
                ref: 'usernameRef',
                // name is not defined
              } as unknown as Element,
              {
                id: 'submit-btn',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                action: {onSuccess: 'next-node'},
              } as Element,
            ],
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [createEdge('edge-1', 'view-1', 'next-node', 'submit-btn_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        // The input should use ref as the identifier since name is not defined
        expect(result.nodes[0].prompts?.[0].inputs?.[0].identifier).toBe('usernameRef');
      });

      it('should fall back to component id when both name and ref are not strings', () => {
        const components: Element[] = [
          {
            id: 'block-1',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              {
                id: 'input-fallback-id',
                type: ElementTypes.TextInput,
                category: ElementCategories.Field,
                // Neither name nor ref are defined
              } as unknown as Element,
              {
                id: 'submit-btn',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                action: {onSuccess: 'next-node'},
              } as Element,
            ],
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [createEdge('edge-1', 'view-1', 'next-node', 'submit-btn_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        // The input should use id as the identifier fallback
        expect(result.nodes[0].prompts?.[0].inputs?.[0].identifier).toBe('input-fallback-id');
      });

      it('should handle all input element types', () => {
        const inputTypes = [
          ElementTypes.TextInput,
          ElementTypes.PasswordInput,
          ElementTypes.EmailInput,
          ElementTypes.PhoneInput,
          ElementTypes.NumberInput,
          ElementTypes.DateInput,
          ElementTypes.OtpInput,
          ElementTypes.Checkbox,
          ElementTypes.Dropdown,
        ];

        const inputComponents: Element[] = inputTypes.map((type, index) => ({
          id: `input-${index}`,
          type,
          category: ElementCategories.Field,
          name: `field-${index}`,
        })) as unknown as Element[];

        // Wrap inputs in a BLOCK with an action button
        const components: Element[] = [
          {
            id: 'block-1',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              ...inputComponents,
              {
                id: 'submit-btn',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                action: {onSuccess: 'next-node'},
              } as Element,
            ],
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [createEdge('edge-1', 'view-1', 'next-node', 'submit-btn_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        // All inputs are in the prompts array (associated with the action in the same BLOCK)
        expect(result.nodes[0].prompts).toHaveLength(1);
        expect(result.nodes[0].prompts?.[0].inputs).toHaveLength(inputTypes.length);
      });
    });

    describe('Consent Prompt Input Assignment', () => {
      it('should assign consent input only for prompt actions routed to ConsentExecutor', () => {
        const components: Element[] = [
          {
            id: 'consent-block',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              {
                id: 'username-input',
                type: ElementTypes.TextInput,
                category: ElementCategories.Field,
                name: 'username',
              } as unknown as Element,
              {
                id: 'allow-btn',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                action: {onSuccess: 'consent-exec-1'},
              } as Element,
            ],
          } as unknown as Element,
          {
            id: 'deny-btn',
            type: ElementTypes.Action,
            category: ElementCategories.Action,
            action: {onSuccess: 'consent-exec-1'},
          } as Element,
          {
            id: 'other-btn',
            type: ElementTypes.Action,
            category: ElementCategories.Action,
            action: {onSuccess: 'end-1'},
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('view-consent-1', StepTypes.View, {x: 0, y: 0}, {components}),
            createNode(
              'consent-exec-1',
              StepTypes.Execution,
              {x: 300, y: 0},
              {
                action: {executor: {name: 'ConsentExecutor'}},
              },
            ),
            createNode('end-1', StepTypes.End, {x: 600, y: 0}),
          ],
          edges: [
            createEdge('edge-allow', 'view-consent-1', 'consent-exec-1', 'allow-btn_NEXT'),
            createEdge('edge-deny', 'view-consent-1', 'consent-exec-1', 'deny-btn_NEXT'),
            createEdge('edge-other', 'view-consent-1', 'end-1', 'other-btn_NEXT'),
            createEdge('edge-success', 'consent-exec-1', 'end-1'),
            createEdge(
              'edge-incomplete',
              'consent-exec-1',
              'view-consent-1',
              `consent-exec-1${VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX}`,
            ),
          ],
        };

        const result = transformReactFlow(canvasData);

        const promptNode = result.nodes.find((n) => n.id === 'view-consent-1');
        const allowPrompt = promptNode?.prompts?.find((prompt) => prompt.action?.ref === 'allow-btn');
        const denyPrompt = promptNode?.prompts?.find((prompt) => prompt.action?.ref === 'deny-btn');
        const otherPrompt = promptNode?.prompts?.find((prompt) => prompt.action?.ref === 'other-btn');

        expect(allowPrompt?.inputs).toEqual([
          {
            ref: 'username-input',
            type: ElementTypes.TextInput,
            identifier: 'username',
            required: false,
          },
          {
            ref: 'input-generated-id',
            type: 'CONSENT_INPUT',
            identifier: 'consent_decisions',
            required: true,
          },
        ]);

        expect(denyPrompt?.inputs).toEqual([
          {
            ref: 'input-generated-id',
            type: 'CONSENT_INPUT',
            identifier: 'consent_decisions',
            required: true,
          },
        ]);

        expect(otherPrompt?.inputs).toBeUndefined();
      });
    });

    describe('Display-only VIEW nodes', () => {
      it('should set next field when VIEW node has only display components and an outgoing edge', () => {
        const components: Element[] = [
          {
            id: 'heading-1',
            type: ElementTypes.Text,
            category: ElementCategories.Display,
          } as Element,
          {
            id: 'body-1',
            type: ElementTypes.Text,
            category: ElementCategories.Display,
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [
            createNode('display-view-1', StepTypes.View, {x: 0, y: 0}, {components}),
            createNode('end-1', StepTypes.End),
          ],
          edges: [createEdge('edge-1', 'display-view-1', 'end-1')],
        };

        const result = transformReactFlow(canvasData);

        const viewNode = result.nodes.find((n) => n.id === 'display-view-1');
        expect(viewNode?.next).toBe('end-1');
        expect(viewNode?.prompts).toBeUndefined();
      });

      it('should not set next when VIEW has only display components but no outgoing edge', () => {
        const components: Element[] = [
          {
            id: 'text-1',
            type: ElementTypes.Text,
            category: ElementCategories.Display,
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('display-view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [],
        };

        const result = transformReactFlow(canvasData);

        const viewNode = result.nodes[0];
        expect(viewNode.next).toBeUndefined();
        expect(viewNode.prompts).toBeUndefined();
      });

      it('should set prompts (not next) when VIEW node has a top-level ACTION component', () => {
        const components: Element[] = [
          {
            id: 'text-1',
            type: ElementTypes.Text,
            category: ElementCategories.Display,
          } as Element,
          {
            id: 'button-1',
            type: ElementTypes.Action,
            category: ElementCategories.Action,
            action: {onSuccess: 'end-1'},
          } as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [createEdge('edge-1', 'view-1', 'end-1', 'button-1_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        const viewNode = result.nodes[0];
        expect(viewNode.prompts).toHaveLength(1);
        expect(viewNode.next).toBeUndefined();
      });

      it('should set prompts (not next) when ACTION is nested inside a BLOCK', () => {
        const components: Element[] = [
          {
            id: 'block-1',
            type: 'BLOCK',
            category: ElementCategories.Block,
            components: [
              {
                id: 'button-1',
                type: ElementTypes.Action,
                category: ElementCategories.Action,
                action: {onSuccess: 'end-1'},
              } as Element,
            ],
          } as unknown as Element,
        ];

        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
          edges: [createEdge('edge-1', 'view-1', 'end-1', 'button-1_NEXT')],
        };

        const result = transformReactFlow(canvasData);

        const viewNode = result.nodes[0];
        expect(viewNode.prompts).toHaveLength(1);
        expect(viewNode.next).toBeUndefined();
      });

      it('should not set next when VIEW components array is empty', () => {
        const canvasData: ReactFlowCanvasData = {
          nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components: []})],
          edges: [createEdge('edge-1', 'view-1', 'end-1')],
        };

        const result = transformReactFlow(canvasData);

        // meta.components is set but empty; no prompts and next falls through
        const viewNode = result.nodes[0];
        expect(viewNode.prompts).toBeUndefined();
        expect(viewNode.next).toBeUndefined();
      });
    });
  });

  describe('validateFlowGraph', () => {
    it('should return empty array for valid flow', () => {
      const flowGraph: FlowGraph = {
        nodes: [
          {
            id: 'start-1',
            type: 'START',
            layout: {size: {width: 100, height: 50}, position: {x: 0, y: 0}},
            onSuccess: 'end-1',
          },
          {id: 'end-1', type: 'END', layout: {size: {width: 100, height: 50}, position: {x: 100, y: 0}}},
        ],
      };

      const errors = validateFlowGraph(flowGraph);

      expect(errors).toHaveLength(0);
    });

    it('should detect duplicate node IDs', () => {
      const flowGraph: FlowGraph = {
        nodes: [
          {id: 'node-1', type: 'START', layout: {size: {width: 100, height: 50}, position: {x: 0, y: 0}}},
          {id: 'node-1', type: 'END', layout: {size: {width: 100, height: 50}, position: {x: 100, y: 0}}},
        ],
      };

      const errors = validateFlowGraph(flowGraph);

      expect(errors).toContain('Duplicate node IDs found: node-1');
    });

    it('should detect missing START node', () => {
      const flowGraph: FlowGraph = {
        nodes: [{id: 'end-1', type: 'END', layout: {size: {width: 100, height: 50}, position: {x: 0, y: 0}}}],
      };

      const errors = validateFlowGraph(flowGraph);

      expect(errors).toContain('Flow must have at least one START node');
    });

    it('should detect missing END node', () => {
      const flowGraph: FlowGraph = {
        nodes: [{id: 'start-1', type: 'START', layout: {size: {width: 100, height: 50}, position: {x: 0, y: 0}}}],
      };

      const errors = validateFlowGraph(flowGraph);

      expect(errors).toContain('Flow must have at least one END node');
    });

    it('should detect invalid onSuccess reference', () => {
      const flowGraph: FlowGraph = {
        nodes: [
          {
            id: 'start-1',
            type: 'START',
            layout: {size: {width: 100, height: 50}, position: {x: 0, y: 0}},
            onSuccess: 'non-existent',
          },
          {id: 'end-1', type: 'END', layout: {size: {width: 100, height: 50}, position: {x: 100, y: 0}}},
        ],
      };

      const errors = validateFlowGraph(flowGraph);

      expect(errors).toContain('Node start-1: onSuccess references non-existent node non-existent');
    });

    it('should detect invalid onFailure reference', () => {
      const flowGraph: FlowGraph = {
        nodes: [
          {id: 'start-1', type: 'START', layout: {size: {width: 100, height: 50}, position: {x: 0, y: 0}}},
          {
            id: 'exec-1',
            type: 'TASK_EXECUTION',
            layout: {size: {width: 100, height: 50}, position: {x: 50, y: 0}},
            onFailure: 'non-existent',
          },
          {id: 'end-1', type: 'END', layout: {size: {width: 100, height: 50}, position: {x: 100, y: 0}}},
        ],
      };

      const errors = validateFlowGraph(flowGraph);

      expect(errors).toContain('Node exec-1: onFailure references non-existent node non-existent');
    });

    it('should detect invalid onIncomplete reference', () => {
      const flowGraph: FlowGraph = {
        nodes: [
          {id: 'start-1', type: 'START', layout: {size: {width: 100, height: 50}, position: {x: 0, y: 0}}},
          {
            id: 'exec-1',
            type: 'TASK_EXECUTION',
            layout: {size: {width: 100, height: 50}, position: {x: 50, y: 0}},
            onIncomplete: 'non-existent',
          },
          {id: 'end-1', type: 'END', layout: {size: {width: 100, height: 50}, position: {x: 100, y: 0}}},
        ],
      };

      const errors = validateFlowGraph(flowGraph);

      expect(errors).toContain('Node exec-1: onIncomplete references non-existent node non-existent');
    });

    it('should detect invalid action nextNode reference', () => {
      const flowGraph: FlowGraph = {
        nodes: [
          {
            id: 'start-1',
            type: 'START',
            layout: {size: {width: 100, height: 50}, position: {x: 0, y: 0}},
          },
          {
            id: 'prompt-1',
            type: 'PROMPT',
            layout: {size: {width: 100, height: 50}, position: {x: 50, y: 0}},
            prompts: [{action: {ref: 'button-1', nextNode: 'non-existent'}}],
          },
          {id: 'end-1', type: 'END', layout: {size: {width: 100, height: 50}, position: {x: 100, y: 0}}},
        ],
      };

      const errors = validateFlowGraph(flowGraph);

      expect(errors).toContain('Node prompt-1, action button-1: nextNode references non-existent node non-existent');
    });
  });

  describe('createFlowConfiguration', () => {
    it('should create flow configuration with default values', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('start-1', StaticStepTypes.Start), createNode('end-1', StepTypes.End)],
        edges: [],
      };

      const config = createFlowConfiguration(canvasData);

      expect(config.name).toBe('New Flow');
      expect(config.handle).toBe('new-flow');
      expect(config.flowType).toBe('AUTHENTICATION');
      expect(config.nodes).toHaveLength(2);
    });

    it('should create flow configuration with custom values', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('start-1', StaticStepTypes.Start), createNode('end-1', StepTypes.End)],
        edges: [],
      };

      const config = createFlowConfiguration(canvasData, 'Custom Flow', 'custom-flow', 'REGISTRATION');

      expect(config.name).toBe('Custom Flow');
      expect(config.handle).toBe('custom-flow');
      expect(config.flowType).toBe('REGISTRATION');
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty canvas data', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [],
        edges: [],
      };

      const result = transformReactFlow(canvasData);

      expect(result.nodes).toHaveLength(0);
    });

    it('should handle node with unknown type', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('unknown-1', 'UNKNOWN_TYPE')],
        edges: [],
      };

      const result = transformReactFlow(canvasData);

      expect(result.nodes[0].type).toBe('UNKNOWN_TYPE');
    });

    it('should handle node without type', () => {
      const node: Node<StepData> = {
        id: 'node-1',
        position: {x: 0, y: 0},
        data: {},
      };

      const canvasData: ReactFlowCanvasData = {
        nodes: [node],
        edges: [],
      };

      const result = transformReactFlow(canvasData);

      expect(result.nodes[0].type).toBe('UNKNOWN');
    });

    it('should handle VIEW node without components', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('view-1', StepTypes.View)],
        edges: [],
      };

      const result = transformReactFlow(canvasData);

      expect(result.nodes[0].meta).toBeUndefined();
      expect(result.nodes[0].prompts).toBeUndefined();
    });

    it('should round position values', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('node-1', StepTypes.View, {x: 100.7, y: 200.3})],
        edges: [],
      };

      const result = transformReactFlow(canvasData);

      expect(result.nodes[0].layout.position).toEqual({x: 101, y: 200});
    });

    it('should handle action without next node', () => {
      const components: Element[] = [
        {
          id: 'button-1',
          type: ElementTypes.Action,
          category: ElementCategories.Action,
          action: {}, // No next defined
        } as Element,
      ];

      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
        edges: [], // No edges either
      };

      const result = transformReactFlow(canvasData);

      // Action without next should not be included, so prompts is undefined
      expect(result.nodes[0].prompts).toBeUndefined();
    });

    it('should include executor in action when present', () => {
      const components: Element[] = [
        {
          id: 'button-1',
          type: ElementTypes.Action,
          category: ElementCategories.Action,
          action: {
            onSuccess: 'exec-1',
            executor: {name: 'TestExecutor'},
          },
        } as unknown as Element,
      ];

      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('view-1', StepTypes.View, {x: 0, y: 0}, {components})],
        edges: [createEdge('edge-1', 'view-1', 'exec-1', 'button-1_NEXT')],
      };

      const result = transformReactFlow(canvasData);

      // Executor should be in the prompts array action
      expect(result.nodes[0].prompts?.[0].action?.executor).toEqual({name: 'TestExecutor'});
    });
  });

  describe('CALL node transformation', () => {
    it('should map CALL step type to CALL flow node type', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('call-1', StepTypes.Call, {x: 10, y: 20})],
        edges: [],
      };

      const result = transformReactFlow(canvasData);

      expect(result.nodes[0].type).toBe('CALL');
    });

    it('should read flow.ref from data.flow', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [
          createNode('call-1', StepTypes.Call, {x: 0, y: 0}, {
            flow: {ref: 'referenced-flow-id'},
          } as unknown as StepData),
        ],
        edges: [],
      };

      const result = transformReactFlow(canvasData);

      expect(result.nodes[0].flow).toEqual({ref: 'referenced-flow-id'});
    });

    it('should fall back to data.action.flow.ref when data.flow is absent', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [
          createNode('call-1', StepTypes.Call, {x: 0, y: 0}, {
            action: {type: 'CALL', flow: {ref: 'flow-from-action'}},
          } as unknown as StepData),
        ],
        edges: [],
      };

      const result = transformReactFlow(canvasData);

      expect(result.nodes[0].flow).toEqual({ref: 'flow-from-action'});
    });

    it('should omit flow when neither data.flow nor data.action.flow provides a ref', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('call-1', StepTypes.Call)],
        edges: [],
      };

      const result = transformReactFlow(canvasData);

      expect(result.nodes[0].flow).toBeUndefined();
    });

    it('should derive onSuccess from the next-handle edge', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('call-1', StepTypes.Call), createNode('success-1', StepTypes.End)],
        edges: [createEdge('e-success', 'call-1', 'success-1', 'call-1_NEXT')],
      };

      const result = transformReactFlow(canvasData);

      const callNode = result.nodes.find((n) => n.id === 'call-1')!;
      expect(callNode.onSuccess).toBe('success-1');
      expect(callNode.onFailure).toBeUndefined();
    });

    it('should derive onFailure from the failure-handle edge', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [createNode('call-1', StepTypes.Call), createNode('failure-1', StepTypes.End)],
        edges: [createEdge('e-failure', 'call-1', 'failure-1', 'failure')],
      };

      const result = transformReactFlow(canvasData);

      const callNode = result.nodes.find((n) => n.id === 'call-1')!;
      expect(callNode.onFailure).toBe('failure-1');
    });

    it('should populate both onSuccess and onFailure when both edges exist', () => {
      const canvasData: ReactFlowCanvasData = {
        nodes: [
          createNode('call-1', StepTypes.Call, {x: 0, y: 0}, {flow: {ref: 'ref-1'}} as unknown as StepData),
          createNode('success-1', StepTypes.End),
          createNode('failure-1', StepTypes.End),
        ],
        edges: [
          createEdge('e-success', 'call-1', 'success-1', 'call-1_NEXT'),
          createEdge('e-failure', 'call-1', 'failure-1', 'failure'),
        ],
      };

      const result = transformReactFlow(canvasData);

      const callNode = result.nodes.find((n) => n.id === 'call-1')!;
      expect(callNode.flow).toEqual({ref: 'ref-1'});
      expect(callNode.onSuccess).toBe('success-1');
      expect(callNode.onFailure).toBe('failure-1');
    });
  });
});
