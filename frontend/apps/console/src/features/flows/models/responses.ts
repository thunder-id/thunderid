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

import type {ApiPaginationLink} from '@thunderid/types';
import type {FlowNodeType, FlowType} from './flows';

/**
 * Basic flow definition returned in list responses.
 */
export interface BasicFlowDefinition {
  /**
   * Unique identifier for the flow
   */
  id: string;
  /**
   * Type of flow (AUTHENTICATION or REGISTRATION)
   */
  flowType: FlowType;
  /**
   * Name of the flow
   */
  name: string;
  /**
   * URL-friendly handle for the flow (auto-generated from name)
   */
  handle: string;
  /**
   * The version number that is currently active
   */
  activeVersion: number;
  /**
   * Timestamp when the flow was initially created
   */
  createdAt: string;
  /**
   * Timestamp when the flow was last modified
   */
  updatedAt: string;

  /**
   * Whether the flow is read-only and cannot be modified or deleted.
   */
  isReadOnly?: boolean;
}

/**
 * Flow List Response
 *
 * Response structure for paginated flow list queries.
 * Contains pagination metadata along with the list of flows.
 */
export interface FlowListResponse {
  /**
   * Number of results that match the listing operation
   */
  totalResults: number;
  /**
   * Index of the first element of the page (offset + 1)
   */
  startIndex: number;
  /**
   * Number of elements in the returned page
   */
  count: number;
  /**
   * Array of basic flow information
   */
  flows: BasicFlowDefinition[];
  /**
   * Navigation links for pagination
   */
  links?: ApiPaginationLink[];
}

/**
 * Layout information for a flow node.
 */
export interface FlowNodeLayout {
  /**
   * Size of the node
   */
  size: {
    width: number;
    height: number;
  };
  /**
   * Position of the node
   */
  position: {
    x: number;
    y: number;
  };
}

/**
 * UI metadata for PROMPT nodes.
 */
export interface FlowNodeMeta {
  /**
   * UI components to render
   */
  components?: unknown[];
}

/**
 * Input definition for flow nodes.
 */
export interface FlowNodeInput {
  /**
   * Reference to the input component ID
   */
  ref?: string;
  /**
   * Input type (TEXT_INPUT, PASSWORD_INPUT, OTP_INPUT, etc.)
   */
  type: string;
  /**
   * The mapped attribute identifier
   */
  identifier: string;
  /**
   * Whether this input is required
   */
  required: boolean;
}

/**
 * Action definition for PROMPT nodes.
 */
export interface FlowNodeAction {
  /**
   * Reference to the action component ID
   */
  ref: string;
  /**
   * ID of the next node to navigate to
   */
  nextNode: string;
  /**
   * Executor configuration for actions that trigger executors
   */
  executor?: FlowExecutor;
}

/**
 * A {@link FlowPrompt} represents a single logical prompt within a PROMPT node.
 * Inputs are associated with actions based on their shared container (BLOCK) in the
 * layout/metadata structure: only the inputs that are placed in the same BLOCK as
 * a given action (or its ancestors) are considered to belong to that action.
 *
 * This scoping rule is important when a PROMPT node contains multiple actions
 * (for example, several buttons) and different subsets of inputs should be
 * submitted or validated with each action.
 */
export interface FlowPrompt {
  /**
   * Input fields associated with this action. These are the inputs that share the
   * same container (BLOCK) hierarchy as the {@link FlowNodeAction} in the prompt structure.
   */
  inputs?: FlowNodeInput[];
  /**
   * The action for this prompt. This action is scoped to, and operates on, the
   * inputs defined in the same container (BLOCK) ancestry as this prompt.
   */
  action?: FlowNodeAction;
}

/**
 * Executor configuration for TASK_EXECUTION nodes.
 */
export interface FlowExecutor {
  /**
   * Name of the registered executor
   */
  name: string;
  /**
   * Input definitions for this executor
   */
  inputs?: FlowNodeInput[];
  /**
   * Additional executor properties
   */
  [key: string]: unknown;
}

/**
 * Flow node definition matching the API specification.
 */
export interface FlowNode {
  /**
   * Unique identifier for the node within the flow
   */
  id: string;
  /**
   * Type of node
   */
  type: FlowNodeType;
  /**
   * Layout information for the node (position and size)
   */
  layout?: FlowNodeLayout;
  /**
   * UI metadata for PROMPT nodes
   */
  meta?: FlowNodeMeta;
  /**
   * Prompt definitions for PROMPT nodes.
   * Each prompt groups inputs with an action button.
   */
  prompts?: FlowPrompt[];
  /**
   * Node-level properties for configuration
   */
  properties?: Record<string, unknown>;
  /**
   * Executor configuration for TASK_EXECUTION nodes
   */
  executor?: FlowExecutor;
  /**
   * Next node ID on successful execution
   */
  onSuccess?: string;
  /**
   * Next node ID on failed execution
   */
  onFailure?: string;
  /**
   * Next node ID on incomplete execution
   */
  onIncomplete?: string;
  /**
   * For display-only PROMPT nodes: ID of the next node. Mutually exclusive with 'prompts'.
   */
  next?: string;
  /**
   * For display-only PROMPT nodes: textual message for non-verbose mode.
   */
  message?: string;
  /**
   * Node-level condition for execution
   */
  condition?: {
    key: string;
    value: string;
    onSkip?: string;
  };
}

/**
 * Request body for creating a new flow.
 */
export interface CreateFlowRequest {
  /**
   * Name of the flow
   */
  name: string;
  /**
   * URL-friendly handle for the flow (auto-generated from name)
   */
  handle: string;
  /**
   * Type of flow
   */
  flowType: FlowType;
  /**
   * List of nodes that define the flow graph
   */
  nodes: FlowNode[];
}

/**
 * Request body for updating an existing flow.
 */
export interface UpdateFlowRequest {
  /**
   * Name of the flow
   */
  name: string;
  /**
   * URL-friendly handle for the flow (auto-generated from name)
   */
  handle: string;
  /**
   * Type of flow
   */
  flowType: FlowType;
  /**
   * List of nodes that define the flow graph
   */
  nodes: FlowNode[];
}

/**
 * Full flow definition response from the API.
 */
export interface FlowDefinitionResponse {
  /**
   * Unique identifier for the flow
   */
  id: string;
  /**
   * Name of the flow
   */
  name: string;
  /**
   * URL-friendly handle for the flow (auto-generated from name)
   */
  handle: string;
  /**
   * Type of flow
   */
  flowType: FlowType;
  /**
   * The version number that is currently active
   */
  activeVersion: number;
  /**
   * List of nodes that define the flow graph
   */
  nodes: FlowNode[];
  /**
   * Timestamp when the flow was initially created
   */
  createdAt: string;
  /**
   * Timestamp when the flow was last modified
   */
  updatedAt: string;

  /**
   * Whether the flow is read-only and cannot be modified or deleted.
   */
  isReadOnly?: boolean;
}

/**
 * A single resource that references a flow.
 */
export interface FlowUsage {
  resourceType: string;
  id: string;
  displayName: string;
  behaviorOnDelete: 'fallback' | 'cascade';
}

/**
 * Per-resource-type count of usages, keyed by resource type
 * (e.g. `application`, `agent`). Null when the counts could not be determined.
 */
export type FlowUsagesSummary = Record<string, number> | null;

/**
 * Response for the flow usages endpoint.
 * totalResults is null when usage data is unavailable; 0 means confirmed empty.
 */
export interface FlowUsagesResponse {
  totalResults: number | null;
  count: number;
  summary: FlowUsagesSummary;
  usages: FlowUsage[];
}
