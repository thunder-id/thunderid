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

import type {Element} from '../models/elements';

/**
 * Recursively searches `components` for the element with id `targetId`, at any
 * nesting depth, and replaces it with the result of `updater`. Containers that
 * don't hold the target (directly or through a descendant) are returned unchanged.
 */
const updateNestedComponent = (
  components: Element[],
  targetId: string,
  updater: (target: Element) => Element,
): Element[] =>
  components.map((component: Element) => {
    if (component.id === targetId) {
      return updater(component);
    }

    if (component.components) {
      return {
        ...component,
        components: updateNestedComponent(component.components, targetId, updater),
      };
    }

    return component;
  });

/**
 * Recursively searches `components` for the container (an element with a
 * `components` array) that directly holds an element with id `childId`, at
 * any nesting depth.
 */
export const findContainingComponent = (components: Element[], childId: string): Element | undefined => {
  for (const component of components) {
    if (!component.components) {
      continue;
    }

    if (component.components.some((child: Element) => child.id === childId)) {
      return component;
    }

    const nestedMatch = findContainingComponent(component.components, childId);
    if (nestedMatch) {
      return nestedMatch;
    }
  }

  return undefined;
};

export default updateNestedComponent;
