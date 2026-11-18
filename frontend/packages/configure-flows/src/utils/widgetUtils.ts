// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import type {Step} from '../models/steps';
import type {Widget} from '../models/widget';

const DEFAULT_NODE_WIDTH = 200;
const DEFAULT_NODE_HEIGHT = 100;
const WIDGET_CLUSTER_GAP = 80;

export function widgetNeedsViewContainer(widget: Widget): boolean {
  const widgetSteps =
    (
      widget?.config?.data as {
        steps?: {__generationMeta__?: {strategy?: string}}[];
      }
    )?.steps ?? [];

  if (widgetSteps.length === 0) {
    return true;
  }

  // Widgets need a container when they have no generated steps yet, or when at
  // least one generated step must merge into an existing drop point. Missing
  // generation metadata is treated the same as "no merge strategy", so those
  // steps stay on the standalone path.
  return widgetSteps.some((step) => step?.__generationMeta__?.strategy === 'MERGE_WITH_DROP_POINT');
}

/**
 * Moves a freshly dropped widget's nodes below the existing nodes they would overlap. Widget steps
 * carry fixed positions, so without this a widget dropped twice stacks its nodes on the first copy.
 *
 * @param preExistingNodes - The nodes present before the widget was dropped.
 * @param resultNodes - The full node set after the widget was loaded.
 * @param widgetSteps - The widget's step definitions, which carry the cluster's positions and sizes.
 * @param resolvedIds - Map of replacer key to concrete id, from `updateTemplatePlaceholderReferences`.
 * @returns The node set with the widget's own nodes shifted clear of the existing ones.
 */
export function placeWidgetCluster(
  preExistingNodes: Node[],
  resultNodes: Node[],
  widgetSteps: Step[],
  resolvedIds: Map<string, string>,
): Node[] {
  const resultIds = new Set<string>(resultNodes.map((node: Node) => node.id));
  // Skip steps merged into the drop point, and steps auto-wiring dropped in favour of an existing node.
  const clusterSteps: Step[] = widgetSteps.filter(
    (step: Step) =>
      (step.__generationMeta__ as {strategy?: string} | undefined)?.strategy !== 'MERGE_WITH_DROP_POINT' &&
      resultIds.has(resolvedIds.get(step.id.replace(/[{}]/g, '')) ?? step.id),
  );

  if (preExistingNodes.length === 0 || clusterSteps.length === 0) {
    return resultNodes;
  }

  const left: number = Math.min(...clusterSteps.map((step: Step) => step.position.x));
  const top: number = Math.min(...clusterSteps.map((step: Step) => step.position.y));
  const right: number = Math.max(
    ...clusterSteps.map((step: Step) => step.position.x + (step.size?.width ?? DEFAULT_NODE_WIDTH)),
  );
  const bottom: number = Math.max(
    ...clusterSteps.map((step: Step) => step.position.y + (step.size?.height ?? DEFAULT_NODE_HEIGHT)),
  );

  const inColumn = preExistingNodes
    .map((node: Node) => ({
      top: node.position.y,
      bottom: node.position.y + (node.measured?.height ?? node.height ?? DEFAULT_NODE_HEIGHT),
      left: node.position.x,
      right: node.position.x + (node.measured?.width ?? node.width ?? DEFAULT_NODE_WIDTH),
    }))
    .filter((box) => box.left < right + WIDGET_CLUSTER_GAP && box.right + WIDGET_CLUSTER_GAP > left);

  if (!inColumn.some((box) => box.top < bottom + WIDGET_CLUSTER_GAP && box.bottom + WIDGET_CLUSTER_GAP > top)) {
    return resultNodes;
  }

  const shift: number = Math.max(...inColumn.map((box) => box.bottom)) + WIDGET_CLUSTER_GAP - top;
  const preIds = new Set<string>(preExistingNodes.map((node: Node) => node.id));

  return resultNodes.map((node: Node) =>
    preIds.has(node.id) ? node : {...node, position: {x: node.position.x, y: node.position.y + shift}},
  );
}
