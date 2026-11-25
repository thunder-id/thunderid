// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import {StaticStepTypes} from '../models/steps';

/**
 * Finds the variables nodes every route from the start passes before it reaches a step, nearest first. The server
 * refuses a step that refers to a variable when some route reaches the step without passing the node that declares
 * it, so only these nodes declare variables a step can use.
 *
 * @param stepId - The step that would refer to the variables
 * @param nodes - Every node of the flow
 * @param edges - Every edge of the flow
 * @returns The ids of the variables nodes that come before the step on every route, nearest first
 */
export default function findVariablesNodesBefore(stepId: string, nodes: Node[], edges: Edge[]): string[] {
  const starts: string[] = nodes.filter((node: Node) => node.type === StaticStepTypes.Start).map((node) => node.id);
  const successors = new Map<string, string[]>();
  const predecessors = new Map<string, string[]>();

  edges.forEach((edge: Edge) => {
    successors.set(edge.source, [...(successors.get(edge.source) ?? []), edge.target]);
    predecessors.set(edge.target, [...(predecessors.get(edge.target) ?? []), edge.source]);
  });

  // The nodes a route from the start reaches, without passing the avoided node.
  const reachableAvoiding = (avoided?: string): Set<string> => {
    const seen = new Set<string>();
    const queue: string[] = starts.filter((start: string) => start !== avoided);

    queue.forEach((start: string) => seen.add(start));

    while (queue.length > 0) {
      const current = queue.shift()!;

      (successors.get(current) ?? []).forEach((next: string) => {
        if (next !== avoided && !seen.has(next)) {
          seen.add(next);
          queue.push(next);
        }
      });
    }

    return seen;
  };

  // A step no route reaches has nothing before it.
  if (!reachableAvoiding().has(stepId)) {
    return [];
  }

  // How far each node is from the step, walking the edges backwards.
  const distance = new Map<string, number>([[stepId, 0]]);
  const queue: string[] = [stepId];

  while (queue.length > 0) {
    const current = queue.shift()!;

    (predecessors.get(current) ?? []).forEach((previous: string) => {
      if (!distance.has(previous)) {
        distance.set(previous, (distance.get(current) ?? 0) + 1);
        queue.push(previous);
      }
    });
  }

  return nodes
    .filter((node: Node) => node.type === StaticStepTypes.Variables && node.id !== stepId)
    .filter((node: Node) => !reachableAvoiding(node.id).has(stepId))
    .map((node: Node) => node.id)
    .sort((a: string, b: string) => (distance.get(a) ?? Infinity) - (distance.get(b) ?? Infinity));
}
