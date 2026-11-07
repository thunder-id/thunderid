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
  /**
   * Roles the share mandates in this unit. Only meaningful with reach `unit`.
   */
  roles?: string[];
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
 * Where a policy turns assignments off, so the organization units reached there can't assign the
 * shared role. Only a resource file or the API sets these rules; the screen shows and keeps them.
 */
export interface AssignmentsOffRules {
  /**
   * Off for every organization unit an "Every organization unit" section reaches.
   */
  everyUnit: boolean;
  /**
   * Off for the child units that all child units reach.
   */
  allChildUnits: boolean;
  /**
   * Off for the top units that "Every tree" reaches.
   */
  everyTree: boolean;
  /**
   * Named child units and top units of other trees it is off for.
   */
  ouIds: string[];
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
  /**
   * Whether the switch drops a rule turning a role's assignments off, which the console can't set
   * again. Organization units granted again afterwards can assign the role.
   */
  losesAssignmentsOff: boolean;
}
