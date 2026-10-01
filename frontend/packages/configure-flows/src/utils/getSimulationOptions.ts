// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import VisualFlowConstants from '../constants/VisualFlowConstants';
import type {Element} from '../models/elements';
import type {StepData} from '../models/steps';

/**
 * Kinds of transitions a simulation step can take.
 */
export const SimulationOptionKinds = {
  Action: 'action',
  Success: 'success',
  Failure: 'failure',
  Incomplete: 'incomplete',
} as const;

export type SimulationOptionKinds = (typeof SimulationOptionKinds)[keyof typeof SimulationOptionKinds];

/**
 * An outgoing transition from the current simulation step.
 */
export interface SimulationOption {
  /**
   * Id of the edge representing the transition.
   */
  edgeId: string;
  /**
   * Id of the node the transition leads to.
   */
  targetNodeId: string;
  /**
   * Kind of the transition, used for labeling and styling.
   */
  kind: SimulationOptionKinds;
  /**
   * Label of the triggering component (e.g. button text), when the transition
   * originates from a user action inside a view.
   */
  actionLabel?: string;
  /**
   * Id of the triggering component inside the source node, when the transition
   * originates from a user action.
   */
  sourceComponentId?: string;
}

function findComponentById(components: Element[] | undefined, id: string): Element | undefined {
  if (!components) {
    return undefined;
  }

  for (const component of components) {
    if (component.id === id) {
      return component;
    }
    const nested = findComponentById(component.components, id);
    if (nested) {
      return nested;
    }
  }

  return undefined;
}

function resolveOption(edge: Edge, sourceNode: Node | undefined): Omit<SimulationOption, 'edgeId' | 'targetNodeId'> {
  const sourceHandle = edge.sourceHandle ?? '';

  if (sourceHandle === 'failure') {
    return {kind: SimulationOptionKinds.Failure};
  }

  if (sourceHandle.endsWith(VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX)) {
    return {kind: SimulationOptionKinds.Incomplete};
  }

  // Handles are `${elementOrStepId}_NEXT`. When the prefix is a component inside the
  // source node (e.g. a button), the transition is a user action — label it with the
  // component's text.
  const handlePrefix = sourceHandle.replace(VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX, '');

  if (sourceNode && handlePrefix && handlePrefix !== sourceNode.id) {
    const component = findComponentById((sourceNode.data as StepData | undefined)?.components, handlePrefix);
    const actionLabel = (component as (Element & {label?: string}) | undefined)?.label;

    if (component) {
      return {kind: SimulationOptionKinds.Action, actionLabel, sourceComponentId: component.id};
    }
  }

  return {kind: SimulationOptionKinds.Success};
}

/**
 * Computes the outgoing transitions of a node for flow simulation.
 *
 * @param nodeId - Id of the current simulation node.
 * @param nodes - All nodes on the canvas.
 * @param edges - All edges on the canvas.
 * @returns The available transitions, user actions first.
 */
function getSimulationOptions(nodeId: string, nodes: Node[], edges: Edge[]): SimulationOption[] {
  const sourceNode = nodes.find((node: Node) => node.id === nodeId);
  const nodeIds = new Set(nodes.map((node: Node) => node.id));

  const kindOrder: Record<SimulationOptionKinds, number> = {
    [SimulationOptionKinds.Action]: 0,
    [SimulationOptionKinds.Success]: 1,
    [SimulationOptionKinds.Incomplete]: 2,
    [SimulationOptionKinds.Failure]: 3,
  };

  return edges
    .filter((edge: Edge) => edge.source === nodeId && nodeIds.has(edge.target))
    .map((edge: Edge) => ({
      edgeId: edge.id,
      targetNodeId: edge.target,
      ...resolveOption(edge, sourceNode),
    }))
    .sort((a: SimulationOption, b: SimulationOption) => kindOrder[a.kind] - kindOrder[b.kind]);
}

export default getSimulationOptions;
