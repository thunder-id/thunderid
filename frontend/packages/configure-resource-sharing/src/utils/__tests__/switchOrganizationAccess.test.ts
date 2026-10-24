// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {SharingPolicy, SharingPolicySections} from '../../models/sharing-policy';
import sharingPolicyToDraft from '../sharingPolicyToDraft';
import switchOrganizationAccess, {getBlockedNoneReason} from '../switchOrganizationAccess';

const policyWith = (sections: SharingPolicySections): SharingPolicy => ({
  id: 'p1',
  stage: 'share',
  version: 1,
  ...sections,
});

describe('switchOrganizationAccess', () => {
  it('clears the exceptions when leaving all child units', () => {
    const saved = policyWith({childUnits: {mode: 'all', excludedOuIds: ['trials']}});
    const result = switchOrganizationAccess(sharingPolicyToDraft(saved), 'childUnitsAccess', 'selected', saved);

    expect(result.next.childUnitsAccess).toBe('selected');
    expect(result.next.childUnitExclusions).toEqual([]);
    expect(result.cleared).toEqual([{kind: 'childUnitExclusions', ouIds: ['trials']}]);
    expect(result.losesEveryTree).toBe(false);
  });

  it('clears the granted units when leaving selected child units, and keeps the trees', () => {
    const saved = policyWith({
      childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]},
      otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]},
    });
    const result = switchOrganizationAccess(sharingPolicyToDraft(saved), 'childUnitsAccess', 'none', saved);

    expect(result.next.childUnits).toEqual([]);
    expect(result.next.trees).toEqual(['northwind']);
    expect(result.cleared).toEqual([{kind: 'childUnits', ouIds: ['retail']}]);
  });

  it('clears the trees when leaving selected trees, and keeps the child units', () => {
    const saved = policyWith({
      childUnits: {mode: 'all'},
      otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]},
    });
    const result = switchOrganizationAccess(sharingPolicyToDraft(saved), 'otherTreesAccess', 'none', saved);

    expect(result.next.childUnitsAccess).toBe('all');
    expect(result.cleared).toEqual([{kind: 'trees', ouIds: ['northwind']}]);
  });

  it('clears both sections when stopping sharing', () => {
    const saved = policyWith({
      childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]},
      otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]},
    });
    const result = switchOrganizationAccess(sharingPolicyToDraft(saved), 'scope', 'owner', saved);

    expect(result.next.scope).toBe('owner');
    expect(result.next.childUnitsAccess).toBe('none');
    expect(result.next.otherTreesAccess).toBe('none');
    expect(result.cleared).toEqual([
      {kind: 'childUnits', ouIds: ['retail']},
      {kind: 'trees', ouIds: ['northwind']},
    ]);
  });

  it('starts chosen units at all child units, clearing nothing, for an unshared resource', () => {
    const result = switchOrganizationAccess(sharingPolicyToDraft(undefined), 'scope', 'chosen', undefined);

    expect(result.next.scope).toBe('chosen');
    expect(result.next.childUnitsAccess).toBe('all');
    expect(result.next.otherTreesAccess).toBe('none');
    expect(result.cleared).toEqual([]);
  });

  it('starts at all child units when changing the scope from every organization unit', () => {
    const saved = policyWith({allUnits: {excludedOuIds: ['sandbox']}});
    const result = switchOrganizationAccess(sharingPolicyToDraft(saved), 'scope', 'chosen', saved);

    expect(result.next.scope).toBe('chosen');
    expect(result.next.childUnitsAccess).toBe('all');
    expect(result.next.otherTreesAccess).toBe('none');
    expect(result.next.everyUnitExclusions).toEqual([]);
    expect(result.losesEveryUnit).toBe(true);
    expect(result.cleared).toEqual([{kind: 'everyUnit', ouIds: ['sandbox']}]);
  });

  it('drops every tree when leaving all child units changes a section reaching a whole group', () => {
    const saved = policyWith({childUnits: {mode: 'all'}, otherTreeRoots: {mode: 'all', excludedOuIds: ['contoso']}});
    const result = switchOrganizationAccess(sharingPolicyToDraft(saved), 'childUnitsAccess', 'selected', saved);

    expect(result.next.otherTreesAccess).toBe('none');
    expect(result.losesEveryTree).toBe(true);
    expect(result.cleared).toContainEqual({kind: 'everyTree', ouIds: ['contoso']});
  });

  it('drops every tree when adding all child units beside it', () => {
    const saved = policyWith({
      childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]},
      otherTreeRoots: {mode: 'all'},
    });
    const result = switchOrganizationAccess(sharingPolicyToDraft(saved), 'childUnitsAccess', 'all', saved);

    expect(result.losesEveryTree).toBe(true);
  });

  it('keeps every tree when the child units switch between named choices', () => {
    const saved = policyWith({
      childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]},
      otherTreeRoots: {mode: 'all'},
    });
    const result = switchOrganizationAccess(sharingPolicyToDraft(saved), 'childUnitsAccess', 'none', saved);

    expect(result.next.otherTreesAccess).toBe('every');
    expect(result.losesEveryTree).toBe(false);
  });

  it('clears the every-tree exceptions when picking trees instead', () => {
    const saved = policyWith({childUnits: {mode: 'all'}, otherTreeRoots: {mode: 'all', excludedOuIds: ['contoso']}});
    const result = switchOrganizationAccess(sharingPolicyToDraft(saved), 'otherTreesAccess', 'selected', saved);

    expect(result.next.everyTreeExclusions).toEqual([]);
    expect(result.cleared).toEqual([{kind: 'everyTree', ouIds: ['contoso']}]);
  });
});

describe('getBlockedNoneReason', () => {
  it('blocks None in one section while the other section is None', () => {
    const saved = policyWith({childUnits: {mode: 'all'}});
    const draft = sharingPolicyToDraft(saved);

    expect(getBlockedNoneReason(draft, 'childUnitsAccess', saved)).toBe('otherIsNone');
    expect(getBlockedNoneReason(draft, 'otherTreesAccess', saved)).toBeNull();
  });

  it('blocks None for child units when it would also drop every tree', () => {
    const saved = policyWith({childUnits: {mode: 'all'}, otherTreeRoots: {mode: 'all'}});

    expect(getBlockedNoneReason(sharingPolicyToDraft(saved), 'childUnitsAccess', saved)).toBe('dropsEveryTree');
  });

  it('allows None in either section while both grant something', () => {
    const saved = policyWith({
      childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]},
      otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]},
    });
    const draft = sharingPolicyToDraft(saved);

    expect(getBlockedNoneReason(draft, 'childUnitsAccess', saved)).toBeNull();
    expect(getBlockedNoneReason(draft, 'otherTreesAccess', saved)).toBeNull();
  });

  it('blocks nothing outside the chosen scope', () => {
    expect(getBlockedNoneReason(sharingPolicyToDraft(undefined), 'childUnitsAccess', undefined)).toBeNull();
  });
});
