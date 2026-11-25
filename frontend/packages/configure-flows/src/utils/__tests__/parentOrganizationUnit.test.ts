// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {FlowType} from '../../models/flows';
import type {FlowNode} from '../../models/responses';
import {fillParentOrganizationUnit, PARENT_OU_ID_PLACEHOLDER} from '../parentOrganizationUnit';

const resolver = (properties: Record<string, unknown>, id = 'resolve_ou'): FlowNode =>
  ({id, type: 'TASK_EXECUTION', executor: {name: 'OUResolverExecutor'}, properties}) as unknown as FlowNode;

describe('fillParentOrganizationUnit', () => {
  it.each([[''], [undefined], [PARENT_OU_ID_PLACEHOLDER]])(
    'sets an unset parent (%s) of a prompting Resolve OU step in a sign-in flow',
    (parentOuId) => {
      const nodes = [resolver({resolveFrom: 'prompt', parentOuId, directChildrenOnly: true})];

      expect(fillParentOrganizationUnit(nodes, FlowType.AUTHENTICATION, 'ou-1')).toEqual([
        resolver({resolveFrom: 'prompt', parentOuId: 'ou-1', directChildrenOnly: true}),
      ]);
    },
  );

  it('keeps a parent the template or the designer already set', () => {
    const nodes = [resolver({resolveFrom: 'prompt', parentOuId: 'ou-chosen'})];

    expect(fillParentOrganizationUnit(nodes, FlowType.AUTHENTICATION, 'ou-1')).toEqual(nodes);
  });

  it('leaves steps that take no parent alone: other strategies, other flow types and other executors', () => {
    const nodes = [
      resolver({resolveFrom: 'promptAll'}, 'all'),
      resolver({resolveFrom: 'caller'}, 'caller'),
      {id: 'other', type: 'TASK_EXECUTION', executor: {name: 'OUExecutor'}, properties: {parentOuId: ''}},
    ] as unknown as FlowNode[];

    expect(fillParentOrganizationUnit(nodes, FlowType.AUTHENTICATION, 'ou-1')).toEqual(nodes);
    expect(
      fillParentOrganizationUnit([resolver({resolveFrom: 'prompt', parentOuId: ''})], FlowType.REGISTRATION, 'ou-1'),
    ).toEqual([resolver({resolveFrom: 'prompt', parentOuId: ''})]);
  });
});
