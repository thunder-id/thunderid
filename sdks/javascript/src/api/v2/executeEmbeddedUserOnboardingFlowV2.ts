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

import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import {EmbeddedFlowType} from '../../models/embedded-flow';
import {EmbeddedFlowExecuteRequestConfig as EmbeddedFlowExecuteRequestConfigV2} from '../../models/v2/embedded-flow-v2';

/**
 * Response from the user onboarding flow execution.
 */
export interface EmbeddedUserOnboardingFlowResponse {
  /**
   * Data for the current step including components and additional data.
   */
  data?: {
    /**
     * Additional data from the flow step (e.g., inviteLink).
     */
    additionalData?: Record<string, string>;

    /**
     * UI components to render for the current step.
     */
    components?: any[];
  };

  /**
   * Unique identifier for the flow execution.
   */
  executionId: string;

  /**
   * Reason for failure if flowStatus is ERROR.
   */
  failureReason?: string;

  /**
   * Current status of the flow.
   */
  flowStatus: 'INCOMPLETE' | 'COMPLETE' | 'ERROR';

  /**
   * Type of the current step in the flow.
   */
  type?: 'VIEW' | 'REDIRECTION';
}

/**
 * Executes an embedded user onboarding flow by sending a request to the flow execution endpoint.
 *
 * This function handles both:
 * - Admin flow: Initiates onboarding, collects user details, generates invite link
 * - End-user flow: Validates invite token and allows password setting
 *
 * @param requestConfig - Request configuration object containing URL, payload, and optional auth token.
 * @returns A promise that resolves with the flow execution response.
 * @throws ThunderIDAPIError when the request fails or URL is invalid.
 *
 * @example
 * ```typescript
 * // Admin initiating user onboarding (requires auth token)
 * const response = await executeEmbeddedUserOnboardingFlowV2({
 *   baseUrl: "https://api.thunder.io",
 *   payload: {
 *     flowType: "USER_ONBOARDING"
 *   },
 *   headers: {
 *     Authorization: `Bearer ${accessToken}`
 *   }
 * });
 *
 * // End-user accepting invite (no auth required)
 * const response = await executeEmbeddedUserOnboardingFlowV2({
 *   baseUrl: "https://api.thunder.io",
 *   payload: {
 *     executionId: "flow-id-from-url",
 *     inputs: { inviteToken: "token-from-url" }
 *   }
 * });
 * ```
 */
const executeEmbeddedUserOnboardingFlowV2 = async ({
  url,
  baseUrl,
  payload,
  ...requestConfig
}: EmbeddedFlowExecuteRequestConfigV2): Promise<EmbeddedUserOnboardingFlowResponse> => {
  if (!payload) {
    throw new ThunderIDAPIError(
      'User onboarding payload is required',
      'executeEmbeddedUserOnboardingFlow-ValidationError-002',
      'javascript',
      400,
      'If a user onboarding payload is not provided, the request cannot be constructed correctly.',
    );
  }

  const endpoint: string = url ?? `${baseUrl}/flow/execute`;

  // Strip any user-provided 'verbose' parameter as it should only be used internally
  const cleanPayload: typeof payload =
    typeof payload === 'object' && payload !== null
      ? Object.fromEntries(Object.entries(payload).filter(([key]: [string, unknown]) => key !== 'verbose'))
      : payload;

  // `verbose: true` is required to get the `meta` field in the response that includes component details.
  // Add verbose:true for initial requests or flow continuation without inputs
  const hasOnlyFlowType: boolean =
    typeof cleanPayload === 'object' &&
    cleanPayload !== null &&
    'flowType' in cleanPayload &&
    Object.keys(cleanPayload).length === 1;
  const hasOnlyFlowId: boolean =
    typeof cleanPayload === 'object' &&
    cleanPayload !== null &&
    'executionId' in cleanPayload &&
    Object.keys(cleanPayload).length === 1;
  const hasFlowIdWithInputs: boolean =
    typeof cleanPayload === 'object' &&
    cleanPayload !== null &&
    'executionId' in cleanPayload &&
    'inputs' in cleanPayload;

  // Add verbose for initial requests and when continuing with inputs
  const requestPayload: Record<string, unknown> =
    hasOnlyFlowType || hasOnlyFlowId || hasFlowIdWithInputs ? {...cleanPayload, verbose: true} : cleanPayload;

  // Ensure flowType is USER_ONBOARDING for initial requests
  if ('flowType' in requestPayload && requestPayload['flowType'] !== EmbeddedFlowType.UserOnboarding) {
    requestPayload['flowType'] = EmbeddedFlowType.UserOnboarding;
  }

  const response: Response = await fetch(endpoint, {
    ...requestConfig,
    body: JSON.stringify(requestPayload),
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...requestConfig.headers,
    },
    method: requestConfig.method || 'POST',
  });

  if (!response.ok) {
    const errorText: string = await response.text();

    throw new ThunderIDAPIError(
      errorText,
      'executeEmbeddedUserOnboardingFlow-ResponseError-001',
      'javascript',
      response.status,
      response.statusText,
      'User onboarding request failed',
    );
  }

  const flowResponse: EmbeddedUserOnboardingFlowResponse = await response.json();

  return flowResponse;
};

export default executeEmbeddedUserOnboardingFlowV2;
