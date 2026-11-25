// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import buildTreeItems from './buildTreeItems';
import type {OrganizationUnit} from '../models/organization-unit';
import type {OrganizationUnitTreeItem} from '../models/organization-unit-tree';

/**
 * Builds the tree of a page of a project's placement listing: the system organization unit with the project's
 * organization units placed under it as its children, and the project's other top organization units beside it.
 * The system organization unit shows only those children, since its other children belong to other projects.
 *
 * @param ous - A page of the placement listing
 * @param systemOuId - The system organization unit, which a later page does not list again
 * @returns The top items, and the items to add under the system organization unit
 */
export default function buildPlacementTreeItems(
  ous: OrganizationUnit[],
  systemOuId?: string,
): {topItems: OrganizationUnitTreeItem[]; underSystem: OrganizationUnitTreeItem[]} {
  const systemId = systemOuId ?? ous.find((ou) => ou.isSystem)?.id;
  const isUnderSystem = (ou: OrganizationUnit): boolean => Boolean(systemId) && !ou.isSystem && ou.parent === systemId;

  return {
    topItems: buildTreeItems(ous.filter((ou) => !isUnderSystem(ou))),
    underSystem: buildTreeItems(ous.filter(isUnderSystem)),
  };
}
