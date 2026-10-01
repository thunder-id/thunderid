// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

  /**
   * SignOut flows terminate an established SSO session
   */
  SIGNOUT: 'SIGNOUT',

  /**
   * Administration flows orchestrate authenticated administrative operations
   */
  ADMINISTRATION: 'ADMINISTRATION',
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

  /**
   * Cross-flow invocation node that transfers execution to another flow and resumes
   * at the configured target when the callee completes.
   */
  CALL: 'CALL',
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
