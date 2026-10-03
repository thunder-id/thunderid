// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import VisualFlowConstants from '../../constants/VisualFlowConstants';
import type {Element} from '../../models/elements';
import {StepTypes} from '../../models/steps';
import {transformReactFlow} from '../../utils/reactFlowTransformer';
import steps from '../steps.json';

interface StepResource {
  type?: string;
  display?: {label?: string};
  data?: {components?: Record<string, unknown>[]};
}

const linkPromptView = (): StepResource =>
  (steps as StepResource[]).find((step) => step.display?.label === 'Link Prompt View')!;

const promptBlock = (): Record<string, unknown>[] => {
  const block = (linkPromptView().data?.components ?? []).find((component) => component.type === 'BLOCK') as {
    components: Record<string, unknown>[];
  };

  return block.components;
};

describe('steps catalog', () => {
  // The identifiers the backend matches by exact string: the action types LinkingExecutor switches
  // on, and the flow data key it publishes the matched account under. Getting either wrong renders
  // a prompt that names no account, or whose answer the executor never sees.
  it('ships the link prompt view with the identifiers LinkingExecutor expects', () => {
    const view = linkPromptView();

    expect(view).toBeDefined();
    expect(view.type).toBe(StepTypes.View);

    expect(
      (view.data?.components ?? []).some(
        (component) => component.type === 'KEY_VALUE_LIST' && component.source === 'linkingPromptDetails',
      ),
    ).toBe(true);

    // Two buttons and no field: the answer is which action was raised, never a collected value.
    expect(promptBlock()).toEqual([
      expect.objectContaining({type: 'ACTION', actionType: 'CONFIRM', eventType: 'SUBMIT'}),
      expect.objectContaining({type: 'ACTION', actionType: 'REJECT', eventType: 'SUBMIT'}),
    ]);
    expect(promptBlock().some((component) => component.category === 'FIELD')).toBe(false);
  });

  // LinkingExecutor reads the action type off ForwardedData, which the prompt node fills from the
  // serialized action. A button whose actionType is dropped on the way out is silently inert.
  it('serializes both buttons into prompt actions carrying their action type', () => {
    const components = (linkPromptView().data?.components ?? []).map((component, index) =>
      component.type === 'BLOCK'
        ? {
            ...component,
            components: (component.components as Record<string, unknown>[]).map((child, childIndex) => ({
              ...child,
              id: `action_${childIndex}`,
            })),
          }
        : {...component, id: `component_${index}`},
    );

    const result = transformReactFlow({
      nodes: [
        {
          id: 'linking_prompt',
          type: StepTypes.View,
          position: {x: 0, y: 0},
          data: {components: components as unknown as Element[]},
        },
      ],
      edges: ['action_0', 'action_1'].map((actionId, index) => ({
        id: `edge-${index}`,
        source: 'linking_prompt',
        target: 'linking',
        sourceHandle: `${actionId}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
      })),
    });

    const prompts = result.nodes.flatMap((node) => node.prompts ?? []);

    expect(prompts.map((prompt) => prompt.action?.type)).toEqual(['CONFIRM', 'REJECT']);
    prompts.forEach((prompt) => expect(prompt.inputs ?? []).toHaveLength(0));
  });
});
