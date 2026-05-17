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
import {useUpdateNodeInternals} from '@xyflow/react';
import type {UpdateNodeInternals} from '@xyflow/system';
import cloneDeep from 'lodash-es/cloneDeep';
import {useRef} from 'react';
import useFlowEvents from './useFlowEvents';
import useFlowPlugins from './useFlowPlugins';
import type {Base} from '../models/base';
import {type Element} from '../models/elements';
import type {MetadataInterface} from '../models/metadata';
import {ResourceTypes, type Resource} from '../models/resources';
import {StepTypes, type Step, type StepData} from '../models/steps';
import {type Template} from '../models/templates';
import type {Widget} from '../models/widget';
import autoAssignConnections from '../utils/autoAssignConnections';
import generateResourceId from '../utils/generateResourceId';
import {widgetNeedsViewContainer} from '../utils/widgetUtils';

/**
 * Props interface for useResourceAdd hook
 */
export interface UseResourceAddProps {
  onTemplateLoad: (template: Template) => [Node[], Edge[], Resource?, string?];
  onWidgetLoad: (
    widget: Widget,
    targetResource: Resource,
    currentNodes: Node[],
    edges: Edge[],
    removeTargetViewWhenStandalone?: boolean,
  ) => [Node[], Edge[], Resource | null, string | null];
  onStepLoad: (step: Step) => Step;
  setNodes: React.Dispatch<React.SetStateAction<Node[]>>;
  setEdges: React.Dispatch<React.SetStateAction<Edge[]>>;
  // External dependencies - passed from parent to avoid calling hooks inside this hook
  generateStepElement: (element: Element) => Element;
  metadata?: MetadataInterface;
  onResourceDropOnCanvas: (element: Base, nodeId: string) => void;
}

/**
 * Hook that provides a stable handleOnAdd callback for adding resources to the flow.
 *
 * - Returns a stable function reference that NEVER changes
 * - ALL dependencies are stored in refs and read at call time
 * - The actual work only happens when the function is called (on button click)
 * - Minimal work during render - just ref assignments
 */
const useResourceAdd = (props: UseResourceAddProps): ((resource: Resource) => void) => {
  // Get references from hooks - only ReactFlow hooks needed here
  const reactFlowInstance = useReactFlow();
  const {notifyElementAdded} = useFlowEvents();
  const {emitTemplateLoad} = useFlowPlugins();
  const updateNodeInternals: UpdateNodeInternals = useUpdateNodeInternals();

  // Store ALL dependencies in refs - updated every render
  const depsRef = useRef({
    props,
    reactFlowInstance,
    updateNodeInternals,
    notifyElementAdded,
    emitTemplateLoad,
  });

  // Update refs every render (minimal overhead - just assignment)
  depsRef.current = {
    props,
    reactFlowInstance,
    updateNodeInternals,
    notifyElementAdded,
    emitTemplateLoad,
  };

  // Store a stable reference to the handler function itself
  const handlerRef = useRef<((resource: Resource) => void) | null>(null);

  // Create handler only once - reads ALL deps from ref at call time
  handlerRef.current ??= (resource: Resource): void => {
    // Get ALL latest values at call time from the ref
    const {props: currentProps, reactFlowInstance: rf, updateNodeInternals: updateInternals} = depsRef.current;

    const {
      onTemplateLoad,
      onWidgetLoad,
      onStepLoad,
      setNodes,
      setEdges,
      generateStepElement,
      metadata,
      onResourceDropOnCanvas,
    } = currentProps;
    const {screenToFlowPosition, getNodes, getEdges, updateNodeData, fitView} = rf;

    const clonedResource: Resource = cloneDeep(resource);

    // Handle templates
    if (resource.resourceType === ResourceTypes.Template) {
      const template = clonedResource as Template;
      depsRef.current.emitTemplateLoad(template);

      const [newNodes, newEdges] = onTemplateLoad(template);

      if (metadata?.executorConnections) {
        autoAssignConnections(newNodes, metadata.executorConnections);
      }

      const updateAllNodeInternals = (nodesToUpdate: Node[]): void => {
        nodesToUpdate.forEach((node: Node) => {
          updateInternals(node.id);
          if (node.data?.components) {
            (node.data.components as Element[]).forEach((component: Element) => {
              updateInternals(component.id);
              if (component?.components) {
                component.components.forEach((nestedComponent: Element) => {
                  updateInternals(nestedComponent.id);
                });
              }
            });
          }
        });
      };

      // Skip auto-layout for starter templates - use positions from template directly
      setNodes(newNodes);
      setEdges([...newEdges]);
      requestAnimationFrame(() => {
        updateAllNodeInternals(newNodes);
        requestAnimationFrame(() => {
          fitView({padding: 0.2, duration: 300}).catch(() => {
            // Ignore fitView errors
          });
        });
      });

      // Dispatch custom event to notify about element addition (for auto-layout hint)
      depsRef.current.notifyElementAdded('template');

      // Don't open properties panel for templates - just track the resource without opening panel
      return;
    }

    // Handle widgets
    if (resource.resourceType === ResourceTypes.Widget) {
      const currentNodes = getNodes();
      const currentEdges = getEdges();
      const widget = clonedResource as Widget;
      const needsViewContainer = widgetNeedsViewContainer(widget);
      const existingViewStep = needsViewContainer
        ? currentNodes.find((node) => node.type === StepTypes.View)
        : undefined;
      let targetViewStep: Step;
      let nodesToPass: Node[];

      if (existingViewStep) {
        const nodeData = existingViewStep.data as StepData | undefined;
        targetViewStep = {...existingViewStep, data: {...nodeData}} as Step;
        nodesToPass = currentNodes;
      } else if (needsViewContainer) {
        const position: XYPosition = screenToFlowPosition({
          x: window.innerWidth / 2,
          y: window.innerHeight / 2,
        });
        targetViewStep = {
          category: ResourceTypes.Step,
          data: {components: []},
          deletable: true,
          id: generateResourceId(StepTypes.View.toLowerCase()),
          position,
          resourceType: ResourceTypes.Step,
          type: StepTypes.View,
        } as Step;
        nodesToPass = [...currentNodes, targetViewStep];
      } else {
        targetViewStep = {} as Step;
        nodesToPass = currentNodes;
      }

      const [newNodes, newEdges] = onWidgetLoad(
        widget,
        targetViewStep,
        nodesToPass,
        currentEdges,
        needsViewContainer && !existingViewStep,
      );

      if (metadata?.executorConnections) {
        autoAssignConnections(newNodes, metadata.executorConnections);
      }

      const updateAllNodeInternals = (nodesToUpdate: Node[]): void => {
        nodesToUpdate.forEach((node: Node) => {
          updateInternals(node.id);
          if (node.data?.components) {
            (node.data.components as Element[]).forEach((component: Element) => {
              updateInternals(component.id);
              if (component?.components) {
                component.components.forEach((nestedComponent: Element) => {
                  updateInternals(nestedComponent.id);
                });
              }
            });
          }
        });
      };

      // Skip auto-layout for widgets - use positions from widget definition directly
      setNodes(newNodes);
      setEdges([...newEdges]);
      requestAnimationFrame(() => {
        updateAllNodeInternals(newNodes);
        requestAnimationFrame(() => {
          fitView({padding: 0.2, duration: 300}).catch(() => {
            // Ignore fitView errors
          });
        });
      });

      // Dispatch custom event to notify about element addition (for auto-layout hint)
      depsRef.current.notifyElementAdded('widget');

      // Don't open properties panel for widgets - just track the resource without opening panel
      return;
    }

    // Handle steps
    if (resource.resourceType === ResourceTypes.Step) {
      const position: XYPosition = screenToFlowPosition({
        x: window.innerWidth / 2,
        y: window.innerHeight / 2,
      });

      let generatedStep: Step = {
        ...clonedResource,
        data: {components: [], ...(clonedResource?.data ?? {})},
        deletable: true,
        id: generateResourceId(clonedResource.type.toLowerCase()),
        position,
      } as Step;

      generatedStep = onStepLoad(generatedStep);
      setNodes((prevNodes: Node[]) => [...prevNodes, generatedStep]);
      onResourceDropOnCanvas(generatedStep, '');

      // Dispatch custom event to notify about element addition (for auto-layout hint)
      depsRef.current.notifyElementAdded('step');
      return;
    }

    // Handle elements
    if (resource.resourceType === ResourceTypes.Element) {
      const currentNodes = getNodes();
      const existingViewStep = currentNodes.find((node) => node.type === StepTypes.View);
      if (existingViewStep) {
        const generatedElement: Element = generateStepElement(clonedResource as Element);
        updateNodeData(existingViewStep.id, (node: Node) => {
          const nodeData = node?.data as StepData | undefined;
          const existingComponents: Element[] = nodeData?.components ?? [];
          return {components: [...existingComponents, generatedElement]};
        });
        requestAnimationFrame(() => {
          updateInternals(existingViewStep.id);
        });
        onResourceDropOnCanvas(generatedElement, existingViewStep.id);
      }
    }
  };

  // Always return a function, even if handlerRef.current is null (should never happen)
  return handlerRef.current ?? (() => null);
};

export default useResourceAdd;
