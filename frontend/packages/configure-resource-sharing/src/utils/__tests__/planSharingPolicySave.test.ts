// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {SharingPolicy, SharingPolicySections} from '../../models/sharing-policy';
import planSharingPolicySave, {canEditSharingSectionsInPlace, haveSameSharingSections} from '../planSharingPolicySave';

const policyWith = (sections: SharingPolicySections): SharingPolicy => ({
  id: 'p1',
  stage: 'share',
  version: 4,
  ...sections,
});

describe('planSharingPolicySave', () => {
  it('does nothing when an unshared resource stays unshared', () => {
    expect(planSharingPolicySave(undefined, {})).toEqual({type: 'none'});
  });

  it('creates a policy for a resource that is not shared yet', () => {
    const sections: SharingPolicySections = {childUnits: {mode: 'all'}};

    expect(planSharingPolicySave(undefined, sections)).toEqual({type: 'create', sections});
  });

  it('deletes the policy when nothing is shared any more', () => {
    expect(planSharingPolicySave(policyWith({childUnits: {mode: 'all'}}), {})).toEqual({
      type: 'delete',
      policyId: 'p1',
    });
  });

  it('does nothing when the sections are unchanged, in any order', () => {
    const current = policyWith({
      childUnits: {mode: 'all', excludedOuIds: ['b', 'a']},
      otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}, {ouId: 'contoso'}]},
    });

    expect(
      planSharingPolicySave(current, {
        otherTreeRoots: {mode: 'selected', units: [{ouId: 'contoso'}, {ouId: 'northwind'}]},
        childUnits: {mode: 'all', excludedOuIds: ['a', 'b']},
      }),
    ).toEqual({type: 'none'});
  });

  it('edits in place when only exclusions and named units change', () => {
    const sections: SharingPolicySections = {
      childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]},
      otherTreeRoots: {mode: 'all', excludedOuIds: ['contoso']},
    };

    expect(planSharingPolicySave(policyWith({otherTreeRoots: {mode: 'all'}}), sections)).toEqual({
      type: 'update',
      policyId: 'p1',
      version: 4,
      sections,
    });
  });

  it('edits in place to restate every organization unit with new exclusions', () => {
    const sections: SharingPolicySections = {allUnits: {excludedOuIds: ['sandbox']}};

    expect(planSharingPolicySave(policyWith({allUnits: {}}), sections)).toEqual({
      type: 'update',
      policyId: 'p1',
      version: 4,
      sections,
    });
  });

  it('replaces the policy when a section reaching a whole group is added', () => {
    const sections: SharingPolicySections = {childUnits: {mode: 'all'}};

    expect(
      planSharingPolicySave(
        policyWith({childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]}}),
        sections,
      ),
    ).toEqual({type: 'replace', policyId: 'p1', sections});
  });

  it('replaces the policy when leaving every organization unit', () => {
    const sections: SharingPolicySections = {childUnits: {mode: 'all'}};

    expect(planSharingPolicySave(policyWith({allUnits: {}}), sections)).toEqual({
      type: 'replace',
      policyId: 'p1',
      sections,
    });
  });
});

describe('haveSameSharingSections', () => {
  it('tells apart named units that reach differently', () => {
    expect(
      haveSameSharingSections(
        {childUnits: {mode: 'selected', units: [{ouId: 'a', reach: 'unit'}]}},
        {childUnits: {mode: 'selected', units: [{ouId: 'a', reach: 'subtree'}]}},
      ),
    ).toBe(false);
  });
});

describe('canEditSharingSectionsInPlace', () => {
  it('allows a named section to appear or go away', () => {
    expect(
      canEditSharingSectionsInPlace(
        {childUnits: {mode: 'all'}},
        {childUnits: {mode: 'all'}, otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]}},
      ),
    ).toBe(true);
    expect(
      canEditSharingSectionsInPlace(
        {childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]}, otherTreeRoots: {mode: 'all'}},
        {otherTreeRoots: {mode: 'all'}},
      ),
    ).toBe(true);
  });

  it('refuses dropping a section reaching a whole group', () => {
    expect(
      canEditSharingSectionsInPlace(
        {childUnits: {mode: 'all'}, otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]}},
        {otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]}},
      ),
    ).toBe(false);
  });

  it('refuses converting a section between naming units and reaching a whole group', () => {
    expect(
      canEditSharingSectionsInPlace(
        {childUnits: {mode: 'all'}},
        {childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]}},
      ),
    ).toBe(false);
  });
});
