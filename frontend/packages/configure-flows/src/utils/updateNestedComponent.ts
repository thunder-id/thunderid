// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
