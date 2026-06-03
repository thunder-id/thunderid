/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
  extends Pick<OrganizationUnit, 'id' | 'handle' | 'description' | 'logoUrl' | 'isReadOnly'> {
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
