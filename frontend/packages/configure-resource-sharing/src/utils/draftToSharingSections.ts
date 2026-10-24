// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationAccessDraft} from '../models/organization-access';
import type {SharingPolicySections} from '../models/sharing-policy';

function withExclusions<T extends object>(section: T, excluded: string[]): T & {excludedOuIds?: string[]} {
  return excluded.length > 0 ? {...section, excludedOuIds: [...excluded]} : section;
}

/**
 * Reports whether a set of sections reaches no organization unit, which is a resource no policy
 * shares rather than a policy that can be written.
 */
export function hasNoSharingSections(sections: SharingPolicySections): boolean {
  return !sections.allUnits && !sections.childUnits && !sections.otherTreeRoots;
}

/**
 * Writes an Organization Access draft as the sections of a sharing policy.
 *
 * A section that grants nothing is left out, so an empty result means the resource is not shared at
 * all and its policy has to go rather than be written. That is the case for the `owner` scope, and
 * for a `chosen` scope whose child-unit and other-tree sections grant nothing.
 *
 * @param draft - The draft to write
 * @returns The policy sections the draft amounts to
 */
export default function draftToSharingSections(draft: OrganizationAccessDraft): SharingPolicySections {
  if (draft.scope === 'owner') {
    return {};
  }
  if (draft.scope === 'everyUnit') {
    return {allUnits: withExclusions({}, draft.everyUnitExclusions)};
  }

  const sections: SharingPolicySections = {};

  if (draft.childUnitsAccess === 'all') {
    sections.childUnits = withExclusions({mode: 'all' as const}, draft.childUnitExclusions);
  }
  if (draft.childUnitsAccess === 'selected' && draft.childUnits.length > 0) {
    sections.childUnits = {
      mode: 'selected',
      units: draft.childUnits.map((unit) => ({ouId: unit.ouId, reach: unit.reach})),
    };
  }

  if (draft.otherTreesAccess === 'every') {
    sections.otherTreeRoots = withExclusions({mode: 'all' as const}, draft.everyTreeExclusions);
  }
  if (draft.otherTreesAccess === 'selected' && draft.trees.length > 0) {
    sections.otherTreeRoots = {mode: 'selected', units: draft.trees.map((ouId) => ({ouId}))};
  }

  return sections;
}
