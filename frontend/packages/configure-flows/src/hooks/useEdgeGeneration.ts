// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {MarkerType, type Edge, type Node} from '@xyflow/react';
import {useCallback} from 'react';
import FlowConstants from '../constants/FlowConstants';
import VisualFlowConstants from '../constants/VisualFlowConstants';
import {ElementTypes, type Element} from '../models/elements';
import {StepTypes, type Step} from '../models/steps';

// Use centralized constants
const {
  START_STEP_ID: INITIAL_FLOW_START_STEP_ID,
  END_STEP_ID: INITIAL_FLOW_USER_ONBOARD_STEP_ID,
  DEFAULT_EDGE_TYPE,
} = FlowConstants;

/**
 * Helper to create an edge with standard configuration.
 */
const createEdge = (id: string, source: string, sourceHandle: string, target: string): Edge => ({
  animated: false,
  id,
  markerEnd: {
    type: MarkerType.Arrow,
  },
  source,
  sourceHandle,
  target,
  type: DEFAULT_EDGE_TYPE,
});

/**
 * Props for useEdgeGeneration hook.
 */
export interface UseEdgeGenerationProps {
  /**
   * The ID of the start step.
   * @default 'START'
   */
  startStepId?: string;
  /**
   * The ID of the end/user onboard step.
   * @default 'END'
   */
  endStepId?: string;
}

/**
 * Return type for useEdgeGeneration hook.
 */
export interface UseEdgeGenerationReturn {
  /**
   * Generates edges for the flow based on step configuration.
   */
  generateEdges: (flowSteps: Step[]) => Edge[];
  /**
   * Validates edges to ensure all targets exist.
   * Accepts Node[] or Step[] since it only needs the id property.
   */
  validateEdges: (edges: Edge[], nodes: Node[]) => Edge[];
}

/**
 * Hook for generating and validating edges in the login flow builder.
 *
 * This hook encapsulates the edge generation logic which:
 * 1. Creates edge from START to first step
 * 2. Creates edges based on button actions within steps
 * 3. Creates edges based on step-level actions
 * 4. Ensures connection to END step if not explicitly configured
 *
 * @param props - Configuration for edge generation.
 * @returns Functions for generating and validating edges.
 */
const useEdgeGeneration = (props?: UseEdgeGenerationProps): UseEdgeGenerationReturn => {
  const startStepId = props?.startStepId ?? INITIAL_FLOW_START_STEP_ID;
  const defaultEndStepId = props?.endStepId ?? INITIAL_FLOW_USER_ONBOARD_STEP_ID;

  /**
   * Generate edges for the flow based on step configurations.
   */
  const generateEdges = useCallback(
    (flowSteps: Step[]): Edge[] => {
      const generatedEdges: Edge[] = [];

      // Get all step IDs for validation
      const stepIds = new Set(flowSteps.map((step: Step) => step.id));

      // Find the user onboard step
      const userOnboardStep = flowSteps.find((step: Step) => step.type === StepTypes.End);
      const userOnboardStepId = userOnboardStep?.id ?? defaultEndStepId;

      // Track if we've created an edge to the user onboard step
      let userOnboardEdgeCreated = false;

      /**
       * Recursively find all ACTION and RESEND buttons within a component tree.
       * This handles nested BLOCK structures where buttons may be deeply nested.
       */
      const findActionButtons = (component: Element): Element[] => {
        const buttons: Element[] = [];

        // Check if this component is an ACTION or RESEND button
        if (component.type === ElementTypes.Action || component.type === ElementTypes.Resend) {
          buttons.push(component);
        }

        // Recursively search nested components (handles BLOCK containers)
        if (component.components) {
          component.components.forEach((nestedComponent) => {
            buttons.push(...findActionButtons(nestedComponent));
          });
        }

        return buttons;
      };

      /**
       * Create edges for a button based on its action configuration.
       */
      const createEdgesForButton = (step: Step, button: Element): void => {
        const sourceHandle = `${button.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`;

        if (button.action?.onSuccess) {
          if (stepIds.has(button.action.onSuccess)) {
            // Valid step reference
            generatedEdges.push(createEdge(button.id, step.id, sourceHandle, button.action.onSuccess));
            if (button.action.onSuccess === userOnboardStepId) {
              userOnboardEdgeCreated = true;
            }
          } else if (button.action.onSuccess === StepTypes.End) {
            // Reference to END type - connect to actual end step
            generatedEdges.push(createEdge(button.id, step.id, sourceHandle, userOnboardStepId));
            userOnboardEdgeCreated = true;
          }
        } else {
          // No explicit next - connect to user onboard step
          generatedEdges.push(createEdge(button.id, step.id, sourceHandle, userOnboardStepId));
          userOnboardEdgeCreated = true;
        }
      };

      // Create edge from START to first step
      if (flowSteps.length > 0) {
        let [firstStep] = flowSteps;

        // Skip START node if it's the first step
        if (firstStep.id === startStepId && flowSteps.length > 1) {
          [, firstStep] = flowSteps;
        }

        if (firstStep && firstStep.id !== startStepId) {
          generatedEdges.push(
            createEdge(
              `${startStepId}-${firstStep.id}`,
              startStepId,
              `${startStepId}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
              firstStep.id,
            ),
          );
        }
      }

      // Process each step for action-based edges
      flowSteps
        .filter((step) => step.type !== StepTypes.End)
        .forEach((step) => {
          // Process components with actions - recursively find all ACTION and RESEND buttons
          // This handles nested BLOCK structures (e.g., social login buttons wrapped in ACTION BLOCK containers)
          if (step.data?.components) {
            step.data.components.forEach((component) => {
              const actionButtons = findActionButtons(component);
              actionButtons.forEach((button) => createEdgesForButton(step, button));
            });
          }

          // Process step-level actions
          if (step.data?.action?.onSuccess) {
            const sourceHandle = `${step.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`;

            if (stepIds.has(step.data.action.onSuccess)) {
              generatedEdges.push(
                createEdge(
                  `${step.id}-to-${step.data.action.onSuccess}`,
                  step.id,
                  sourceHandle,
                  step.data.action.onSuccess,
                ),
              );
              if (step.data.action.onSuccess === userOnboardStepId) {
                userOnboardEdgeCreated = true;
              }
            } else if (step.data.action.onSuccess === StepTypes.End) {
              generatedEdges.push(
                createEdge(`${step.id}-to-${userOnboardStepId}`, step.id, sourceHandle, userOnboardStepId),
              );
              userOnboardEdgeCreated = true;
            }
          }

          // Process step-level onFailure actions
          if (step.data?.action?.onFailure && stepIds.has(step.data.action.onFailure)) {
            generatedEdges.push(
              createEdge(
                `${step.id}-failure-to-${step.data.action.onFailure}`,
                step.id,
                'failure',
                step.data.action.onFailure,
              ),
            );
          }
        });

      // If no edge to user onboard was created, connect the last view step
      if (!userOnboardEdgeCreated && flowSteps.length > 0) {
        const viewSteps = flowSteps.filter((step: Step) => step.type === StepTypes.View);

        if (viewSteps.length > 0) {
          const lastViewStep = viewSteps[viewSteps.length - 1];
          let buttonId: string | null = null;

          // Try to find a button to use for the connection (recursively search nested structures)
          if (lastViewStep.data?.components) {
            const firstActionButton = lastViewStep.data.components
              .flatMap((component) => findActionButtons(component))
              .find((btn) => btn !== undefined);
            if (firstActionButton) {
              buttonId = firstActionButton.id;
            }
          }

          const edgeId = buttonId ?? `${lastViewStep.id}-to-${userOnboardStepId}`;
          const sourceHandle = buttonId
            ? `${buttonId}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`
            : `${lastViewStep.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`;

          generatedEdges.push(createEdge(edgeId, lastViewStep.id, sourceHandle, userOnboardStepId));
        }
      }

      return generatedEdges;
    },
    [startStepId, defaultEndStepId],
  );

  /**
   * Validate edges to ensure all targets exist in the flow.
   * Accepts Node[] or Step[] since it only needs the id property.
   */
  const validateEdges = useCallback((edges: Edge[], nodes: Node[]): Edge[] => {
    const stepIds = new Set(nodes.map((node) => node.id));

    // Always include START and END as valid targets
    stepIds.add(INITIAL_FLOW_START_STEP_ID);
    stepIds.add(INITIAL_FLOW_USER_ONBOARD_STEP_ID);

    return edges.filter((edge) => {
      const targetExists = stepIds.has(edge.target);
      const sourceExists = stepIds.has(edge.source);

      return targetExists && sourceExists;
    });
  }, []);

  return {
    generateEdges,
    validateEdges,
  };
};

export default useEdgeGeneration;
