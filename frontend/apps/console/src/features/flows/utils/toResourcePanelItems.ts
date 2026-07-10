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
