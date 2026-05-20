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

import {EmbeddedFlowResponseData as EmbeddedFlowResponseDataV2} from './embedded-flow-v2';
import {
  EmbeddedFlowResponseType as EmbeddedFlowResponseTypeV1,
  EmbeddedFlowType as EmbeddedFlowTypeV1,
} from '../embedded-flow';

/**
 * Status enumeration for ThunderID embedded sign-in flow operations.
 *
 * These statuses indicate the current state of the sign-in flow and determine
 * the next action required by the client application. Each status provides
 * specific guidance on how to proceed with the authentication process.
 *
 * @example
 * ```typescript
 * switch (response.flowStatus) {
 *   case EmbeddedSignInFlowStatus.Incomplete:
 *     // More user input needed - render form components
 *     break;
 *   case EmbeddedSignInFlowStatus.Complete:
 *     // Authentication successful - handle completion
 *     break;
 *   case EmbeddedSignInFlowStatus.Error:
 *     // Authentication failed - show error message
 *     break;
 * }
 * ```
 *
 * @experimental Part of the new ThunderID API
 */
export enum EmbeddedSignInFlowStatus {
  /**
   * Sign-in flow completed successfully.
   *
   * The user has been authenticated and the flow can proceed to
   * OAuth2 completion or redirection. Check for redirectUrl or
   * assertion data in the response.
   */
  Complete = 'COMPLETE',

  /**
   * Sign-in flow encountered an error.
   *
   * Authentication failed due to invalid credentials, system error,
   * or other issues. Check error details in the response and handle
   * appropriately (retry, show error message, etc.).
   */
  Error = 'ERROR',

  /**
   * Sign-in flow requires additional user input.
   *
   * More authentication steps are needed. The response will contain
   * components in data.meta.components that should be rendered to
   * collect additional user input (e.g., MFA, password, etc.).
   */
  Incomplete = 'INCOMPLETE',
}

/**
 * Type enumeration for ThunderID embedded sign-in flow responses.
 *
 * Determines the nature of the flow response and how the client should
 * handle the returned data. This affects both UI rendering and flow
 * continuation logic.
 *
 * @experimental Part of the new ThunderID API
 */
export enum EmbeddedSignInFlowType {
  /**
   * Response requires external redirection.
   *
   * Used for social login providers, external identity providers,
   * or other flows that require navigating to an external URL.
   * The response will contain redirection information.
   */
  Redirection = 'REDIRECTION',

  /**
   * Response contains view components for rendering.
   *
   * Standard embedded flow response containing UI components
   * that should be rendered within the current application
   * context. Most common type for embedded authentication.
   */
  View = 'VIEW',
}

/**
 * Extended response structure for ThunderID embedded sign-in flow.
 *
 * This interface defines additional properties that are added at the SDK level
 * to enhance the basic API response with client-side computed values. These
 * properties provide convenience for common post-authentication operations.
 *
 * @remarks This response structure is enhanced by the SDK and contains
 * properties beyond the raw API response. It's designed to simplify
 * post-authentication handling for client applications.
 *
 * @experimental This interface is part of the new ThunderID platform
 */
export interface ExtendedEmbeddedSignInFlowResponse {
  /**
   * Computed redirect URL for post-authentication navigation.
   *
   * This URL is determined by the SDK based on the flow completion result
   * and configured redirect settings. When present, the client application
   * should navigate to this URL to complete the authentication process.
   *
   * @example "https://myapp.com/dashboard?session=abc123"
   */
  redirectUrl?: string;
}

/**
 * Primary response structure for ThunderID embedded sign-in flow operations.
 *
 * This is the main response interface returned by the sign-in API, combining
 * the enhanced SDK properties with the core API response data. It provides all
 * information needed to handle the current state of the authentication flow.
 *
 * The response structure adapts based on the flow status:
 * - INCOMPLETE: Contains components for user interaction
 * - COMPLETE: Contains completion data and potential redirection info
 * - ERROR: Contains error information for troubleshooting
 *
 * @example
 * ```typescript
 * const response: EmbeddedSignInFlowResponse = {
 *   executionId: "flow_12345",
 *   flowStatus: EmbeddedSignInFlowStatus.Incomplete,
 *   type: EmbeddedSignInFlowType.View,
 *   data: {
 *     meta: {
 *       components: [
 *         {
 *           id: "username_field",
 *           type: EmbeddedFlowComponentType.TextInput,
 *           label: "Username",
 *           required: true
 *         }
 *       ]
 *     }
 *   }
 * };
 * ```
 *
 * @experimental This interface is part of the new ThunderID platform
 */
export interface EmbeddedSignInFlowResponse extends ExtendedEmbeddedSignInFlowResponse {
  /**
   * JWT assertion returned when the flow reaches COMPLETE status on the V2 platform.
   * Used to establish the session without a separate OAuth2 redirect.
   */
  assertion?: string;

  /**
   * Per-step challenge token for replay protection.
   * Must be included in the next request to continue this flow.
   */
  challengeToken?: string;

  /**
   * Core response data containing UI components and flow metadata.
   * Includes both modern meta.components structure and legacy fields for compatibility.
   */
  data: EmbeddedFlowResponseDataV2 & {
    /**
     * Legacy action definitions for backward compatibility.
     * @deprecated Use data.meta.components for new implementations
     */
    actions?: {
      /** Unique action identifier */
      id: string;
      /** Action type identifier */
      type: EmbeddedFlowResponseTypeV1;
    }[];

    /**
     * Legacy input field definitions for backward compatibility.
     * @deprecated Use data.meta.components for new implementations
     */
    inputs?: {
      /** Field name identifier */
      name: string;
      /** Whether the field is required */
      required: boolean;
      /** Input field type */
      type: string;
    }[];
  };

  /**
   * Unique identifier for this specific flow instance.
   * Used to maintain state across multiple API calls during the authentication process.
   */
  executionId: string;

  /**
   * Optional reason for flow failure in case of an error.
   * Provides additional context when flowStatus is set to ERROR.
   */
  failureReason?: string;

  /**
   * Current status of the sign-in flow.
   * Determines the next action required by the client application.
   */
  flowStatus: EmbeddedSignInFlowStatus;

  /**
   * Type of response indicating how to handle the returned data.
   * Affects both UI rendering and navigation logic.
   */
  type: EmbeddedSignInFlowType;
}

/**
 * Response structure for completed ThunderID embedded sign-in flows.
 *
 * This interface defines the response format when the embedded sign-in flow
 * reaches the COMPLETE status and requires OAuth2 flow completion. It contains
 * the redirect URI that should be used for the final authentication step.
 *
 * @example
 * ```typescript
 * const completeResponse: EmbeddedSignInFlowCompleteResponse = {
 *   redirect_uri: "https://myapp.com/callback?code=abc123&state=xyz789"
 * };
 *
 * // Typically handled automatically by the SDK
 * window.location.href = completeResponse.redirect_uri;
 * ```
 *
 * @experimental This interface is part of the new ThunderID platform
 */
export interface EmbeddedSignInFlowCompleteResponse {
  /**
   * OAuth2 redirect URI for completing the authentication flow.
   *
   * Contains the final redirect URL with authorization code, state,
   * and other OAuth2 parameters needed to complete the authentication
   * process. This URL should be navigated to automatically or manually
   * depending on the application's requirements.
   */
  redirect_uri: string;
}

/**
 * Request payload for initiating ThunderID embedded sign-in flows.
 *
 * This type defines the minimum required information to start a new
 * embedded sign-in flow. The flow type determines the kind of authentication
 * process that will be initiated (e.g., standard login, MFA, etc.).
 *
 * @example
 * ```typescript
 * const initRequest: EmbeddedSignInFlowInitiateRequest = {
 *   applicationId: "app_12345",
 *   flowType: EmbeddedFlowType.Authentication
 * };
 *
 * const response = await executeEmbeddedSignInFlow({
 *   baseUrl: "https://api.asgardeo.io/t/myorg",
 *   payload: initRequest
 * });
 * ```
 *
 * @experimental This type is part of the new ThunderID platform
 */
export interface EmbeddedSignInFlowInitiateRequest {
  /**
   * Unique identifier of the application initiating the sign-in flow.
   * Must be a valid application ID registered in the ThunderID organization.
   */
  applicationId: string;

  /**
   * Type of embedded flow to initiate.
   * Determines the authentication process and available options.
   */
  flowType: EmbeddedFlowTypeV1;
}

/**
 * Request payload for executing steps in ThunderID embedded sign-in flows.
 *
 * This interface defines the structure for subsequent requests after flow initiation.
 * It supports both continuing existing flows (with executionId) and submitting user
 * input data collected from the rendered components.
 *
 * @example
 * ```typescript
 * // Continue existing flow with user input
 * const stepRequest: EmbeddedSignInFlowRequest = {
 *   executionId: "flow_12345",
 *   action: "action_001",
 *   inputs: {
 *     username: "user@example.com",
 *     password: "securePassword123"
 *   }
 * };
 *
 * // Submit to continue the flow
 * const response = await executeEmbeddedSignInFlow({
 *   baseUrl: "https://api.asgardeo.io/t/myorg",
 *   payload: stepRequest
 * });
 * ```
 *
 * @experimental This interface is part of the new ThunderID platform
 */
export interface EmbeddedSignInFlowRequest extends Partial<EmbeddedSignInFlowInitiateRequest> {
  /**
   * Identifier of the specific action being triggered.
   * Corresponds to action components in the UI (e.g., submit button, social login).
   */
  action?: string;

  /**
   * Per-step challenge token received from the previous flow response.
   * Required when continuing an existing flow to prevent replay attacks.
   */
  challengeToken?: string;

  /**
   * Identifier of the flow instance to continue.
   * Required when submitting data for an existing flow.
   */
  executionId?: string;

  /**
   * User input data collected from the form components.
   * Keys should match the component identifiers from the response.
   *
   * @example
   * ```typescript
   * {
   *   "username": "john.doe@example.com",
   *   "password": "mySecurePassword",
   *   "rememberMe": true
   * }
   * ```
   */
  inputs?: Record<string, any>;
}
