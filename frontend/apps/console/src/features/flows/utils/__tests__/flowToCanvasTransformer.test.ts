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

import {describe, it, expect} from 'vitest';
import VisualFlowConstants from '../../constants/VisualFlowConstants';
import type {FlowDefinitionResponse, FlowNode} from '../../models/responses';
import {StaticStepTypes, StepTypes} from '../../models/steps';
import {transformFlowToCanvas} from '../flowToCanvasTransformer';

describe('flowToCanvasTransformer', () => {
  const createBaseFlowData = (nodes: FlowNode[]): FlowDefinitionResponse => ({
    id: 'test-flow',
    name: 'Test Flow',
    handle: 'test-flow',
    flowType: 'AUTHENTICATION',
    activeVersion: 1,
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-01T00:00:00Z',
    nodes,
  });

  describe('transformFlowToCanvas', () => {
    describe('Basic Transformation', () => {
      it('should transform an empty flow', () => {
        const flowData = createBaseFlowData([]);
        const result = transformFlowToCanvas(flowData);

        expect(result.nodes).toEqual([]);
        expect(result.edges).toEqual([]);
        expect(result.viewport).toEqual({x: 0, y: 0, zoom: 1});
      });

      it('should transform START node correctly', () => {
        const flowData = createBaseFlowData([
          {
            id: 'start-node',
            type: 'START',
            layout: {position: {x: 0, y: 0}, size: {width: 100, height: 50}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes).toHaveLength(1);
        expect(result.nodes[0]).toMatchObject({
          id: 'start-node',
          type: StaticStepTypes.Start,
          deletable: false,
          data: {displayOnly: true},
        });
      });

      it('should transform END node correctly', () => {
        const flowData = createBaseFlowData([
          {
            id: 'end-node',
            type: 'END',
            layout: {position: {x: 500, y: 0}, size: {width: 100, height: 50}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes).toHaveLength(1);
        expect(result.nodes[0]).toMatchObject({
          id: 'end-node',
          type: StepTypes.End,
          deletable: false,
          resourceType: 'STEP',
          category: 'INTERFACE',
        });
      });

      it('should transform PROMPT node correctly', () => {
        const flowData = createBaseFlowData([
          {
            id: 'prompt-node',
            type: 'PROMPT',
            layout: {position: {x: 200, y: 0}, size: {width: 300, height: 200}},
            meta: {
              components: [{id: 'text-1', type: 'TEXT', content: 'Welcome'}],
            },
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes).toHaveLength(1);
        expect(result.nodes[0]).toMatchObject({
          id: 'prompt-node',
          type: StepTypes.View,
          resourceType: 'STEP',
          category: 'INTERFACE',
        });
        expect(result.nodes[0].data.components).toHaveLength(1);
      });

      it('should transform TASK_EXECUTION node correctly', () => {
        const flowData = createBaseFlowData([
          {
            id: 'task-node',
            type: 'TASK_EXECUTION',
            executor: {name: 'UserOnboardingExecutor'},
            onSuccess: 'next-node',
            layout: {position: {x: 300, y: 0}, size: {width: 200, height: 100}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes).toHaveLength(1);
        expect(result.nodes[0]).toMatchObject({
          id: 'task-node',
          type: StepTypes.Execution,
          resourceType: 'STEP',
          category: 'WORKFLOW',
        });
        expect(result.nodes[0].data.action).toMatchObject({
          type: 'EXECUTOR',
          executor: {name: 'UserOnboardingExecutor'},
          onSuccess: 'next-node',
        });
        expect(result.nodes[0].data.action).not.toHaveProperty('onIncomplete');
      });

      it('should include onIncomplete in TASK_EXECUTION node when present', () => {
        const flowData = createBaseFlowData([
          {
            id: 'task-node',
            type: 'TASK_EXECUTION',
            executor: {name: 'UserOnboardingExecutor'},
            onSuccess: 'next-node',
            onIncomplete: 'incomplete-node',
            layout: {position: {x: 300, y: 0}, size: {width: 200, height: 100}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes[0].data.action).toHaveProperty('onIncomplete');
        expect(result.nodes[0].data.action?.onIncomplete).toBe('incomplete-node');
      });

      it('should transform DECISION node correctly', () => {
        const flowData = createBaseFlowData([
          {
            id: 'decision-node',
            type: 'DECISION' as FlowNode['type'],
            onSuccess: 'success-node',
            onFailure: 'failure-node',
            layout: {position: {x: 300, y: 0}, size: {width: 150, height: 100}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes).toHaveLength(1);
        expect(result.nodes[0]).toMatchObject({
          id: 'decision-node',
          type: StepTypes.Rule,
        });
      });
    });

    describe('Node Positions', () => {
      it('should preserve node layout positions', () => {
        const flowData = createBaseFlowData([
          {
            id: 'node-1',
            type: 'START',
            layout: {position: {x: 100, y: 200}, size: {width: 100, height: 50}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes[0].position).toEqual({x: 100, y: 200});
      });

      it('should use default position when layout is missing', () => {
        const flowData = createBaseFlowData([
          {
            id: 'node-1',
            type: 'START',
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes[0].position).toEqual({x: 0, y: 0});
      });

      it('should preserve measured dimensions from layout size', () => {
        const flowData = createBaseFlowData([
          {
            id: 'node-1',
            type: 'PROMPT',
            layout: {position: {x: 0, y: 0}, size: {width: 400, height: 300}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes[0].measured).toEqual({width: 400, height: 300});
      });
    });

    describe('Edge Generation', () => {
      it('should generate edge from START to next node', () => {
        const flowData = createBaseFlowData([
          {
            id: 'start-node',
            type: 'START',
            onSuccess: 'prompt-node',
            layout: {position: {x: 0, y: 0}, size: {width: 100, height: 50}},
          },
          {
            id: 'prompt-node',
            type: 'PROMPT',
            layout: {position: {x: 200, y: 0}, size: {width: 300, height: 200}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(1);
        expect(result.edges[0]).toMatchObject({
          id: 'start-node-prompt-node',
          source: 'start-node',
          target: 'prompt-node',
          type: 'smoothstep',
        });
      });

      it('should generate edges from PROMPT node actions', () => {
        const flowData = createBaseFlowData([
          {
            id: 'prompt-node',
            type: 'PROMPT',
            prompts: [
              {action: {ref: 'button-1', nextNode: 'next-node-1'}},
              {action: {ref: 'button-2', nextNode: 'next-node-2'}},
            ],
            layout: {position: {x: 0, y: 0}, size: {width: 300, height: 200}},
          },
          {
            id: 'next-node-1',
            type: 'PROMPT',
            layout: {position: {x: 200, y: 0}, size: {width: 300, height: 200}},
          },
          {
            id: 'next-node-2',
            type: 'END',
            layout: {position: {x: 200, y: 200}, size: {width: 100, height: 50}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(2);
        expect(result.edges.some((e) => e.id === 'button-1')).toBe(true);
        expect(result.edges.some((e) => e.id === 'button-2')).toBe(true);
      });

      it('should generate edge from TASK_EXECUTION to next node', () => {
        const flowData = createBaseFlowData([
          {
            id: 'task-node',
            type: 'TASK_EXECUTION',
            onSuccess: 'end-node',
            layout: {position: {x: 0, y: 0}, size: {width: 200, height: 100}},
          },
          {
            id: 'end-node',
            type: 'END',
            layout: {position: {x: 200, y: 0}, size: {width: 100, height: 50}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(1);
        expect(result.edges[0].source).toBe('task-node');
        expect(result.edges[0].target).toBe('end-node');
      });

      it('should generate success and failure edges from DECISION node', () => {
        const flowData = createBaseFlowData([
          {
            id: 'decision-node',
            type: 'DECISION' as FlowNode['type'],
            onSuccess: 'success-node',
            onFailure: 'failure-node',
            layout: {position: {x: 0, y: 0}, size: {width: 150, height: 100}},
          },
          {
            id: 'success-node',
            type: 'END',
            layout: {position: {x: 200, y: 0}, size: {width: 100, height: 50}},
          },
          {
            id: 'failure-node',
            type: 'PROMPT',
            layout: {position: {x: 200, y: 200}, size: {width: 300, height: 200}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(2);
        expect(result.edges.some((e) => e.target === 'success-node')).toBe(true);
        expect(result.edges.some((e) => e.target === 'failure-node')).toBe(true);
      });

      it('should generate incomplete edge from TASK_EXECUTION node', () => {
        const flowData = createBaseFlowData([
          {
            id: 'task-node',
            type: 'TASK_EXECUTION',
            onSuccess: 'success-node',
            onIncomplete: 'incomplete-node',
            layout: {position: {x: 0, y: 0}, size: {width: 200, height: 100}},
          },
          {
            id: 'success-node',
            type: 'END',
            layout: {position: {x: 200, y: 0}, size: {width: 100, height: 50}},
          },
          {
            id: 'incomplete-node',
            type: 'PROMPT',
            layout: {position: {x: 200, y: 200}, size: {width: 300, height: 200}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(2);
        const incompleteEdge = result.edges.find((e) =>
          e.sourceHandle?.endsWith(VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX),
        );
        expect(incompleteEdge).toBeDefined();
        expect(incompleteEdge?.target).toBe('incomplete-node');
      });

      it('should not generate edge for non-existent target node', () => {
        const flowData = createBaseFlowData([
          {
            id: 'start-node',
            type: 'START',
            onSuccess: 'non-existent-node',
            layout: {position: {x: 0, y: 0}, size: {width: 100, height: 50}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(0);
      });

      it('should generate edge from display-only PROMPT node using next field', () => {
        const flowData = createBaseFlowData([
          {
            id: 'display-node',
            type: 'PROMPT',
            next: 'end-node',
            layout: {position: {x: 0, y: 0}, size: {width: 300, height: 200}},
          },
          {
            id: 'end-node',
            type: 'END',
            layout: {position: {x: 400, y: 0}, size: {width: 100, height: 50}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(1);
        const edge = result.edges[0];
        expect(edge.source).toBe('display-node');
        expect(edge.target).toBe('end-node');
        expect(edge.sourceHandle).toBe(`display-node${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`);
      });

      it('should not generate edge when next field references non-existent node', () => {
        const flowData = createBaseFlowData([
          {
            id: 'display-node',
            type: 'PROMPT',
            next: 'non-existent-node',
            layout: {position: {x: 0, y: 0}, size: {width: 300, height: 200}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(0);
      });

      it('should prefer prompt action edges over next field when PROMPT node has both', () => {
        const flowData = createBaseFlowData([
          {
            id: 'prompt-node',
            type: 'PROMPT',
            next: 'end-node',
            prompts: [{action: {ref: 'submit-btn', nextNode: 'end-node'}}],
            layout: {position: {x: 0, y: 0}, size: {width: 300, height: 200}},
          },
          {
            id: 'end-node',
            type: 'END',
            layout: {position: {x: 400, y: 0}, size: {width: 100, height: 50}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(1);
        // Should use action ref as edge ID (prompt source handle), not the node-level handle
        expect(result.edges[0].id).toBe('submit-btn');
        expect(result.edges[0].sourceHandle).toBe(`submit-btn${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`);
      });
    });

    describe('Component Transformation', () => {
      it('should restore button action from node actions', () => {
        const flowData = createBaseFlowData([
          {
            id: 'prompt-node',
            type: 'PROMPT',
            meta: {
              components: [{id: 'submit-btn', type: 'ACTION', label: 'Submit'}],
            },
            prompts: [{action: {ref: 'submit-btn', nextNode: 'next-node', executor: {name: 'SomeExecutor'}}}],
            layout: {position: {x: 0, y: 0}, size: {width: 300, height: 200}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        const component = result.nodes[0].data.components?.[0] as Record<string, unknown> | undefined;
        expect(component).toBeDefined();
        expect(component?.action).toMatchObject({
          type: 'EXECUTOR',
          onSuccess: 'next-node',
          executor: {name: 'SomeExecutor'},
        });
      });

      it('should normalize INPUT element properties', () => {
        const flowData = createBaseFlowData([
          {
            id: 'prompt-node',
            type: 'PROMPT',
            meta: {
              components: [
                {id: 'email-input', type: 'EMAIL_INPUT', variant: 'EMAIL'},
                {id: 'password-input', type: 'PASSWORD_INPUT', variant: 'PASSWORD'},
              ],
            },
            layout: {position: {x: 0, y: 0}, size: {width: 300, height: 200}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        const components = result.nodes[0].data.components as Record<string, unknown>[] | undefined;
        expect(components?.[0]?.inputType).toBe('email');
        expect(components?.[1]?.inputType).toBe('password');
      });

      it('should handle nested components (forms)', () => {
        const flowData = createBaseFlowData([
          {
            id: 'prompt-node',
            type: 'PROMPT',
            meta: {
              components: [
                {
                  id: 'form-1',
                  type: 'FORM',
                  components: [
                    {id: 'input-1', type: 'TEXT_INPUT'},
                    {id: 'button-1', type: 'ACTION'},
                  ],
                },
              ],
            },
            prompts: [{action: {ref: 'button-1', nextNode: 'next-node'}}],
            layout: {position: {x: 0, y: 0}, size: {width: 300, height: 200}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        const form = result.nodes[0].data.components?.[0] as Record<string, unknown> | undefined;
        expect(form).toBeDefined();
        const formComponents = form?.components as Record<string, unknown>[] | undefined;
        expect(formComponents).toHaveLength(2);
        expect(formComponents?.[1]?.action).toBeDefined();
      });
    });

    describe('Viewport Calculation', () => {
      it('should return default viewport for empty nodes', () => {
        const flowData = createBaseFlowData([]);
        const result = transformFlowToCanvas(flowData);

        expect(result.viewport).toEqual({x: 0, y: 0, zoom: 1});
      });

      it('should calculate viewport based on node positions', () => {
        const flowData = createBaseFlowData([
          {
            id: 'node-1',
            type: 'START',
            layout: {position: {x: 0, y: 0}, size: {width: 100, height: 50}},
          },
          {
            id: 'node-2',
            type: 'END',
            layout: {position: {x: 500, y: 300}, size: {width: 100, height: 50}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        // Viewport should be calculated
        expect(result.viewport.zoom).toBeGreaterThan(0);
        expect(result.viewport.zoom).toBeLessThanOrEqual(1);
      });
    });

    describe('Node Deletability', () => {
      it('should mark START node as non-deletable', () => {
        const flowData = createBaseFlowData([
          {id: 'start', type: 'START', layout: {position: {x: 0, y: 0}, size: {width: 100, height: 50}}},
        ]);

        const result = transformFlowToCanvas(flowData);
        expect(result.nodes[0].deletable).toBe(false);
      });

      it('should mark END node as non-deletable', () => {
        const flowData = createBaseFlowData([
          {id: 'end', type: 'END', layout: {position: {x: 0, y: 0}, size: {width: 100, height: 50}}},
        ]);

        const result = transformFlowToCanvas(flowData);
        expect(result.nodes[0].deletable).toBe(false);
      });

      it('should mark PROMPT node as deletable', () => {
        const flowData = createBaseFlowData([
          {id: 'prompt', type: 'PROMPT', layout: {position: {x: 0, y: 0}, size: {width: 300, height: 200}}},
        ]);

        const result = transformFlowToCanvas(flowData);
        expect(result.nodes[0].deletable).toBe(true);
      });

      it('should mark TASK_EXECUTION node as deletable', () => {
        const flowData = createBaseFlowData([
          {id: 'task', type: 'TASK_EXECUTION', layout: {position: {x: 0, y: 0}, size: {width: 200, height: 100}}},
        ]);

        const result = transformFlowToCanvas(flowData);
        expect(result.nodes[0].deletable).toBe(true);
      });
    });

    describe('CALL node transformation', () => {
      it('should transform CALL node with flow ref, resourceType and category', () => {
        const flowData = createBaseFlowData([
          {
            id: 'call-1',
            type: 'CALL',
            flow: {ref: 'referenced-flow'},
            onSuccess: 'next-node',
            onFailure: 'failure-node',
            layout: {position: {x: 300, y: 400}, size: {width: 260, height: 120}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes).toHaveLength(1);
        expect(result.nodes[0]).toMatchObject({
          id: 'call-1',
          type: StepTypes.Call,
          resourceType: 'STEP',
          category: 'WORKFLOW',
        });
        expect(result.nodes[0].data).toMatchObject({
          flow: {ref: 'referenced-flow'},
          action: {
            type: 'CALL',
            flow: {ref: 'referenced-flow'},
            onSuccess: 'next-node',
            onFailure: 'failure-node',
          },
        });
      });

      it('should default flow to {ref: ""} when the API node has no flow field', () => {
        const flowData = createBaseFlowData([
          {
            id: 'call-1',
            type: 'CALL',
            layout: {position: {x: 0, y: 0}, size: {width: 260, height: 120}},
          },
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.nodes[0].data).toMatchObject({
          flow: {ref: ''},
          action: {type: 'CALL', flow: {ref: ''}},
        });
      });

      it('should generate the onSuccess edge from a CALL node using the _NEXT source handle', () => {
        const flowData = createBaseFlowData([
          {id: 'call-1', type: 'CALL', flow: {ref: 'ref'}, onSuccess: 'next-1'},
          {id: 'next-1', type: 'END'},
        ]);

        const result = transformFlowToCanvas(flowData);

        const successEdge = result.edges.find((e) => e.id === 'call-1-to-next-1');
        expect(successEdge).toBeDefined();
        expect(successEdge).toMatchObject({
          source: 'call-1',
          target: 'next-1',
          sourceHandle: `call-1${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
          type: 'smoothstep',
        });
      });

      it('should generate the onFailure edge from a CALL node using the "failure" source handle', () => {
        const flowData = createBaseFlowData([
          {id: 'call-1', type: 'CALL', flow: {ref: 'ref'}, onFailure: 'fail-1'},
          {id: 'fail-1', type: 'END'},
        ]);

        const result = transformFlowToCanvas(flowData);

        const failureEdge = result.edges.find((e) => e.id === 'call-1-failure-to-fail-1');
        expect(failureEdge).toBeDefined();
        expect(failureEdge).toMatchObject({
          source: 'call-1',
          target: 'fail-1',
          sourceHandle: 'failure',
        });
      });

      it('should not emit edges when CALL onSuccess/onFailure targets are missing from the node list', () => {
        const flowData = createBaseFlowData([
          {id: 'call-1', type: 'CALL', flow: {ref: 'ref'}, onSuccess: 'missing-1', onFailure: 'missing-2'},
        ]);

        const result = transformFlowToCanvas(flowData);

        expect(result.edges).toHaveLength(0);
      });
    });
  });
});
