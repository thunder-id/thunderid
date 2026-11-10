// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from './organization-unit';

/**
 * Request body for creating an organization unit.
 *
 * Derived from {@link OrganizationUnit} by picking only the fields
 * required for creation. Server-generated fields (id, themeId, layoutId, logoUrl)
 * are excluded.
 *
 * @public
 * @remarks
 * Used with POST /organization-units endpoint.
 *
 * @example
 * ```typescript
 * const request: CreateOrganizationUnitRequest = {
 *   handle: 'engineering',
 *   name: 'Engineering Department',
 *   description: 'Software engineering team',
 *   parent: 'root-ou-id'
 * };
 * ```
 */
export type CreateOrganizationUnitRequest = Pick<
  OrganizationUnit,
  'handle' | 'name' | 'description' | 'parent' | 'projectId'
>;

/**
 * Request body for updating an organization unit.
 *
 * Derived from {@link OrganizationUnit} by omitting only the server-managed `id` field.
 * All other fields, including `layoutId` and the default-flow fields, can be updated.
 *
 * @public
 * @remarks
 * Used with PUT /organization-units/:id endpoint.
 *
 * @example
 * ```typescript
 * const request: UpdateOrganizationUnitRequest = {
 *   handle: 'engineering',
 *   name: 'Engineering Department (Updated)',
 *   description: 'Updated description',
 *   parent: 'root-ou-id',
 *   themeId: '96c62e6d-9297-4295-8195-d28dfe0c9ff7',
 *   logoUrl: 'https://example.com/new-logo.png'
 * };
 * ```
 */
export type UpdateOrganizationUnitRequest = Omit<OrganizationUnit, 'id'>;

/**
 * Query parameters for listing organization units.
 *
 * Supports pagination through limit and offset parameters.
 *
 * @public
 * @remarks
 * Used with GET /organization-units endpoint.
 *
 * @example
 * ```typescript
 * const params: OrganizationUnitListParams = {
 *   limit: 10,
 *   offset: 0
 * };
 * ```
 */
export interface OrganizationUnitListParams {
  /**
   * Maximum number of results to return
   * @example 10
   */
  limit?: number;

  /**
   * Number of results to skip for pagination
   * @example 0
   */
  offset?: number;

  /**
   * List only the organization units of this project
   * @example '019a3000-0000-7000-8000-0000000000a1'
   */
  projectId?: string;

  /**
   * With `projectId`, list where a project resource can be placed: the system organization unit
   * first, then the project's top organization units
   */
  placement?: boolean;

  /**
   * With `placement`, keep only the placements in this organization unit's subtree
   */
  within?: string;
}
