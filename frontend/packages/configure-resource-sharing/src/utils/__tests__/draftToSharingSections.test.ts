// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {OrganizationAccessDraft} from '../../models/organization-access';
import type {SharingPolicy} from '../../models/sharing-policy';
import draftToSharingSections, {hasNoSharingSections} from '../draftToSharingSections';
import sharingPolicyToDraft, {EMPTY_ORGANIZATION_ACCESS_DRAFT} from '../sharingPolicyToDraft';

const editsTerms = (field: string): boolean => field.startsWith('terms.');

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

  it('writes the terms of the units reached into all child units', () => {
    const childUnitRules = {'terms.region': {editable: true, allowedValues: ['c-1']}};

    expect(draftToSharingSections(chosen({childUnitsAccess: 'all', childUnitRules}), undefined, editsTerms)).toEqual({
      childUnits: {mode: 'all', overlayRules: childUnitRules},
    });
  });

  it('writes each selected child unit with the terms of its own', () => {
    const retailRules = {'terms.region': {editable: false, value: ['c-1']}};

    expect(
      draftToSharingSections(
        chosen({
          childUnitsAccess: 'selected',
          childUnits: [
            {ouId: 'retail', reach: 'unit', rules: retailRules},
            {ouId: 'banking', reach: 'subtree'},
          ],
        }),
      ),
    ).toEqual({
      childUnits: {
        mode: 'selected',
        units: [
          {ouId: 'retail', reach: 'unit', overlayRules: retailRules},
          {ouId: 'banking', reach: 'subtree'},
        ],
      },
    });
  });

  it('leaves out terms when there are none', () => {
    expect(draftToSharingSections(chosen({childUnitsAccess: 'all', childUnitRules: {}}))).toEqual({
      childUnits: {mode: 'all'},
    });
  });
});

describe('hasNoSharingSections', () => {
  it('tells sections that reach nobody apart from ones that reach someone', () => {
    expect(hasNoSharingSections({})).toBe(true);
    expect(hasNoSharingSections({allUnits: {}})).toBe(false);
  });
});

describe('mandated roles', () => {
  const rule = (value: string[]) => ({roles: {editable: false, value}});

  it('writes the roles of a unit-only unit as a fixed roles rule', () => {
    expect(
      draftToSharingSections(
        chosen({
          childUnitsAccess: 'selected',
          childUnits: [
            {ouId: 'retail', reach: 'unit', roles: ['r1', 'r2']},
            {ouId: 'trials', reach: 'unit'},
          ],
        }),
      ),
    ).toEqual({
      childUnits: {
        mode: 'selected',
        units: [
          {ouId: 'retail', reach: 'unit', overlayRules: rule(['r1', 'r2'])},
          {ouId: 'trials', reach: 'unit'},
        ],
      },
    });
  });

  it('leaves the rule out for a subtree reach and for an empty list', () => {
    expect(
      draftToSharingSections(
        chosen({
          childUnitsAccess: 'selected',
          childUnits: [
            {ouId: 'retail', reach: 'subtree', roles: ['r1']},
            {ouId: 'trials', reach: 'unit', roles: []},
          ],
        }),
      ),
    ).toEqual({
      childUnits: {
        mode: 'selected',
        units: [
          {ouId: 'retail', reach: 'subtree'},
          {ouId: 'trials', reach: 'unit'},
        ],
      },
    });
  });

  it('round-trips the roles through the draft', () => {
    const policy: SharingPolicy = {
      id: 'p1',
      stage: 'share',
      version: 1,
      childUnits: {
        mode: 'selected',
        units: [
          {ouId: 'retail', reach: 'unit', overlayRules: rule(['r1', 'r2'])},
          {ouId: 'trials', reach: 'subtree'},
        ],
      },
    };

    const draft = sharingPolicyToDraft(policy);

    expect(draft.childUnits).toEqual([
      {ouId: 'retail', reach: 'unit', roles: ['r1', 'r2']},
      {ouId: 'trials', reach: 'subtree'},
    ]);
    expect(draftToSharingSections(draft)).toEqual({childUnits: policy.childUnits});
  });
});

describe('draftToSharingSections, keeping the rules the screen does not edit', () => {
  const off = {assignments: {editable: false}};
  const policyWith = (sections: Partial<SharingPolicy>): SharingPolicy => ({
    id: 'p1',
    stage: 'share',
    version: 1,
    ...sections,
  });

  it('keeps a section rule while the section stays the same kind', () => {
    const saved = policyWith({
      childUnits: {mode: 'all', excludedOuIds: ['trials'], overlayRules: off},
      otherTreeRoots: {mode: 'all', overlayRules: off},
    });
    const draft = {...sharingPolicyToDraft(saved), childUnitExclusions: []};

    expect(draftToSharingSections(draft, saved)).toEqual({
      childUnits: {mode: 'all', overlayRules: off},
      otherTreeRoots: {mode: 'all', overlayRules: off},
    });
    expect(draftToSharingSections({...draft, scope: 'everyUnit'}, policyWith({allUnits: {overlayRules: off}}))).toEqual(
      {allUnits: {overlayRules: off}},
    );
  });

  it('keeps the edited rules of the every-unit and other-tree sections, which the draft holds none of', () => {
    const terms = {'terms.region': {editable: true}};
    const saved = policyWith({
      childUnits: {mode: 'all', overlayRules: terms},
      otherTreeRoots: {mode: 'all', overlayRules: terms},
    });
    const draft = {...sharingPolicyToDraft(saved, editsTerms), childUnitRules: {'terms.region': {editable: false}}};

    // The child units take the terms the draft holds, and the other trees keep what was saved.
    expect(draftToSharingSections(draft, saved, editsTerms)).toEqual({
      childUnits: {mode: 'all', overlayRules: {'terms.region': {editable: false}}},
      otherTreeRoots: {mode: 'all', overlayRules: terms},
    });
    expect(
      draftToSharingSections({...draft, scope: 'everyUnit'}, policyWith({allUnits: {overlayRules: terms}})),
    ).toEqual({allUnits: {overlayRules: terms}});
  });

  it('keeps every rule of a child-unit section when the resource edits none', () => {
    const terms = {'terms.region': {editable: true}};
    const saved = policyWith({childUnits: {mode: 'all', overlayRules: terms}});

    expect(draftToSharingSections(sharingPolicyToDraft(saved), saved)).toEqual({
      childUnits: {mode: 'all', overlayRules: terms},
    });
  });

  it('does not bring back a term removed from the draft of all child units, and keeps the rules it does not edit', () => {
    const off = {assignments: {editable: false}};
    const saved = policyWith({
      childUnits: {
        mode: 'all',
        overlayRules: {...off, 'terms.region': {editable: true}, 'terms.tier': {editable: false, value: ['gold']}},
      },
    });
    const draft = {...sharingPolicyToDraft(saved, editsTerms), childUnitRules: {'terms.region': {editable: true}}};

    // One term is gone from the draft and does not return from the saved policy; the rule the screen doesn't edit stays.
    expect(draftToSharingSections(draft, saved, editsTerms)).toEqual({
      childUnits: {mode: 'all', overlayRules: {...off, 'terms.region': {editable: true}}},
    });
    expect(draftToSharingSections({...draft, childUnitRules: {}}, saved, editsTerms)).toEqual({
      childUnits: {mode: 'all', overlayRules: off},
    });
  });

  it('writes no terms for a selected unit whose terms were removed, and leaves the saved ones out of its rules', () => {
    const saved = policyWith({
      childUnits: {
        mode: 'selected',
        units: [
          {ouId: 'retail', reach: 'subtree', overlayRules: {...off, 'terms.region': {editable: true}}},
          {ouId: 'banking', reach: 'subtree', overlayRules: {'terms.region': {editable: false, value: ['eu']}}},
        ],
      },
    });
    const draft = sharingPolicyToDraft(saved, editsTerms);
    const withoutTerms = {
      ...draft,
      childUnits: draft.childUnits.map(({rules: _rules, ...unit}) => {
        void _rules;
        return unit;
      }),
    };

    expect(draftToSharingSections(withoutTerms, saved, editsTerms)).toEqual({
      childUnits: {
        mode: 'selected',
        units: [
          {ouId: 'retail', reach: 'subtree', overlayRules: off},
          {ouId: 'banking', reach: 'subtree'},
        ],
      },
    });
  });

  it('keeps a named unit rule while the unit stays, whatever its reach, and not for units added later', () => {
    const saved = policyWith({
      childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit', overlayRules: off}]},
      otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind', overlayRules: off}]},
    });
    const draft = chosen({
      childUnitsAccess: 'selected',
      childUnits: [
        {ouId: 'retail', reach: 'subtree'},
        {ouId: 'trials', reach: 'unit'},
      ],
      otherTreesAccess: 'selected',
      trees: ['northwind', 'contoso'],
    });

    expect(draftToSharingSections(draft, saved)).toEqual({
      childUnits: {
        mode: 'selected',
        units: [
          {ouId: 'retail', reach: 'subtree', overlayRules: off},
          {ouId: 'trials', reach: 'unit'},
        ],
      },
      otherTreeRoots: {mode: 'selected', units: [{ouId: 'northwind', overlayRules: off}, {ouId: 'contoso'}]},
    });
  });

  it('drops a section rule when the section changes kind', () => {
    const saved = policyWith({childUnits: {mode: 'all', overlayRules: off}});

    expect(
      draftToSharingSections(
        chosen({childUnitsAccess: 'selected', childUnits: [{ouId: 'retail', reach: 'unit'}]}),
        saved,
      ),
    ).toEqual({childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]}});
  });

  it('writes mandated roles from the draft rather than the saved policy', () => {
    const saved = policyWith({
      childUnits: {
        mode: 'selected',
        units: [{ouId: 'retail', reach: 'unit', overlayRules: {roles: {editable: false, value: ['viewer']}}}],
      },
    });

    expect(
      draftToSharingSections(
        chosen({childUnitsAccess: 'selected', childUnits: [{ouId: 'retail', reach: 'unit', roles: []}]}),
        saved,
      ),
    ).toEqual({childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]}});
  });
});
