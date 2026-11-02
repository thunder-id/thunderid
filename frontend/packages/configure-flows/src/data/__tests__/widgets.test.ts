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
      executor?: {name?: string; mode?: string};
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

const linkingWidgets = [WidgetTypes.AccountLinkingVerification, WidgetTypes.AccountLinkingVerificationWithSelection];

describe('widgets catalog', () => {
  // The linking node reads the refusal, so REJECT has to point back at it, and the node only ever
  // reaches its prompt through onIncomplete. Either link broken and the dropped cluster is a prompt
  // nobody answers. The confirmation leads straight to the CALL step that runs the verification flow.
  it.each(linkingWidgets)('drops a verification cluster wired to its prompt in %s', (type) => {
    const linking = linkingStep(type);

    expect(linking.data?.action?.onIncomplete).toBe('{{LINKING_PROMPT_VIEW_STEP_ID}}');

    expect(promptButtons(type)).toEqual([
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
  it.each(linkingWidgets)('anchors the verification cluster on executors the builder knows in %s', (type) => {
    const knownExecutors = new Set<string>(Object.values(ExecutionTypes));
    const autoWire = autoWireOf(type);
    const anchors = [...(autoWire.spliceAfter ?? []), ...(autoWire.spliceBefore ?? [])];

    expect(anchors.length).toBeGreaterThan(0);
    anchors.forEach((anchor) => expect(knownExecutors.has(anchor.executorName)).toBe(true));
  });

  // The verification runs as an ordinary sign-in in a callee frame, so the step the confirmation leads
  // to is a CALL that offers sign-in flows and hands the flow back to the linking node once it returns.
  it.each(linkingWidgets)('verifies through a sign-in flow and returns to the linking node in %s', (type) => {
    const call = stepsOf(type).find((step) => step.type === 'CALL');

    expect(call?.id).toBe('{{LINKING_VERIFY_CALL_STEP_ID}}');
    expect(call?.data?.flow?.filterFlowType).toBe('AUTHENTICATION');
    expect(call?.data?.action?.type).toBe('CALL');
    expect(call?.data?.action?.onSuccess).toBe('{{LINKING_EXECUTOR_STEP_ID}}');
  });

  // The plain widget links without picking between users that hold the identity.
  it('does not resolve between linked users in the plain widget', () => {
    const steps = stepsOf(WidgetTypes.AccountLinkingVerification);

    expect(steps.some((step) => step.data?.action?.executor?.name === 'IdentifyingExecutor')).toBe(false);
  });

  // An identity linked to several users is picked between before linking runs. The resolver is the
  // cluster's entry, passes on to the linking node, and asks through a picker that comes back to it.
  it('resolves between linked users before linking', () => {
    const steps = stepsOf(WidgetTypes.AccountLinkingVerificationWithSelection);
    const resolver = steps.find((step) => step.id === '{{LINK_RESOLVER_STEP_ID}}');
    const picker = steps.find((step) => step.id === '{{LINK_PICKER_VIEW_STEP_ID}}');
    const entry = (
      widgets.find((widget) => widget.type === WidgetTypes.AccountLinkingVerificationWithSelection)?.config?.data as {
        __generationMeta__?: {autoWire?: {entry?: {stepRef?: string}}};
      }
    )?.__generationMeta__?.autoWire?.entry?.stepRef;

    expect(entry).toBe('LINK_RESOLVER_STEP_ID');
    expect(resolver?.data?.action?.executor).toEqual({name: 'IdentifyingExecutor', mode: 'resolve'});
    expect(resolver?.data?.action?.onSuccess).toBe('{{LINKING_EXECUTOR_STEP_ID}}');
    expect(resolver?.data?.action?.onIncomplete).toBe('{{LINK_PICKER_VIEW_STEP_ID}}');

    const block = (picker?.data?.components ?? []).find((component) => component.type === 'BLOCK') as {
      components: Record<string, unknown>[];
    };
    expect(block.components).toEqual([
      expect.objectContaining({type: 'SELECT', ref: 'ouHandle'}),
      expect.objectContaining({action: {type: 'NEXT', onSuccess: '{{LINK_RESOLVER_STEP_ID}}'}}),
    ]);
  });
});
