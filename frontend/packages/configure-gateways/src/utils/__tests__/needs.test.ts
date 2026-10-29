// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {Change} from '../../models/deploy';
import {withNeeded} from '../needs';

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
