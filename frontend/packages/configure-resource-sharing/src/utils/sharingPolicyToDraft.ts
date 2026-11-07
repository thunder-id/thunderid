// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {hasNoSharingSections} from './draftToSharingSections';
import type {OrganizationAccessDraft} from '../models/organization-access';
import type {OverlayRules, SharingOverlayRules, SharingPolicy} from '../models/sharing-policy';

/**
 * The terms a policy sets for a flow's variables, which the screen edits, or `undefined` when it sets none.
 */
function variableRulesOf(rules: SharingOverlayRules | undefined): OverlayRules | undefined {
  const entries = Object.entries(rules ?? {}).filter(([field]) => field.startsWith('variables.'));
  return entries.length > 0 ? Object.fromEntries(entries) : undefined;
}

/**
 * The draft of a resource no policy shares: only its owner can use it.
 */
export const EMPTY_ORGANIZATION_ACCESS_DRAFT: OrganizationAccessDraft = {
  scope: 'owner',
  childUnitsAccess: 'none',
  childUnitExclusions: [],
  childUnits: [],
  everyUnitExclusions: [],
  otherTreesAccess: 'none',
  trees: [],
  everyTreeExclusions: [],
};

/**
 * Reads a resource's owner-issued sharing policy into the form the Organization Access screen edits.
 *
 * Child units and other trees are read independently, so a policy that reaches only other trees
 * reads as no child units beside the trees it names.
 *
 * @param policy - The owner's policy, or `undefined` when the resource is not shared
 * @returns The draft the policy amounts to
 */
export default function sharingPolicyToDraft(policy: SharingPolicy | undefined): OrganizationAccessDraft {
  const draft: OrganizationAccessDraft = {
    ...EMPTY_ORGANIZATION_ACCESS_DRAFT,
    childUnitExclusions: [],
    childUnits: [],
    everyUnitExclusions: [],
    trees: [],
    everyTreeExclusions: [],
  };

  if (!policy || hasNoSharingSections(policy)) {
    return draft;
  }

  if (policy.allUnits) {
    draft.scope = 'everyUnit';
    draft.everyUnitExclusions = [...(policy.allUnits.excludedOuIds ?? [])];
    return draft;
  }

  draft.scope = 'chosen';

  const {childUnits, otherTreeRoots} = policy;
  if (childUnits?.mode === 'all') {
    draft.childUnitsAccess = 'all';
    draft.childUnitExclusions = [...(childUnits.excludedOuIds ?? [])];
    const variableRules = variableRulesOf(childUnits.overlayRules);
    if (variableRules) draft.childUnitRules = variableRules;
  } else if (childUnits?.mode === 'selected') {
    draft.childUnitsAccess = 'selected';
    draft.childUnits = childUnits.units.map((unit) => ({
      ouId: unit.ouId,
      reach: unit.reach,
      ...(unit.overlayRules?.roles ? {roles: [...unit.overlayRules.roles.value]} : {}),
      ...(variableRulesOf(unit.overlayRules) ? {rules: variableRulesOf(unit.overlayRules)} : {}),
    }));
  }

  if (otherTreeRoots?.mode === 'all') {
    draft.otherTreesAccess = 'every';
    draft.everyTreeExclusions = [...(otherTreeRoots.excludedOuIds ?? [])];
  } else if (otherTreeRoots?.mode === 'selected') {
    draft.otherTreesAccess = 'selected';
    draft.trees = otherTreeRoots.units.map((unit) => unit.ouId);
  }

  return draft;
}
