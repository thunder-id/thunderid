// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {OrganizationAccessDraft} from '../../models/organization-access';
import type {SharingPolicy} from '../../models/sharing-policy';
import draftToSharingSections, {hasNoSharingSections} from '../draftToSharingSections';
import sharingPolicyToDraft, {EMPTY_ORGANIZATION_ACCESS_DRAFT} from '../sharingPolicyToDraft';

const chosen = (overrides: Partial<OrganizationAccessDraft>): OrganizationAccessDraft => ({
  ...EMPTY_ORGANIZATION_ACCESS_DRAFT,
  scope: 'chosen',
  ...overrides,
});

describe('draftToSharingSections', () => {
  it('writes owner-only access as no sections', () => {
    expect(draftToSharingSections({...EMPTY_ORGANIZATION_ACCESS_DRAFT, trees: ['northwind']})).toEqual({});
  });

  it('writes chosen units with nothing granted as no sections', () => {
    expect(draftToSharingSections(chosen({}))).toEqual({});
  });

  it('leaves out a selected section with nothing picked yet', () => {
    expect(
      draftToSharingSections(chosen({childUnitsAccess: 'selected', otherTreesAccess: 'selected', trees: []})),
    ).toEqual({});
  });

  it('writes all child units with exclusions and selected trees', () => {
    expect(
      draftToSharingSections(
        chosen({
          childUnitsAccess: 'all',
          childUnitExclusions: ['trials'],
          otherTreesAccess: 'selected',
          trees: ['northwind'],
        }),
      ),
    ).toEqual({
      childUnits: {mode: 'all', excludedOuIds: ['trials']},
      otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]},
    });
  });

  it('omits empty exclusions', () => {
    expect(draftToSharingSections(chosen({childUnitsAccess: 'all'}))).toEqual({childUnits: {mode: 'all'}});
  });

  it('writes selected units with their reach', () => {
    expect(
      draftToSharingSections(
        chosen({
          childUnitsAccess: 'selected',
          childUnits: [
            {ouId: 'wholesale', reach: 'unit'},
            {ouId: 'retail', reach: 'subtree'},
          ],
        }),
      ),
    ).toEqual({
      childUnits: {
        mode: 'selected',
        units: [
          {ouId: 'wholesale', reach: 'unit'},
          {ouId: 'retail', reach: 'subtree'},
        ],
      },
    });
  });

  it('writes other trees on their own, with no child units', () => {
    expect(draftToSharingSections(chosen({otherTreesAccess: 'selected', trees: ['northwind']}))).toEqual({
      otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind'}]},
    });
  });

  it('writes every tree instead of the selected trees', () => {
    expect(
      draftToSharingSections(
        chosen({otherTreesAccess: 'every', trees: ['northwind'], everyTreeExclusions: ['contoso']}),
      ),
    ).toEqual({otherTreeRoots: {mode: 'all', excludedOuIds: ['contoso']}});
  });

  it('writes the every organization unit scope alone', () => {
    expect(
      draftToSharingSections({
        ...EMPTY_ORGANIZATION_ACCESS_DRAFT,
        scope: 'everyUnit',
        everyUnitExclusions: ['sandbox'],
        childUnitsAccess: 'all',
      }),
    ).toEqual({allUnits: {excludedOuIds: ['sandbox']}});
  });

  it('round-trips a policy through the draft', () => {
    const policy: SharingPolicy = {
      id: 'p1',
      stage: 'share',
      version: 3,
      childUnits: {mode: 'all', excludedOuIds: ['trials']},
      otherTreeRoots: {mode: 'all', excludedOuIds: ['contoso']},
    };

    expect(draftToSharingSections(sharingPolicyToDraft(policy))).toEqual({
      childUnits: policy.childUnits,
      otherTreeRoots: policy.otherTreeRoots,
    });
  });
});

describe('hasNoSharingSections', () => {
  it('tells sections that reach nobody apart from ones that reach someone', () => {
    expect(hasNoSharingSections({})).toBe(true);
    expect(hasNoSharingSections({allUnits: {}})).toBe(false);
  });
});
