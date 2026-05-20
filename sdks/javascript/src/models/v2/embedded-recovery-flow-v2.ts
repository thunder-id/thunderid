/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {EmbeddedFlowType} from '../embedded-flow';

/**
 * Status enumeration for the embedded recovery flow operations.
 *
 * @experimental Part of the new ThunderID API
 */
export enum EmbeddedRecoveryFlowStatus {
  /**
   * Recovery flow completed successfully.
   */
  Complete = 'COMPLETE',

  /**
   * Recovery flow encountered an error and cannot proceed.
   *
   * @see {@link EmbeddedRecoveryFlowErrorResponse} for error response structure
   */
  Error = 'ERROR',

  /**
   * Recovery flow requires additional user input.
   */
  Incomplete = 'INCOMPLETE',
}

/**
 * Type enumeration for embedded recovery flow responses.
 *
 * @experimental Part of the new ThunderID API
 */
export enum EmbeddedRecoveryFlowType {
  /**
   * Response requires external redirection.
   */
  Redirection = 'REDIRECTION',

  /**
   * Response contains view components for rendering.
   */
  View = 'VIEW',
}

/**
 * Response structure for the embedded recovery flow.
 *
 * @experimental Part of the new ThunderID API
 */
export interface EmbeddedRecoveryFlowResponse {
  /**
   * Per-step challenge token for replay protection.
   * Must be included in the next request to continue this flow.
   */
  challengeToken?: string;

  /**
   * Flow data containing UI components for the current step.
   */
  data: {
    /**
     * Additional data from the flow step.
     */
    additionalData?: Record<string, any>;

    /**
     * UI components to render for the current step.
     */
    components?: any[];

    /**
     * Redirect URL if type is REDIRECTION.
     */
    redirectURL?: string;
  };

  /**
   * Unique identifier for this recovery flow execution.
   */
  executionId: string;

  /**
   * Optional reason for failure when flowStatus is ERROR.
   */
  failureReason?: string;

  /**
   * Current status of the recovery flow.
   */
  flowStatus: EmbeddedRecoveryFlowStatus;

  /**
   * Type of response, indicating the expected user interaction.
   */
  type: EmbeddedRecoveryFlowType;
}

/**
 * Request payload for initiating the embedded recovery flow.
 *
 * @experimental Part of the new ThunderID API
 */
export interface EmbeddedRecoveryFlowInitiateRequest {
  applicationId?: string;
  flowType: EmbeddedFlowType.Recovery;
}

/**
 * Request payload for executing steps in the embedded recovery flow.
 *
 * @experimental Part of the new ThunderID API
 */
export interface EmbeddedRecoveryFlowRequest extends Partial<EmbeddedRecoveryFlowInitiateRequest> {
  action?: string;
  challengeToken?: string;
  executionId?: string;
  inputs?: Record<string, any>;
}

/**
 * Error response structure for the embedded recovery flow.
 *
 * @experimental Part of the new ThunderID API
 */
export interface EmbeddedRecoveryFlowErrorResponse {
  /**
   * Additional response data, typically empty for error responses.
   */
  data: Record<string, any>;

  /**
   * Unique identifier for the recovery flow instance that failed.
   */
  executionId: string;

  /**
   * Human-readable explanation of why the recovery operation failed.
   */
  failureReason: string;

  /**
   * Status of the recovery flow — always ERROR for this interface.
   */
  flowStatus: EmbeddedRecoveryFlowStatus;
}
