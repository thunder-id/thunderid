// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import cloneDeep from 'lodash-es/cloneDeep';
import {EXECUTION_STACK_NODE_TYPE} from './compactGraphTransforms';
import generateResourceId from './generateResourceId';
import type {Element} from '../models/elements';
import type {StepData} from '../models/steps';

/**
 * Canvas offset applied to a duplicated node so the copy lands visibly beside
 * its source instead of exactly on top of it.
 */
export const DUPLICATE_NODE_OFFSET = 48;

/**
 * Whether a canvas node can be duplicated. Singleton steps (START/END) are
 * marked `deletable: false` and must stay unique per flow; synthetic
 * compact-mode stack nodes are display-only and have no state to copy.
 */
export function canDuplicateNode(node: Node): boolean {
  return node.deletable !== false && node.type !== EXECUTION_STACK_NODE_TYPE;
}

function generateUniqueResourceId(prefix: string, takenIds: ReadonlySet<string>): string {
  let id = generateResourceId(prefix);
  while (takenIds.has(id)) {
    id = generateResourceId(prefix);
  }
  return id;
}

function regenerateComponentIds(components: Element[], takenIds: Set<string>): Element[] {
  return components.map((component: Element) => {
    const id = generateUniqueResourceId(component.type?.toLowerCase() || 'component', takenIds);
    takenIds.add(id);
    return {
      ...component,
      id,
      ...(component.components ? {components: regenerateComponentIds(component.components, takenIds)} : {}),
    };
  });
}

/**
 * Creates a detached copy of a canvas node: a fresh id for the node and for
 * every nested component (component ids must stay unique across the canvas for
 * drag-and-drop and validation), configuration kept as-is, and no incoming or
 * outgoing edges. The copy is offset from its source and returned selected so
 * keyboard actions (drag, delete, duplicate again) chain naturally.
 *
 * @param node - The canvas node to copy.
 * @param existingIds - Ids already present on the canvas, used to guarantee the
 *   generated ids are unique.
 * @returns The duplicated node.
 */
export function duplicateFlowNode(node: Node, existingIds: Iterable<string>): Node {
  const takenIds = new Set(existingIds);
  const id = generateUniqueResourceId(node.type?.toLowerCase() ?? 'step', takenIds);
  takenIds.add(id);

  const data = cloneDeep(node.data) as StepData | undefined;
  if (data && Array.isArray(data.components)) {
    data.components = regenerateComponentIds(data.components, takenIds);
  }

  return {
    ...node,
    id,
    data: (data ?? {}) as Node['data'],
    position: {x: node.position.x + DUPLICATE_NODE_OFFSET, y: node.position.y + DUPLICATE_NODE_OFFSET},
    selected: true,
    dragging: false,
  };
}
