// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import {MarkerType} from '@xyflow/react';
import VisualFlowConstants from '../constants/VisualFlowConstants';
import type {Element} from '../models/elements';

/**
 * Generates edges for components that have actions with 'onSuccess' references
 * but don't have corresponding edges in the current edge set.
 *
 * @param currentEdges - The current set of edges in the flow.
 * @param currentNodes - The current set of nodes in the flow.
 * @param edgeStyle - The style to apply to generated edges.
 * @returns Array of missing edges that should be added to the flow.
 */
const generateUnconnectedEdges = (currentEdges: Edge[], currentNodes: Node[], edgeStyle: string): Edge[] => {
  const nodeIds = new Set<string>(currentNodes.map((node: Node) => node.id));
  const missingEdges: Edge[] = [];

  const processAction = (stepId: string, resourceId: string, action: unknown): void => {
    if (action && typeof action === 'object') {
      // Process onSuccess edges
      if ('onSuccess' in action && action.onSuccess) {
        const buttonId: string = resourceId;
        const expectedTarget: string = action.onSuccess as string;

        // Ensure expected target exists in nodes
        if (nodeIds.has(expectedTarget)) {
          const existingEdge: Edge | undefined = currentEdges.find(
            (edge: Edge) =>
              edge.source === stepId &&
              edge.sourceHandle === `${buttonId}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
          );

          // If no edge exists or it's pointing to the wrong node, add a missing edge
          if (existingEdge?.target !== expectedTarget) {
            missingEdges.push({
              animated: false,
              id: `${buttonId}_MISSING_EDGE`,
              markerEnd: {
                type: MarkerType.Arrow,
              },
              source: stepId,
              sourceHandle: `${buttonId}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
              target: expectedTarget,
              type: edgeStyle,
            });
          }
        }
      }

      // Process onFailure edges
      if ('onFailure' in action && action.onFailure) {
        const expectedFailureTarget: string = action.onFailure as string;

        // Ensure expected target exists in nodes
        if (nodeIds.has(expectedFailureTarget)) {
          const existingFailureEdge: Edge | undefined = currentEdges.find(
            (edge: Edge) => edge.source === stepId && edge.sourceHandle === 'failure',
          );

          // If no edge exists or it's pointing to the wrong node, add a missing edge
          if (existingFailureEdge?.target !== expectedFailureTarget) {
            missingEdges.push({
              animated: false,
              id: `${stepId}_FAILURE_MISSING_EDGE`,
              markerEnd: {
                type: MarkerType.Arrow,
              },
              source: stepId,
              sourceHandle: 'failure',
              target: expectedFailureTarget,
              type: edgeStyle,
            });
          }
        }
      }

      // Process onIncomplete edges
      if ('onIncomplete' in action && action.onIncomplete) {
        const expectedIncompleteTarget: string = action.onIncomplete as string;

        if (nodeIds.has(expectedIncompleteTarget)) {
          const existingIncompleteEdge: Edge | undefined = currentEdges.find(
            (edge: Edge) =>
              edge.source === stepId &&
              edge.sourceHandle === `${stepId}${VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX}`,
          );

          if (existingIncompleteEdge?.target !== expectedIncompleteTarget) {
            missingEdges.push({
              animated: false,
              id: `${stepId}_INCOMPLETE_MISSING_EDGE`,
              markerEnd: {
                type: MarkerType.Arrow,
              },
              source: stepId,
              sourceHandle: `${stepId}${VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX}`,
              target: expectedIncompleteTarget,
              type: edgeStyle,
            });
          }
        }
      }
    }
  };

  currentNodes.forEach((node: Node) => {
    if (!node.data) {
      return;
    }

    if (node.data?.['components']) {
      (node.data['components'] as Element[]).forEach((component: Element) => {
        processAction(node.id, component.id, component.action);

        // Process `FORM` components.
        if (component?.components) {
          component.components.forEach((nestedComponent: Element) =>
            processAction(node?.id, nestedComponent.id, nestedComponent.action),
          );
        }
      });
    }

    if (node.data?.['action']) {
      processAction(node.id, node.id, node.data['action']);
    }
  });

  return missingEdges;
};

export default generateUnconnectedEdges;
