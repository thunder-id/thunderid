// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from './organization-unit';

/**
 * Organization Unit Tree Item
 *
 * Lightweight representation of an organization unit used in tree views.
 * Contains only the fields necessary for rendering hierarchical tree components.
 *
 * @public
 * @remarks
 * This model is used by the organization unit sidebar tree view.
 * The `isPlaceholder` flag indicates a loading state for lazy-loaded children.
 *
 * @example
 * ```typescript
 * const treeItem: OrganizationUnitTreeItem = {
 *   id: '550e8400-e29b-41d4-a716-446655440000',
 *   label: 'Engineering',
 *   handle: 'engineering',
 *   children: [
 *     { id: 'child-id', label: 'Frontend', handle: 'frontend' }
 *   ]
 * };
 * ```
 */
export interface OrganizationUnitTreeItem
  extends Pick<OrganizationUnit, 'id' | 'handle' | 'description' | 'logoUrl' | 'isReadOnly' | 'allowedActions'> {
  /**
   * Display label shown in the tree view
   * @example 'Engineering'
   */
  label: string;

  /**
   * Whether this item is a placeholder for lazy loading
   * Used to indicate that child items are being fetched
   */
  isPlaceholder?: boolean;

  /**
   * Child organization unit tree items
   */
  children?: OrganizationUnitTreeItem[];
}
