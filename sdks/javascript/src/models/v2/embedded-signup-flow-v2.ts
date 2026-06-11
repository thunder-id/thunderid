/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {FlowExecutionError} from './embedded-flow-v2';
import {
  EmbeddedFlowResponseType as EmbeddedFlowResponseTypeV1,
  EmbeddedFlowType as EmbeddedFlowTypeV1,
} from '../embedded-flow';

/**
 * Status enumeration for ThunderID embedded sign-up flow operations.
 *
 * These statuses indicate the current state of the registration flow and determine
 * the next action required by the client application. Each status provides specific
 * guidance on how to proceed with the user registration process.
 *
 * @example
 * ```typescript
 * switch (response.flowStatus) {
 *   case EmbeddedSignUpFlowStatus.Incomplete:
 *     // More user input needed - render registration form components
 *     break;
 *   case EmbeddedSignUpFlowStatus.Complete:
 *     // Registration successful - handle completion
 *     break;
 *   case EmbeddedSignUpFlowStatus.Error:
 *     // Registration failed - show error details
 *     const errorResponse = response as EmbeddedSignUpFlowErrorResponse;
 *     showError(errorResponse.error.description.defaultValue);
 *     break;
 * }
 * ```
 *
 * @experimental Part of the new ThunderID API
 */
export enum EmbeddedSignUpFlowStatus {
  /**
   * Sign-up flow completed successfully.
   *
   * The user has successfully registered and the flow can proceed to
   * OAuth2 completion or redirection. Check for redirectUrl or assertion
   * data in the response for next steps.
   */
  Complete = 'COMPLETE',

  /**
   * Sign-up flow encountered an error and cannot proceed.
   *
   * Registration failed due to validation errors, duplicate user,
   * system errors, or other issues. The response will be of type
   * `EmbeddedSignUpFlowErrorResponse` containing detailed failure
   * information that can be displayed to the user.
   *
   * @see {@link EmbeddedSignUpFlowErrorResponse} for error response structure
   */
  Error = 'ERROR',

  /**
   * Sign-up flow requires additional user input.
   *
   * More registration steps are needed. The response will contain
   * components in data.meta.components that should be rendered to
   * collect additional user information (e.g., profile data, verification).
   */
  Incomplete = 'INCOMPLETE',
}

/**
 * Type enumeration for ThunderID embedded sign-up flow responses.
 *
 * Determines the nature of the registration flow response and how the client
 * should handle the returned data. This affects both UI rendering and flow
 * continuation logic during the user registration process.
 *
 * @experimental Part of the new ThunderID API
 */
export enum EmbeddedSignUpFlowType {
  /**
   * Response requires external redirection.
   *
   * Used for social registration providers, external identity providers,
   * or other flows that require navigating to an external URL during
   * the registration process. The response will contain redirection information.
   */
  Redirection = 'REDIRECTION',

  /**
   * Response contains view components for rendering.
   *
   * Standard embedded registration flow response containing UI components
   * that should be rendered within the current application context.
   * Most common type for embedded user registration.
   */
  View = 'VIEW',
}

/**
 * Extended response structure for the embedded sign-up flow.
 * @remarks This response is only done from the SDK level.
 * @experimental
 */
export interface ExtendedEmbeddedSignUpFlowResponse {
  /**
   * The URL to redirect the user after completing the sign-up flow.
   */
  redirectUrl?: string;
}

/**
 * Response structure for the new ThunderID embedded sign-up flow.
 *
 * This interface defines the structure for successful sign-up flow responses
 * from ThunderID APIs. For error responses, see `EmbeddedSignUpFlowErrorResponse`.
 *
 * **Flow States:**
 * - `INCOMPLETE`: More user input required, `data` contains form components
 * - `COMPLETE`: Sign-up finished, may contain redirect information
 * - For `ERROR` status, a separate `EmbeddedSignUpFlowErrorResponse` structure is used
 *
 * **Component-Driven UI:**
 * The `data.inputs` and `data.actions` are transformed by the React transformer
 * into component-driven format for consistent UI rendering across different
 * ThunderID versions.
 *
 * @experimental Part of the new ThunderID API
 * @see {@link EmbeddedSignUpFlowErrorResponse} for error response structure
 * @see {@link EmbeddedSignUpFlowStatus} for available flow statuses
 */
export interface EmbeddedSignUpFlowResponse extends ExtendedEmbeddedSignUpFlowResponse {
  /**
   * Per-step challenge token for replay protection.
   * Must be included in the next request to continue this flow.
   */
  challengeToken?: string;

  /**
   * Flow data containing form inputs and available actions.
   * This is transformed to component-driven format by the React transformer.
   */
  data: {
    /**
     * Available actions the user can take (e.g., form submission, social sign-up).
     */
    actions?: {
      id: string;
      type: EmbeddedFlowResponseTypeV1;
    }[];

    /**
     * Input fields required for the current step of the sign-up flow.
     */
    inputs?: {
      name: string;
      required: boolean;
      type: string;
    }[];
  };

  /**
   * Unique identifier for this sign-up flow instance.
   */
  executionId: string;

  /**
   * Structured error details when flowStatus is ERROR.
   * Contains an error code and i18n-ready message/description fields.
   */
  error?: FlowExecutionError;

  /**
   * Current status of the sign-up flow.
   * Determines whether more input is needed or the flow is complete.
   */
  flowStatus: EmbeddedSignUpFlowStatus;

  /**
   * Type of response, indicating the expected user interaction.
   */
  type: EmbeddedSignUpFlowType;
}

/**
 * Response structure for the new ThunderID embedded sign-up flow when the flow is complete.
 * @experimental
 */
export interface EmbeddedSignUpFlowCompleteResponse {
  redirect_uri: string;
}

/**
 * Request payload for initiating the new ThunderID embedded sign-up flow.
 * @experimental
 */
export interface EmbeddedSignUpFlowInitiateRequest {
  applicationId: string;
  flowType: EmbeddedFlowTypeV1;
  /**
   * OAuth2 scopes to request during flow initialization.
   * When provided, these scopes are forwarded to the platform at flow start.
   */
  scopes?: string | string[];
}

/**
 * Request payload for executing steps in the new ThunderID embedded sign-up flow.
 * @experimental
 */
export interface EmbeddedSignUpFlowRequest extends Partial<EmbeddedSignUpFlowInitiateRequest> {
  action?: string;
  challengeToken?: string;
  executionId?: string;
  inputs?: Record<string, any>;
}

/**
 * Error response structure for the ThunderID embedded sign-up flow.
 *
 * Returned when sign-up operations fail. Contains a structured `error` object
 * with i18n-ready message and description fields.
 *
 * @example
 * ```typescript
 * const errorResponse: EmbeddedSignUpFlowErrorResponse = {
 *   executionId: "0ccfeaf9-18b3-43a5-bcc1-07d863dcb2c0",
 *   flowStatus: EmbeddedSignUpFlowStatus.Error,
 *   data: {},
 *   error: {
 *     code: "FEE-60005",
 *     message: { key: "flows.errors.user_exists", defaultValue: "User already exists." },
 *     description: { key: "flows.errors.user_exists_desc", defaultValue: "User already exists with the provided username." }
 *   }
 * };
 * ```
 *
 * @experimental This is part of the new ThunderID API and may change in future versions
 * @see {@link EmbeddedSignUpFlowStatus.Error} for the error status enum value
 * @see {@link EmbeddedSignUpFlowResponse} for the corresponding success response structure
 */
export interface EmbeddedSignUpFlowErrorResponse {
  data: Record<string, any>;

  error: FlowExecutionError;

  executionId: string;

  flowStatus: EmbeddedSignUpFlowStatus;
}
