// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The body of a create project request (`POST /projects`).
 *
 * @public
 */
export interface CreateProjectRequest {
  handle: string;
  displayName: string;
  description?: string;
}
