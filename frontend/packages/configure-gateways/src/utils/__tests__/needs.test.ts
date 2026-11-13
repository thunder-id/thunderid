// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {Change} from '../../models/deploy';
import {withNeeded, withReplaced} from '../needs';

const changes: Change[] = [
  {
    key: 'application/app-1',
    resourceType: 'application',
    change: 'added',
    needs: ['flow/f-1', 'organization_unit/ou-2'],
  },
  {key: 'flow/f-1', resourceType: 'flow', change: 'unchanged'},
  {
    key: 'organization_unit/ou-2',
    resourceType: 'organization_unit',
    change: 'added',
    needs: ['organization_unit/ou-1'],
  },
  {key: 'organization_unit/ou-1', resourceType: 'organization_unit', change: 'updated'},
  {key: 'organization_unit/ou-9', resourceType: 'organization_unit', change: 'added'},
  {key: 'role/r-1', resourceType: 'role', change: 'deleted', needs: ['organization_unit/ou-9']},
];

describe('withNeeded', () => {
  it('brings along what a selection needs that the gateway lacks, through what those need', () => {
    const required = withNeeded(new Set(['application/app-1']), changes);

    expect([...required.entries()]).toEqual([
      ['organization_unit/ou-2', 'application/app-1'],
      ['organization_unit/ou-1', 'application/app-1'],
    ]);
  });

  it('leaves out what the gateway already holds and what is ticked by hand', () => {
    const required = withNeeded(new Set(['application/app-1', 'organization_unit/ou-2']), changes);

    expect([...required.keys()]).toEqual(['organization_unit/ou-1']);
    expect(required.has('flow/f-1')).toBe(false);
  });

  it('follows no need of a removal, and needs nothing for a selection of none', () => {
    expect(withNeeded(new Set(['role/r-1']), changes).size).toBe(0);
    expect(withNeeded(new Set(), changes).size).toBe(0);
  });
});

describe('withReplaced', () => {
  const replacing: Change[] = [
    {
      key: 'resource_server/rs-new',
      resourceType: 'resource_server',
      change: 'added',
      replaces: 'resource_server/rs-old',
    },
    {key: 'resource_server/rs-old', resourceType: 'resource_server', change: 'deleted'},
    {key: 'flow/f-new', resourceType: 'flow', change: 'added', replaces: 'flow/f-gone'},
  ];

  it('brings the removal a selected or brought addition replaces, and keeps what was brought', () => {
    expect([...withReplaced(new Set(['resource_server/rs-new']), new Map(), replacing)]).toEqual([
      ['resource_server/rs-old', 'resource_server/rs-new'],
    ]);
    const brought = new Map([['resource_server/rs-new', 'application/app-1']]);
    expect([...withReplaced(new Set(), brought, replacing)]).toEqual([
      ['resource_server/rs-new', 'application/app-1'],
      ['resource_server/rs-old', 'resource_server/rs-new'],
    ]);
  });

  it('brings nothing for an addition left out, or one whose replaced removal is not offered', () => {
    expect(withReplaced(new Set(), new Map(), replacing).size).toBe(0);
    expect(withReplaced(new Set(['flow/f-new']), new Map(), replacing).size).toBe(0);
  });

  it.each([
    ['held back', {heldBack: true}],
    ['not run by the source', {sourceExcluded: true}],
  ])('brings no replaced removal that is %s', (_, flag: Partial<Change>) => {
    const held = replacing.map((change: Change) =>
      change.key === 'resource_server/rs-old' ? {...change, ...flag} : change,
    );

    expect(withReplaced(new Set(['resource_server/rs-new']), new Map(), held).size).toBe(0);
  });
});
