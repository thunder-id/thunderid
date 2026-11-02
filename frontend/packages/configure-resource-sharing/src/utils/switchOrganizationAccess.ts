// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import draftToSharingSections from './draftToSharingSections';
import {canEditSharingSectionsInPlace} from './planSharingPolicySave';
import type {AccessSwitch, ClearedAccessItem, OrganizationAccessDraft} from '../models/organization-access';
import type {SharingPolicy} from '../models/sharing-policy';

function clearChildUnits(draft: OrganizationAccessDraft): void {
  draft.childUnitsAccess = 'none';
  draft.childUnitExclusions = [];
  draft.childUnits = [];
}

function clearOtherTrees(draft: OrganizationAccessDraft): void {
  draft.otherTreesAccess = 'none';
  draft.trees = [];
  draft.everyTreeExclusions = [];
}

/**
 * Applies a switch between Organization Access choices and reports what it clears.
 *
 * Each choice owns its own settings, so they never carry over: leaving `all` child units clears
 * its exceptions, leaving `selected` clears the granted units, and switching the scope clears both
 * sections. Entering the `chosen` scope starts at all child units, so it grants something from the
 * start.
 *
 * "Every tree" can only be kept by editing the saved policy in place. When the switch changes which
 * sections reaching a whole group the policy holds, saving has to replace the policy, and a replacement cannot carry
 * a deployment-wide target, so "Every tree" is dropped with it.
 *
 * @param draft - The draft before the switch
 * @param field - The choice being switched
 * @param to - The new value of that choice
 * @param saved - The policy as last read, against which an in-place edit is judged
 * @returns The draft after the switch and what it cleared
 */
export default function switchOrganizationAccess(
  draft: OrganizationAccessDraft,
  field: AccessSwitch['field'],
  to: AccessSwitch['to'],
  saved: SharingPolicy | undefined,
): AccessSwitch {
  const next: OrganizationAccessDraft = {...draft, [field]: to};

  if (field === 'scope') {
    if (draft.scope === 'everyUnit') next.everyUnitExclusions = [];
    clearChildUnits(next);
    clearOtherTrees(next);
    if (to === 'chosen') next.childUnitsAccess = 'all';
  } else if (field === 'childUnitsAccess') {
    if (draft.childUnitsAccess === 'all') next.childUnitExclusions = [];
    if (draft.childUnitsAccess === 'selected') next.childUnits = [];
  } else {
    if (draft.otherTreesAccess === 'selected') next.trees = [];
    if (draft.otherTreesAccess === 'every') next.everyTreeExclusions = [];
  }

  if (next.otherTreesAccess === 'every' && !canEditSharingSectionsInPlace(saved ?? {}, draftToSharingSections(next))) {
    clearOtherTrees(next);
  }

  const cleared: ClearedAccessItem[] = [];
  const dropped = (before: string[], after: string[]): string[] => before.filter((id) => !after.includes(id));

  const losesEveryUnit = draft.scope === 'everyUnit' && next.scope !== 'everyUnit';
  const losesEveryTree = draft.otherTreesAccess === 'every' && next.otherTreesAccess !== 'every';

  if (losesEveryUnit) {
    cleared.push({kind: 'everyUnit', ouIds: [...draft.everyUnitExclusions]});
  }
  const droppedExclusions = dropped(draft.childUnitExclusions, next.childUnitExclusions);
  if (droppedExclusions.length > 0) {
    cleared.push({kind: 'childUnitExclusions', ouIds: droppedExclusions});
  }
  const droppedUnits = dropped(
    draft.childUnits.map((unit) => unit.ouId),
    next.childUnits.map((unit) => unit.ouId),
  );
  if (droppedUnits.length > 0) {
    cleared.push({kind: 'childUnits', ouIds: droppedUnits});
  }
  if (losesEveryTree) {
    cleared.push({kind: 'everyTree', ouIds: [...draft.everyTreeExclusions]});
  }
  const droppedTrees = dropped(draft.trees, next.trees);
  if (droppedTrees.length > 0) {
    cleared.push({kind: 'trees', ouIds: droppedTrees});
  }

  return {field, to, next, cleared, losesEveryUnit, losesEveryTree};
}

/**
 * Why picking None in a section isn't allowed. Under the `chosen` scope, the child-unit and
 * other-tree sections can't both be None: `otherIsNone` when the other section already is, and
 * `dropsEveryTree` when the switch would also drop "Every tree" and leave nothing granted.
 */
export type BlockedNoneReason = 'otherIsNone' | 'dropsEveryTree';

/**
 * Reports whether picking None in one section of the `chosen` scope would leave nothing granted.
 *
 * @param draft - The draft as it stands
 * @param field - The section whose None choice is asked about
 * @param saved - The policy as last read, against which an in-place edit is judged
 * @returns Why None can't be picked, or `null` when it can
 */
export function getBlockedNoneReason(
  draft: OrganizationAccessDraft,
  field: 'childUnitsAccess' | 'otherTreesAccess',
  saved: SharingPolicy | undefined,
): BlockedNoneReason | null {
  if (draft.scope !== 'chosen' || draft[field] === 'none') return null;

  const {next} = switchOrganizationAccess(draft, field, 'none', saved);
  if (next.childUnitsAccess !== 'none' || next.otherTreesAccess !== 'none') return null;

  const other = field === 'childUnitsAccess' ? draft.otherTreesAccess : draft.childUnitsAccess;
  return other === 'none' ? 'otherIsNone' : 'dropsEveryTree';
}
