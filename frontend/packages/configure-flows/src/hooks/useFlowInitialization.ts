// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import type {UpdateNodeInternals} from '@xyflow/system';
import cloneDeep from 'lodash-es/cloneDeep';
import {useCallback, useLayoutEffect, useMemo, useRef, useEffect} from 'react';
import FlowConstants from '../constants/FlowConstants';
import type {Element} from '../models/elements';
import type {Resources} from '../models/resources';
import type {FlowDefinitionResponse, FlowNode} from '../models/responses';
import {StaticStepTypes, StepTypes, type Step} from '../models/steps';
import {type Template, TemplateTypes, type TemplateReplacer} from '../models/templates';
import {transformFlowToCanvas} from '../utils/flowToCanvasTransformer';
import generateIdsForResources from '../utils/generateIdsForResources';
import resolveComponentMetadata from '../utils/resolveComponentMetadata';
import resolveStepMetadata from '../utils/resolveStepMetadata';
import updateTemplatePlaceholderReferences from '../utils/updateTemplatePlaceholderReferences';

/**
 * Props for the useFlowInitialization hook.
 */
export interface UseFlowInitializationProps {
  /** Resources containing templates, steps, etc. */
  resources: Resources;
  /** Flow ID if editing an existing flow. */
  flowId?: string;
  /** Existing flow data if editing. */
  existingFlowData?: FlowDefinitionResponse;
  /** Whether the existing flow data is still loading. */
  isLoadingExistingFlow: boolean;
  /** Function to set nodes in the flow. */
  setNodes: React.Dispatch<React.SetStateAction<Node[]>>;
  /** Function to set edges in the flow. */
  setEdges: React.Dispatch<React.SetStateAction<Edge[]>>;
  /** Function to update node internals for rendering. */
  updateNodeInternals: UpdateNodeInternals;
  /** Function to generate edges from steps. */
  generateEdges: (flowSteps: Step[]) => Edge[];
  /** Function to validate edges against nodes. */
  validateEdges: (edges: Edge[], nodes: Node[]) => Edge[];
  /** Current edge style. */
  edgeStyle: string;
  /** Callback when auto-layout is needed. */
  onNeedsAutoLayout: (needsLayout: boolean) => void;
}

/**
 * Return type for the useFlowInitialization hook.
 */
export interface UseFlowInitializationReturn {
  /** Initial nodes based on the basic template. */
  initialNodes: Node[];
  /** Generate steps with START node and metadata. */
  generateSteps: (stepNodes: Node[]) => Node[];
  /** Get components from the blank template. */
  getBlankTemplateComponents: () => Element[];
  /** Update all node internals for rendering. */
  updateAllNodeInternals: (nodes: Node[]) => void;
}

/**
 * Hook to handle flow initialization logic including loading existing flows,
 * generating initial nodes from templates, and managing node internals updates.
 *
 * @param props - Configuration options for the hook.
 * @returns Flow initialization utilities.
 */
const useFlowInitialization = (props: UseFlowInitializationProps): UseFlowInitializationReturn => {
  const {
    resources,
    flowId,
    existingFlowData,
    isLoadingExistingFlow,
    setNodes,
    setEdges,
    updateNodeInternals,
    generateEdges,
    validateEdges,
    edgeStyle,
    onNeedsAutoLayout,
  } = props;

  const INITIAL_FLOW_START_STEP_ID = FlowConstants.START_STEP_ID;

  const flowUpdatesInProgress = useRef<boolean>(false);
  const nodesUpdatedRef = useRef<boolean>(false);
  // Use a ref to track the current edge style to avoid re-running initialization when it changes
  const edgeStyleRef = useRef<string>(edgeStyle);

  // Keep the ref in sync with the prop
  useEffect(() => {
    edgeStyleRef.current = edgeStyle;
  }, [edgeStyle]);

  /**
   * Helper function to update node internals for a given node and its components.
   */
  const updateAllNodeInternals = useCallback(
    (updatedNodes: Node[]): void => {
      updatedNodes.forEach((node: Node) => {
        updateNodeInternals(node.id);

        if (node.data?.['components']) {
          (node.data['components'] as Element[]).forEach((component: Element) => {
            updateNodeInternals(component.id);

            if (component?.components) {
              component.components.forEach((nestedComponent: Element) => {
                updateNodeInternals(nestedComponent.id);
              });
            }
          });
        }
      });
    },
    [updateNodeInternals],
  );

  /**
   * Determines if a step is deletable based on its type.
   */
  const isStepDeletable = useCallback((): boolean => true, []);

  /**
   * Get components from the blank template.
   */
  const getBlankTemplateComponents = useCallback((): Element[] => {
    const blankTemplate: Template | undefined = cloneDeep(
      resources?.templates?.find((template: Template) => template.type === TemplateTypes.Blank),
    );

    if (!blankTemplate) {
      return [];
    }

    if (blankTemplate.config && blankTemplate.config.data?.steps?.length > 0) {
      blankTemplate.config.data.steps[0].id = INITIAL_FLOW_START_STEP_ID;
    }

    return resolveComponentMetadata(
      resources,
      generateIdsForResources<Template>(blankTemplate)?.config?.data?.steps[0]?.data?.components,
    );
  }, [resources, INITIAL_FLOW_START_STEP_ID]);

  /**
   * Generate steps with START node and resolved metadata.
   */
  const generateSteps = useCallback(
    (stepNodes: Node[]): Node[] => {
      // Check if template already includes a START step with custom position
      const existingStartStep = stepNodes.find((step) => step.type === StaticStepTypes.Start);
      const startPosition = existingStartStep?.position ?? {x: -300, y: 330};

      const START_STEP: Node = {
        data: {
          displayOnly: true,
        },
        deletable: false,
        id: INITIAL_FLOW_START_STEP_ID,
        position: startPosition,
        type: StaticStepTypes.Start,
      };

      // Filter out START step from stepNodes if it exists (we'll add our own)
      const nonStartSteps = stepNodes.filter((step) => step.type !== StaticStepTypes.Start);

      return generateIdsForResources<Node[]>(
        resolveStepMetadata(resources, [
          START_STEP,
          ...nonStartSteps.map((step: Node) => ({
            data: step.data?.['components']
              ? {
                  ...step.data,
                  components: resolveComponentMetadata(resources, step.data['components'] as Element[]),
                }
              : (step.data ?? {}),
            deletable: step.type === StepTypes.End ? false : isStepDeletable(),
            id: step.id,
            position: step.position,
            type: step.type,
          })),
        ] as Step[]),
      );
    },
    [resources, INITIAL_FLOW_START_STEP_ID, isStepDeletable],
  );

  /**
   * Initial nodes based on the basic template.
   */
  const initialNodes: Node[] = useMemo<Node[]>(() => {
    const basicTemplate: Template | undefined = cloneDeep(
      resources?.templates?.find((template: Template) => template.type === TemplateTypes.Basic),
    );

    const templateSteps: Step[] = basicTemplate?.config?.data?.steps ?? [];

    const replacers: TemplateReplacer[] | undefined = basicTemplate?.config?.data?.__generationMeta__?.replacers;

    // generateSteps handles {{ID}} placeholders via generateIdsForResources
    // updateTemplatePlaceholderReferences handles named placeholders like {{GOOGLE_EXECUTION_STEP_ID}}
    const generatedSteps = generateSteps(templateSteps);

    return replacers ? updateTemplatePlaceholderReferences(generatedSteps, replacers)[0] : generatedSteps;
  }, [generateSteps, resources?.templates]);

  /**
   * Uses a sequence of state updates and RAF calls.
   */
  const updateFlowWithSequence = useCallback(
    (updatedSteps: Node[]): void => {
      if (flowUpdatesInProgress.current) {
        return;
      }

      flowUpdatesInProgress.current = true;
      nodesUpdatedRef.current = false;

      setNodes(() => {
        nodesUpdatedRef.current = true;
        return updatedSteps;
      });

      const updateSequence = () => {
        if (!nodesUpdatedRef.current) {
          requestAnimationFrame(updateSequence);
          return;
        }

        updateAllNodeInternals(updatedSteps);

        requestAnimationFrame(() => {
          const generatedEdges: Edge[] = generateEdges(updatedSteps as Step[]);
          const validatedEdges: Edge[] = validateEdges(generatedEdges, updatedSteps);

          // Apply current edge style to all edges (using ref to avoid dependency)
          const styledEdges = validatedEdges.map((edge) => ({
            ...edge,
            type: edgeStyleRef.current,
          }));

          setEdges(() => styledEdges);

          flowUpdatesInProgress.current = false;
        });
      };

      requestAnimationFrame(updateSequence);
    },
    [setNodes, setEdges, updateAllNodeInternals, generateEdges, validateEdges],
  );

  /**
   * Effect that updates the flow with the existing flow data or default template.
   */
  useLayoutEffect(() => {
    // Skip if we're still loading the existing flow data
    if (flowId && isLoadingExistingFlow) {
      return;
    }

    // If we have an existing flow, transform and load it
    if (flowId && existingFlowData) {
      const canvasData = transformFlowToCanvas(existingFlowData);

      // Check if any nodes lack layout data (position at origin indicates missing layout)
      const nodesWithoutLayout = existingFlowData.nodes.filter((node: FlowNode) => !node.layout?.position);
      const flowNeedsAutoLayout = nodesWithoutLayout.length > 1;
      onNeedsAutoLayout(flowNeedsAutoLayout);

      // Process nodes to resolve metadata without adding a new START node
      const processedNodes = generateIdsForResources<Node[]>(
        resolveStepMetadata(
          resources,
          canvasData.nodes.map((node: Node) => ({
            data: node.data?.['components']
              ? {
                  ...node.data,
                  components: resolveComponentMetadata(resources, node.data['components'] as Element[]),
                }
              : (node.data ?? {}),
            deletable: node.type !== StepTypes.End && node.type !== StaticStepTypes.Start,
            id: node.id,
            position: node.position,
            type: node.type,
          })) as Step[],
        ),
      );

      // Set nodes and edges from the transformed flow data
      setNodes(processedNodes);
      setEdges(
        canvasData.edges.map((edge) => ({
          ...edge,
          type: edgeStyleRef.current,
        })),
      );

      // Update all node internals after setting nodes
      setTimeout(() => {
        updateAllNodeInternals(processedNodes);
      }, 0);
    } else if (!flowId) {
      // No flowId provided - load the default basic auth flow template
      updateFlowWithSequence(initialNodes);
    }
    // Note: edgeStyle is intentionally excluded from dependencies - edge style changes
    // are handled separately in FlowBuilder.tsx to avoid re-initializing the flow
  }, [
    flowId,
    existingFlowData,
    isLoadingExistingFlow,
    resources,
    setNodes,
    setEdges,
    updateAllNodeInternals,
    updateFlowWithSequence,
    initialNodes,
    onNeedsAutoLayout,
  ]);

  return {
    initialNodes,
    generateSteps,
    getBlankTemplateComponents,
    updateAllNodeInternals,
  };
};

export default useFlowInitialization;
