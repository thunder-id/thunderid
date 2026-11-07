// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AssignmentsOffRules} from '../models/organization-access';
import type {SharingOverlayRules, SharingPolicySections} from '../models/sharing-policy';

const isOff = (rules: SharingOverlayRules | undefined): boolean => rules?.assignments?.editable === false;

/**
 * Reports where a set of sections turns assignments off, the rule a role's policy can carry.
 *
 * @param sections - The sections to read
 * @returns The sections and named units whose organization units can't assign the role
 */
export default function getAssignmentsOffRules(sections: SharingPolicySections): AssignmentsOffRules {
  const {allUnits, childUnits, otherTreeRoots} = sections;
  const namedUnits = [
    ...(childUnits?.mode === 'selected' ? childUnits.units : []),
    ...(otherTreeRoots?.mode === 'selected' ? otherTreeRoots.units : []),
  ];

  return {
    everyUnit: isOff(allUnits?.overlayRules),
    allChildUnits: childUnits?.mode === 'all' && isOff(childUnits.overlayRules),
    everyTree: otherTreeRoots?.mode === 'all' && isOff(otherTreeRoots.overlayRules),
    ouIds: namedUnits.filter((unit) => isOff(unit.overlayRules)).map((unit) => unit.ouId),
  };
}
