// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A project: the applications and agents that name it, deployed and promoted together.
 *
 * @public
 */
export interface Project {
  /** What deploys name the project by. */
  id: string;
  /** What the project is called. */
  displayName: string;
}
