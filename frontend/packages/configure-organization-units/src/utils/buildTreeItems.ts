// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import OrganizationUnitTreeConstants from '../constants/organization-unit-tree-constants';
import type {OrganizationUnit} from '../models/organization-unit';
import type {OrganizationUnitTreeItem} from '../models/organization-unit-tree';

export default function buildTreeItems(ous: OrganizationUnit[]): OrganizationUnitTreeItem[] {
  return ous.map((ou) => ({
    id: ou.id,
    label: ou.name,
    handle: ou.handle,
    description: ou.description,
    logoUrl: ou.logoUrl,
    isReadOnly: ou.isReadOnly,
    allowedActions: ou.allowedActions,
    // A unit whose children the caller may not list is a leaf.
    children: ou.allowedActions?.includes('listChildren')
      ? [
          {
            id: `${ou.id}${OrganizationUnitTreeConstants.PLACEHOLDER_SUFFIX}`,
            label: '',
            handle: '',
            isPlaceholder: true,
          },
        ]
      : [],
  }));
}
