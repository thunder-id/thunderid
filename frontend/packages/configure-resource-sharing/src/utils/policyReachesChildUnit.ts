// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SharingPolicySections} from '../models/sharing-policy';

/**
 * Whether a policy its issuer states reaches one of the issuer's direct children. A direct child is
 * reached only by the issuer's own policy, so its sections decide: every unit or every child unit
 * that does not withhold the child, or a named child unit.
 *
 * @param policy - The sections of a policy the child's parent issued
 * @param childOuId - A direct child of the policy's issuer
 * @returns Whether the policy reaches the child
 *
 * @public
 */
export default function policyReachesChildUnit(policy: SharingPolicySections, childOuId: string): boolean {
  if (policy.allUnits && !policy.allUnits.excludedOuIds?.includes(childOuId)) {
    return true;
  }

  const {childUnits} = policy;
  if (!childUnits) {
    return false;
  }
  if (childUnits.mode === 'all') {
    return !childUnits.excludedOuIds?.includes(childOuId);
  }
  return childUnits.units.some((unit) => unit.ouId === childOuId);
}
