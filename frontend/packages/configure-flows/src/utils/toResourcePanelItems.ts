// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import kebabCase from 'lodash-es/kebabCase';
import type {Resource} from '../models/resources';

/**
 * A resource on the panel together with a stable identifier unique within its section.
 */
export interface ResourcePanelListItem {
  /**
   * Stable identifier for drag-and-drop and rendering keys.
   */
  id: string;
  /**
   * The resource itself.
   */
  resource: Resource;
}

/**
 * Prepares resources for rendering in a resource panel section.
 *
 * Resources with `display.showOnResourcePanel === false` are excluded, and each
 * remaining resource gets a stable identifier unique within the section, even when
 * resources share the same type and label.
 *
 * @param resources - Resources of a panel section.
 * @param sectionId - Identifier of the section, used as the id prefix.
 * @returns Panel list items with stable identifiers.
 */
function toResourcePanelItems(resources: Resource[] | undefined, sectionId: string): ResourcePanelListItem[] {
  const idOccurrences = new Map<string, number>();

  return (resources ?? [])
    .filter((resource: Resource) => resource.display?.showOnResourcePanel !== false)
    .map((resource: Resource) => {
      const baseId = `${sectionId}-${resource.resourceType}-${resource.type}-${kebabCase(resource.display?.label)}`;
      const occurrence: number = (idOccurrences.get(baseId) ?? 0) + 1;

      idOccurrences.set(baseId, occurrence);

      return {
        id: occurrence === 1 ? baseId : `${baseId}-${occurrence}`,
        resource,
      };
    });
}

export default toResourcePanelItems;
