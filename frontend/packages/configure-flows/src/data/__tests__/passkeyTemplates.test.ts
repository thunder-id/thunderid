// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {FlowNode} from '../../models/responses';
import templates from '../templates.json';
import widgets from '../widgets.json';

/** A step in the flow builder's own format, used by starter templates and widgets. */
interface BuilderStep {
  id?: string;
  type: string;
  data?: {
    action?: {executor?: {name?: string; mode?: string}; onSuccess?: string; onFailure?: string};
    components?: unknown[];
  };
}

/** A node reduced to what the routing check needs. */
interface GraphNode {
  id: string;
  type: string;
  executor?: {name?: string; mode?: string};
  onFailure?: string;
  next: string[];
}

interface Source {
  type?: string;
  config?: {nodes?: FlowNode[]; data?: {steps?: BuilderStep[]}};
}

/** Every `action.onSuccess` in a builder view's components. */
const viewTargets = (value: unknown): string[] => {
  if (Array.isArray(value)) return value.flatMap(viewTargets);
  if (!value || typeof value !== 'object') return [];
  const {action, ...rest} = value as {action?: {onSuccess?: string}};
  return [...(action?.onSuccess ? [action.onSuccess] : []), ...viewTargets(Object.values(rest))];
};

const fromFlowNode = (node: FlowNode): GraphNode => ({
  id: node.id,
  type: node.type,
  executor: node.executor,
  onFailure: node.onFailure,
  next: [
    ...(node.prompts ?? []).flatMap((prompt) => prompt.action?.nextNode ?? []),
    ...(node.onSuccess ? [node.onSuccess] : []),
  ],
});

const fromBuilderStep = (step: BuilderStep & {id: string}): GraphNode => ({
  id: step.id,
  type: step.type,
  executor: step.data?.action?.executor,
  onFailure: step.data?.action?.onFailure,
  next: [...viewTargets(step.data?.components), ...(step.data?.action?.onSuccess ? [step.data.action.onSuccess] : [])],
});

/** Every step that checks a passkey response, with the nodes of its template or widget. */
const passkeyResponseSteps = (sources: Source[], label: string) =>
  sources.flatMap((source) => {
    const nodes = [
      ...(source.config?.nodes ?? []).map(fromFlowNode),
      ...(source.config?.data?.steps ?? [])
        .filter((step): step is BuilderStep & {id: string} => !!step.id)
        .map(fromBuilderStep),
    ];
    return nodes
      .filter(
        (node) =>
          node.executor?.name === 'PasskeyAuthExecutor' &&
          (node.executor.mode === 'verify' || node.executor.mode === 'register_finish'),
      )
      .map((node) => ({name: `${label} ${source.type} ${node.id}`, node, nodes}));
  });

/** Whether a new passkey ceremony can be started from the given node. */
const leadsToNewAttempt = (fromId: string, nodes: GraphNode[]): boolean => {
  const seen = new Set<string>();
  const queue = [fromId];
  while (queue.length) {
    const id = queue.shift();
    const node = nodes.find((candidate) => candidate.id === id);
    if (!node || seen.has(node.id)) continue;
    seen.add(node.id);
    if (node.executor?.mode === 'challenge' || node.executor?.mode === 'register_start') return true;
    queue.push(...node.next);
  }
  return false;
};

const steps = [
  ...passkeyResponseSteps(templates as Source[], 'template'),
  ...passkeyResponseSteps(widgets as Source[], 'widget'),
];

describe('passkey templates and widgets', () => {
  it('have passkey response steps in both formats', () => {
    expect(steps.some(({name}) => name.startsWith('template'))).toBe(true);
    expect(steps.some(({name}) => name.startsWith('widget'))).toBe(true);
  });

  // A cancelled prompt or a rejected passkey fails the step. Without a route to a prompt, the flow ends there.
  it.each(steps)('$name sends a failed passkey to a prompt that can try again', ({node, nodes}) => {
    const prompt = nodes.find((candidate) => candidate.id === node.onFailure);

    expect(['PROMPT', 'VIEW']).toContain(prompt?.type);
    expect(leadsToNewAttempt(prompt!.id, nodes)).toBe(true);
  });
});
