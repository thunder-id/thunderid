// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import getAssignmentsOffRules from '../getAssignmentsOffRules';

const off = {assignments: {editable: false}};

describe('getAssignmentsOffRules', () => {
  it('finds nothing in sections without assignments rules', () => {
    expect(
      getAssignmentsOffRules({
        childUnits: {mode: 'all'},
        otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind', overlayRules: {assignments: {editable: true}}}]},
      }),
    ).toEqual({everyUnit: false, allChildUnits: false, everyTree: false, ouIds: []});
  });

  it('finds the sections and named units that turn assignments off', () => {
    expect(getAssignmentsOffRules({allUnits: {overlayRules: off}})).toEqual({
      everyUnit: true,
      allChildUnits: false,
      everyTree: false,
      ouIds: [],
    });
    expect(
      getAssignmentsOffRules({
        childUnits: {
          mode: 'selected',
          units: [
            {ouId: 'retail', reach: 'unit', overlayRules: off},
            {ouId: 'trials', reach: 'subtree'},
          ],
        },
        otherTreeRoots: {mode: 'all', overlayRules: off},
      }),
    ).toEqual({everyUnit: false, allChildUnits: false, everyTree: true, ouIds: ['retail']});
    expect(getAssignmentsOffRules({childUnits: {mode: 'all', overlayRules: off}}).allChildUnits).toBe(true);
  });
});
