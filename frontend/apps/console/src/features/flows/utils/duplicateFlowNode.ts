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

/**
 * Collects every identifier already in use on the canvas: node ids plus the
 * ids of all nested components. Seeds the allocator that keeps generated ids
 * collision-free.
 *
 * @param nodes - The canvas nodes to collect ids from.
 * @returns The set of ids in use.
 */
export function collectCanvasIds(nodes: Node[]): Set<string> {
  const ids = new Set<string>();
  const collectComponentIds = (components: Element[]): void => {
    components.forEach((component: Element) => {
      ids.add(component.id);
      if (component.components) {
        collectComponentIds(component.components);
      }
    });
  };
  nodes.forEach((node: Node) => {
    ids.add(node.id);
    const components = (node.data as StepData | undefined)?.components;
    if (Array.isArray(components)) {
      collectComponentIds(components);
    }
  });
  return ids;
}

/**
 * Generates a resource id with the given prefix that is not already taken.
 *
 * @param prefix - Prefix for the generated id, usually the resource type.
 * @param takenIds - Ids already in use; the generated id is guaranteed absent.
 * @returns The generated id.
 */
export function generateUniqueResourceId(prefix: string, takenIds: ReadonlySet<string>): string {
  let id = generateResourceId(prefix);
  while (takenIds.has(id)) {
    id = generateResourceId(prefix);
  }
  return id;
}

/**
 * Regenerates the id of every component in the tree, registering each new id
 * into `takenIds` so later allocations cannot collide with it.
 */
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
 * @param takenIds - Ids already in use on the canvas, ideally seeded via
 *   {@link collectCanvasIds}. Every id generated for the copy is added to the
 *   set, so passing the same set across several calls keeps all copies
 *   collision-free with each other.
 * @returns The duplicated node.
 */
export function duplicateFlowNode(node: Node, takenIds: Set<string>): Node {
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
