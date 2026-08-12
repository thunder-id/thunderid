// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type Edge, type Node, useReactFlow} from '@xyflow/react';
import {useEffect} from 'react';
import useFlowPlugins from './useFlowPlugins';
import useUIPanelState from './useUIPanelState';
import VisualFlowConstants from '../constants/VisualFlowConstants';
import {type Element} from '../models/elements';
import {StepTypes} from '../models/steps';

/**
 * Custom hook to handle the deletion of execution resources in the flow builder.
 *
 * This hook registers an event listener for node deletion events and ensures that
 * any associated execution action nodes are also deleted when an execution node is removed.
 */
const useDeleteExecutionResource = (): void => {
  const {setIsOpenResourcePropertiesPanel} = useUIPanelState();
  const {getEdges, getNodes, updateNodeData} = useReactFlow();
  const {onNodeDelete} = useFlowPlugins();

  /**
   * Deletes associated execution components when execution nodes are removed.
   *
   * This utility function ensures that when an execution node is deleted from the flow,
   * any related execution initiation action components are also removed to maintain consistency.
   *
   * @param deleted - An array of nodes that have been deleted from the flow.
   */
  function deleteExecutionActionNode(deleted: Node[]): boolean {
    const nodes: Node[] = getNodes();
    const edges: Edge[] = getEdges();
    const actionNodes: Node[] = [];
    const actionComponentIds: string[] = [];

    deleted.forEach((node: Node) => {
      if (node?.type === StepTypes.Execution) {
        const actionNode: Node[] = nodes?.filter((n: Node) =>
          edges?.some((edge: Edge) => {
            if (
              edge.target === node.id &&
              edge.source === n.id &&
              edge?.sourceHandle?.includes(VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX)
            ) {
              actionComponentIds.push(
                edge.sourceHandle.slice(0, -VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX.length),
              );

              return true;
            }

            return false;
          }),
        );

        if (actionNode?.length > 0) {
          actionNodes.push(...actionNode);
        }
      }
    });

    // If no action nodes are found, return true to indicate no further action is needed.
    if (actionNodes.length === 0) {
      return true;
    }

    actionNodes.forEach((actionNode: Node) => {
      updateNodeData(actionNode.id, (node: Node) => {
        const components: Element[] = (node.data.components as Element[])?.filter(
          (component: Element) => !actionComponentIds.includes(component.id),
        );

        return {
          components,
        };
      });
    });
    setIsOpenResourcePropertiesPanel(false);

    return true;
  }

  // eslint-disable-next-line react-hooks/exhaustive-deps -- handlers use state-getter pattern (getNodes, getEdges) so they're safe with empty deps
  useEffect(() => onNodeDelete(deleteExecutionActionNode), [onNodeDelete]);
};

export default useDeleteExecutionResource;
