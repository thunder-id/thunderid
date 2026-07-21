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

import type {Edge, Node} from '@xyflow/react';

const SIMULATION_CLASS_PATTERN = /\bsimulation-[\w-]+\b/g;

function hasSimulationClass(item: {className?: string}): boolean {
  return Boolean(item.className?.includes('simulation-'));
}

function cleanClassName(className: string): string | undefined {
  const cleaned = className.replace(SIMULATION_CLASS_PATTERN, '').replace(/\s+/g, ' ').trim();
  return cleaned === '' ? undefined : cleaned;
}

/**
 * Combines an element's own classes with the given simulation presentation
 * classes, replacing any previous simulation decoration while preserving
 * everything else the node/edge already carried.
 */
export function withSimulationClasses(className: string | undefined, simulationClasses: string): string {
  const base = className ? cleanClassName(className) : undefined;
  return base ? `${base} ${simulationClasses}` : simulationClasses;
}

/**
 * Removes simulation presentation classes from nodes. The flow preview styles the
 * canvas by decorating the node objects handed to React Flow; anything that reads
 * nodes back from the React Flow store (drag collision resolution, auto layout,
 * save) must strip them so preview styling never leaks into canvas state or
 * persisted layout data. Returns the input array untouched when nothing to strip.
 */
export function stripSimulationNodeClasses(nodes: Node[]): Node[] {
  if (!nodes.some(hasSimulationClass)) {
    return nodes;
  }
  return nodes.map((node: Node) =>
    hasSimulationClass(node) ? {...node, className: cleanClassName(node.className!)} : node,
  );
}

/**
 * Removes simulation presentation classes (and the paired traversal animation)
 * from edges. See {@link stripSimulationNodeClasses}.
 */
export function stripSimulationEdgeClasses(edges: Edge[]): Edge[] {
  if (!edges.some(hasSimulationClass)) {
    return edges;
  }
  return edges.map((edge: Edge) =>
    hasSimulationClass(edge) ? {...edge, className: cleanClassName(edge.className!), animated: false} : edge,
  );
}
