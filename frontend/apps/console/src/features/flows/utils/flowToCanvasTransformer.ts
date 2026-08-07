// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import {MarkerType} from '@xyflow/react';
import VisualFlowConstants from '../constants/VisualFlowConstants';
import executors from '../data/executors.json';
import type {Element} from '../models/elements';
import {ElementTypes} from '../models/elements';
import type {FlowDefinitionResponse, FlowNode, FlowNodeAction, FlowPrompt} from '../models/responses';
import type {StepData} from '../models/steps';
import {StepTypes, StaticStepTypes} from '../models/steps';

/**
 * Set of input element types for quick lookup
 */
const INPUT_ELEMENT_TYPES = new Set<string>([
  ElementTypes.TextInput,
  ElementTypes.PasswordInput,
  ElementTypes.EmailInput,
  ElementTypes.PhoneInput,
  ElementTypes.NumberInput,
  ElementTypes.DateInput,
  ElementTypes.OtpInput,
  ElementTypes.Checkbox,
  ElementTypes.Dropdown,
]);

/**
 * Extended node type with custom properties used by the canvas
 */
interface CanvasNode extends Node<StepData> {
  resourceType?: string;
  category?: string;
}

/**
 * React Flow canvas data structure
 */
interface ReactFlowCanvasData {
  nodes: CanvasNode[];
  edges: Edge[];
  viewport: {
    x: number;
    y: number;
    zoom: number;
  };
}

/**
 * Maps API node types to canvas step types
 */
const NODE_TO_STEP_TYPE_MAP: Record<string, string> = {
  PROMPT: StepTypes.View,
  TASK_EXECUTION: StepTypes.Execution,
  DECISION: StepTypes.Rule,
  END: StepTypes.End,
  CALL: StepTypes.Call,
  START: StaticStepTypes.Start,
};

/**
 * Default layout dimensions for nodes without layout info
 */
const DEFAULT_LAYOUT = {
  width: 200,
  height: 100,
  position: {x: 0, y: 0},
};

/**
 * Branching outcomes whose presence on a node's action drives the failure/incomplete output handles
 */
const BRANCHING_OUTCOME_KEYS = ['onFailure', 'onIncomplete'] as const;

/**
 * Shape of an executor definition read from the resource catalog
 */
type ExecutorDefinition = {
  data?: {
    action?: {
      executor?: {name?: string; mode?: string};
      onFailure?: string;
      onIncomplete?: string;
    };
  };
};

/**
 * Builds the catalog lookup key for an executor. Mode is part of the key because the same executor
 * can declare different outcomes per mode (e.g. `OTPExecutor` generate vs verify).
 */
function getExecutorKey(name?: string, mode?: string): string {
  return `${name ?? ''}::${mode ?? ''}`;
}

/**
 * Branching outcomes each executor declares, keyed by executor name and mode.
 *
 * Supporting an outcome is a property of the executor definition, not of whichever edges happened
 * to be connected when the flow was saved, so this is the authoritative source on load.
 */
const EXECUTOR_DECLARED_OUTCOMES: Map<string, string[]> = new Map(
  (executors as ExecutorDefinition[]).map((definition: ExecutorDefinition) => {
    const action = definition.data?.action;

    return [
      getExecutorKey(action?.executor?.name, action?.executor?.mode),
      BRANCHING_OUTCOME_KEYS.filter((key: string) => action?.[key as 'onFailure' | 'onIncomplete'] !== undefined),
    ];
  }),
);

/**
 * Resolves the branching outcome keys for a TASK_EXECUTION node.
 *
 * A flow only persists an outcome key when it was wired to an edge at save time, so a node saved
 * without a failure/incomplete connection loses the key and the canvas stops rendering its handle.
 * Re-deriving the keys the executor declares keeps those handles available on every load, while
 * persisted targets still win over the empty defaults. Nodes whose executor is not in the catalog
 * keep exactly the keys the flow persisted.
 */
function resolveBranchingOutcomes(apiNode: FlowNode): Record<string, string> {
  const outcomes: Record<string, string> = {};
  const declaredOutcomes = EXECUTOR_DECLARED_OUTCOMES.get(
    getExecutorKey(apiNode.executor?.name, apiNode.executor?.mode as string | undefined),
  );

  declaredOutcomes?.forEach((key: string) => {
    outcomes[key] = '';
  });

  BRANCHING_OUTCOME_KEYS.forEach((key: 'onFailure' | 'onIncomplete') => {
    if (apiNode[key] !== undefined) {
      outcomes[key] = apiNode[key] as string;
    }
  });

  return outcomes;
}

/**
 * Normalizes INPUT element properties to top-level format.
 * Properties are kept at the top level (new format) and inputType is derived from variant.
 */
function normalizeInputProperties(component: Record<string, unknown>): Record<string, unknown> {
  // Derive inputType from variant if not already present
  const {inputType: existingInputType} = component;
  let inputType = existingInputType;
  if (inputType === undefined) {
    if (component.variant === 'PASSWORD') {
      inputType = 'password';
    } else if (component.variant === 'EMAIL') {
      inputType = 'email';
    } else if (component.variant === 'TELEPHONE') {
      inputType = 'tel';
    } else if (component.variant === 'NUMBER') {
      inputType = 'number';
    } else {
      inputType = 'text';
    }
  }

  return {
    ...component,
    inputType,
  };
}

/**
 * Reconstructs the action property for button components based on the node's actions array.
 */
function restoreButtonAction(
  component: Record<string, unknown>,
  nodeActions: FlowNodeAction[] | undefined,
): Record<string, unknown> {
  const componentId = component.id as string;
  const matchingAction = nodeActions?.find((action) => action.ref === componentId);

  if (matchingAction) {
    return {
      ...component,
      // Backfill the prompt action's type onto the element, which is where the property panel reads
      // it from and where serialization projects it back out of. Only when the element does not
      // already carry one: an unwired button keeps its type on the element alone, since a prompt
      // action is only emitted once the button has a nextNode. Clearing the selector writes an empty
      // string rather than dropping the key, and that empty string is persisted, so treat it as no
      // type rather than as a type worth preserving.
      ...(matchingAction.type && !component.actionType ? {actionType: matchingAction.type} : {}),
      action: {
        type: matchingAction.executor ? 'EXECUTOR' : 'NEXT',
        onSuccess: matchingAction.nextNode,
        ...(matchingAction.executor && {executor: matchingAction.executor}),
      },
    };
  }

  return component;
}

/**
 * Walks a component tree looking for a RICH_TEXT whose author-defined `action.ref` matches
 * the given ref. Used at edge-generation time so the resulting edge attaches to the
 * component-scoped source handle (`${component.id}${NEXT_HANDLE_SUFFIX}`) rather than an
 * `action.ref`-scoped one that doesn't exist for rich text.
 */
function findRichTextComponentByActionRef(components: Element[] | undefined, actionRef: string): Element | undefined {
  if (!components || components.length === 0) {
    return undefined;
  }

  for (const component of components) {
    const richAction = (component as {action?: {ref?: string}}).action;
    if (component.type === ElementTypes.RichText && richAction?.ref === actionRef) {
      return component;
    }
    if (component.components) {
      const found = findRichTextComponentByActionRef(component.components, actionRef);
      if (found) {
        return found;
      }
    }
  }

  return undefined;
}

/**
 * Extracts actions from the prompts array.
 * Flattens the prompts structure into a simple list of actions.
 */
function extractActionsFromPrompts(prompts: FlowPrompt[] | undefined): FlowNodeAction[] {
  if (!prompts || prompts.length === 0) {
    return [];
  }

  return prompts.filter((prompt) => prompt.action !== undefined).map((prompt) => prompt.action!);
}

/**
 * Gets actions from a node's prompts array.
 */
function getNodeActions(apiNode: FlowNode): FlowNodeAction[] | undefined {
  if (apiNode.prompts && apiNode.prompts.length > 0) {
    return extractActionsFromPrompts(apiNode.prompts);
  }
  return undefined;
}

/**
 * Recursively restores components from the API format to canvas format.
 * This handles nested components (like forms containing inputs and buttons).
 */
function restoreComponents(components: unknown[] | undefined, nodeActions: FlowNodeAction[] | undefined): Element[] {
  if (!components || components.length === 0) {
    return [];
  }

  return components.map((comp) => {
    const component = comp as Record<string, unknown>;
    let restoredComponent: Record<string, unknown> = component;

    // Normalize INPUT element properties (ensure inputType is set)
    if (INPUT_ELEMENT_TYPES.has(component.type as string)) {
      restoredComponent = normalizeInputProperties(component);
    }

    // Restore action for ACTION elements
    if (component.type === ElementTypes.Action || component.type === ElementTypes.Resend) {
      restoredComponent = restoreButtonAction(restoredComponent, nodeActions);
    }

    // Recursively process nested components (e.g., Form components)
    if (restoredComponent.components && Array.isArray(restoredComponent.components)) {
      restoredComponent = {
        ...restoredComponent,
        components: restoreComponents(restoredComponent.components as unknown[], nodeActions),
      };
    }

    return restoredComponent as unknown as Element;
  });
}

/**
 * Transforms an API flow node to a React Flow canvas node.
 */
function transformNodeToCanvas(apiNode: FlowNode): CanvasNode {
  const stepType = NODE_TO_STEP_TYPE_MAP[apiNode.type] ?? apiNode.type;

  // Build the base node structure
  const canvasNode: CanvasNode = {
    id: apiNode.id,
    type: stepType,
    position: {
      x: apiNode.layout?.position?.x ?? DEFAULT_LAYOUT.position.x,
      y: apiNode.layout?.position?.y ?? DEFAULT_LAYOUT.position.y,
    },
    measured: apiNode.layout?.size
      ? {
          width: apiNode.layout.size.width,
          height: apiNode.layout.size.height,
        }
      : undefined,
    deletable: stepType !== StaticStepTypes.Start && stepType !== StepTypes.End,
    data: {},
  };

  // Handle START node
  if (stepType === StaticStepTypes.Start) {
    canvasNode.data = {
      displayOnly: true,
    };
    canvasNode.deletable = false;
  }

  // Handle PROMPT/VIEW nodes with UI components
  if (stepType === StepTypes.View && apiNode.meta?.components) {
    // Get actions from prompts
    const nodeActions = getNodeActions(apiNode);
    const restoredComponents = restoreComponents(apiNode.meta.components, nodeActions);

    canvasNode.data = {
      components: restoredComponents,
    };

    // Also set the node's resourceType and other step metadata for VIEW
    canvasNode.resourceType = 'STEP';
    canvasNode.category = 'INTERFACE';
  }

  // Handle TASK_EXECUTION nodes
  if (stepType === StepTypes.Execution) {
    canvasNode.data = {
      action: {
        type: 'EXECUTOR',
        executor: apiNode.executor,
        onSuccess: apiNode.onSuccess,
        // Carry the branching outcomes the executor declares, not just the ones the flow
        // persisted. Their presence drives the failure/incomplete output handles, so an executor
        // that supports them keeps its handles even when they were never connected.
        ...resolveBranchingOutcomes(apiNode),
      },
    };

    if (apiNode.properties) {
      canvasNode.data.properties = apiNode.properties;
    }

    canvasNode.resourceType = 'STEP';
    canvasNode.category = 'WORKFLOW';
  }

  // Handle CALL nodes (cross-flow invocation)
  if (stepType === StepTypes.Call) {
    canvasNode.data = {
      flow: apiNode.flow ?? {ref: ''},
      action: {
        type: 'CALL',
        flow: apiNode.flow ?? {ref: ''},
        onSuccess: apiNode.onSuccess,
        onFailure: apiNode.onFailure,
      },
    };
    canvasNode.resourceType = 'STEP';
    canvasNode.category = 'WORKFLOW';
  }

  // Handle END nodes
  if (stepType === StepTypes.End) {
    if (apiNode.meta?.components) {
      const nodeActions = getNodeActions(apiNode);
      const restoredComponents = restoreComponents(apiNode.meta.components, nodeActions);
      canvasNode.data = {
        components: restoredComponents,
      };
    }

    // Set executor for END node (LoginCompletionExecutor)
    canvasNode.data = {
      ...canvasNode.data,
      action: {
        type: 'EXECUTOR',
        executor: {
          name: 'LoginCompletionExecutor',
        },
      },
    };

    canvasNode.deletable = false;
    canvasNode.resourceType = 'STEP';
    canvasNode.category = 'INTERFACE';
  }

  return canvasNode;
}

/**
 * Generates edges from the flow nodes.
 * Edges are derived from node connections (onSuccess, actions, etc.)
 */
function generateEdgesFromNodes(apiNodes: FlowNode[]): Edge[] {
  const edges: Edge[] = [];
  const nodeIds = new Set<string>(apiNodes.map((node) => node.id));

  apiNodes.forEach((node) => {
    const stepType = NODE_TO_STEP_TYPE_MAP[node.type] ?? node.type;

    // Handle START node -> first step connection
    if (stepType === StaticStepTypes.Start && node.onSuccess && nodeIds.has(node.onSuccess)) {
      edges.push({
        id: `${node.id}-${node.onSuccess}`,
        source: node.id,
        sourceHandle: `${node.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
        target: node.onSuccess,
        type: 'smoothstep',
        animated: false,
        markerEnd: {
          type: MarkerType.Arrow,
        },
      });
    }

    // Handle PROMPT/VIEW node button actions
    if (stepType === StepTypes.View) {
      const nodeActions = getNodeActions(node);
      if (nodeActions) {
        nodeActions.forEach((action) => {
          if (action.nextNode && nodeIds.has(action.nextNode)) {
            // For RICH_TEXT the source handle is scoped by component.id (matches the
            // widget-drop convention). For ACTION buttons, action.ref already equals
            // component.id so the same lookup naturally works.
            const richTextSource = findRichTextComponentByActionRef(
              node.meta?.components as Element[] | undefined,
              action.ref,
            );
            const handleBase = richTextSource ? richTextSource.id : action.ref;
            edges.push({
              id: action.ref,
              source: node.id,
              sourceHandle: `${handleBase}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
              target: action.nextNode,
              type: 'smoothstep',
              animated: false,
              markerEnd: {
                type: MarkerType.Arrow,
              },
            });
          }
        });
      } else if (node.next && nodeIds.has(node.next)) {
        // Display-only PROMPT node: uses the node-level source handle
        edges.push({
          id: `${node.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
          source: node.id,
          sourceHandle: `${node.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
          target: node.next,
          type: 'smoothstep',
          animated: false,
          markerEnd: {
            type: MarkerType.Arrow,
          },
        });
      }
    }

    // Handle TASK_EXECUTION node -> next step connection
    if (stepType === StepTypes.Execution && node.onSuccess && nodeIds.has(node.onSuccess)) {
      edges.push({
        id: `${node.id}-to-${node.onSuccess}`,
        source: node.id,
        sourceHandle: `${node.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
        target: node.onSuccess,
        type: 'smoothstep',
        animated: false,
        markerEnd: {
          type: MarkerType.Arrow,
        },
      });

      // Handle onFailure for TASK_EXECUTION nodes (branching support)
      if (node.onFailure && nodeIds.has(node.onFailure)) {
        edges.push({
          id: `${node.id}-failure-to-${node.onFailure}`,
          source: node.id,
          sourceHandle: 'failure',
          target: node.onFailure,
          type: 'smoothstep',
          animated: false,
          markerEnd: {
            type: MarkerType.Arrow,
          },
        });
      }

      // Handle onIncomplete for TASK_EXECUTION nodes
      if (node.onIncomplete && nodeIds.has(node.onIncomplete)) {
        edges.push({
          id: `${node.id}-incomplete-to-${node.onIncomplete}`,
          source: node.id,
          sourceHandle: `${node.id}${VisualFlowConstants.FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX}`,
          target: node.onIncomplete,
          type: 'smoothstep',
          animated: false,
          markerEnd: {
            type: MarkerType.Arrow,
          },
        });
      }
    }

    // Handle DECISION/RULE node connections
    if (stepType === StepTypes.Rule && node.onSuccess && nodeIds.has(node.onSuccess)) {
      edges.push({
        id: `${node.id}-to-${node.onSuccess}`,
        source: node.id,
        sourceHandle: `${node.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
        target: node.onSuccess,
        type: 'smoothstep',
        animated: false,
        markerEnd: {
          type: MarkerType.Arrow,
        },
      });

      // Handle onFailure for decision nodes
      if (node.onFailure && nodeIds.has(node.onFailure)) {
        edges.push({
          id: `${node.id}-failure-to-${node.onFailure}`,
          source: node.id,
          sourceHandle: 'failure',
          target: node.onFailure,
          type: 'smoothstep',
          animated: false,
          markerEnd: {
            type: MarkerType.Arrow,
          },
        });
      }
    }

    // Handle CALL node connections
    if (stepType === StepTypes.Call) {
      if (node.onSuccess && nodeIds.has(node.onSuccess)) {
        edges.push({
          id: `${node.id}-to-${node.onSuccess}`,
          source: node.id,
          sourceHandle: `${node.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`,
          target: node.onSuccess,
          type: 'smoothstep',
          animated: false,
          markerEnd: {
            type: MarkerType.Arrow,
          },
        });
      }

      if (node.onFailure && nodeIds.has(node.onFailure)) {
        edges.push({
          id: `${node.id}-failure-to-${node.onFailure}`,
          source: node.id,
          sourceHandle: 'failure',
          target: node.onFailure,
          type: 'smoothstep',
          animated: false,
          markerEnd: {
            type: MarkerType.Arrow,
          },
        });
      }
    }
  });

  return edges;
}

/**
 * Calculates a reasonable viewport based on the node positions.
 */
function calculateViewport(nodes: CanvasNode[]): {x: number; y: number; zoom: number} {
  if (nodes.length === 0) {
    return {x: 0, y: 0, zoom: 1};
  }

  // Find the bounding box of all nodes
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;

  nodes.forEach((node) => {
    const width = node.measured?.width ?? DEFAULT_LAYOUT.width;
    const height = node.measured?.height ?? DEFAULT_LAYOUT.height;

    minX = Math.min(minX, node.position.x);
    minY = Math.min(minY, node.position.y);
    maxX = Math.max(maxX, node.position.x + width);
    maxY = Math.max(maxY, node.position.y + height);
  });

  // Center the viewport on the flow
  const centerX = (minX + maxX) / 2;
  const centerY = (minY + maxY) / 2;

  // Calculate zoom to fit (assuming a viewport of roughly 1200x800)
  const flowWidth = maxX - minX + 200; // Add padding
  const flowHeight = maxY - minY + 200;
  const viewportWidth = 1200;
  const viewportHeight = 800;

  const zoomX = viewportWidth / flowWidth;
  const zoomY = viewportHeight / flowHeight;
  const zoom = Math.min(Math.max(Math.min(zoomX, zoomY), 0.3), 1);

  return {
    x: viewportWidth / 2 - centerX * zoom,
    y: viewportHeight / 2 - centerY * zoom,
    zoom,
  };
}

/**
 * Main transformer function that converts flow definition data to React Flow canvas format.
 * This is the reverse of the reactFlowTransformer which converts canvas to API format.
 *
 * @param flowData - The flow definition response from the API
 * @returns The React Flow canvas data (nodes, edges, viewport)
 */
export function transformFlowToCanvas(flowData: FlowDefinitionResponse): ReactFlowCanvasData {
  // Transform API nodes to canvas nodes
  const canvasNodes: CanvasNode[] = flowData.nodes.map((node) => transformNodeToCanvas(node));

  // Generate edges from node connections
  const canvasEdges: Edge[] = generateEdgesFromNodes(flowData.nodes);

  // Calculate a reasonable viewport
  const viewport = calculateViewport(canvasNodes);

  return {
    nodes: canvasNodes,
    edges: canvasEdges,
    viewport,
  };
}

export type {ReactFlowCanvasData, CanvasNode};
