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

import {type Edge, type Node, type XYPosition, useReactFlow} from '@xyflow/react';
import cloneDeep from 'lodash-es/cloneDeep';
import {useRef} from 'react';
import useFlowEvents from './useFlowEvents';
import type {DragSourceData, DragTargetData, DragEventWithNative} from '../models/drag-drop';
import type {Element} from '../models/elements';
import {BlockTypes, ElementCategories} from '../models/elements';
import type {MetadataInterface} from '../models/metadata';
import {ResourceTypes, type Resource} from '../models/resources';
import {StepTypes, type Step, type StepData} from '../models/steps';
import type {Widget} from '../models/widget';
import autoAssignConnections from '../utils/autoAssignConnections';
import generateResourceId from '../utils/generateResourceId';

/**
 * Props interface for useContainerDialogConfirm hook
 */
export interface UseContainerDialogConfirmProps {
  dropScenario: 'form-on-canvas' | 'input-on-canvas' | 'input-on-view' | 'widget-on-canvas';
  handleContainerDialogClose: () => void;
  generateStepElement: (element: Element) => Element;
  onStepLoad: (step: Step) => Step;
  setNodes: React.Dispatch<React.SetStateAction<Node[]>>;
  setEdges: React.Dispatch<React.SetStateAction<Edge[]>>;
  onResourceDropOnCanvas: (resource: Resource | Step, parentId: string) => void;
  onWidgetLoad: (
    widget: Widget,
    targetResource: Resource,
    currentNodes: Node[],
    edges: Edge[],
    removeTargetViewWhenStandalone?: boolean,
  ) => [Node[], Edge[], Resource | null, string | null];
  metadata?: MetadataInterface;
  pendingDropRef: React.MutableRefObject<{
    event: DragEventWithNative;
    sourceData: DragSourceData;
    targetData: DragTargetData;
  } | null>;
}

/**
 * Hook that provides stable callback for container dialog confirm.
 *
 * - Returns stable function reference that NEVER changes
 * - ALL dependencies stored in refs and read at call time
 * - The actual work only happens when the function is called (on dialog confirm)
 * - Minimal work during render - just ref assignments
 */
const useContainerDialogConfirm = (props: UseContainerDialogConfirmProps): (() => void) => {
  // Get references from ReactFlow hooks
  const reactFlowInstance = useReactFlow();
  const {notifyElementAdded} = useFlowEvents();

  // Store ALL dependencies in refs - updated every render
  const depsRef = useRef({
    props,
    reactFlowInstance,
    notifyElementAdded,
  });

  // Update refs every render (minimal overhead - just assignment)
  depsRef.current = {
    props,
    reactFlowInstance,
    notifyElementAdded,
  };

  // Store stable reference to handler function
  const handlerRef = useRef<(() => void) | null>(null);

  // Create handler only once - reads ALL deps from ref at call time
  handlerRef.current ??= (): void => {
    const {props: currentProps, reactFlowInstance: rf} = depsRef.current;
    const {
      dropScenario,
      handleContainerDialogClose,
      generateStepElement,
      onStepLoad,
      setNodes,
      setEdges,
      onResourceDropOnCanvas,
      onWidgetLoad,
      metadata,
      pendingDropRef,
    } = currentProps;
    const {screenToFlowPosition, updateNodeData, getNodes, getEdges} = rf;

    const pendingData = pendingDropRef.current;

    if (!pendingData) {
      handleContainerDialogClose();
      return;
    }

    const {event, sourceData, targetData} = pendingData;
    const droppedResource: Resource | undefined = cloneDeep(sourceData.dragged);

    if (!droppedResource || !event.nativeEvent) {
      handleContainerDialogClose();
      return;
    }

    // Type guard to ensure nativeEvent is a MouseEvent
    const {nativeEvent} = event;
    if (!nativeEvent || !('clientX' in nativeEvent) || !('clientY' in nativeEvent)) {
      handleContainerDialogClose();
      return;
    }

    // After validation, we know nativeEvent has clientX/clientY properties
    const position: XYPosition = screenToFlowPosition({
      x: (nativeEvent as MouseEvent).clientX,
      y: (nativeEvent as MouseEvent).clientY,
    });

    // Generate the dropped element with a unique ID
    const generatedElement: Element = generateStepElement(droppedResource as Element);

    if (dropScenario === 'form-on-canvas') {
      // Create a View step with the Form inside
      let generatedViewStep: Step = {
        category: ResourceTypes.Step,
        data: {
          components: [generatedElement],
        },
        deletable: true,
        id: generateResourceId(StepTypes.View.toLowerCase()),
        position,
        resourceType: ResourceTypes.Step,
        type: StepTypes.View,
      } as Step;

      generatedViewStep = onStepLoad(generatedViewStep);
      setNodes((prevNodes: Node[]) => [...prevNodes, generatedViewStep]);
      onResourceDropOnCanvas(generatedViewStep, '');

      // Dispatch custom event to notify about element addition (for auto-layout hint)
      depsRef.current.notifyElementAdded('step');
    } else if (dropScenario === 'input-on-canvas') {
      // Create a Form element containing the Input
      const formElement: Element = {
        resourceType: ResourceTypes.Element,
        category: ElementCategories.Block,
        type: BlockTypes.Form,
        id: generateResourceId(BlockTypes.Form.toLowerCase()),
        config: {},
        components: [generatedElement],
      } as Element;

      // Create a View step with the Form (containing the Input) inside
      let generatedViewStep: Step = {
        category: ResourceTypes.Step,
        data: {
          components: [formElement],
        },
        deletable: true,
        id: generateResourceId(StepTypes.View.toLowerCase()),
        position,
        resourceType: ResourceTypes.Step,
        type: StepTypes.View,
      } as Step;

      generatedViewStep = onStepLoad(generatedViewStep);
      setNodes((prevNodes: Node[]) => [...prevNodes, generatedViewStep]);
      onResourceDropOnCanvas(generatedViewStep, '');

      // Dispatch custom event to notify about element addition (for auto-layout hint)
      depsRef.current.notifyElementAdded('step');
    } else if (dropScenario === 'input-on-view') {
      // Create a Form element containing the Input and add it to the View
      const formElement: Element = {
        resourceType: ResourceTypes.Element,
        category: ElementCategories.Block,
        type: BlockTypes.Form,
        id: generateResourceId(BlockTypes.Form.toLowerCase()),
        config: {},
        components: [generatedElement],
      } as Element;

      const targetStepId = targetData.stepId;
      if (targetStepId) {
        updateNodeData(targetStepId, (node: Node) => {
          const existingComponents: Element[] = (node?.data as StepData)?.components ?? [];
          return {
            components: [...existingComponents, formElement],
          };
        });
        onResourceDropOnCanvas(formElement, targetStepId);
      }
    } else if (dropScenario === 'widget-on-canvas') {
      // Create an empty View step - do NOT call onStepLoad here as it would add default components
      // The onWidgetLoad function will handle populating the View with widget-specific content
      const generatedViewStep: Step = {
        category: ResourceTypes.Step,
        data: {
          components: [],
        },
        deletable: true,
        id: generateResourceId(StepTypes.View.toLowerCase()),
        position,
        resourceType: ResourceTypes.Step,
        type: StepTypes.View,
      } as Step;

      const currentNodes = getNodes();
      const currentEdges = getEdges();

      // Use onWidgetLoad to properly load the widget into the View
      const [newNodes, newEdges, defaultPropertySelector, defaultPropertySectorStepId] = onWidgetLoad(
        droppedResource as Widget,
        generatedViewStep,
        [...currentNodes, generatedViewStep],
        currentEdges,
        true,
      );

      // Auto-assign connections for execution steps.
      if (metadata?.executorConnections) {
        autoAssignConnections(newNodes, metadata.executorConnections);
      }

      setNodes(() => newNodes);
      setEdges(() => newEdges);

      onResourceDropOnCanvas(
        defaultPropertySelector ?? droppedResource,
        defaultPropertySectorStepId ?? generatedViewStep.id ?? '',
      );

      // Dispatch custom event to notify about element addition (for auto-layout hint)
      depsRef.current.notifyElementAdded('widget');
    }

    // Close the dialog and clear pending data
    handleContainerDialogClose();
  };

  return handlerRef.current;
};

export default useContainerDialogConfirm;
