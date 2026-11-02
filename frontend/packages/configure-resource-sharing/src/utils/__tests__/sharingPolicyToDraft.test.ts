// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {SharingPolicy, SharingPolicySections} from '../../models/sharing-policy';
import sharingPolicyToDraft, {EMPTY_ORGANIZATION_ACCESS_DRAFT} from '../sharingPolicyToDraft';

const policyWith = (sections: SharingPolicySections): SharingPolicy => ({
  id: 'p1',
  stage: 'share',
  version: 1,
  ...sections,
});

describe('sharingPolicyToDraft', () => {
  it('reads a missing policy as owner-only access', () => {
    expect(sharingPolicyToDraft(undefined)).toEqual(EMPTY_ORGANIZATION_ACCESS_DRAFT);
  });

  it('reads a policy with no sections as owner-only access', () => {
    expect(sharingPolicyToDraft(policyWith({}))).toEqual(EMPTY_ORGANIZATION_ACCESS_DRAFT);
  });

  it('reads all child units with their exclusions', () => {
    const draft = sharingPolicyToDraft(policyWith({childUnits: {mode: 'all', excludedOuIds: ['trials']}}));

    expect(draft.scope).toBe('chosen');
    expect(draft.childUnitsAccess).toBe('all');
    expect(draft.childUnitExclusions).toEqual(['trials']);
    expect(draft.otherTreesAccess).toBe('none');
  });

  it('reads selected child units with their reach', () => {
    const draft = sharingPolicyToDraft(
      policyWith({
        childUnits: {
          mode: 'selected',
          units: [
            {ouId: 'wholesale', reach: 'unit'},
            {ouId: 'retail', reach: 'subtree'},
          ],
        },
      }),
    );

    expect(draft.childUnitsAccess).toBe('selected');
    expect(draft.childUnits).toEqual([
      {ouId: 'wholesale', reach: 'unit'},
      {ouId: 'retail', reach: 'subtree'},
    ]);
  });

  it('reads a policy reaching only other trees as no child units beside selected trees', () => {
    const draft = sharingPolicyToDraft(policyWith({otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]}}));

    expect(draft.scope).toBe('chosen');
    expect(draft.childUnitsAccess).toBe('none');
    expect(draft.otherTreesAccess).toBe('selected');
    expect(draft.trees).toEqual(['northwind']);
  });

  it('reads all units as the every organization unit scope', () => {
    const draft = sharingPolicyToDraft(policyWith({allUnits: {excludedOuIds: ['sandbox']}}));

    expect(draft.scope).toBe('everyUnit');
    expect(draft.everyUnitExclusions).toEqual(['sandbox']);
  });

  it('reads every other tree independently of the child units', () => {
    const draft = sharingPolicyToDraft(
      policyWith({childUnits: {mode: 'all'}, otherTreeRoots: {mode: 'all', excludedOuIds: ['contoso']}}),
    );

    expect(draft.childUnitsAccess).toBe('all');
    expect(draft.otherTreesAccess).toBe('every');
    expect(draft.everyTreeExclusions).toEqual(['contoso']);
  });

  it('does not share list instances between drafts', () => {
    const first = sharingPolicyToDraft(undefined);
    first.trees.push('x');

    expect(sharingPolicyToDraft(undefined).trees).toEqual([]);
  });
});
