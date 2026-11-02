// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SharingPolicySections} from './sharing-policy';

/**
 * Who can use a resource, at the top level: only its owner, the units chosen in the child-unit and
 * other-tree sections, or every unit of every tree. `everyUnit` is only offered while the saved
 * policy already holds it.
 */
export type OrganizationAccessScope = 'owner' | 'chosen' | 'everyUnit';

/**
 * Which organization units of the owner's own tree can use a resource, when the scope is `chosen`.
 */
export type ChildUnitsAccess = 'none' | 'all' | 'selected';

/**
 * Which other trees' top units can use a resource, when the scope is `chosen`. `every` is only
 * offered while the saved policy already holds it.
 */
export type OtherTreesAccess = 'none' | 'selected' | 'every';

/**
 * How far below a granted child unit access reaches.
 */
export type ChildUnitReach = 'unit' | 'subtree';

export interface GrantedChildUnit {
  ouId: string;
  reach: ChildUnitReach;
}

/**
 * The editable form of a resource's sharing policy, in the terms the Organization Access screen
 * presents. Each choice owns its own fields, so switching a choice clears the fields it leaves.
 *
 * The child-unit and other-tree choices are independent of each other, and only apply when the
 * scope is `chosen`.
 */
export interface OrganizationAccessDraft {
  scope: OrganizationAccessScope;
  childUnitsAccess: ChildUnitsAccess;
  /**
   * Direct children of the owner carved out of `all`.
   */
  childUnitExclusions: string[];
  /**
   * Units granted under `selected`.
   */
  childUnits: GrantedChildUnit[];
  /**
   * Units carved out of the `everyUnit` scope, at any depth and in any tree.
   */
  everyUnitExclusions: string[];
  otherTreesAccess: OtherTreesAccess;
  /**
   * Top units granted under `selected`.
   */
  trees: string[];
  /**
   * Top units carved out of `every`.
   */
  everyTreeExclusions: string[];
}

/**
 * The deployment-wide choices a resource's saved policy holds. The console can keep them but not
 * create them, so the screen offers each only while the saved policy has it.
 */
export interface OrganizationAccessOffers {
  everyUnit: boolean;
  everyTree: boolean;
}

/**
 * What saving a draft does to the resource's sharing policy.
 */
export type SharingPolicySavePlan =
  | {type: 'none'}
  | {type: 'create'; sections: SharingPolicySections}
  | {type: 'update'; policyId: string; version: number; sections: SharingPolicySections}
  | {type: 'delete'; policyId: string}
  | {type: 'replace'; policyId: string; sections: SharingPolicySections};

/**
 * One group of settings a choice switch clears, named by kind so the dialog can describe it.
 */
export interface ClearedAccessItem {
  kind: 'childUnitExclusions' | 'childUnits' | 'everyUnit' | 'trees' | 'everyTree';
  ouIds: string[];
}

/**
 * A switch between choices, and what it would clear.
 */
export interface AccessSwitch {
  field: 'scope' | 'childUnitsAccess' | 'otherTreesAccess';
  to: OrganizationAccessScope | ChildUnitsAccess | OtherTreesAccess;
  next: OrganizationAccessDraft;
  cleared: ClearedAccessItem[];
  /**
   * Whether the switch drops a deployment-wide choice the console cannot bring back.
   */
  losesEveryUnit: boolean;
  losesEveryTree: boolean;
}
