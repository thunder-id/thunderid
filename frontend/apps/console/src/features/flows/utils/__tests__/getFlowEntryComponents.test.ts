/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import {FlowNodeType} from '../../models/flows';
import type {FlowDefinitionResponse, FlowNode} from '../../models/responses';
import getFlowEntryComponents from '../getFlowEntryComponents';

describe('getFlowEntryComponents', () => {
  const createFlow = (nodes: FlowNode[]): FlowDefinitionResponse => ({
    id: 'flow-1',
    name: 'Test Flow',
    handle: 'test-flow',
    flowType: 'AUTHENTICATION',
    activeVersion: 1,
    nodes,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  });

  const promptComponents = [{id: 'username', type: 'TEXT_INPUT'}];

  it('returns the components of the PROMPT node reached from START via onSuccess', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt'},
      {id: 'prompt', type: FlowNodeType.PROMPT, meta: {components: promptComponents}},
    ]);

    expect(getFlowEntryComponents(flow)).toEqual(promptComponents);
  });

  it('walks through TASK_EXECUTION nodes to reach the first PROMPT', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'task'},
      {id: 'task', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'prompt'},
      {id: 'prompt', type: FlowNodeType.PROMPT, meta: {components: promptComponents}},
    ]);

    expect(getFlowEntryComponents(flow)).toEqual(promptComponents);
  });

  it('falls back to the first PROMPT node with components when START is not connected', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START},
      {id: 'prompt', type: FlowNodeType.PROMPT, meta: {components: promptComponents}},
    ]);

    expect(getFlowEntryComponents(flow)).toEqual(promptComponents);
  });

  it('skips PROMPT nodes without components when traversing', () => {
    const second = [{id: 'password', type: 'PASSWORD_INPUT'}];
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'empty-prompt'},
      {id: 'empty-prompt', type: FlowNodeType.PROMPT, meta: {components: []}, onSuccess: 'prompt'},
      {id: 'prompt', type: FlowNodeType.PROMPT, meta: {components: second}},
    ]);

    expect(getFlowEntryComponents(flow)).toEqual(second);
  });

  it('returns null when the flow has no PROMPT node with components', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'end'},
      {id: 'end', type: FlowNodeType.END},
    ]);

    expect(getFlowEntryComponents(flow)).toBeNull();
  });

  it('does not loop forever on a cyclic graph', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'a'},
      {id: 'a', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'b'},
      {id: 'b', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'a'},
    ]);

    expect(getFlowEntryComponents(flow)).toBeNull();
  });

  it('returns null for an undefined flow', () => {
    expect(getFlowEntryComponents(undefined)).toBeNull();
  });

  it('returns null for a flow with no nodes', () => {
    expect(getFlowEntryComponents(createFlow([]))).toBeNull();
  });
});
