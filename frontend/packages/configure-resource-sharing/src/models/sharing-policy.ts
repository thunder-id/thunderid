// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Every organization unit in the deployment, at any depth. Only a resource file or an imported
 * document can introduce it: a create may not state it, and an edit may only keep it.
 */
export interface AllUnitsSection {
  /**
   * Organization units withheld from the section, each taking everything beneath it with it.
   */
  excludedOuIds?: string[];
}

/**
 * How far access carries from a named child unit: `unit` reaches it alone, `subtree` reaches it and
 * everything below it.
 */
export type SharingReach = 'unit' | 'subtree';

export interface SharingChildUnit {
  ouId: string;
  reach: SharingReach;
}

/**
 * The organization units beneath the issuer: all of them, or the named direct children.
 */
export type ChildUnitsSection =
  | {
      mode: 'all';
      /**
       * Direct children of the issuer withheld from the section, each with everything beneath it.
       */
      excludedOuIds?: string[];
    }
  | {mode: 'selected'; units: SharingChildUnit[]};

export interface SharingRootUnit {
  ouId: string;
}

/**
 * The top units of other trees: the named ones, or every one. `all` only comes from a resource file
 * or an imported document: a create may not state it, and an edit may only keep it.
 */
export type OtherTreeRootsSection =
  | {
      mode: 'all';
      /**
       * Top units withheld from the section, and with them their trees.
       */
      excludedOuIds?: string[];
    }
  | {mode: 'selected'; units: SharingRootUnit[]};

/**
 * The sections of a sharing policy. A section the policy does not state is left out, and a policy
 * stating `allUnits` states nothing else.
 */
export interface SharingPolicySections {
  allUnits?: AllUnitsSection;
  childUnits?: ChildUnitsSection;
  otherTreeRoots?: OtherTreeRootsSection;
}

/**
 * A recorded sharing policy, as every resource that mounts sharing endpoints returns it.
 */
export interface SharingPolicy extends SharingPolicySections {
  id: string;
  /**
   * The organization unit whose decision this policy is.
   */
  initiatingOuId?: string;
  /**
   * `share` when the owner issued it, `reshare` when a unit it reached passed access on.
   */
  stage: 'share' | 'reshare';
  parentPolicyId?: string;
  /**
   * Set when a resource file declares the policy. It cannot be edited or deleted through the API.
   */
  isReadOnly?: boolean;
  version: number;
}

export interface SharingPolicyListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  policies: SharingPolicy[];
}

/**
 * The body of a sharing policy create or replace.
 */
export interface SharingPolicyRequest extends SharingPolicySections {
  /**
   * The version the caller believes it is editing. Required on a replace, ignored on a create.
   */
  version?: number;
}
