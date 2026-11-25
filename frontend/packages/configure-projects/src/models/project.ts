// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A project, as the project API lists it.
 *
 * @public
 */
export interface Project {
  id: string;
  handle: string;
  displayName: string;
  description?: string;
  createdAt: string;
}

/**
 * A page of projects, oldest first.
 *
 * @public
 */
export interface ProjectListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  projects: Project[];
}
