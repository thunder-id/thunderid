// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A role a user share can mandate in a child unit.
 */
export interface MandatableRole {
  id: string;
  name: string;
  description?: string;
  /**
   * The unit that owns the role.
   */
  ouId: string;
}

/**
 * What a user share can mandate in one child unit.
 */
export interface MandatableRoles {
  /**
   * The issuer's own roles shared with the unit.
   */
  candidates: MandatableRole[];
  /**
   * The name of every role the issuer can see, keyed by role ID, to name a mandated role that is no
   * longer a candidate.
   */
  names: Record<string, string>;
}
