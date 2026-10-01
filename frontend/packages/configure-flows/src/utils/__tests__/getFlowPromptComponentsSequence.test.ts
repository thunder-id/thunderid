// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {FlowNodeType} from '../../models/flows';
import type {FlowDefinitionResponse, FlowNode} from '../../models/responses';
import generateFlowGraph from '../generateFlowGraph';
import getFlowPromptComponentsSequence from '../getFlowPromptComponentsSequence';

describe('getFlowPromptComponentsSequence', () => {
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
  const thirdPromptComponents = [{id: 'confirm', type: 'TEXT'}];

  it('returns the single screen of a flow with only one PROMPT', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt'},
      {id: 'prompt', type: FlowNodeType.PROMPT, meta: {components: firstPromptComponents}},
    ]);

    expect(getFlowPromptComponentsSequence(flow)).toEqual([firstPromptComponents]);
  });

  it('walks through TASK_EXECUTION nodes to reach subsequent PROMPT screens', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt1'},
      {id: 'prompt1', type: FlowNodeType.PROMPT, meta: {components: firstPromptComponents}, onSuccess: 'task'},
      {id: 'task', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'prompt2'},
      {id: 'prompt2', type: FlowNodeType.PROMPT, meta: {components: secondPromptComponents}},
    ]);

    expect(getFlowPromptComponentsSequence(flow)).toEqual([firstPromptComponents, secondPromptComponents]);
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

    expect(getFlowPromptComponentsSequence(flow)).toEqual([firstPromptComponents, secondPromptComponents]);
  });

  it('follows a display-only PROMPT node routed via `next` rather than `prompts`', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt1'},
      {
        id: 'prompt1',
        type: FlowNodeType.PROMPT,
        meta: {components: firstPromptComponents},
        prompts: [{action: {ref: 'action_submit', nextNode: 'wait'}}],
      },
      {id: 'wait', type: FlowNodeType.PROMPT, meta: {components: secondPromptComponents}, next: 'prompt3'},
      {id: 'prompt3', type: FlowNodeType.PROMPT, meta: {components: thirdPromptComponents}},
    ]);

    expect(getFlowPromptComponentsSequence(flow)).toEqual([
      firstPromptComponents,
      secondPromptComponents,
      thirdPromptComponents,
    ]);
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

    expect(getFlowPromptComponentsSequence(flow)).toEqual([firstPromptComponents, secondPromptComponents]);
  });

  it('skips PROMPT nodes without components when walking, but keeps going', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt1'},
      {id: 'prompt1', type: FlowNodeType.PROMPT, meta: {components: firstPromptComponents}, onSuccess: 'empty-prompt'},
      {id: 'empty-prompt', type: FlowNodeType.PROMPT, meta: {components: []}, onSuccess: 'prompt2'},
      {id: 'prompt2', type: FlowNodeType.PROMPT, meta: {components: secondPromptComponents}},
    ]);

    expect(getFlowPromptComponentsSequence(flow)).toEqual([firstPromptComponents, secondPromptComponents]);
  });

  it('falls back to the first PROMPT node with components when START is not connected', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START},
      {id: 'prompt', type: FlowNodeType.PROMPT, meta: {components: firstPromptComponents}},
    ]);

    expect(getFlowPromptComponentsSequence(flow)).toEqual([firstPromptComponents]);
  });

  it('returns null when the flow has no PROMPT node with components', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'end'},
      {id: 'end', type: FlowNodeType.END},
    ]);

    expect(getFlowPromptComponentsSequence(flow)).toBeNull();
  });

  it('does not loop forever on a cyclic graph', () => {
    const flow = createFlow([
      {id: 'start', type: FlowNodeType.START, onSuccess: 'a'},
      {id: 'a', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'b'},
      {id: 'b', type: FlowNodeType.TASK_EXECUTION, onSuccess: 'a'},
    ]);

    expect(getFlowPromptComponentsSequence(flow)).toBeNull();
  });

  it('stops within MAX_STEPS on a very long acyclic chain rather than hanging', () => {
    const nodes: FlowNode[] = [{id: 'start', type: FlowNodeType.START, onSuccess: 'n0'}];
    for (let i = 0; i < 100; i += 1) {
      nodes.push({id: `n${i}`, type: FlowNodeType.TASK_EXECUTION, onSuccess: `n${i + 1}`});
    }
    const flow = createFlow(nodes);

    expect(getFlowPromptComponentsSequence(flow)).toBeNull();
  });

  it('returns null for an undefined flow', () => {
    expect(getFlowPromptComponentsSequence(undefined)).toBeNull();
  });

  it('returns null for a flow with no nodes', () => {
    expect(getFlowPromptComponentsSequence(createFlow([]))).toBeNull();
  });

  it('previews Magic Link on the first screen when it is enabled alongside Credentials Auth', () => {
    const generated = generateFlowGraph({
      hasCredentialsAuth: true,
      hasPasskey: false,
      hasMagicLink: true,
      hasSmsOtp: false,
    });

    const screens = getFlowPromptComponentsSequence(createFlow(generated.nodes));

    expect(screens?.[0]?.find((c) => (c as {id: string}).id === 'block_magic_link')).toBeDefined();
    expect(screens).toHaveLength(1);
  });
});
