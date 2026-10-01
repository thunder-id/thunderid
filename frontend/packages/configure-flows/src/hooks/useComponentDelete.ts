// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useReactFlow} from '@xyflow/react';
import {useCallback} from 'react';
import {type Element} from '../models/elements';

/**
 * Represents the contract returned by the `useComponentDelete` hook.
 */
export interface UseComponentDelete {
  /**
   * Deletes a given component from a specified step.
   *
   * @param stepId - ID of the step (node) from which the component should be deleted.
   * @param component - The component to be removed.
   */
  deleteComponent: (stepId: string, component: Element) => void;
}

/**
 * Custom React hook to provide functionality for deleting a component from a step node.
 *
 * It leverages the `useReactFlow` hook from `@xyflow/react` to update node data and
 * performs a deep recursive deletion by matching component IDs.
 *
 * @returns An object with a `deleteComponent` function to remove a component from a step.
 */
const useComponentDelete = (): UseComponentDelete => {
  const {updateNodeData} = useReactFlow();

  /**
   * Deletes a component from the list of components in a step node by ID.
   *
   * @param stepId - The unique identifier of the step node.
   * @param component - The component object to be deleted.
   */
  const deleteComponent = useCallback(
    (stepId: string, component: Element): void => {
      /**
       * Recursively filters out the specified component and its children.
       * Creates new arrays/objects immutably without needing cloneDeep.
       *
       * @param components - Array of components to update.
       * @returns A new array of components with the specified one removed.
       */
      const updateComponent = (components: Element[]): Element[] =>
        components?.reduce((acc: Element[], _component: Element) => {
          if (_component.id === component.id) {
            return acc;
          }

          const updatedComponent = _component.components
            ? {..._component, components: updateComponent(_component.components)}
            : _component;

          acc.push(updatedComponent);

          return acc;
        }, []);

      updateNodeData(stepId, (node: {data?: {components?: Element[]}}) => {
        const components: Element[] = updateComponent(node?.data?.components ?? []);

        return {
          components,
        };
      });
    },
    [updateNodeData],
  );

  return {deleteComponent};
};

export default useComponentDelete;
