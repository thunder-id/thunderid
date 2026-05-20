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

import type {BaseConfig} from './base';
import type {FlowTypes} from './metadata';

/**
 * Enumeration of available flow types in the platform.
 *
 * @public
 * @remarks
 * These flow types define the category of user journey being handled:
 * - AUTHENTICATION: For user login and sign-in processes
 * - REGISTRATION: For user signup and account creation processes
 *
 * @example
 * ```typescript
 * // Filter flows by type
 * const authFlows = flows.filter(flow => flow.flowType === FlowType.AUTHENTICATION);
 * const regFlows = flows.filter(flow => flow.flowType === FlowType.REGISTRATION);
 * ```
 */
export const FlowType = {
  /**
   * Authentication flows handle user login and sign-in processes
   */
  AUTHENTICATION: 'AUTHENTICATION',

  /**
   * Registration flows handle user signup and account creation processes
   */
  REGISTRATION: 'REGISTRATION',

  /**
   * User onboarding flows handle invited user provisioning within an organization
   */
  USER_ONBOARDING: 'USER_ONBOARDING',

  /**
   * Recovery flows handle password and account recovery processes
   */
  RECOVERY: 'RECOVERY',
} as const;

/**
 * Type representing the keys of FlowType enumeration.
 * @public
 */
export type FlowType = (typeof FlowType)[keyof typeof FlowType];

/**
 * Enumeration of node types available in flow definitions.
 *
 * @public
 * @remarks
 * Each flow is composed of connected nodes that define the user journey:
 * - START: Entry point of the flow
 * - PROMPT: Interactive UI components for user input
 * - TASK_EXECUTION: Background server operations
 * - END: Terminal point of the flow
 *
 * @example
 * ```typescript
 * const startNode = {
 *   id: 'node_001',
 *   type: NodeType.START,
 *   onSuccess: 'node_002'
 * };
 * ```
 */
export const FlowNodeType = {
  /**
   * Initial node indicating the starting point of the flow
   */
  START: 'START',
  /**
   * Interactive UI node that displays components and collects user input
   */
  PROMPT: 'PROMPT',

  /**
   * Background executor node that performs server-side operations
   */
  TASK_EXECUTION: 'TASK_EXECUTION',

  /**
   * Terminal node indicating the end of the flow
   */
  END: 'END',
} as const;

/**
 * Type representing the keys of NodeType enumeration.
 * @public
 */
export type FlowNodeType = (typeof FlowNodeType)[keyof typeof FlowNodeType];

/**
 * Interface for Flow completion configurations.
 * Flow completion configs originate from end-step resources, which expose `BaseConfig`
 * metadata in addition to arbitrary backend data.
 */
export type FlowCompletionConfigsInterface = BaseConfig | Record<string, unknown>;

/**
 * Interface for Flow local history.
 */
export interface FlowsHistoryInterface {
  /**
   * Author of the change.
   */
  author: {
    userName: string;
  };
  /**
   * Entire flow as an object.
   */
  flowData: Record<string, unknown>;
  /**
   * Flow saved at timestamp.
   */
  timestamp: number;
}

export interface FlowConfigInterface {
  flowType: FlowTypes;
  isEnabled: boolean;
  flowCompletionConfigs: FlowCompletionConfigsInterface;
}
