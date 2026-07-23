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
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {move} from '@dnd-kit/helpers';
import {type DragDropEventHandlers} from '@dnd-kit/react';
import {type Edge, type Node, type XYPosition, useReactFlow, useUpdateNodeInternals} from '@xyflow/react';
import type {UpdateNodeInternals} from '@xyflow/system';
import cloneDeep from 'lodash-es/cloneDeep';
import {useRef} from 'react';
import type {DragSourceData, DragTargetData} from '../models/drag-drop';
import type {Element} from '../models/elements';
import type {MetadataInterface} from '../models/metadata';
import {ResourceTypes, type Resource} from '../models/resources';
import type {Step, StepData} from '../models/steps';
import type {Widget} from '../models/widget';
import autoAssignConnections from '../utils/autoAssignConnections';
import generateResourceId from '../utils/generateResourceId';
import updateNestedComponent from '../utils/updateNestedComponent';
import {widgetNeedsViewContainer} from '../utils/widgetUtils';

/**
 * Props interface for useDragDropHandlers hook
 */
export interface UseDragDropHandlersProps {
  onStepLoad: (step: Step) => Step;
  setNodes: React.Dispatch<React.SetStateAction<Node[]>>;
  setEdges: React.Dispatch<React.SetStateAction<Edge[]>>;
  onResourceDropOnCanvas: (resource: Resource | Step, parentId: string) => void;
  generateStepElement: (element: Element) => Element;
  mutateComponents: (components: Element[]) => Element[];
  onWidgetLoad: (
    widget: Widget,
    targetResource: Resource,
    currentNodes: Node[],
    edges: Edge[],
  ) => [Node[], Edge[], Resource | null, string | null];
  metadata?: MetadataInterface;
}

/**
 * Return type for useDragDropHandlers hook
 */
export interface UseDragDropHandlersReturn {
  addCanvasNode: (
    event: Parameters<DragDropEventHandlers['onDragEnd']>[0],
    sourceData: DragSourceData,
    targetData: DragTargetData,
  ) => void;
  addToView: (
    event: Parameters<DragDropEventHandlers['onDragEnd']>[0],
    sourceData: DragSourceData,
    targetData: DragTargetData,
  ) => void;
  addToForm: (
    event: Parameters<DragDropEventHandlers['onDragEnd']>[0],
    sourceData: DragSourceData,
    targetData: DragTargetData,
  ) => void;
  addToViewAtIndex: (sourceData: DragSourceData, targetStepId: string, targetElementId: string) => void;
  addToFormAtIndex: (sourceData: DragSourceData, targetStepId: string, formId: string, targetElementId: string) => void;
}

/**
 * Hook that provides stable callbacks for drag and drop handlers.
 *
 * - Returns stable function references that NEVER change
 * - ALL dependencies stored in refs and read at call time
 * - The actual work only happens when the function is called (on drop)
 * - Minimal work during render - just ref assignments
 */
const useDragDropHandlers = (props: UseDragDropHandlersProps): UseDragDropHandlersReturn => {
  // Get references from ReactFlow hooks
  const reactFlowInstance = useReactFlow();
  const updateNodeInternals: UpdateNodeInternals = useUpdateNodeInternals();

  // Store ALL dependencies in refs - updated every render
  const depsRef = useRef({
    props,
    reactFlowInstance,
    updateNodeInternals,
  });

  // Update refs every render (minimal overhead - just assignment)
  depsRef.current = {
    props,
    reactFlowInstance,
    updateNodeInternals,
  };

  // Store stable references to handler functions
  const handlersRef = useRef<UseDragDropHandlersReturn | null>(null);

  // Create handlers only once - reads ALL deps from ref at call time
  handlersRef.current ??= {
    addCanvasNode: (event, sourceData): void => {
      const {props: currentProps, reactFlowInstance: rf} = depsRef.current;
      const {onStepLoad, setNodes, setEdges, onResourceDropOnCanvas, onWidgetLoad, metadata} = currentProps;
      const {screenToFlowPosition, getNodes, getEdges} = rf;

      const sourceResource: Resource | undefined = cloneDeep(sourceData.dragged);

      if (!sourceResource || !event.nativeEvent) {
        return;
      }

      if (sourceResource.resourceType === ResourceTypes.Widget) {
        const widget = sourceResource as Widget;

        if (widgetNeedsViewContainer(widget)) {
          return;
        }

        const currentNodes = getNodes();
        const currentEdges = getEdges();
        const [newNodes, newEdges, defaultPropertySelector, defaultPropertySelectorStepId] = onWidgetLoad(
          widget,
          widget,
          currentNodes,
          currentEdges,
        );

        if (metadata?.executorConnections) {
          autoAssignConnections(newNodes, metadata.executorConnections);
        }

        setNodes(() => newNodes);
        setEdges(() => newEdges);
        onResourceDropOnCanvas(defaultPropertySelector ?? widget, defaultPropertySelectorStepId ?? '');

        return;
      }

      // Type guard to ensure nativeEvent is a MouseEvent
      const {nativeEvent} = event;
      if (!('clientX' in nativeEvent) || !('clientY' in nativeEvent)) {
        return;
      }

      const {clientX, clientY} = nativeEvent as MouseEvent;

      const position: XYPosition = screenToFlowPosition({
        x: clientX,
        y: clientY,
      });

      const existingData =
        'data' in sourceResource && typeof sourceResource.data === 'object' && sourceResource.data !== null
          ? sourceResource.data
          : {};

      let generatedStep: Step = {
        ...sourceResource,
        data: {
          components: [],
          ...existingData,
        },
        deletable: true,
        id: generateResourceId(sourceResource.type.toLowerCase()),
        position,
      } as Step;

      // Decorate the step with any additional information
      generatedStep = onStepLoad(generatedStep);

      setNodes((prevNodes: Node[]) => [...prevNodes, generatedStep]);

      onResourceDropOnCanvas(generatedStep, '');
    },

    addToView: (event, sourceData, targetData): void => {
      const {props: currentProps, reactFlowInstance: rf, updateNodeInternals: updateInternals} = depsRef.current;
      const {
        generateStepElement,
        onWidgetLoad,
        setNodes,
        setEdges,
        onResourceDropOnCanvas,
        mutateComponents,
        metadata,
      } = currentProps;
      const {updateNodeData, getNodes, getEdges} = rf;

      const {dragged: sourceResource} = sourceData;
      const {stepId: targetStepId, droppedOn: targetResource} = targetData;

      // Special handling for widgets
      if (sourceResource?.resourceType === ResourceTypes.Widget && targetResource) {
        const currentNodes = getNodes();
        const currentEdges = getEdges();

        const [newNodes, newEdges, defaultPropertySelector, defaultPropertySelectorStepId] = onWidgetLoad(
          sourceResource as Widget,
          targetResource,
          currentNodes,
          currentEdges,
        );

        // Auto-assign connections for execution steps
        if (metadata?.executorConnections) {
          autoAssignConnections(newNodes, metadata.executorConnections);
        }

        setNodes(() => newNodes);
        setEdges(() => newEdges);

        onResourceDropOnCanvas(
          defaultPropertySelector ?? sourceResource,
          defaultPropertySelectorStepId ?? targetStepId ?? '',
        );

        return;
      }

      // Regular element drops
      if (sourceResource && targetStepId) {
        const generatedElement: Element = generateStepElement(sourceResource);

        updateNodeData(targetStepId, (node: Node) => {
          const nodeData = node?.data as StepData | undefined;
          const updatedComponents: Element[] = move([...(nodeData?.components ?? [])], event);

          return {
            components: mutateComponents([...updatedComponents, generatedElement]),
          };
        });

        // Update node internals to fix handle positions after adding element
        requestAnimationFrame(() => {
          updateInternals(targetStepId);
        });

        onResourceDropOnCanvas(generatedElement, targetStepId);
      }
    },

    addToForm: (event, sourceData, targetData): void => {
      const {props: currentProps, reactFlowInstance: rf, updateNodeInternals: updateInternals} = depsRef.current;
      const {generateStepElement, onResourceDropOnCanvas, mutateComponents} = currentProps;
      const {updateNodeData} = rf;

      const {dragged: sourceResource} = sourceData;
      const {stepId: targetStepId, droppedOn: targetResource} = targetData;

      if (sourceResource && targetStepId && targetResource) {
        const generatedElement: Element = generateStepElement(sourceResource);

        updateNodeData(targetStepId, (node: Node) => {
          const nodeData = node?.data as StepData | undefined;
          const updatedComponents: Element[] = updateNestedComponent(
            nodeData?.components ?? [],
            targetResource.id,
            (target: Element) => ({
              ...target,
              components: move([...(target.components ?? [])], event).concat(generatedElement),
            }),
          );

          return {
            components: mutateComponents(updatedComponents),
          };
        });

        // Update node internals to fix handle positions after adding element
        requestAnimationFrame(() => {
          updateInternals(targetStepId);
        });

        onResourceDropOnCanvas(generatedElement, targetStepId);
      }
    },

    addToViewAtIndex: (sourceData, targetStepId, targetElementId): void => {
      const {props: currentProps, reactFlowInstance: rf, updateNodeInternals: updateInternals} = depsRef.current;
      const {
        generateStepElement,
        onWidgetLoad,
        setNodes,
        setEdges,
        onResourceDropOnCanvas,
        mutateComponents,
        metadata,
      } = currentProps;
      const {updateNodeData, getNodes, getEdges} = rf;

      const {dragged: sourceResource} = sourceData;

      if (!sourceResource || !targetStepId) {
        return;
      }

      // Check if this is a widget drop - widgets need special handling
      if (sourceResource.resourceType === ResourceTypes.Widget) {
        const targetNode = getNodes().find((n) => n.id === targetStepId);

        if (targetNode) {
          const currentNodes = getNodes();
          const currentEdges = getEdges();

          const [newNodes, newEdges, defaultPropertySelector, defaultPropertySelectorStepId] = onWidgetLoad(
            sourceResource as Widget,
            targetNode as Resource,
            currentNodes,
            currentEdges,
          );

          // Now we need to reorder the components to insert at the correct position
          const updatedNodes = newNodes.map((node) => {
            if (node.id === targetStepId) {
              const nodeData = node.data as StepData | undefined;
              const components: Element[] = nodeData?.components ?? [];

              if (components.length === 0) {
                return node;
              }

              // The widget button was appended to the end, find it (it's the last element)
              const widgetButton = components[components.length - 1];
              // Get all components except the last one (immutably)
              const componentsWithoutLast = components.slice(0, -1);

              // Find the target index and insert there
              const targetIndex = componentsWithoutLast.findIndex((c) => c.id === targetElementId);

              // Create new array with widget inserted at target index
              const reorderedComponents =
                targetIndex !== -1
                  ? [
                      ...componentsWithoutLast.slice(0, targetIndex),
                      widgetButton,
                      ...componentsWithoutLast.slice(targetIndex),
                    ]
                  : [...componentsWithoutLast, widgetButton];

              return {
                ...node,
                data: {
                  ...nodeData,
                  components: mutateComponents(reorderedComponents),
                },
              };
            }
            return node;
          });

          // Auto-assign connections for execution steps
          if (metadata?.executorConnections) {
            autoAssignConnections(updatedNodes, metadata.executorConnections);
          }

          setNodes(() => updatedNodes);
          setEdges(() => newEdges);

          onResourceDropOnCanvas(
            defaultPropertySelector ?? sourceResource,
            defaultPropertySelectorStepId ?? targetStepId,
          );
        }
      } else {
        // Regular element drop at specific index
        const generatedElement: Element = generateStepElement(sourceResource);

        updateNodeData(targetStepId, (node: Node) => {
          const nodeData = node?.data as StepData | undefined;
          const components: Element[] = nodeData?.components ?? [];

          // Find the index of the target element
          const targetIndex = components.findIndex((c) => c.id === targetElementId);

          // Create new array with element inserted at target index
          const updatedComponents =
            targetIndex !== -1
              ? [...components.slice(0, targetIndex), generatedElement, ...components.slice(targetIndex)]
              : [...components, generatedElement];

          return {
            components: mutateComponents(updatedComponents),
          };
        });

        // Update node internals to fix handle positions after adding element
        requestAnimationFrame(() => {
          updateInternals(targetStepId);
        });

        onResourceDropOnCanvas(generatedElement, targetStepId);
      }
    },

    addToFormAtIndex: (sourceData, targetStepId, formId, targetElementId): void => {
      const {props: currentProps, updateNodeInternals: updateInternals} = depsRef.current;
      const {generateStepElement, onResourceDropOnCanvas, mutateComponents} = currentProps;
      const {updateNodeData} = depsRef.current.reactFlowInstance;

      const {dragged: sourceResource} = sourceData;

      if (!sourceResource || !targetStepId || !formId) {
        return;
      }

      const generatedElement: Element = generateStepElement(sourceResource);

      updateNodeData(targetStepId, (node: Node) => {
        const nodeData = node?.data as StepData | undefined;
        const components: Element[] = nodeData?.components ?? [];

        // Find the form/stack container and insert at the target index within it
        const updatedComponents = updateNestedComponent(components, formId, (target: Element) => {
          const containerComponents: Element[] = target.components ?? [];
          const targetIndex = containerComponents.findIndex((c) => c.id === targetElementId);

          const updatedContainerComponents =
            targetIndex !== -1
              ? [
                  ...containerComponents.slice(0, targetIndex),
                  generatedElement,
                  ...containerComponents.slice(targetIndex),
                ]
              : [...containerComponents, generatedElement];

          return {
            ...target,
            components: updatedContainerComponents,
          };
        });

        return {
          components: mutateComponents(updatedComponents),
        };
      });

      // Update node internals to fix handle positions after adding element
      requestAnimationFrame(() => {
        updateInternals(targetStepId);
      });

      onResourceDropOnCanvas(generatedElement, targetStepId);
    },
  };

  return handlersRef.current;
};

export default useDragDropHandlers;
