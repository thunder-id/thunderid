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

export enum EmbeddedFlowType {
  Authentication = 'AUTHENTICATION',
  Recovery = 'RECOVERY',
  Registration = 'REGISTRATION',
  UserOnboarding = 'USER_ONBOARDING',
}

export interface EmbeddedFlowExecuteRequestPayload {
  actionId?: string;
  flowType: EmbeddedFlowType;
  inputs?: Record<string, any>;
}

export interface EmbeddedFlowExecuteResponse {
  data: EmbeddedSignUpFlowData;
  flowId: string;
  flowStatus: EmbeddedFlowStatus;
  type: EmbeddedFlowResponseType;
}

export enum EmbeddedFlowStatus {
  Complete = 'COMPLETE',
  Incomplete = 'INCOMPLETE',
}

export enum EmbeddedFlowResponseType {
  Redirection = 'REDIRECTION',
  View = 'VIEW',
}

export interface EmbeddedSignUpFlowData {
  additionalData?: Record<string, any>;
  components?: EmbeddedFlowComponent[];
  redirectURL?: string;
}

export interface EmbeddedFlowComponent {
  components: EmbeddedFlowComponent[];
  config: Record<string, unknown>;
  id: string;
  type: EmbeddedFlowComponentType | string;
  variant?: string;
}

export enum EmbeddedFlowComponentType {
  Button = 'BUTTON',
  Checkbox = 'CHECKBOX',
  Divider = 'DIVIDER',
  Form = 'FORM',
  Image = 'IMAGE',
  Input = 'INPUT',
  Radio = 'RADIO',
  Select = 'SELECT',
  Typography = 'TYPOGRAPHY',
}

/**
 * Request configuration for executing embedded flow operations.
 *
 * This interface extends standard HTTP request configuration with additional
 * properties specific to embedded flow execution, such as base URL and payload data.
 *
 * @template T - Type of the payload data being sent with the request
 */
export interface EmbeddedFlowExecuteRequestConfig<T = any> extends Partial<Request> {
  /**
   * Base URL for the API endpoint.
   * This is typically the ThunderID organization URL.
   */
  baseUrl?: string;

  /**
   * Payload data to be sent with the request.
   * The structure depends on the specific flow operation being executed.
   */
  payload?: T;

  /**
   * Full URL for the API endpoint.
   * If provided, this overrides the baseUrl.
   */
  url?: string;
}

/**
 * Error response structure for ThunderIDV1 embedded flow operations.
 *
 * This interface defines the structure of error responses returned by ThunderIDV1 APIs
 * when flow operations (such as sign-up or sign-in) fail. This format is distinct from
 * ThunderIDV2's error format which uses `failureReason` instead of `code`/`description`.
 *
 * **Key Characteristics:**
 * - Uses structured error codes (e.g., "FEE-60005") for programmatic error handling
 * - Provides both a brief `message` and detailed `description` for context
 * - Includes `flowType` to identify which flow operation failed
 *
 * **Comparison with ThunderIDV2:**
 * - **ThunderIDV1**: Uses `code`, `message`, `description` fields
 * - **ThunderIDV2**: Uses `flowStatus: "ERROR"` with `failureReason` field
 *
 * **Error Handling:**
 * This error response format is automatically detected and processed by the
 * `extractErrorMessage()` and `checkForErrorResponse()` functions in the React
 * transformer to extract meaningful error messages for display to users.
 *
 * @example
 * ```typescript
 * // Typical ThunderIDV1 error response
 * const errorResponse: EmbeddedFlowExecuteErrorResponse = {
 *   code: "FEE-60005",
 *   message: "Error while provisioning user.",
 *   description: "Error occurred while provisioning user in the request of flow id: ac57315c-6ca6-49dc-8664-fcdcff354f46",
 *   flowType: "REGISTRATION"
 * };
 *
 * // The transformer will extract: "Error occurred while provisioning user in the request of flow id: ac57315c-6ca6-49dc-8664-fcdcff354f46"
 * // (Prefers description over message as it's usually more detailed)
 * ```
 *
 * @see {@link EmbeddedSignUpFlowErrorResponse} for the ThunderIDV2 equivalent error structure
 */
export interface EmbeddedFlowExecuteErrorResponse {
  /**
   * Structured error code identifying the type of error.
   *
   * Format typically follows pattern like "FEE-XXXXX" where:
   * - "FEE" indicates Flow Execution Error
   * - XXXXX is a numeric identifier for the specific error type
   *
   * @example "FEE-60005" - User provisioning error
   */
  code: string;

  /**
   * Detailed error description with contextual information.
   *
   * This field usually contains more specific information about the error,
   * including flow IDs, operation details, and other debugging context.
   * The transformer prefers this field over `message` when extracting
   * error messages for display to users.
   *
   * @example "Error occurred while provisioning user in the request of flow id: ac57315c-6ca6-49dc-8664-fcdcff354f46"
   */
  description: string;

  /**
   * Type of flow operation that encountered the error.
   *
   * Currently only supports 'REGISTRATION' but may be extended to
   * include other flow types (e.g., 'LOGIN', 'PASSWORD_RESET') in the future.
   */
  flowType: 'REGISTRATION' | 'RECOVERY';

  /**
   * Brief error message describing what went wrong.
   *
   * This is typically a short, high-level description of the error.
   * For more detailed information, refer to the `description` field.
   *
   * @example "Error while provisioning user."
   */
  message: string;
}
