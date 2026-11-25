// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import templates from '../templates.json';

interface Prompt {
  inputs?: {type?: string}[];
  action?: {nextNode?: string};
}

interface Node {
  id: string;
  type: string;
  onIncomplete?: string;
  executor?: {name: string};
  prompts?: Prompt[];
}

interface Template {
  type?: string;
  flowType?: string;
  config?: {nodes?: Node[]};
}

/** Every password check in the sign-in templates, with the nodes of its template. */
const passwordChecks = (templates as Template[])
  .filter((template) => template.flowType === 'AUTHENTICATION')
  .flatMap((template) => {
    const nodes = template.config?.nodes ?? [];
    return nodes
      .filter((node) => node.executor?.name === 'CredentialsAuthExecutor')
      .map((node) => ({name: `${template.type} ${node.id}`, node, nodes}));
  });

describe('sign-in templates', () => {
  it('have password checks', () => {
    expect(passwordChecks.length).toBeGreaterThan(0);
  });

  // A wrong password leaves the check incomplete. Without a way back to the form, the sign-in ends there.
  it.each(passwordChecks)('$name sends a wrong password back to the form that asked for it', ({node, nodes}) => {
    const form = nodes.find((candidate) => candidate.id === node.onIncomplete);

    expect(form?.type).toBe('PROMPT');
    expect(
      form?.prompts?.some(
        (prompt) =>
          prompt.action?.nextNode === node.id && prompt.inputs?.some((input) => input.type === 'PASSWORD_INPUT'),
      ),
    ).toBe(true);
  });
});
