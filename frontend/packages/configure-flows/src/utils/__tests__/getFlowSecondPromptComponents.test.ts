// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {FlowNodeType} from '../../models/flows';
import type {FlowDefinitionResponse, FlowNode} from '../../models/responses';
import getFlowSecondPromptComponents from '../getFlowSecondPromptComponents';

describe('getFlowSecondPromptComponents', () => {
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

  const firstPromptComponents = [{id: 'username', type: 'TEXT_INPUT'}];
  const secondPromptComponents = [{id: 'otp', type: 'OTP_INPUT'}];

  it('returns null when the flow only has one PROMPT screen', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt'},
      {id: 'prompt', type: FlowNodeType.PROMPT, meta: {components: firstPromptComponents}},
    ]);

    expect(getFlowSecondPromptComponents(flow)).toBeNull();
  });

  it('walks through TASK_EXECUTION nodes to reach the second PROMPT', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt1'},
      {id: 'prompt1', type: FlowNodeType.PROMPT, meta: {components: firstPromptComponents}, onSuccess: 'task'},
      {id: 'task', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'prompt2'},
      {id: 'prompt2', type: FlowNodeType.PROMPT, meta: {components: secondPromptComponents}},
    ]);

    expect(getFlowSecondPromptComponents(flow)).toEqual(secondPromptComponents);
  });

  it('follows a PROMPT node routed via prompts[].action.nextNode rather than onSuccess', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt1'},
      {
        id: 'prompt1',
        type: FlowNodeType.PROMPT,
        meta: {components: firstPromptComponents},
        prompts: [{action: {ref: 'action_submit', nextNode: 'task'}}],
      },
      {id: 'task', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'prompt2'},
      {id: 'prompt2', type: FlowNodeType.PROMPT, meta: {components: secondPromptComponents}},
    ]);

    expect(getFlowSecondPromptComponents(flow)).toEqual(secondPromptComponents);
  });

  it('mirrors the credentials -> OTP MFA chain used by the generated flow graph', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt_credentials'},
      {
        id: 'prompt_credentials',
        type: FlowNodeType.PROMPT,
        meta: {components: firstPromptComponents},
        prompts: [{action: {ref: 'action_submit', nextNode: 'credentials_auth'}}],
      },
      {id: 'credentials_auth', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'generate_otp'},
      {id: 'generate_otp', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'sms_send'},
      {id: 'sms_send', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'view_otp'},
      {id: 'view_otp', type: FlowNodeType.PROMPT, meta: {components: secondPromptComponents}},
    ]);

    expect(getFlowSecondPromptComponents(flow)).toEqual(secondPromptComponents);
  });

  it('skips PROMPT nodes without components when counting screens', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt1'},
      {id: 'prompt1', type: FlowNodeType.PROMPT, meta: {components: firstPromptComponents}, onSuccess: 'empty-prompt'},
      {id: 'empty-prompt', type: FlowNodeType.PROMPT, meta: {components: []}, onSuccess: 'prompt2'},
      {id: 'prompt2', type: FlowNodeType.PROMPT, meta: {components: secondPromptComponents}},
    ]);

    expect(getFlowSecondPromptComponents(flow)).toEqual(secondPromptComponents);
  });

  it('does not loop forever on a cyclic graph', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt1'},
      {id: 'prompt1', type: FlowNodeType.PROMPT, meta: {components: firstPromptComponents}, onSuccess: 'a'},
      {id: 'a', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'b'},
      {id: 'b', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'a'},
    ]);

    expect(getFlowSecondPromptComponents(flow)).toBeNull();
  });

  it('returns null for an undefined flow', () => {
    expect(getFlowSecondPromptComponents(undefined)).toBeNull();
  });

  it('returns null for a flow with no nodes', () => {
    expect(getFlowSecondPromptComponents(createFlow([]))).toBeNull();
  });
});
