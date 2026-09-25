// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, EdgeTypes, Node, NodeTypes} from '@xyflow/react';
import type {Context, Dispatch, FunctionComponent, SetStateAction} from 'react';
import {createContext} from 'react';
import type {FlowCompletionConfigsInterface} from '../models/flows';
import type {MetadataInterface} from '../models/metadata';
import type {Resource} from '../models/resources';
import type {EdgeStyleTypes} from '../models/steps';
import type {GraphValidationRule} from '../validation/validation-rules';

/**
 * Props interface of {@link FlowConfigContext}
 */
export interface FlowConfigContextProps {
  /**
   * The factory for creating components.
   */
  ElementFactory: FunctionComponent<{resource?: Resource; stepId: string; [key: string]: unknown}>;
  /**
   * The wrapper for the resource properties factory.
   */
  ResourceProperties: FunctionComponent<{
    properties?: Record<string, unknown>;
    resource: Resource;
    onChange: (
      propertyKey: string,
      newValue: string | boolean | number | object,
      resource: Resource,
      debounce?: boolean,
    ) => void;
    onVariantChange?: (variant: string, resource?: Partial<Resource>) => void;
  }>;
  /**
   * Flow completion configurations.
   */
  flowCompletionConfigs: FlowCompletionConfigsInterface;
  /**
   * Set the flow completion configurations.
   */
  setFlowCompletionConfigs: Dispatch<SetStateAction<FlowCompletionConfigsInterface>>;
  /**
   * Metadata for the current flow builder.
   */
  metadata?: MetadataInterface;
  /**
   * Indicates whether the flow metadata is still loading.
   */
  isFlowMetadataLoading?: boolean;
  /**
   * Indicates whether the flow is in verbose mode (showing executors and their edges).
   */
  isVerboseMode: boolean;
  /**
   * Function to toggle or set the verbose mode state.
   */
  setIsVerboseMode: Dispatch<SetStateAction<boolean>>;
  /**
   * The current edge style type for the flow canvas.
   */
  edgeStyle: EdgeStyleTypes;
  /**
   * Function to set the edge style type for the flow canvas.
   */
  setEdgeStyle: Dispatch<SetStateAction<EdgeStyleTypes>>;
  /**
   * Whether node drags snap to the canvas background grid.
   */
  isSnapToGridEnabled: boolean;
  /**
   * Function to toggle or set the snap-to-grid state.
   */
  setIsSnapToGridEnabled: Dispatch<SetStateAction<boolean>>;
  /**
   * Whether the canvas navigation minimap is shown.
   */
  isMiniMapVisible: boolean;
  /**
   * Function to toggle or set the minimap visibility.
   */
  setIsMiniMapVisible: Dispatch<SetStateAction<boolean>>;
  /**
   * Node types active in the flow.
   */
  flowNodeTypes: NodeTypes;
  /**
   * Edge types active in the flow.
   */
  flowEdgeTypes: EdgeTypes;
  /**
   * Function to set the node types active in the flow.
   */
  setFlowNodeTypes: Dispatch<SetStateAction<NodeTypes>>;
  /**
   * Function to set the edge types active in the flow.
   */
  setFlowEdgeTypes: Dispatch<SetStateAction<EdgeTypes>>;
  /**
   * Function to add a resource (element, widget, template, step) to the flow.
   */
  addResourceToFlow?: (resource: Resource) => void;
  /**
   * Publishes the current flow configuration.
   */
  publishFlow?: () => Promise<boolean>;
  /**
   * Current React Flow nodes. Set by DecoratedVisualFlow so that
   * ValidationProvider can compute notifications without relying on
   * the inner ReactFlowProvider store.
   */
  flowNodes: Node[];
  /**
   * Setter to push the latest nodes into the shared context.
   */
  setFlowNodes: Dispatch<SetStateAction<Node[]>>;
  /**
   * Current React Flow edges, alongside {@link flowNodes}, so graph validation
   * rules can also check what a node or one of its elements connects to.
   */
  flowEdges: Edge[];
  /**
   * Setter to push the latest edges into the shared context.
   */
  setFlowEdges: Dispatch<SetStateAction<Edge[]>>;
  /**
   * Cross-node validation rules active for the current flow. Empty by
   * default; flow-type-specific hosts register the rules that apply
   * (e.g. the SSO pairing rules for AUTHENTICATION flows).
   */
  graphValidationRules: GraphValidationRule[];
  /**
   * Setter to register the graph validation rules for the current flow.
   */
  setGraphValidationRules: Dispatch<SetStateAction<GraphValidationRule[]>>;
}

const FlowConfigContext: Context<FlowConfigContextProps | undefined> = createContext<
  FlowConfigContextProps | undefined
>(undefined);

FlowConfigContext.displayName = 'FlowConfigContext';

export default FlowConfigContext;
