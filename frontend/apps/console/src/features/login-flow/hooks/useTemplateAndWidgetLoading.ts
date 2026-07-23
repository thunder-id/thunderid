/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import cloneDeep from 'lodash-es/cloneDeep';
import isEmpty from 'lodash-es/isEmpty';
import mergeWith from 'lodash-es/mergeWith';
import {useCallback} from 'react';
import {mutateComponents} from '../utils/componentMutations';
import generateUnconnectedEdges from '../utils/edgeUtils';
import useFlowConfig from '@/features/flows/hooks/useFlowConfig';
import useGenerateStepElement from '@/features/flows/hooks/useGenerateStepElement';
import {BlockTypes, type Element} from '@/features/flows/models/elements';
import {ResourceTypes, type Resource, type Resources} from '@/features/flows/models/resources';
import {StepTypes, type Step, type StepData} from '@/features/flows/models/steps';
import {type Template, TemplateTypes, type TemplateReplacer} from '@/features/flows/models/templates';
import type {Widget} from '@/features/flows/models/widget';
import autoWireWidget, {type AutoWireMeta} from '@/features/flows/utils/autoWireWidget';
import generateIdsForResources from '@/features/flows/utils/generateIdsForResources';
import resolveComponentMetadata from '@/features/flows/utils/resolveComponentMetadata';
import resolveStepMetadata from '@/features/flows/utils/resolveStepMetadata';
import updateTemplatePlaceholderReferences from '@/features/flows/utils/updateTemplatePlaceholderReferences';

/**
 * Props for the useTemplateAndWidgetLoading hook.
 */
export interface UseTemplateAndWidgetLoadingProps {
  /** Flow builder resources. */
  resources: Resources;
  /** Function to generate steps. */
  generateSteps: (stepNodes: Node[]) => Node[];
  /** Function to generate edges from steps. */
  generateEdges: (flowSteps: Step[]) => Edge[];
  /** Function to validate edges against nodes. */
  validateEdges: (edges: Edge[], nodes: Node[]) => Edge[];
  /** Function to get blank template components. */
  getBlankTemplateComponents: () => Element[];
  /** Function to set nodes. */
  setNodes: React.Dispatch<React.SetStateAction<Node[]>>;
  /** Function to update node internals. */
  updateNodeInternals: (nodeId: string) => void;
}

/**
 * Return type for the useTemplateAndWidgetLoading hook.
 */
export interface UseTemplateAndWidgetLoadingReturn {
  /** Handle loading a step. */
  handleStepLoad: (step: Step) => Step;
  /** Handle loading a template. */
  handleTemplateLoad: (template: Template) => [Node[], Edge[], Resource?, string?];
  /** Handle loading a widget. */
  handleWidgetLoad: (
    widget: Widget,
    targetResource: Resource,
    currentNodes: Node[],
    currentEdges: Edge[],
  ) => [Node[], Edge[], Resource | null, string | null];
  /** Handle adding a resource from the panel. */
  handleResourceAdd: (resource: Resource) => void;
}

/**
 * Hook to handle template, widget, step, and resource loading logic.
 *
 * @param props - Configuration options for the hook.
 * @returns Loading handlers.
 */
const useTemplateAndWidgetLoading = (props: UseTemplateAndWidgetLoadingProps): UseTemplateAndWidgetLoadingReturn => {
  const {
    resources,
    generateSteps,
    generateEdges,
    validateEdges,
    getBlankTemplateComponents,
    setNodes,
    updateNodeInternals,
  } = props;

  const {setFlowCompletionConfigs, edgeStyle} = useFlowConfig();
  const {generateStepElement} = useGenerateStepElement();

  /**
   * Handle loading a step.
   */
  const handleStepLoad = useCallback(
    (step: Step): Step => {
      // If the step is of type `VIEW` and has no components, set the default components.
      if (step.type === StepTypes.View) {
        if (isEmpty(step?.data?.components)) {
          return {
            ...step,
            data: {
              ...step.data,
              components: getBlankTemplateComponents(),
            },
          };
        }
      }

      const processedStep: Step = generateIdsForResources<Step>(step);

      if (processedStep?.data?.components) {
        processedStep.data.components = resolveComponentMetadata(resources, processedStep.data.components);
      }

      return resolveStepMetadata(resources, [processedStep])[0];
    },
    [resources, getBlankTemplateComponents],
  );

  /**
   * Handle loading a template.
   */
  const handleTemplateLoad = useCallback(
    (template: Template): [Node[], Edge[], Resource?, string?] => {
      if (!template?.config?.data?.steps) {
        return [[], [], {} as Resource, ''];
      }

      const replacers: TemplateReplacer[] | undefined = template?.config?.data?.__generationMeta__?.replacers;

      // Check for End steps and set flow completion configs before processing
      template.config.data.steps.forEach((step: Step) => {
        if (step.type === StepTypes.End) {
          if (step?.config) {
            setFlowCompletionConfigs(step.config);
          }
        }
      });

      const templateSteps: Node[] = replacers
        ? updateTemplatePlaceholderReferences(generateSteps(template.config.data.steps), replacers)[0]
        : generateSteps(template.config.data.steps);

      const generatedTemplateEdges: Edge[] = validateEdges(generateEdges(templateSteps as Step[]), templateSteps);
      // Apply current edge style to all template edges
      const templateEdges: Edge[] = generatedTemplateEdges.map((edge) => ({
        ...edge,
        type: edgeStyle,
      }));

      // Handle BASIC_FEDERATED template case.
      if (template.type === TemplateTypes.BasicFederated) {
        const googleExecutionStep: Node | undefined = templateSteps.find(
          (step: Node) => step.type === StepTypes.Execution,
        );

        if (googleExecutionStep) {
          return [templateSteps, templateEdges, googleExecutionStep as unknown as Resource, googleExecutionStep.id];
        }
      }

      return [templateSteps, templateEdges, {} as Resource, ''];
    },
    [generateSteps, generateEdges, validateEdges, setFlowCompletionConfigs, edgeStyle],
  );

  /**
   * Handle loading a widget.
   */
  const handleWidgetLoad = useCallback(
    (
      widget: Widget,
      targetResource: Resource,
      currentNodes: Node[],
      currentEdges: Edge[],
    ): [Node[], Edge[], Resource | null, string | null] => {
      const widgetFlow = widget.config.data as {
        steps?: Step[];

        __generationMeta__?: {
          replacers?: TemplateReplacer[];
          defaultPropertySelectorId?: string;
          autoWire?: AutoWireMeta;
        };
      };

      if (!widgetFlow?.steps) {
        return [currentNodes, currentEdges, null, null];
      }

      let newNodes: Node[] = cloneDeep(currentNodes);
      let newEdges: Edge[] = cloneDeep(currentEdges);

      // Custom merge function to handle components specifically
      const customMerge = (objValue: unknown, srcValue: unknown, key: string): Element[] | undefined => {
        // Check if the key is 'components' and both are arrays
        if (key === 'components' && Array.isArray(objValue) && Array.isArray(srcValue)) {
          // Concatenate the arrays - don't use unionWith as it prevents adding multiple
          // similar components (like multiple social login buttons) before IDs are generated
          return [...(objValue as Element[]), ...(srcValue as Element[])];
        }

        return undefined;
      };

      widgetFlow.steps.forEach((step: Step) => {
        if (
          step.__generationMeta__ &&
          typeof step.__generationMeta__ === 'object' &&
          'strategy' in step.__generationMeta__
        ) {
          const {strategy} = step.__generationMeta__ as {strategy?: string};

          if (strategy === 'MERGE_WITH_DROP_POINT') {
            newNodes = newNodes.map((node: Node) => {
              if (node.id === targetResource.id) {
                // Use mergeWith with the custom merge function
                return mergeWith(node, step, customMerge);
              }

              return node;
            });
          }
        } else {
          newNodes = [...newNodes, step] as Node[];
        }
      });

      const replacers = widgetFlow.__generationMeta__?.replacers ?? [];

      const defaultPropertySelectorId = widgetFlow.__generationMeta__?.defaultPropertySelectorId;
      let defaultPropertySelectorStepId: string | null = null;
      let defaultPropertySelector: Resource | null = null;

      // Resolve step & component metadata.
      newNodes = resolveStepMetadata(
        resources,
        generateIdsForResources<Node[]>(
          newNodes.map((step: Node) => ({
            data:
              (step.data?.components
                ? {
                    ...step.data,
                    components: resolveComponentMetadata(resources, step.data.components as Element[]),
                  }
                : step.data) ?? {},
            deletable: true,
            id: step.id,
            position: step.position,
            type: step.type,
          })),
        ) as Step[],
      ) as Node[];

      // Find default property selector
      newNodes.forEach((node: Node) => {
        if (node.id === defaultPropertySelectorId) {
          defaultPropertySelectorStepId = node.id;
          defaultPropertySelector = node as Resource;

          return;
        }

        if (!isEmpty(node?.data?.components)) {
          (node.data.components as Element[]).forEach((component: Element) => {
            if (component.id === defaultPropertySelectorId) {
              defaultPropertySelectorStepId = node.id;
              defaultPropertySelector = component as Resource;

              return;
            }

            if (!isEmpty(component?.components)) {
              if (component.id === defaultPropertySelectorId) {
                defaultPropertySelectorStepId = node.id;
                defaultPropertySelector = component as Resource;
              }
            }
          });
        }
      });

      const [updatedNodes, replacedPlaceholders] = updateTemplatePlaceholderReferences(
        generateIdsForResources(newNodes),
        replacers,
      );

      newEdges = [...newEdges, ...generateUnconnectedEdges(newEdges, updatedNodes, edgeStyle)];

      const wired = autoWireWidget(
        currentNodes,
        updatedNodes,
        newEdges,
        widgetFlow.__generationMeta__?.autoWire,
        replacedPlaceholders,
        edgeStyle,
      );

      // Check if `defaultPropertySelector.id` is in the `replacedPlaceholders`.
      // If so, update them with the replaced value.
      if (defaultPropertySelector && 'id' in defaultPropertySelector) {
        const selectorId = (defaultPropertySelector as Resource & {id: string}).id;

        if (typeof selectorId === 'string') {
          const cleanedId = selectorId.replace(/[{}]/g, '');

          if (replacedPlaceholders.has(cleanedId)) {
            const replacedId = replacedPlaceholders.get(cleanedId);

            if (replacedId) {
              (defaultPropertySelector as {id: string}).id = replacedId;
            }
          }
        }
      }

      // Check if `defaultPropertySelectorStepId` is in the `replacedPlaceholders`.
      // If so, update them with the replaced value.
      if (defaultPropertySelectorStepId) {
        const stepId: string = defaultPropertySelectorStepId;
        const cleanedId = stepId.replace(/[{}]/g, '');

        if (replacedPlaceholders.has(cleanedId)) {
          const replacedId = replacedPlaceholders.get(cleanedId);

          if (replacedId) {
            defaultPropertySelectorStepId = replacedId;
          }
        }
      }

      return [wired.nodes, wired.edges, defaultPropertySelector, defaultPropertySelectorStepId];
    },
    [resources, edgeStyle],
  );

  /**
   * Handle adding a resource (like Form) from the resource panel via the + icon.
   * This finds or creates a View step and adds the element to it.
   */
  const handleResourceAdd = useCallback(
    (resource: Resource): void => {
      if (resource.resourceType !== ResourceTypes.Element) {
        return;
      }

      const element = resource as Element;
      const generatedElement: Element = generateStepElement(element);

      // Try to find an existing View step
      setNodes((prevNodes: Node[]) => {
        const existingViewStep = prevNodes.find((node) => node.type === StepTypes.View);

        if (existingViewStep) {
          // Add to existing View
          return prevNodes.map((node) => {
            if (node.id === existingViewStep.id) {
              const nodeData = node.data as StepData | undefined;
              const existingComponents: Element[] = nodeData?.components ?? [];

              // For Forms, replace any existing Form (only one Form per View)
              let updatedComponents: Element[];
              if (generatedElement.type === BlockTypes.Form) {
                updatedComponents = [
                  ...existingComponents.filter((comp: Element) => comp.type !== BlockTypes.Form),
                  generatedElement,
                ];
              } else {
                updatedComponents = [...existingComponents, generatedElement];
              }

              const mutatedComponents: Element[] | undefined = mutateComponents(updatedComponents);

              // Schedule node internals update
              queueMicrotask(() => {
                updateNodeInternals(existingViewStep.id);
                updateNodeInternals(generatedElement.id);
              });

              return {
                ...node,
                data: {
                  ...nodeData,
                  components: mutatedComponents,
                },
              };
            }

            return node;
          });
        }

        // If no View exists, do nothing (user should add a View step first)
        return prevNodes;
      });
    },
    [generateStepElement, setNodes, updateNodeInternals],
  );

  return {
    handleStepLoad,
    handleTemplateLoad,
    handleWidgetLoad,
    handleResourceAdd,
  };
};

export default useTemplateAndWidgetLoading;
