// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import type {UpdateNodeInternals} from '@xyflow/system';
import cloneDeep from 'lodash-es/cloneDeep';
import {useCallback} from 'react';
import useGenerateStepElement from './useGenerateStepElement';
import {BlockTypes, ElementCategories, type Element} from '../models/elements';
import {ResourceTypes} from '../models/resources';
import {StepTypes} from '../models/steps';
import {INPUT_ELEMENT_TYPES, mutateComponents} from '../utils/componentMutations';
import generateIdsForResources from '../utils/generateIdsForResources';

/**
 * Props for the useElementAddition hook.
 */
export interface UseElementAdditionProps {
  /** Function to set nodes in the flow. */
  setNodes: React.Dispatch<React.SetStateAction<Node[]>>;
  /** Function to update node internals for rendering. */
  updateNodeInternals: UpdateNodeInternals;
}

/**
 * Return type for the useElementAddition hook.
 */
export interface UseElementAdditionReturn {
  /** Add an element to a view from the context menu. */
  handleAddElementToView: (element: Element, viewId: string) => void;
  /** Add an element to a container (form or stack) from the context menu. */
  handleAddElementToForm: (element: Element, containerId: string) => void;
}

/**
 * Whether the component tree holds a container with the given id. Containers can be
 * nested (a stack inside a form), so the search walks the whole tree.
 */
function containsComponent(components: Element[], containerId: string): boolean {
  return components.some(
    (component: Element) => component.id === containerId || containsComponent(component.components ?? [], containerId),
  );
}

/**
 * Appends an element to the container with the given id, at any nesting depth.
 */
function appendToComponent(components: Element[], containerId: string, element: Element): Element[] {
  return components.map((component: Element) => {
    if (component.id === containerId) {
      return {...component, components: [...(component.components ?? []), element]};
    }
    if (component.components) {
      return {...component, components: appendToComponent(component.components, containerId, element)};
    }
    return component;
  });
}

/**
 * Hook to handle adding elements to views and forms.
 * Manages the logic for inserting elements into the correct containers
 * with proper form creation and mutation.
 *
 * @param props - Configuration options for the hook.
 * @returns Element addition handlers.
 */
const useElementAddition = (props: UseElementAdditionProps): UseElementAdditionReturn => {
  const {setNodes, updateNodeInternals} = props;
  const {generateStepElement} = useGenerateStepElement();

  /**
   * Callback for adding an element to a view from the context menu.
   * Adds the element to the specified View.
   */
  const handleAddElementToView = useCallback(
    (element: Element, viewId: string): void => {
      // Use generateStepElement to properly apply variants and generate unique IDs
      const generatedElement: Element = generateStepElement(element);
      let viewStepId: string | null = null;

      setNodes((prevNodes: Node[]) => {
        // Find the View node with the given viewId
        const existingViewStep = prevNodes.find((node) => node.id === viewId && node.type === StepTypes.View);

        if (!existingViewStep) {
          return prevNodes; // No View exists, do nothing
        }

        // Store view ID for later use
        viewStepId = existingViewStep.id;

        // For INPUT elements, add them to Form (create Form if needed)
        if (INPUT_ELEMENT_TYPES.has(element.type)) {
          return prevNodes.map((node) => {
            if (node.id === existingViewStep.id) {
              const nodeData = node.data as {components?: Element[]} | undefined;
              const existingComponents: Element[] = cloneDeep(nodeData?.components ?? []);

              // Find existing Form in the View
              const existingForm = existingComponents.find((comp: Element) => comp.type === BlockTypes.Form);

              if (existingForm) {
                // Add input to existing Form
                const updatedForm: Element = {
                  ...existingForm,
                  components: [...(existingForm.components ?? []), generatedElement],
                };

                const componentsWithoutForm = existingComponents.filter(
                  (comp: Element) => comp.type !== BlockTypes.Form,
                );

                return {
                  ...node,
                  data: {
                    ...nodeData,
                    components: mutateComponents([...componentsWithoutForm, updatedForm]),
                  },
                };
              }

              // No Form exists - create a new Form with the input
              const newForm: Element = generateIdsForResources<Element>({
                id: '{{ID}}',
                resourceType: ResourceTypes.Element,
                category: ElementCategories.Block,
                type: BlockTypes.Form,
                version: '0.1.0',
                deprecated: false,
                display: {
                  label: 'Form',
                  image: 'assets/images/icons/form.svg',
                },
                config: {},
                components: [generatedElement],
              } as Element);

              return {
                ...node,
                data: {
                  ...nodeData,
                  components: mutateComponents([...existingComponents, newForm]),
                },
              };
            }
            return node;
          });
        }

        // For other elements (Buttons, etc.), add to existing View
        return prevNodes.map((node) => {
          if (node.id === existingViewStep.id) {
            const nodeData = node.data as {components?: Element[]} | undefined;
            const existingComponents: Element[] = cloneDeep(nodeData?.components ?? []);

            return {
              ...node,
              data: {
                ...nodeData,
                components: mutateComponents([...existingComponents, generatedElement]),
              },
            };
          }
          return node;
        });
      });

      // Schedule node internals update after state has been updated
      if (viewStepId) {
        queueMicrotask(() => {
          updateNodeInternals(viewStepId!);
        });
      }
    },
    [setNodes, updateNodeInternals, generateStepElement],
  );

  /**
   * Callback for adding an element to a form from the context menu.
   * Adds the element to the specified form.
   */
  const handleAddElementToForm = useCallback(
    (element: Element, formId: string): void => {
      // Use generateStepElement to properly apply variants and generate unique IDs
      const generatedElement: Element = generateStepElement(element);
      let viewStepId: string | null = null;

      setNodes((prevNodes: Node[]) => {
        // Find the View node that contains the target container. Containers nest
        // (a stack inside a form), so both the lookup and the insert recurse.
        const existingViewStep = prevNodes.find((node) => {
          if (node.type !== StepTypes.View) return false;
          const nodeData = node.data as {components?: Element[]} | undefined;
          return containsComponent(nodeData?.components ?? [], formId);
        });

        if (!existingViewStep) {
          return prevNodes;
        }

        viewStepId = existingViewStep.id;

        return prevNodes.map((node) => {
          if (node.id === existingViewStep.id) {
            const nodeData = node.data as {components?: Element[]} | undefined;
            const existingComponents: Element[] = cloneDeep(nodeData?.components ?? []);

            // Find the container and add the element to it
            const updatedComponents = appendToComponent(existingComponents, formId, generatedElement);

            return {
              ...node,
              data: {
                ...nodeData,
                components: mutateComponents(updatedComponents),
              },
            };
          }
          return node;
        });
      });

      // Schedule node internals update after state has been updated
      if (viewStepId) {
        queueMicrotask(() => {
          updateNodeInternals(viewStepId!);
        });
      }
    },
    [setNodes, updateNodeInternals, generateStepElement],
  );

  return {
    handleAddElementToView,
    handleAddElementToForm,
  };
};

export default useElementAddition;
