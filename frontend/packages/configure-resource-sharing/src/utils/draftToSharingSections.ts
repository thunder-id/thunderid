// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationAccessDraft} from '../models/organization-access';
import type {SharingOverlayRules, SharingPolicySections} from '../models/sharing-policy';

function withExclusions<T extends object>(section: T, excluded: string[]): T & {excludedOuIds?: string[]} {
  return excluded.length > 0 ? {...section, excludedOuIds: [...excluded]} : section;
}

/**
 * The rules of a saved section or unit that the screen doesn't edit, which a save keeps as they are.
 * The screen edits only a user's mandated `roles`, so everything else, such as a role's
 * `assignments`, is kept.
 */
function keptOverlayRules(rules: SharingOverlayRules | undefined): SharingOverlayRules {
  const {roles: _roles, ...kept} = rules ?? {};
  void _roles;
  return kept;
}

function withOverlayRules<T extends object>(
  target: T,
  rules: SharingOverlayRules,
): T & {overlayRules?: SharingOverlayRules} {
  return Object.keys(rules).length > 0 ? {...target, overlayRules: rules} : target;
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
 * Rules the screen doesn't edit come from `saved`: a section keeps them while it stays the same kind,
 * and a named unit keeps them while the draft still names it.
 *
 * @param draft - The draft to write
 * @param saved - The policy the draft started from, whose rules the screen doesn't edit are kept
 * @returns The policy sections the draft amounts to
 */
export default function draftToSharingSections(
  draft: OrganizationAccessDraft,
  saved?: SharingPolicySections,
): SharingPolicySections {
  if (draft.scope === 'owner') {
    return {};
  }
  if (draft.scope === 'everyUnit') {
    return {
      allUnits: withOverlayRules(
        withExclusions({}, draft.everyUnitExclusions),
        keptOverlayRules(saved?.allUnits?.overlayRules),
      ),
    };
  }

  const sections: SharingPolicySections = {};
  const savedChildUnits = saved?.childUnits;
  const savedTrees = saved?.otherTreeRoots;

  if (draft.childUnitsAccess === 'all') {
    sections.childUnits = withOverlayRules(
      withExclusions({mode: 'all' as const}, draft.childUnitExclusions),
      keptOverlayRules(savedChildUnits?.mode === 'all' ? savedChildUnits.overlayRules : undefined),
    );
  }
  if (draft.childUnitsAccess === 'selected' && draft.childUnits.length > 0) {
    sections.childUnits = {
      mode: 'selected',
      units: draft.childUnits.map((unit) => {
        const savedUnit =
          savedChildUnits?.mode === 'selected'
            ? savedChildUnits.units.find((candidate) => candidate.ouId === unit.ouId)
            : undefined;
        const rules = keptOverlayRules(savedUnit?.overlayRules);
        // The server refuses a roles rule on a subtree, so one is only written for a unit-only reach.
        if (unit.reach === 'unit' && unit.roles && unit.roles.length > 0) {
          rules.roles = {editable: false, value: [...unit.roles]};
        }
        return withOverlayRules({ouId: unit.ouId, reach: unit.reach}, rules);
      }),
    };
  }

  if (draft.otherTreesAccess === 'every') {
    sections.otherTreeRoots = withOverlayRules(
      withExclusions({mode: 'all' as const}, draft.everyTreeExclusions),
      keptOverlayRules(savedTrees?.mode === 'all' ? savedTrees.overlayRules : undefined),
    );
  }
  if (draft.otherTreesAccess === 'selected' && draft.trees.length > 0) {
    sections.otherTreeRoots = {
      mode: 'selected',
      units: draft.trees.map((ouId) =>
        withOverlayRules(
          {ouId},
          keptOverlayRules(
            savedTrees?.mode === 'selected'
              ? savedTrees.units.find((candidate) => candidate.ouId === ouId)?.overlayRules
              : undefined,
          ),
        ),
      ),
    };
  }

  return sections;
}
