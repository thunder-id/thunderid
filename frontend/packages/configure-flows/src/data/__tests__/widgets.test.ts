// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {ExecutionTypes} from '../../models/steps';
import {WidgetTypes} from '../../models/widget';
import widgets from '../widgets.json';

interface WidgetStep {
  id: string;
  type: string;
  data?: {
    action?: {
      type?: string;
      executor?: {name?: string};
      onSuccess?: string;
      onIncomplete?: string;
    };
    flow?: {ref?: string; filterFlowType?: string};
    properties?: Record<string, unknown>;
    components?: Record<string, unknown>[];
  };
}

const stepsOf = (type: string): WidgetStep[] =>
  (widgets.find((widget) => widget.type === type)?.config?.data as {steps?: WidgetStep[]})?.steps ?? [];

const autoWireOf = (type: string): {spliceAfter?: {executorName: string}[]; spliceBefore?: {executorName: string}[]} =>
  (
    widgets.find((widget) => widget.type === type)?.config?.data as {
      __generationMeta__?: {
        autoWire?: {spliceAfter?: {executorName: string}[]; spliceBefore?: {executorName: string}[]};
      };
    }
  )?.__generationMeta__?.autoWire ?? {};

const linkingStep = (type: string): WidgetStep =>
  stepsOf(type).find((step) => step.data?.action?.executor?.name === 'AccountLinkingExecutor')!;

const promptButtons = (type: string): Record<string, unknown>[] => {
  const view = stepsOf(type).find((step) => step.id === '{{LINKING_PROMPT_VIEW_STEP_ID}}')!;
  const block = (view.data?.components ?? []).find((component) => component.type === 'BLOCK') as {
    components: Record<string, unknown>[];
  };

  return block.components;
};

describe('widgets catalog', () => {
  it('registers every widget under a known type', () => {
    const knownTypes = new Set<string>(Object.values(WidgetTypes));

    widgets.forEach((widget) => {
      expect(knownTypes.has(widget.type ?? '')).toBe(true);
    });
  });

  // The linking node reads the refusal, so REJECT has to point back at it, and the node only ever
  // reaches its prompt through onIncomplete. Either link broken and the dropped cluster is a prompt
  // nobody answers. The confirmation leads straight to the CALL step that runs the verification flow.
  it('drops a verification cluster wired to its prompt', () => {
    const linking = linkingStep(WidgetTypes.AccountLinkingVerification);

    expect(linking.data?.action?.onIncomplete).toBe('{{LINKING_PROMPT_VIEW_STEP_ID}}');

    expect(promptButtons(WidgetTypes.AccountLinkingVerification)).toEqual([
      expect.objectContaining({
        actionType: 'CONFIRM',
        action: {type: 'NEXT', onSuccess: '{{LINKING_VERIFY_CALL_STEP_ID}}'},
      }),
      expect.objectContaining({
        actionType: 'REJECT',
        action: {type: 'NEXT', onSuccess: '{{LINKING_EXECUTOR_STEP_ID}}'},
      }),
    ]);
  });

  // The splice anchors are matched against the executor name on the node, so a name that is not an
  // executor the builder knows silently drops the cluster unconnected.
  it('anchors the verification cluster on executors the builder knows', () => {
    const knownExecutors = new Set<string>(Object.values(ExecutionTypes));
    const autoWire = autoWireOf(WidgetTypes.AccountLinkingVerification);
    const anchors = [...(autoWire.spliceAfter ?? []), ...(autoWire.spliceBefore ?? [])];

    expect(anchors.length).toBeGreaterThan(0);
    anchors.forEach((anchor) => expect(knownExecutors.has(anchor.executorName)).toBe(true));
  });

  // The verification runs as an ordinary sign-in in a callee frame, so the step the confirmation leads
  // to is a CALL that offers sign-in flows and hands the flow back to the linking node once it returns.
  it('verifies through a sign-in flow and returns to the linking node', () => {
    const call = stepsOf(WidgetTypes.AccountLinkingVerification).find((step) => step.type === 'CALL');

    expect(call?.id).toBe('{{LINKING_VERIFY_CALL_STEP_ID}}');
    expect(call?.data?.flow?.filterFlowType).toBe('AUTHENTICATION');
    expect(call?.data?.action?.type).toBe('CALL');
    expect(call?.data?.action?.onSuccess).toBe('{{LINKING_EXECUTOR_STEP_ID}}');
  });
});
